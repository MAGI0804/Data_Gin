package data_svc

import (
	"context"

	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/logger"

	"go.uber.org/zap"
)

type QueryService struct {
	rawDataDAO       *data_dao.RawDataDAO
	processedDataDAO *data_dao.ProcessedDataDAO
	statisticsDAO    *data_dao.StatisticsDAO
}

func NewQueryService() *QueryService {
	return &QueryService{
		rawDataDAO:       data_dao.NewRawDataDAO(),
		processedDataDAO: data_dao.NewProcessedDataDAO(),
		statisticsDAO:    data_dao.NewStatisticsDAO(),
	}
}

// GetRawData 查询原始数据
func (s *QueryService) GetRawData(ctx context.Context, dataType string, dataSourceID uint, status string, limit int) ([]model.RawData, error) {
	logger.Info("查询原始数据", zap.String("data_type", dataType), zap.Uint("data_source_id", dataSourceID), zap.String("status", status), zap.Int("limit", limit))

	var rawDataList []model.RawData
	var err error

	if status != "" {
		// 根据状态查询
		rawDataList, err = s.rawDataDAO.FindByStatus(ctx, status, limit)
	} else if dataSourceID > 0 {
		// 根据数据源ID查询
		rawDataList, err = s.rawDataDAO.FindByDataSource(ctx, dataSourceID, limit)
	} else {
		// 默认查询
		rawDataList, err = s.rawDataDAO.FindByStatus(ctx, "", limit)
	}

	if err != nil {
		logger.Error("查询原始数据失败", zap.Error(err))
		return nil, err
	}

	return rawDataList, nil
}

// GetProcessedData 查询处理后的数据
func (s *QueryService) GetProcessedData(ctx context.Context, dataType string, limit int) ([]model.ProcessedData, error) {
	logger.Info("查询处理后的数据", zap.String("data_type", dataType), zap.Int("limit", limit))

	processedDataList, err := s.processedDataDAO.FindByDataType(ctx, dataType, limit)
	if err != nil {
		logger.Error("查询处理后的数据失败", zap.Error(err))
		return nil, err
	}

	return processedDataList, nil
}

// GetStatistics 查询统计数据
func (s *QueryService) GetStatistics(ctx context.Context, startDate, endDate, dataType string) ([]model.DataStatistics, error) {
	logger.Info("查询统计数据", zap.String("start_date", startDate), zap.String("end_date", endDate), zap.String("data_type", dataType))

	statsList, err := s.statisticsDAO.FindByDateRange(ctx, startDate, endDate, dataType)
	if err != nil {
		logger.Error("查询统计数据失败", zap.Error(err))
		return nil, err
	}

	return statsList, nil
}

// GetProcessedDataByRawDataID 根据原始数据ID查询处理结果
func (s *QueryService) GetProcessedDataByRawDataID(ctx context.Context, rawDataID uint) (*model.ProcessedData, error) {
	logger.Info("根据原始数据ID查询处理结果", zap.Uint("raw_data_id", rawDataID))

	processedData, err := s.processedDataDAO.FindByRawDataID(ctx, rawDataID)
	if err != nil {
		logger.Error("查询处理结果失败", zap.Error(err))
		return nil, err
	}

	return processedData, nil
}
