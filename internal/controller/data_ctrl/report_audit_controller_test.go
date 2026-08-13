package data_ctrl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"gin-biz-web-api/internal/service/data_svc"
)

func TestReportAuditControllerValidatesAndPassesFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &fakeReportAuditService{}
	controller := NewReportAuditControllerWithService(service)
	router := gin.New()
	router.GET("/report-audits", controller.List)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/report-audits?afterId=90&limit=20&action=A&targetType=T&targetId=31", nil))
	if recorder.Code != http.StatusOK || service.query.AfterID != 90 || service.query.Limit != 20 ||
		service.query.Action != "A" || service.query.TargetType != "T" || service.query.TargetID != 31 ||
		recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("response=%d query=%#v headers=%v body=%s", recorder.Code, service.query, recorder.Header(), recorder.Body)
	}

	for _, target := range []string{"/report-audits?afterId=0", "/report-audits?targetId=x", "/report-audits?limit=101"} {
		recorder = httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
		if recorder.Code != http.StatusUnprocessableEntity {
			t.Fatalf("invalid target %s response=%d body=%s", target, recorder.Code, recorder.Body)
		}
	}
}

type fakeReportAuditService struct {
	query data_svc.ReportAuditQuery
}

func (service *fakeReportAuditService) List(_ context.Context, query data_svc.ReportAuditQuery) (*data_svc.ReportAuditListDTO, error) {
	service.query = query
	return &data_svc.ReportAuditListDTO{}, nil
}
