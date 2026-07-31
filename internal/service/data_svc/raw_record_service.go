package data_svc

import (
	"context"
	"time"

	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/model"
)

// RawRecordListQuery is the validated service-layer filter for safe
// raw_records pagination.
type RawRecordListQuery struct {
	Page      int
	PageSize  int
	Source    string
	Status    string
	TraceID   string
	StartTime *time.Time
	EndTime   *time.Time
	Origin    string
}

// RawRecordListItem is the safe raw_records list representation exposed by
// the management API. Raw content, headers, metadata and error details are
// intentionally absent.
type RawRecordListItem struct {
	ID         uint              `json:"id"`
	SourceID   uint              `json:"source_id"`
	SourceCode string            `json:"source_code"`
	Status     string            `json:"status"`
	TraceID    string            `json:"trace_id"`
	ReceivedAt *model.TimeNormal `json:"received_at"`
	CreatedAt  int               `json:"created_at"`
}

type RawRecordListResult struct {
	List       []RawRecordListItem `json:"list"`
	Total      int64               `json:"total"`
	Page       int                 `json:"page"`
	PageSize   int                 `json:"page_size"`
	TotalPages int                 `json:"total_pages"`
}

type RawRecordService struct {
	rawRecordDAO *data_dao.RawRecordDAO
}

func NewRawRecordService() *RawRecordService {
	return &RawRecordService{rawRecordDAO: data_dao.NewRawRecordDAO()}
}

func (s *RawRecordService) List(ctx context.Context, params RawRecordListQuery) (*RawRecordListResult, error) {
	page, pageSize := rawRecordPagination(params.Page, params.PageSize)
	result, err := s.rawRecordDAO.FindPage(ctx, data_dao.RawRecordListQuery{
		Page:      page,
		PageSize:  pageSize,
		Source:    params.Source,
		Status:    params.Status,
		TraceID:   params.TraceID,
		StartTime: params.StartTime,
		EndTime:   params.EndTime,
		Origin:    params.Origin,
	})
	if err != nil {
		return nil, err
	}

	items := make([]RawRecordListItem, 0, len(result.List))
	for _, item := range result.List {
		items = append(items, RawRecordListItem{
			ID:         item.ID,
			SourceID:   item.SourceID,
			SourceCode: item.SourceCode,
			Status:     item.Status,
			TraceID:    item.TraceID,
			ReceivedAt: item.ReceivedAt,
			CreatedAt:  item.CreatedAt,
		})
	}

	totalPages := int(result.Total) / pageSize
	if int(result.Total)%pageSize > 0 {
		totalPages++
	}

	return &RawRecordListResult{
		List:       items,
		Total:      result.Total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

func rawRecordPagination(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	return page, pageSize
}
