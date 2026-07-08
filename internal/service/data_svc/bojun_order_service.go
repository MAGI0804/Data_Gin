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

const (
	bojunOrderSource               = "bojun_order"
	defaultBojunOrderMethod        = "/retail/middleretail.query"
	legacyBojunOrderMethod         = "/retail/retail.query"
	bojunStandardOrderMethodPrefix = "/bos/standard"
)

type rawDataCreator interface {
	Create(ctx context.Context, rawData *model.RawData) (uint, error)
}

type bojunRetailOrderWriter interface {
	ExistsByDocNo(ctx context.Context, docNo string) (bool, error)
	CreateIfNotExists(ctx context.Context, order *model.BojunRetailOrder) (bool, error)
}

type pipelineRunRecorder interface {
	Create(ctx context.Context, run *model.PipelineRun) (uint, error)
	Finish(ctx context.Context, id uint, status string, successCount, failedCount int, errorMessage string) error
}

type BojunOrderService struct {
	rawDataDAO     rawDataCreator
	retailOrderDAO bojunRetailOrderWriter
	pipelineRunDAO pipelineRunRecorder
	pushService    *BojunOrderPushService
}

type BojunOrderSyncResult struct {
	StartTime    string `json:"start_time"`
	EndTime      string `json:"end_time"`
	PageSize     int    `json:"page_size"`
	MaxPages     int    `json:"max_pages"`
	FetchPages   int    `json:"fetch_pages"`
	TotalCount   int    `json:"total_count"`
	SavedCount   int    `json:"saved_count"`
	RetailCount  int    `json:"retail_count"`
	SkippedCount int    `json:"skipped_count"`
	FailedCount  int    `json:"failed_count"`
}

func NewBojunOrderService() *BojunOrderService {
	return &BojunOrderService{
		rawDataDAO:     data_dao.NewRawDataDAO(),
		retailOrderDAO: data_dao.NewBojunRetailOrderDAO(),
		pipelineRunDAO: data_dao.NewPipelineRunDAO(),
		pushService:    NewBojunOrderPushService(),
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
	normalizedStart, normalizedEnd, err := normalizeBojunOrderTimeRange(startTime, endTime)
	result := &BojunOrderSyncResult{
		StartTime: normalizedStart,
		EndTime:   normalizedEnd,
	}
	if err != nil {
		return result, err
	}

	method := bojunOrderMethod()
	pageSize := positiveBojunInt(bojunEnvInt("BOJUN_ORDER_PAGE_SIZE", config.GetInt("Bojun.OrderPageSize", 100)), 100)
	maxPages := positiveBojunInt(bojunEnvInt("BOJUN_ORDER_MAX_PAGES", config.GetInt("Bojun.OrderMaxPages", 20)), 20)
	result.PageSize = pageSize
	result.MaxPages = maxPages

	runID, err := s.createBojunOrderRun(ctx, normalizedStart, normalizedEnd)
	if err != nil {
		return result, err
	}

	for page := 1; page <= maxPages; page++ {
		payload, err := bojun.SendSignedRequest(ctx, method, buildBojunOrderRequestBody(page, pageSize, normalizedStart, normalizedEnd))
		if err != nil {
			result.FailedCount++
			return result, s.finishBojunOrderRun(ctx, runID, result, err)
		}

		records, pageInfo, err := extractBojunOrderRecords(payload)
		if err != nil {
			result.FailedCount++
			return result, s.finishBojunOrderRun(ctx, runID, result, err)
		}
		result.FetchPages++

		for _, record := range records {
			result.TotalCount++
			docNo := stringFromAny(record["docno"])
			if docNo == "" {
				result.FailedCount++
				continue
			}
			exists, err := s.retailOrderDAO.ExistsByDocNo(ctx, docNo)
			if err != nil {
				result.FailedCount++
				continue
			}
			if exists {
				result.SkippedCount++
				continue
			}

			rawData, err := buildBojunOrderRawData(record, method, normalizedStart, normalizedEnd, pageInfo.Current)
			if err != nil {
				result.FailedCount++
				continue
			}
			rawDataID, err := s.rawDataDAO.Create(ctx, rawData)
			if err != nil {
				result.FailedCount++
				continue
			}
			result.SavedCount++

			retailOrder, err := buildBojunRetailOrder(rawDataID, record)
			if err != nil {
				result.FailedCount++
				continue
			}
			created, err := s.retailOrderDAO.CreateIfNotExists(ctx, retailOrder)
			if err != nil {
				result.FailedCount++
				continue
			}
			if !created {
				result.SkippedCount++
				continue
			}
			result.RetailCount++
			if created && s.pushService != nil {
				pushResult := s.pushService.PushNewOrder(ctx, retailOrder)
				if pushResult.Error != nil && !pushResult.Skipped {
					result.FailedCount++
				}
			}
		}

		if pageInfo.TotalPage <= 0 || pageInfo.Current >= pageInfo.TotalPage || len(records) == 0 {
			break
		}
	}

	if err := s.finishBojunOrderRun(ctx, runID, result, nil); err != nil {
		return result, err
	}
	return result, nil
}

func normalizeBojunOrderTimeRange(startTime, endTime string) (string, string, error) {
	start, err := parseBojunOrderTime(startTime)
	if err != nil {
		return "", "", fmt.Errorf("start_time invalid: %w", err)
	}
	end, err := parseBojunOrderTime(endTime)
	if err != nil {
		return start.Format("2006-01-02 15:04:05"), "", fmt.Errorf("end_time invalid: %w", err)
	}
	if !start.Before(end) {
		return start.Format("2006-01-02 15:04:05"), end.Format("2006-01-02 15:04:05"), fmt.Errorf("start_time must be before end_time")
	}
	return start.Format("2006-01-02 15:04:05"), end.Format("2006-01-02 15:04:05"), nil
}

func parseBojunOrderTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("required")
	}
	for _, layout := range []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
	} {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("must use YYYY-MM-DD HH:mm:ss")
}

func bojunOrderMethod() string {
	method := bojunEnvString("BOJUN_ORDER_METHOD", config.GetString("Bojun.OrderMethod", defaultBojunOrderMethod))
	return normalizeBojunOrderMethod(method)
}

func normalizeBojunOrderMethod(method string) string {
	method = strings.TrimSpace(method)
	if method == "" || method == legacyBojunOrderMethod {
		return defaultBojunOrderMethod
	}
	if strings.HasPrefix(method, bojunStandardOrderMethodPrefix) {
		method = strings.TrimPrefix(method, bojunStandardOrderMethodPrefix)
	}
	if !strings.HasPrefix(method, "/") {
		method = "/" + method
	}
	return method
}

func (s *BojunOrderService) createBojunOrderRun(ctx context.Context, startTime, endTime string) (uint, error) {
	startedAt := time.Now()
	return s.pipelineRunDAO.Create(ctx, &model.PipelineRun{
		TraceID:      newTraceID(),
		RunType:      "fetch",
		TriggerType:  "schedule",
		Status:       "running",
		TotalCount:   0,
		SuccessCount: 0,
		FailedCount:  0,
		StartedAt:    &model.TimeNormal{Time: startedAt},
		ErrorMessage: fmt.Sprintf("bojun_order %s - %s", startTime, endTime),
	})
}

func (s *BojunOrderService) finishBojunOrderRun(
	ctx context.Context,
	runID uint,
	result *BojunOrderSyncResult,
	runErr error,
) error {
	status := "success"
	if result.FailedCount > 0 && result.RetailCount > 0 {
		status = "partial_success"
	} else if result.FailedCount > 0 {
		status = "failed"
	}

	errorMessage := ""
	if runErr != nil {
		errorMessage = runErr.Error()
	}
	if err := s.pipelineRunDAO.Finish(ctx, runID, status, result.RetailCount+result.SkippedCount, result.FailedCount, errorMessage); err != nil {
		if runErr != nil {
			return fmt.Errorf("%w; finish run: %v", runErr, err)
		}
		return err
	}
	return runErr
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

func buildBojunRetailOrder(rawDataID uint, record map[string]interface{}) (*model.BojunRetailOrder, error) {
	itemsJSON, err := marshalBojunJSON(record["items"])
	if err != nil {
		return nil, err
	}
	payItemsJSON, err := marshalBojunJSON(record["payItems"])
	if err != nil {
		return nil, err
	}
	rawContentJSON, err := marshalBojunJSON(record)
	if err != nil {
		return nil, err
	}

	retailSaleType := stringFromAny(record["retailsaletype"])
	orderTypeCode, orderTypeName := bojunOrderType(retailSaleType)
	docNo := stringFromAny(record["docno"])
	if docNo == "" {
		return nil, fmt.Errorf("bojun retail order docno is required")
	}
	relatedNormalNo := ""
	if orderTypeCode == "RET" || orderTypeCode == "EXP" {
		relatedNormalNo = relatedBojunNormalDocNo(record)
	}

	return &model.BojunRetailOrder{
		RawDataID:       rawDataID,
		OtherDocNo:      stringFromAny(record["otherdocno"]),
		DocNo:           docNo,
		BillDate:        intFromAny(record["billdate"]),
		RetailBillType:  stringFromAny(record["retailbilltype"]),
		StoreCode:       stringFromAny(record["cStoreCode"]),
		StoreName:       stringFromAny(record["cStoreName"]),
		UploadType:      stringFromAny(record["uploadtype"]),
		VIPNo:           stringFromAny(record["vipno"]),
		RetailTypeName:  stringFromAny(record["cRetailtypeName"]),
		SalesRep:        stringFromAny(record["salesrep"]),
		IsDiscount:      stringFromAny(record["isDis"]),
		VouchersNo:      stringFromAny(record["vouchersNo"]),
		IsIntegral:      stringFromAny(record["isintl"]),
		DocNoIntegral:   intFromAny(record["docnoIntegral"]),
		OrderMark:       stringFromAny(record["ordermark"]),
		RetailSaleType:  retailSaleType,
		OrderTypeCode:   orderTypeCode,
		OrderTypeName:   orderTypeName,
		Description:     stringFromAny(record["description"]),
		TotalLines:      intFromAny(record["totLines"]),
		O2OSoDocNo:      stringFromAny(record["o2oSoDocno"]),
		TotalQty:        intFromAny(record["totQty"]),
		TotalAmtList:    floatFromAny(record["totAmtList"]),
		TotalAmtActual:  floatFromAny(record["totAmtActual"]),
		AvgDiscount:     floatFromAny(record["avgDiscount"]),
		TotalAmtAcc:     floatFromAny(record["totAmtAcc"]),
		TotalAmtAcc1:    floatFromAny(record["totAmtAcc1"]),
		OzID:            stringFromAny(record["ozid"]),
		RelatedNormalNo: relatedNormalNo,
		ItemsJSON:       itemsJSON,
		PayItemsJSON:    payItemsJSON,
		RawContentJSON:  rawContentJSON,
	}, nil
}

func bojunOrderType(retailSaleType string) (string, string) {
	switch strings.ToUpper(strings.TrimSpace(retailSaleType)) {
	case "EXP":
		return "EXP", "换货"
	case "RET":
		return "RET", "退货"
	default:
		return "CMR", "正常零售"
	}
}

func relatedBojunNormalDocNo(record map[string]interface{}) string {
	for _, key := range []string{"otherdocno", "o2oSoDocno", "orgdocno"} {
		if value := stringFromAny(record[key]); value != "" {
			return value
		}
	}

	items, _ := record["items"].([]interface{})
	for _, item := range items {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if value := stringFromAny(itemMap["orgdocno"]); value != "" {
			return value
		}
	}

	description := stringFromAny(record["description"])
	if strings.HasPrefix(description, "由单据") {
		rest := strings.TrimPrefix(description, "由单据")
		if index := strings.Index(rest, "退货产生"); index > 0 {
			return rest[:index]
		}
	}
	return ""
}

func marshalBojunJSON(value interface{}) (string, error) {
	if value == nil {
		return "{}", nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(data), nil
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

func floatFromAny(value interface{}) float64 {
	switch typed := value.(type) {
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case float64:
		return typed
	case string:
		parsed, _ := strconv.ParseFloat(typed, 64)
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
