package data_svc

import (
	"context"
	"errors"
	"testing"
	"time"

	"gin-biz-web-api/internal/reportrepo"
	"gin-biz-web-api/model"
)

func TestReportAuditServiceNormalizesFiltersAndMapsPage(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	store := &fakeReportAuditStore{page: &reportrepo.ReportAuditPage{
		Items: []model.ReportAudit{{
			BaseModel: model.BaseModel{ID: 90}, ActorUserID: 17,
			Action: "REPORT_RESULT_QUERY_SUCCESS", TargetType: "REPORT_RUN", TargetID: 31,
			RequestID: "request-1", DetailJSON: model.JSONText(`{"rowCount":1}`), CreatedAt: now,
		}},
		HasMore: true, NextAfterID: 90,
	}}
	result, err := NewReportAuditServiceWithStore(store).List(t.Context(), ReportAuditQuery{
		AfterID: 100, Limit: 20, Action: " report_result_query_success ", TargetType: " report_run ", TargetID: 31,
	})
	if err != nil || store.query.Action != "REPORT_RESULT_QUERY_SUCCESS" || store.query.TargetType != "REPORT_RUN" ||
		len(result.Items) != 1 || result.Items[0].ID != 90 || !result.HasMore || result.NextAfterID != 90 {
		t.Fatalf("List() = %#v, %v store=%#v", result, err, store)
	}
	if _, err := NewReportAuditServiceWithStore(store).List(t.Context(), ReportAuditQuery{Limit: 101}); !errors.Is(err, ErrReportAuditQueryInvalid) {
		t.Fatalf("invalid limit error = %v", err)
	}
}

type fakeReportAuditStore struct {
	query reportrepo.ReportAuditListQuery
	page  *reportrepo.ReportAuditPage
}

func (store *fakeReportAuditStore) ListReportAudits(_ context.Context, query reportrepo.ReportAuditListQuery) (*reportrepo.ReportAuditPage, error) {
	store.query = query
	return store.page, nil
}
