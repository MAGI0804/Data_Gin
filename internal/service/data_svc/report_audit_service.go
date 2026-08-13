package data_svc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gin-biz-web-api/internal/reportrepo"
	"gin-biz-web-api/model"
)

var ErrReportAuditQueryInvalid = errors.New("report audit query: invalid input")

type reportAuditQueryStore interface {
	ListReportAudits(context.Context, reportrepo.ReportAuditListQuery) (*reportrepo.ReportAuditPage, error)
}

type ReportAuditQuery struct {
	AfterID    uint
	Limit      int
	Action     string
	TargetType string
	TargetID   uint
}

type ReportAuditDTO struct {
	ID          uint           `json:"id"`
	ActorUserID uint           `json:"actorUserId"`
	Action      string         `json:"action"`
	TargetType  string         `json:"targetType"`
	TargetID    uint           `json:"targetId"`
	RequestID   string         `json:"requestId"`
	Detail      model.JSONText `json:"detail,omitempty"`
	CreatedAt   time.Time      `json:"createdAt"`
}

type ReportAuditListDTO struct {
	Items       []ReportAuditDTO `json:"items"`
	HasMore     bool             `json:"hasMore"`
	NextAfterID uint             `json:"nextAfterId,omitempty"`
}

type ReportAuditService struct {
	store reportAuditQueryStore
}

func NewReportAuditService() *ReportAuditService {
	return NewReportAuditServiceWithStore(reportrepo.New())
}

func NewReportAuditServiceWithStore(store reportAuditQueryStore) *ReportAuditService {
	if store == nil {
		panic("report audit query: store is required")
	}
	return &ReportAuditService{store: store}
}

func (service *ReportAuditService) List(ctx context.Context, query ReportAuditQuery) (*ReportAuditListDTO, error) {
	query.Action = strings.ToUpper(strings.TrimSpace(query.Action))
	query.TargetType = strings.ToUpper(strings.TrimSpace(query.TargetType))
	if service == nil || ctx == nil || query.Limit < 1 || query.Limit > 100 || len(query.Action) > 64 || len(query.TargetType) > 32 {
		return nil, ErrReportAuditQueryInvalid
	}
	page, err := service.store.ListReportAudits(ctx, reportrepo.ReportAuditListQuery{
		AfterID: query.AfterID, Limit: query.Limit, Action: query.Action,
		TargetType: query.TargetType, TargetID: query.TargetID,
	})
	if err != nil {
		return nil, fmt.Errorf("report audit query: list: %w", err)
	}
	result := &ReportAuditListDTO{
		Items:   make([]ReportAuditDTO, 0, len(page.Items)),
		HasMore: page.HasMore, NextAfterID: page.NextAfterID,
	}
	for _, row := range page.Items {
		result.Items = append(result.Items, ReportAuditDTO{
			ID: row.ID, ActorUserID: row.ActorUserID, Action: row.Action,
			TargetType: row.TargetType, TargetID: row.TargetID, RequestID: row.RequestID,
			Detail: row.DetailJSON, CreatedAt: row.CreatedAt,
		})
	}
	return result, nil
}
