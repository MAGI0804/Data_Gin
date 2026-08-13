package reportrepo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"gin-biz-web-api/model"

	"gorm.io/gorm"
)

type ReportAuditListQuery struct {
	AfterID    uint
	Limit      int
	Action     string
	TargetType string
	TargetID   uint
}

type ReportAuditPage struct {
	Items       []model.ReportAudit
	HasMore     bool
	NextAfterID uint
}

func (repository *Repository) WriteReportAudit(ctx context.Context, audit model.ReportAudit) error {
	if repository == nil || repository.db == nil || ctx == nil || !validReportAuditActor(audit) ||
		strings.TrimSpace(audit.Action) == "" || strings.TrimSpace(audit.TargetType) == "" || audit.TargetID == 0 ||
		strings.TrimSpace(audit.RequestID) == "" || !validOptionalAuditJSON(audit.DetailJSON) {
		return fmt.Errorf("report audit: invalid record")
	}
	if err := repository.db.WithContext(ctx).Create(&audit).Error; err != nil {
		return fmt.Errorf("report audit: create: %w", err)
	}
	return nil
}

func validReportAuditActor(audit model.ReportAudit) bool {
	switch audit.ActorType {
	case "", model.ReportAuditActorUser:
		return audit.ActorUserID > 0
	case model.ReportAuditActorSystem:
		return audit.ActorUserID == 0
	default:
		return false
	}
}

func (repository *Repository) ListReportAudits(ctx context.Context, query ReportAuditListQuery) (*ReportAuditPage, error) {
	if repository == nil || repository.db == nil || ctx == nil || query.Limit < 1 || query.Limit > 100 {
		return nil, fmt.Errorf("report audit: invalid list request")
	}
	rows := make([]model.ReportAudit, 0, query.Limit+1)
	db := buildReportAuditListQuery(repository.db.WithContext(ctx), query)
	if err := db.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("report audit: list: %w", err)
	}
	page := &ReportAuditPage{Items: make([]model.ReportAudit, 0, query.Limit)}
	if len(rows) > query.Limit {
		page.HasMore = true
		rows = rows[:query.Limit]
	}
	page.Items = append(page.Items, rows...)
	if page.HasMore && len(page.Items) > 0 {
		page.NextAfterID = page.Items[len(page.Items)-1].ID
	}
	return page, nil
}

func buildReportAuditListQuery(db *gorm.DB, query ReportAuditListQuery) *gorm.DB {
	db = db.Model(&model.ReportAudit{})
	if query.AfterID > 0 {
		db = db.Where("id < ?", query.AfterID)
	}
	if query.Action != "" {
		db = db.Where("action = ?", query.Action)
	}
	if query.TargetType != "" {
		db = db.Where("target_type = ?", query.TargetType)
	}
	if query.TargetID > 0 {
		db = db.Where("target_id = ?", query.TargetID)
	}
	return db.Order("id DESC").Limit(query.Limit + 1)
}

func validOptionalAuditJSON(value model.JSONText) bool {
	trimmed := strings.TrimSpace(string(value))
	return trimmed == "" || json.Valid([]byte(trimmed))
}
