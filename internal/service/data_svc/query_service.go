package data_svc

import (
	"context"
	"encoding/json"

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

type RawDataResponse struct {
	ID           uint                   `json:"id"`
	DataSourceID uint                  `json:"data_source_id"`
	ExternalID   string                 `json:"external_id"`
	DataType     string                 `json:"data_type"`
	RawContent   map[string]interface{} `json:"raw_content"`
	Metadata     map[string]interface{} `json:"metadata"`
	Status       string                 `json:"status"`
	ErrorMessage string                 `json:"error_message"`
	ProcessedAt  int                   `json:"processed_at"`
	Remark       string                 `json:"remark"`
	Source       string                 `json:"source"`
	ClientIP     string                 `json:"client_ip"`
	IngestedAt   interface{}           `json:"ingested_at"`
	CreatedAt    interface{}           `json:"created_at"`
	UpdatedAt    interface{}           `json:"updated_at"`
}

type RawDataListResult struct {
	List       []RawDataResponse `json:"list"`
	Total      int64             `json:"total"`
	Page       int               `json:"page"`
	PageSize   int               `json:"page_size"`
	TotalPages int               `json:"total_pages"`
}

func (s *QueryService) GetRawDataList(ctx context.Context, page, pageSize int, source, startTime, endTime string) (*RawDataListResult, error) {
	logger.Info("查询原始数据列表", zap.Int("page", page), zap.Int("page_size", pageSize), zap.String("source", source), zap.String("start_time", startTime), zap.String("end_time", endTime))

	params := data_dao.RawDataQueryParams{
		Source:    source,
		StartTime: startTime,
		EndTime:   endTime,
		Page:      page,
		PageSize:  pageSize,
	}

	result, err := s.rawDataDAO.FindWithPagination(ctx, params)
	if err != nil {
		logger.Error("查询原始数据列表失败", zap.Error(err))
		return nil, err
	}

	list := make([]RawDataResponse, 0, len(result.List))
	for _, item := range result.List {
		var rawContent map[string]interface{}
		if item.RawContent != "" {
			json.Unmarshal([]byte(item.RawContent), &rawContent)
		}

		var metadata map[string]interface{}
		if item.Metadata != "" {
			json.Unmarshal([]byte(item.Metadata), &metadata)
		}

		list = append(list, RawDataResponse{
			ID:           item.ID,
			DataSourceID: item.DataSourceID,
			ExternalID:   item.ExternalID,
			DataType:     item.DataType,
			RawContent:   rawContent,
			Metadata:     metadata,
			Status:       item.Status,
			ErrorMessage: item.ErrorMessage,
			ProcessedAt:  item.ProcessedAt,
			Remark:       item.Remark,
			Source:       item.Source,
			ClientIP:     item.ClientIP,
			IngestedAt:   item.IngestedAt,
			CreatedAt:    item.CreatedAt,
			UpdatedAt:    item.UpdatedAt,
		})
	}

	totalPages := int(result.Total) / pageSize
	if int(result.Total)%pageSize > 0 {
		totalPages++
	}

	return &RawDataListResult{
		List:       list,
		Total:      result.Total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
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
