package data_svc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	trigger "gin-biz-web-api/Trigger"
	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/config"
	"gin-biz-web-api/pkg/logger"
	"gin-biz-web-api/pkg/youzan"

	"go.uber.org/zap"
)

const (
	youzanDistributionPageSize         = 100
	youzanDistributionDecryptBatchSize = 10000
)

type youzanDistributionClient interface {
	FetchOrderPage(ctx context.Context, accessToken, startTime, endTime string, pageNo, pageSize int) ([]map[string]any, bool, error)
	DecryptBatch(ctx context.Context, accessToken string, sources []string) ([]string, error)
}

type youzanDistributionWriter interface {
	CreateBatchIfNotExists(ctx context.Context, orders []model.YouzanDistributionOrder) (int64, error)
}

type YouzanDistributionOrderService struct {
	client         youzanDistributionClient
	writer         youzanDistributionWriter
	getAccessToken func() (string, error)
}

type YouzanDistributionSyncResult struct {
	StartTime     string `json:"start_time"`
	EndTime       string `json:"end_time"`
	PageSize      int    `json:"page_size"`
	FetchPages    int    `json:"fetch_pages"`
	TotalCount    int    `json:"total_count"`
	SavedCount    int    `json:"saved_count"`
	ExistingCount int    `json:"existing_count"`
	FailedCount   int    `json:"failed_count"`
}

func NewYouzanDistributionOrderService() *YouzanDistributionOrderService {
	httpClient := &http.Client{Timeout: 60 * time.Second}
	client := youzan.NewDistributionClient(
		config.GetString("cfg.youzan.orders_url"),
		config.GetString("cfg.youzan.decrypt_url"),
		httpClient,
	)
	return newYouzanDistributionOrderService(
		client,
		data_dao.NewYouzanDistributionOrderDAO(),
		trigger.GetYouzanAccessToken,
	)
}

func newYouzanDistributionOrderService(client youzanDistributionClient, writer youzanDistributionWriter, getAccessToken func() (string, error)) *YouzanDistributionOrderService {
	return &YouzanDistributionOrderService{client: client, writer: writer, getAccessToken: getAccessToken}
}

// SyncRange fetches and persists one page at a time, keeping memory bounded for large backfills.
func (s *YouzanDistributionOrderService) SyncRange(ctx context.Context, startTime, endTime string) (*YouzanDistributionSyncResult, error) {
	startTime, endTime, err := normalizeYouzanDistributionRange(startTime, endTime)
	result := &YouzanDistributionSyncResult{StartTime: startTime, EndTime: endTime, PageSize: youzanDistributionPageSize}
	if err != nil {
		return result, err
	}

	accessToken, err := s.getAccessToken()
	if err != nil {
		return result, fmt.Errorf("get youzan access token: %w", err)
	}
	if strings.TrimSpace(accessToken) == "" {
		return result, fmt.Errorf("get youzan access token: empty token")
	}

	for pageNo := 1; ; pageNo++ {
		orders, hasNext, err := s.client.FetchOrderPage(ctx, accessToken, startTime, endTime, pageNo, youzanDistributionPageSize)
		if err != nil {
			return result, fmt.Errorf("fetch youzan distribution orders page %d: %w", pageNo, err)
		}
		result.FetchPages++
		result.TotalCount += len(orders)

		models, invalidCount, err := s.buildPageModels(ctx, accessToken, orders)
		if err != nil {
			return result, fmt.Errorf("prepare youzan distribution orders page %d: %w", pageNo, err)
		}
		result.FailedCount += invalidCount

		created, err := s.writer.CreateBatchIfNotExists(ctx, models)
		if err != nil {
			return result, fmt.Errorf("save youzan distribution orders page %d: %w", pageNo, err)
		}
		result.SavedCount += int(created)
		result.ExistingCount += len(models) - int(created)

		logger.Info(
			"有赞分销订单分页写入完成",
			zap.Int("page", pageNo),
			zap.Int("fetched", len(orders)),
			zap.Int64("created", created),
			zap.Int("existing", len(models)-int(created)),
			zap.Bool("has_next", hasNext),
		)
		if !hasNext {
			break
		}
	}

	return result, nil
}

func (s *YouzanDistributionOrderService) buildPageModels(ctx context.Context, accessToken string, orders []map[string]any) ([]model.YouzanDistributionOrder, int, error) {
	nicknames := make([]string, 0, len(orders))
	seen := make(map[string]struct{}, len(orders))
	for _, order := range orders {
		nickname := nestedString(order, "buyer_info", "fans_nickname")
		if !shouldDecryptYouzanNickname(nickname) {
			continue
		}
		if _, exists := seen[nickname]; exists {
			continue
		}
		seen[nickname] = struct{}{}
		nicknames = append(nicknames, nickname)
	}

	decrypted, err := s.decryptNicknames(ctx, accessToken, nicknames)
	if err != nil {
		return nil, 0, err
	}

	now := int(time.Now().Unix())
	models := make([]model.YouzanDistributionOrder, 0, len(orders))
	invalidCount := 0
	for _, order := range orders {
		row, err := buildYouzanDistributionOrder(order, decrypted, now)
		if err != nil {
			invalidCount++
			logger.Error("跳过无效有赞分销订单", zap.Error(err))
			continue
		}
		models = append(models, row)
	}
	return models, invalidCount, nil
}

func (s *YouzanDistributionOrderService) decryptNicknames(ctx context.Context, accessToken string, sources []string) (map[string]string, error) {
	result := make(map[string]string, len(sources))
	for start := 0; start < len(sources); start += youzanDistributionDecryptBatchSize {
		end := start + youzanDistributionDecryptBatchSize
		if end > len(sources) {
			end = len(sources)
		}
		batch := sources[start:end]
		values, err := s.client.DecryptBatch(ctx, accessToken, batch)
		if err != nil {
			return nil, fmt.Errorf("decrypt fans_nickname batch %d-%d: %w", start+1, end, err)
		}
		if len(values) != len(batch) {
			return nil, fmt.Errorf("decrypt fans_nickname returned %d items, want %d", len(values), len(batch))
		}
		for index, source := range batch {
			result[source] = values[index]
		}
		logger.Info("有赞分销昵称批量解密完成", zap.Int("batch_start", start+1), zap.Int("batch_end", end))
	}
	return result, nil
}

func buildYouzanDistributionOrder(order map[string]any, decrypted map[string]string, now int) (model.YouzanDistributionOrder, error) {
	tid := nestedString(order, "order_info", "tid")
	if tid == "" {
		return model.YouzanDistributionOrder{}, fmt.Errorf("order_info.tid is empty")
	}
	rawJSON, err := json.Marshal(order)
	if err != nil {
		return model.YouzanDistributionOrder{}, fmt.Errorf("encode order %s: %w", tid, err)
	}
	itemsJSON, err := json.Marshal(order["orders"])
	if err != nil {
		return model.YouzanDistributionOrder{}, fmt.Errorf("encode order items %s: %w", tid, err)
	}

	encryptedNickname := nestedString(order, "buyer_info", "fans_nickname")
	plainNickname := ""
	if shouldDecryptYouzanNickname(encryptedNickname) {
		var found bool
		plainNickname, found = decrypted[encryptedNickname]
		if !found {
			return model.YouzanDistributionOrder{}, fmt.Errorf("decrypted fans_nickname is missing for order %s", tid)
		}
	}
	return model.YouzanDistributionOrder{
		TID:                   tid,
		Status:                nestedString(order, "order_info", "status"),
		StatusStr:             nestedString(order, "order_info", "status_str"),
		ShopName:              nestedString(order, "order_info", "shop_name"),
		NodeKdtID:             nestedInt64(order, "order_info", "node_kdt_id"),
		RootKdtID:             nestedInt64(order, "order_info", "root_kdt_id"),
		Payment:               nestedScalarString(order, "pay_info", "payment"),
		SuccessTime:           parseYouzanDistributionTime(nestedString(order, "order_info", "success_time")),
		CreatedTime:           parseYouzanDistributionTime(nestedString(order, "order_info", "created")),
		FansNicknameEncrypted: encryptedNickname,
		FansNickname:          plainNickname,
		ItemsJSON:             string(itemsJSON),
		RawContentJSON:        string(rawJSON),
		CommonTimestampsField: model.CommonTimestampsField{CreatedAt: now, UpdatedAt: now},
	}, nil
}

func normalizeYouzanDistributionRange(startTime, endTime string) (string, string, error) {
	const layout = "2006-01-02 15:04:05"
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		location = time.FixedZone("CST", 8*60*60)
	}
	start, err := time.ParseInLocation(layout, strings.TrimSpace(startTime), location)
	if err != nil {
		return startTime, endTime, fmt.Errorf("start_time must use %s", layout)
	}
	end, err := time.ParseInLocation(layout, strings.TrimSpace(endTime), location)
	if err != nil {
		return startTime, endTime, fmt.Errorf("end_time must use %s", layout)
	}
	if start.After(end) {
		return startTime, endTime, fmt.Errorf("start_time must not be after end_time")
	}
	return start.Format(layout), end.Format(layout), nil
}

func parseYouzanDistributionTime(value string) *model.TimeNormal {
	if value == "" {
		return nil
	}
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		location = time.FixedZone("CST", 8*60*60)
	}
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02 15:04:05.000"} {
		parsed, err := time.ParseInLocation(layout, value, location)
		if err == nil {
			return &model.TimeNormal{Time: parsed}
		}
	}
	return nil
}

func nestedString(values map[string]any, section, key string) string {
	return nestedScalarString(values, section, key)
}

func nestedScalarString(values map[string]any, section, key string) string {
	nested, ok := values[section].(map[string]any)
	if !ok {
		return ""
	}
	value, exists := nested[key]
	if !exists || value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}

func nestedInt64(values map[string]any, section, key string) int64 {
	value := nestedScalarString(values, section, key)
	var result int64
	_, _ = fmt.Sscan(value, &result)
	return result
}

func shouldDecryptYouzanNickname(value string) bool {
	return strings.TrimSpace(value) != ""
}
