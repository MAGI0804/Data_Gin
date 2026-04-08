package data_svc

import (
	"context"
	"encoding/json"
	"fmt"
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
	rawDataDAO *data_dao.RawDataDAO
}

func NewIngestService() *IngestService {
	return &IngestService{
		rawDataDAO: data_dao.NewRawDataDAO(),
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
		rawData := &model.RawData{
			DataType:   req.DataType,
			RawContent: fmt.Sprintf("%v", item),
			Metadata:   fmt.Sprintf(`{"ingested_at": %d, "source": "%s"}`, time.Now().Unix(), req.DataSource),
			Status:     "pending",
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
			rawData := &model.RawData{
				DataType:   item.DataType,
				RawContent: fmt.Sprintf("%v", data),
				Metadata:   fmt.Sprintf(`{"ingested_at": %d, "source": "%s", "batch_id": "%s"}`, time.Now().Unix(), item.DataSource, req.BatchID),
				Status:     "pending",
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
func (s *IngestService) RawIngestData(ctx context.Context, req *requestbody.RawIngestRequest, clientIP string) (*IngestResult, error) {
	requestID := uuid.New().String()

	// 为缺失的参数设置默认值
	dataSource := req.DataSource
	if dataSource == "" {
		dataSource = "unknown"
	}

	dataType := req.DataType
	if dataType == "" {
		dataType = "raw"
	}

	logger.Info("接收原始格式数据", zap.String("request_id", requestID), zap.String("data_source", dataSource), zap.String("data_type", dataType), zap.String("client_ip", clientIP))

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
	metadata := map[string]interface{}{
		"ingested_at": time.Now().Unix(),
		"source":      dataSource,
		"format":      "raw",
		"client_ip":   clientIP,
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

	// 直接保存原始数据，不进行任何格式验证或转换
	rawData := &model.RawData{
		DataSourceID: 0, // 设置默认值，因为该字段在数据库中是 not null
		DataType:     dataType,
		RawContent:   string(rawContentJSON),
		Metadata:     string(metadataJSON),
		Status:       "pending",
	}

	id, err := s.rawDataDAO.Create(ctx, rawData)
	if err != nil {
		logger.Error("保存原始数据失败", zap.Error(err))
		failedCount++
	} else {
		acceptedCount++

		// 投递异步处理任务
		task := job.NewDataProcessTask(id)

		_, err = jobClient.Client.Enqueue(task)
		if err != nil {
			logger.Error("投递任务失败", zap.Error(err))
		}
	}

	logger.Info("原始格式数据接收完成", zap.String("request_id", requestID), zap.Int("accepted_count", acceptedCount), zap.Int("failed_count", failedCount), zap.String("client_ip", clientIP))

	return &IngestResult{
		RequestID:     requestID,
		AcceptedCount: acceptedCount,
		FailedCount:   failedCount,
	}, nil
}
