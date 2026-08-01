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
	cleanRecordDAO   *data_dao.CleanRecordDAO
	statisticsDAO    *data_dao.StatisticsDAO
}

func NewQueryService() *QueryService {
	return &QueryService{
		rawDataDAO:       data_dao.NewRawDataDAO(),
		processedDataDAO: data_dao.NewProcessedDataDAO(),
		cleanRecordDAO:   data_dao.NewCleanRecordDAO(),
		statisticsDAO:    data_dao.NewStatisticsDAO(),
	}
}

type RawDataResponse struct {
	ID           uint        `json:"id"`
	DataSourceID uint        `json:"data_source_id"`
	ExternalID   string      `json:"external_id"`
	DataType     string      `json:"data_type"`
	RawContent   interface{} `json:"raw_content"`
	Metadata     interface{} `json:"metadata"`
	Status       string      `json:"status"`
	ErrorMessage string      `json:"error_message"`
	ProcessedAt  int         `json:"processed_at"`
	Remark       string      `json:"remark"`
	Source       string      `json:"source"`
	ClientIP     string      `json:"client_ip"`
	IngestedAt   interface{} `json:"ingested_at"`
	CreatedAt    interface{} `json:"created_at"`
	UpdatedAt    interface{} `json:"updated_at"`
}

type RawDataListResult struct {
	List       []RawDataResponse `json:"list"`
	Total      int64             `json:"total"`
	Page       int               `json:"page"`
	PageSize   int               `json:"page_size"`
	TotalPages int               `json:"total_pages"`
}

// RawDataListQuery contains every optional filter accepted by the paginated
// raw-data management query. Keeping this as a value object prevents callers
// from relying on an error-prone positional-parameter contract as filters are
// extended.
type RawDataListQuery struct {
	Page        int
	PageSize    int
	Source      string
	DataType    string
	Status      string
	BusinessKey string
	StartTime   string
	EndTime     string
	Origin      string
}

type ProcessedDataListResult struct {
	List           []model.ProcessedData `json:"list"`
	Total          int64                 `json:"total"`
	Page           int                   `json:"page"`
	PageSize       int                   `json:"page_size"`
	TotalPages     int                   `json:"total_pages"`
	AverageQuality float64               `json:"avg_quality"`
}

type CleanRecordListResult struct {
	List           []model.CleanRecord `json:"list"`
	Total          int64               `json:"total"`
	Page           int                 `json:"page"`
	PageSize       int                 `json:"page_size"`
	TotalPages     int                 `json:"total_pages"`
	AverageQuality float64             `json:"avg_quality"`
}

func (s *QueryService) GetProcessedDataList(ctx context.Context, page, pageSize int, dataType string, minQuality, maxQuality *float64, createdFrom, createdTo int64) (*ProcessedDataListResult, error) {
	result, err := s.processedDataDAO.FindWithPagination(ctx, data_dao.ProcessedDataListQuery{Page: page, PageSize: pageSize, DataType: dataType, MinQuality: minQuality, MaxQuality: maxQuality, CreatedFrom: createdFrom, CreatedTo: createdTo})
	if err != nil {
		return nil, err
	}
	totalPages := int(result.Total) / pageSize
	if int(result.Total)%pageSize > 0 {
		totalPages++
	}
	return &ProcessedDataListResult{List: result.List, Total: result.Total, Page: page, PageSize: pageSize, TotalPages: totalPages, AverageQuality: result.AverageQuality}, nil
}

func (s *QueryService) GetCleanRecordList(ctx context.Context, params data_dao.CleanRecordListQuery) (*CleanRecordListResult, error) {
	result, err := s.cleanRecordDAO.FindWithPagination(ctx, params)
	if err != nil {
		return nil, err
	}
	totalPages := int(result.Total) / params.PageSize
	if int(result.Total)%params.PageSize > 0 {
		totalPages++
	}
	return &CleanRecordListResult{List: result.List, Total: result.Total, Page: params.Page, PageSize: params.PageSize, TotalPages: totalPages, AverageQuality: result.AverageQuality}, nil
}

func (s *QueryService) GetRawDataList(ctx context.Context, query RawDataListQuery) (*RawDataListResult, error) {
	// Keep the service safe for future internal callers as well as the HTTP
	// controller, which already applies the same request defaults.
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 20
	}
	if query.PageSize > 100 {
		query.PageSize = 100
	}
	logger.Info("查询原始数据列表", zap.Int("page", query.Page), zap.Int("page_size", query.PageSize), zap.String("source", query.Source), zap.String("data_type", query.DataType), zap.String("status", query.Status), zap.String("start_time", query.StartTime), zap.String("end_time", query.EndTime), zap.String("origin", query.Origin))

	params := data_dao.RawDataQueryParams{
		Source:      query.Source,
		DataType:    query.DataType,
		Status:      query.Status,
		BusinessKey: query.BusinessKey,
		StartTime:   query.StartTime,
		EndTime:     query.EndTime,
		Origin:      query.Origin,
		Page:        query.Page,
		PageSize:    query.PageSize,
	}

	result, err := s.rawDataDAO.FindWithPagination(ctx, params)
	if err != nil {
		logger.Error("查询原始数据列表失败", zap.Error(err))
		return nil, err
	}

	list := make([]RawDataResponse, 0, len(result.List))
	for _, item := range result.List {
		list = append(list, RawDataResponse{
			ID:           item.ID,
			DataSourceID: item.DataSourceID,
			ExternalID:   item.ExternalID,
			DataType:     item.DataType,
			RawContent:   decodeRawJSON(item.RawContent),
			Metadata:     decodeRawJSON(item.Metadata),
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

	totalPages := int(result.Total) / query.PageSize
	if int(result.Total)%query.PageSize > 0 {
		totalPages++
	}

	return &RawDataListResult{
		List:       list,
		Total:      result.Total,
		Page:       query.Page,
		PageSize:   query.PageSize,
		TotalPages: totalPages,
	}, nil
}

// decodeRawJSON keeps every valid JSON value shape for the management read
// model. Raw ingestion accepts arbitrary JSON, so decoding only into a map
// would silently discard arrays and scalar values before the UI can display a
// redacted preview.
func decodeRawJSON(value string) interface{} {
	if value == "" {
		return nil
	}
	var decoded interface{}
	if err := json.Unmarshal([]byte(value), &decoded); err != nil {
		return nil
	}
	return decoded
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
