package data_svc

import (
	"context"
	"fmt"
	"time"

	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/logger"

	"go.uber.org/zap"
)

type ProcessService struct {
	rawDataDAO       *data_dao.RawDataDAO
	processedDataDAO *data_dao.ProcessedDataDAO
}

func NewProcessService() *ProcessService {
	return &ProcessService{
		rawDataDAO:       data_dao.NewRawDataDAO(),
		processedDataDAO: data_dao.NewProcessedDataDAO(),
	}
}

// ProcessRawData 处理原始数据
func (s *ProcessService) ProcessRawData(ctx context.Context, rawData *model.RawData) (*model.ProcessedData, error) {
	logger.Info("开始处理原始数据", zap.Uint("raw_data_id", rawData.ID), zap.String("data_type", rawData.DataType))

	// 1. 更新原始数据状态为处理中
	err := s.rawDataDAO.UpdateStatus(ctx, rawData.ID, "processing", "")
	if err != nil {
		logger.Error("更新原始数据状态失败", zap.Error(err))
		return nil, fmt.Errorf("更新原始数据状态失败: %w", err)
	}

	// 2. 执行数据处理
	processedData, err := s.processData(ctx, rawData)
	if err != nil {
		// 更新原始数据状态为错误
		s.rawDataDAO.UpdateStatus(ctx, rawData.ID, "error", err.Error())
		logger.Error("数据处理失败", zap.Error(err))
		return nil, fmt.Errorf("数据处理失败: %w", err)
	}

	// 3. 保存处理结果
	_, err = s.processedDataDAO.Create(ctx, processedData)
	if err != nil {
		// 更新原始数据状态为错误
		s.rawDataDAO.UpdateStatus(ctx, rawData.ID, "error", err.Error())
		logger.Error("保存处理结果失败", zap.Error(err))
		return nil, fmt.Errorf("保存处理结果失败: %w", err)
	}

	// 4. 更新原始数据状态为已处理
	err = s.rawDataDAO.UpdateStatus(ctx, rawData.ID, "processed", "")
	if err != nil {
		logger.Error("更新原始数据状态失败", zap.Error(err))
		// 继续执行，不返回错误
	}

	logger.Info("数据处理完成", zap.Uint("raw_data_id", rawData.ID), zap.Uint("processed_data_id", processedData.ID))

	return processedData, nil
}

// processData 执行具体的数据处理逻辑
func (s *ProcessService) processData(ctx context.Context, rawData *model.RawData) (*model.ProcessedData, error) {
	// 根据数据类型执行不同的处理逻辑
	switch rawData.DataType {
	case "api":
		return s.processAPIData(ctx, rawData)
	case "database":
		return s.processDatabaseData(ctx, rawData)
	case "file":
		return s.processFileData(ctx, rawData)
	default:
		return s.processDefaultData(ctx, rawData)
	}
}

// processAPIData 处理API数据
func (s *ProcessService) processAPIData(ctx context.Context, rawData *model.RawData) (*model.ProcessedData, error) {
	// 模拟API数据处理
	logger.Info("处理API数据", zap.Uint("raw_data_id", rawData.ID))

	// 构建处理后的数据
	dataFields := fmt.Sprintf(`{"processed_at": %d, "data_type": "%s", "status": "processed", "original_content": "%s"}`,
		time.Now().Unix(), rawData.DataType, rawData.RawContent)

	return &model.ProcessedData{
		RawDataID:    rawData.ID,
		DataType:     rawData.DataType,
		DataFields:   dataFields,
		QualityScore: 95.5, // 模拟质量评分
		Version:      1,
		IsCurrent:    true,
	}, nil
}

// processDatabaseData 处理数据库数据
func (s *ProcessService) processDatabaseData(ctx context.Context, rawData *model.RawData) (*model.ProcessedData, error) {
	// 模拟数据库数据处理
	logger.Info("处理数据库数据", zap.Uint("raw_data_id", rawData.ID))

	// 构建处理后的数据
	dataFields := fmt.Sprintf(`{"processed_at": %d, "data_type": "%s", "status": "processed", "original_content": "%s"}`,
		time.Now().Unix(), rawData.DataType, rawData.RawContent)

	return &model.ProcessedData{
		RawDataID:    rawData.ID,
		DataType:     rawData.DataType,
		DataFields:   dataFields,
		QualityScore: 98.0, // 模拟质量评分
		Version:      1,
		IsCurrent:    true,
	}, nil
}

// processFileData 处理文件数据
func (s *ProcessService) processFileData(ctx context.Context, rawData *model.RawData) (*model.ProcessedData, error) {
	// 模拟文件数据处理
	logger.Info("处理文件数据", zap.Uint("raw_data_id", rawData.ID))

	// 构建处理后的数据
	dataFields := fmt.Sprintf(`{"processed_at": %d, "data_type": "%s", "status": "processed", "original_content": "%s"}`,
		time.Now().Unix(), rawData.DataType, rawData.RawContent)

	return &model.ProcessedData{
		RawDataID:    rawData.ID,
		DataType:     rawData.DataType,
		DataFields:   dataFields,
		QualityScore: 92.0, // 模拟质量评分
		Version:      1,
		IsCurrent:    true,
	}, nil
}

// processDefaultData 处理默认数据
func (s *ProcessService) processDefaultData(ctx context.Context, rawData *model.RawData) (*model.ProcessedData, error) {
	// 处理默认数据
	logger.Info("处理默认数据", zap.Uint("raw_data_id", rawData.ID), zap.String("data_type", rawData.DataType))

	// 构建处理后的数据
	dataFields := fmt.Sprintf(`{"processed_at": %d, "data_type": "%s", "status": "processed", "original_content": "%s"}`,
		time.Now().Unix(), rawData.DataType, rawData.RawContent)

	return &model.ProcessedData{
		RawDataID:    rawData.ID,
		DataType:     rawData.DataType,
		DataFields:   dataFields,
		QualityScore: 90.0, // 模拟质量评分
		Version:      1,
		IsCurrent:    true,
	}, nil
}
