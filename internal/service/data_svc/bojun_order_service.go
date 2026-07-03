package data_svc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/bojun"
	"gin-biz-web-api/pkg/config"
)

const bojunOrderSource = "bojun_order"

type rawDataCreator interface {
	Create(ctx context.Context, rawData *model.RawData) (uint, error)
}

type BojunOrderService struct {
	rawDataDAO rawDataCreator
}

type BojunOrderSyncResult struct {
	StartTime  string `json:"start_time"`
	EndTime    string `json:"end_time"`
	PageSize   int    `json:"page_size"`
	MaxPages   int    `json:"max_pages"`
	FetchPages int    `json:"fetch_pages"`
	SavedCount int    `json:"saved_count"`
}

func NewBojunOrderService() *BojunOrderService {
	return &BojunOrderService{
		rawDataDAO: data_dao.NewRawDataDAO(),
	}
}

func (s *BojunOrderService) SyncRecentOrders(ctx context.Context) (*BojunOrderSyncResult, error) {
	now := time.Now()
	lookbackMinutes := bojunEnvInt("BOJUN_ORDER_LOOKBACK_MINUTES", config.GetInt("Bojun.OrderLookbackMinutes", 1))
	if lookbackMinutes <= 0 {
		lookbackMinutes = 1
	}
	startTime := now.Add(-time.Duration(lookbackMinutes) * time.Minute).Format("2006-01-02 15:04:05")
	endTime := now.Format("2006-01-02 15:04:05")
	return s.SyncOrders(ctx, startTime, endTime)
}

func (s *BojunOrderService) SyncOrders(ctx context.Context, startTime, endTime string) (*BojunOrderSyncResult, error) {
	method := bojunEnvString("BOJUN_ORDER_METHOD", config.GetString("Bojun.OrderMethod", "/retail/retail.query"))
	pageSize := positiveBojunInt(bojunEnvInt("BOJUN_ORDER_PAGE_SIZE", config.GetInt("Bojun.OrderPageSize", 100)), 100)
	maxPages := positiveBojunInt(bojunEnvInt("BOJUN_ORDER_MAX_PAGES", config.GetInt("Bojun.OrderMaxPages", 20)), 20)
	result := &BojunOrderSyncResult{
		StartTime: startTime,
		EndTime:   endTime,
		PageSize:  pageSize,
		MaxPages:  maxPages,
	}

	for page := 1; page <= maxPages; page++ {
		payload, err := bojun.SendSignedRequest(ctx, method, buildBojunOrderRequestBody(page, pageSize, startTime, endTime))
		if err != nil {
			return result, err
		}

		records, pageInfo, err := extractBojunOrderRecords(payload)
		if err != nil {
			return result, err
		}
		result.FetchPages++

		for _, record := range records {
			rawData, err := buildBojunOrderRawData(record, method, startTime, endTime, pageInfo.Current)
			if err != nil {
				return result, err
			}
			if _, err := s.rawDataDAO.Create(ctx, rawData); err != nil {
				return result, err
			}
			result.SavedCount++
		}

		if pageInfo.TotalPage <= 0 || pageInfo.Current >= pageInfo.TotalPage || len(records) == 0 {
			break
		}
	}

	return result, nil
}

type bojunOrderPageInfo struct {
	Current   int
	TotalPage int
	Total     int
}

func buildBojunOrderRequestBody(page, pageSize int, startTime, endTime string) map[string]interface{} {
	body := map[string]interface{}{
		"current":  page,
		"pageSize": pageSize,
	}
	if startTime != "" {
		body["startTime"] = startTime
	}
	if endTime != "" {
		body["endTime"] = endTime
	}
	return body
}

func extractBojunOrderRecords(payload map[string]interface{}) ([]map[string]interface{}, bojunOrderPageInfo, error) {
	if code := intFromAny(payload["code"]); code != 0 && code != 200 {
		return []map[string]interface{}{}, bojunOrderPageInfo{}, fmt.Errorf("bojun order response code %d", code)
	}

	data, ok := payload["data"].(map[string]interface{})
	if !ok {
		return []map[string]interface{}{}, bojunOrderPageInfo{}, fmt.Errorf("bojun order response missing data")
	}

	rawRecords, ok := data["records"].([]interface{})
	if !ok {
		return []map[string]interface{}{}, bojunOrderPageInfo{}, fmt.Errorf("bojun order response missing records")
	}

	records := make([]map[string]interface{}, 0, len(rawRecords))
	for _, item := range rawRecords {
		record, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		records = append(records, record)
	}

	return records, bojunOrderPageInfo{
		Current:   intFromAny(data["current"]),
		TotalPage: intFromAny(data["totalPage"]),
		Total:     intFromAny(data["total"]),
	}, nil
}

func buildBojunOrderRawData(record map[string]interface{}, method, startTime, endTime string, page int) (*model.RawData, error) {
	rawContent, err := json.Marshal(record)
	if err != nil {
		return nil, err
	}
	ingestedAt := time.Now()
	metadata := map[string]interface{}{
		"source":      bojunOrderSource,
		"remark":      bojunOrderSource,
		"method":      method,
		"start_time":  startTime,
		"end_time":    endTime,
		"page":        page,
		"ingested_at": ingestedAt.Format(time.RFC3339),
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}

	return &model.RawData{
		DataSourceID: 0,
		ExternalID:   stringFromAny(record["docno"]),
		DataType:     "order",
		RawContent:   string(rawContent),
		Metadata:     string(metadataJSON),
		Status:       "pending",
		Remark:       bojunOrderSource,
		Source:       bojunOrderSource,
		IngestedAt:   &ingestedAt,
	}, nil
}

func positiveBojunInt(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

func bojunEnvString(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func bojunEnvInt(key string, fallback int) int {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return fallback
}

func intFromAny(value interface{}) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		var parsed int
		_, _ = fmt.Sscan(typed, &parsed)
		return parsed
	default:
		return 0
	}
}

func stringFromAny(value interface{}) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprintf("%v", value)
}
