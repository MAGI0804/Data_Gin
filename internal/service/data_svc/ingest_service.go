package data_svc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/job"
	"gin-biz-web-api/model"
	jobClient "gin-biz-web-api/pkg/job"
	"gin-biz-web-api/pkg/logger"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type IngestService struct {
	rawDataDAO     *data_dao.RawDataDAO
	rawRecordDAO   *data_dao.RawRecordDAO
	sourceDAO      *data_dao.SourceDefinitionDAO
	pipelineRunDAO *data_dao.PipelineRunDAO
}

func NewIngestService() *IngestService {
	return &IngestService{
		rawDataDAO:     data_dao.NewRawDataDAO(),
		rawRecordDAO:   data_dao.NewRawRecordDAO(),
		sourceDAO:      data_dao.NewSourceDefinitionDAO(),
		pipelineRunDAO: data_dao.NewPipelineRunDAO(),
	}
}

// IngestResult 数据接收结果
type IngestResult struct {
	RequestID     string
	AcceptedCount int
	FailedCount   int
}

// IngestData 接收数据推送
func (s *IngestService) IngestData(ctx context.Context, req *requestbody.IngestRequest) (*IngestResult, error) {
	requestID := uuid.New().String()
	logger.Info("接收数据推送", zap.String("request_id", requestID), zap.String("data_source", req.DataSource), zap.String("data_type", req.DataType))

	var acceptedCount, failedCount int

	for _, item := range req.Data {
		ingestedTime := time.Now()
		metadata := map[string]interface{}{
			"ingested_at": ingestedTime.Unix(),
			"source":      req.DataSource,
		}
		metadataJSON, _ := json.Marshal(metadata)

		rawData := &model.RawData{
			DataType:   req.DataType,
			RawContent: fmt.Sprintf("%v", item),
			Metadata:   string(metadataJSON),
			Status:     "pending",
			Source:     req.DataSource,
			IngestedAt: &ingestedTime,
		}

		id, err := s.rawDataDAO.Create(ctx, rawData)
		if err != nil {
			logger.Error("保存原始数据失败", zap.Error(err))
			failedCount++
			continue
		}

		acceptedCount++

		// 投递异步处理任务
		task := job.NewDataProcessTask(id)

		_, err = jobClient.Client.Enqueue(task)
		if err != nil {
			logger.Error("投递任务失败", zap.Error(err))
			continue
		}
	}

	logger.Info("数据接收完成", zap.String("request_id", requestID), zap.Int("accepted_count", acceptedCount), zap.Int("failed_count", failedCount))

	return &IngestResult{
		RequestID:     requestID,
		AcceptedCount: acceptedCount,
		FailedCount:   failedCount,
	}, nil
}

// BatchIngestResult 批量数据接收结果
type BatchIngestResult struct {
	BatchID       string
	AcceptedCount int
	FailedCount   int
}

// IngestBatchData 批量接收数据
func (s *IngestService) IngestBatchData(ctx context.Context, req *requestbody.BatchIngestRequest) (*BatchIngestResult, error) {
	logger.Info("接收批量数据推送", zap.String("batch_id", req.BatchID), zap.Int("item_count", len(req.Items)))

	var acceptedCount, failedCount int

	for _, item := range req.Items {
		for _, data := range item.Data {
			ingestedTime := time.Now()
			metadata := map[string]interface{}{
				"ingested_at": ingestedTime.Unix(),
				"source":      item.DataSource,
				"batch_id":    req.BatchID,
			}
			metadataJSON, _ := json.Marshal(metadata)

			rawData := &model.RawData{
				DataType:   item.DataType,
				RawContent: fmt.Sprintf("%v", data),
				Metadata:   string(metadataJSON),
				Status:     "pending",
				Source:     item.DataSource,
				IngestedAt: &ingestedTime,
			}

			id, err := s.rawDataDAO.Create(ctx, rawData)
			if err != nil {
				logger.Error("保存原始数据失败", zap.Error(err))
				failedCount++
				continue
			}

			acceptedCount++

			// 投递异步处理任务
			task := job.NewDataProcessTask(id)

			_, err = jobClient.Client.Enqueue(task)
			if err != nil {
				logger.Error("投递任务失败", zap.Error(err))
				continue
			}
		}
	}

	logger.Info("批量数据接收完成", zap.String("batch_id", req.BatchID), zap.Int("accepted_count", acceptedCount), zap.Int("failed_count", failedCount))

	return &BatchIngestResult{
		BatchID:       req.BatchID,
		AcceptedCount: acceptedCount,
		FailedCount:   failedCount,
	}, nil
}

// RawIngestData 接收原始格式数据（用于接收任意格式的数据）
func (s *IngestService) RawIngestData(
	ctx context.Context,
	req *requestbody.RawIngestRequest,
	clientIP string,
	query url.Values,
	headers http.Header,
) (*IngestResult, error) {
	traceID := uuid.New().String()

	dataType := req.DataType
	if dataType == "" {
		dataType = "raw"
	}

	sourceDefinitions, err := s.sourceDAO.FindEnabledWithQueryKey(ctx)
	if err != nil {
		logger.Warn("加载自定义来源参数配置失败，使用默认来源解析", zap.Error(err))
		sourceDefinitions = []model.SourceDefinition{}
	}

	resolvedSource := ResolveRawSource(query, req.DataSource, req.Remark, sourceDefinitions)
	dataSource := resolvedSource.SourceCode

	logger.Info(
		"接收原始格式数据",
		zap.String("trace_id", traceID),
		zap.String("data_source", dataSource),
		zap.String("data_type", dataType),
		zap.String("client_ip", clientIP),
	)

	var acceptedCount, failedCount int

	// 为缺失的原始内容设置默认值
	rawContent := req.RawContent
	if rawContent == nil {
		// 如果原始内容为 nil，使用空对象
		rawContent = map[string]interface{}{}
		logger.Info("Raw content is nil, using empty object")
	} else {
		logger.Info("Raw content received", zap.Any("raw_content", rawContent))
	}

	// 将原始内容转换为 JSON 格式
	rawContentJSON, err := json.Marshal(rawContent)
	if err != nil {
		// 如果转换失败，使用空对象
		rawContentJSON = []byte("{}")
		logger.Warn("Failed to marshal raw content, using empty object", zap.Error(err))
	} else {
		logger.Info("Raw content marshaled", zap.String("raw_content_json", string(rawContentJSON)))
	}

	// 构建元数据 JSON
	ingestedTime := time.Now()
	sourceID := s.resolveSourceID(ctx, dataSource)
	metadata := map[string]interface{}{
		"ingested_at":        ingestedTime.Unix(),
		"trace_id":           traceID,
		"source":             dataSource,
		"source_query_key":   resolvedSource.SourceQueryKey,
		"source_query_value": resolvedSource.SourceQueryValue,
		"query":              query,
		"format":             "raw",
		"client_ip":          clientIP,
	}

	// 添加备注信息
	if req.Remark != "" {
		metadata["remark"] = req.Remark
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		// 如果转换失败，使用空对象
		metadataJSON = []byte("{}")
		logger.Warn("Failed to marshal metadata, using empty object", zap.Error(err))
	}

	// 从metadata中提取字段
	var remark, clientIPStr string
	if v, ok := metadata["remark"].(string); ok {
		remark = v
	}
	if v, ok := metadata["client_ip"].(string); ok {
		clientIPStr = v
	}

	runID, runErr := s.pipelineRunDAO.Create(ctx, &model.PipelineRun{
		TraceID:      traceID,
		RunType:      "ingest",
		TriggerType:  "api",
		SourceID:     sourceID,
		Status:       "running",
		TotalCount:   1,
		SuccessCount: 0,
		FailedCount:  0,
		StartedAt:    &model.TimeNormal{Time: ingestedTime},
	})
	if runErr != nil {
		logger.Warn("创建接收运行记录失败", zap.String("trace_id", traceID), zap.Error(runErr))
	}

	queryJSON := mustMarshalString(query)
	headersJSON := mustMarshalString(redactHeaders(headers))

	rawRecord := &model.RawRecord{
		SourceID:     sourceID,
		SourceCode:   dataSource,
		RawContent:   string(rawContentJSON),
		HeadersJSON:  headersJSON,
		QueryJSON:    queryJSON,
		MetadataJSON: string(metadataJSON),
		Status:       "received",
		TraceID:      traceID,
		ReceivedAt:   &model.TimeNormal{Time: ingestedTime},
	}

	rawRecordID, err := s.rawRecordDAO.Create(ctx, rawRecord)
	if err != nil {
		logger.Error("保存通用原始记录失败", zap.String("trace_id", traceID), zap.Error(err))
		failedCount++
		s.finishIngestRun(ctx, runID, "failed", acceptedCount, failedCount, err.Error())
		return nil, err
	}

	// 直接保存原始数据，不进行任何格式验证或转换
	rawData := &model.RawData{
		DataSourceID: sourceID,
		DataType:     dataType,
		RawContent:   string(rawContentJSON),
		Metadata:     string(metadataJSON),
		Status:       "pending",
		Remark:       remark,
		Source:       dataSource,
		ClientIP:     clientIPStr,
		IngestedAt:   &ingestedTime,
	}

	id, err := s.rawDataDAO.Create(ctx, rawData)
	if err != nil {
		logger.Error("保存原始数据失败", zap.Error(err))
		failedCount++
		s.rawRecordDAO.UpdateStatus(ctx, rawRecordID, "failed", err.Error())
		s.finishIngestRun(ctx, runID, "failed", acceptedCount, failedCount, err.Error())
	} else {
		acceptedCount++

		// 投递异步处理任务
		task := job.NewDataProcessTask(id)

		_, err = jobClient.Client.Enqueue(task)
		if err != nil {
			logger.Error("投递任务失败", zap.Error(err))
			s.rawRecordDAO.UpdateStatus(ctx, rawRecordID, "failed", err.Error())
			s.finishIngestRun(ctx, runID, "failed", 0, 1, err.Error())
		} else {
			s.rawRecordDAO.UpdateStatus(ctx, rawRecordID, "queued", "")
			s.finishIngestRun(ctx, runID, "success", 1, 0, "")
		}
	}

	logger.Info("原始格式数据接收完成", zap.String("trace_id", traceID), zap.Int("accepted_count", acceptedCount), zap.Int("failed_count", failedCount), zap.String("client_ip", clientIP))

	return &IngestResult{
		RequestID:     traceID,
		AcceptedCount: acceptedCount,
		FailedCount:   failedCount,
	}, nil
}

func (s *IngestService) resolveSourceID(ctx context.Context, sourceCode string) uint {
	if strings.TrimSpace(sourceCode) == "" || sourceCode == "unknown" {
		return 0
	}

	source, err := s.sourceDAO.FindByCode(ctx, sourceCode)
	if err != nil {
		return 0
	}

	return source.ID
}

func (s *IngestService) finishIngestRun(ctx context.Context, runID uint, status string, successCount, failedCount int, errorMessage string) {
	if runID == 0 {
		return
	}
	if err := s.pipelineRunDAO.Finish(ctx, runID, status, successCount, failedCount, errorMessage); err != nil {
		logger.Warn("更新接收运行记录失败", zap.Uint("run_id", runID), zap.Error(err))
	}
}

func mustMarshalString(value interface{}) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func redactHeaders(headers http.Header) http.Header {
	redacted := make(http.Header, len(headers))
	for key, values := range headers {
		switch strings.ToLower(key) {
		case "authorization", "cookie", "set-cookie", "x-api-key":
			redacted[key] = []string{"[REDACTED]"}
		default:
			copied := make([]string, len(values))
			copy(copied, values)
			redacted[key] = copied
		}
	}
	return redacted
}
