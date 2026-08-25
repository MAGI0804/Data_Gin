package data_ctrl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gin-biz-web-api/internal/reportoracle"
	"gin-biz-web-api/internal/service/data_svc"
)

func TestReportInputQueryControllerForwardsReportConditionAndSearch(t *testing.T) {
	service := &fakeReportInputQueryService{options: &data_svc.ReportInputOptionListDTO{Items: []reportoracle.InputOption{{ID: "S001", Name: "上海店"}}}}
	controller := NewReportInputQueryControllerWithService(service)
	router := reportControllerRouter()
	router.GET("/reports/:id/input-options/:condition_code", controller.Options)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/reports/9/input-options/store?name=%E4%B8%8A%E6%B5%B7%E5%BA%97", nil))

	if recorder.Code != http.StatusOK || service.actor != 17 || service.reportID != 9 || service.conditionCode != "store" || service.name != "上海店" {
		t.Fatalf("response=%d %s service=%#v", recorder.Code, recorder.Body, service)
	}
	if body := recorder.Body.String(); body == "" || !containsAll(body, `"id":"S001"`, `"name":"上海店"`) {
		t.Fatalf("body = %s", body)
	}
}

type fakeReportInputQueryService struct {
	options       *data_svc.ReportInputOptionListDTO
	actor         uint
	reportID      uint
	conditionCode string
	name          string
}

func (*fakeReportInputQueryService) List() *data_svc.ReportInputQueryListDTO {
	return &data_svc.ReportInputQueryListDTO{Items: []string{"stores"}}
}

func (service *fakeReportInputQueryService) Options(_ context.Context, actor, reportID uint, conditionCode, name string) (*data_svc.ReportInputOptionListDTO, error) {
	service.actor, service.reportID, service.conditionCode, service.name = actor, reportID, conditionCode, name
	return service.options, nil
}

func containsAll(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if !strings.Contains(value, candidate) {
			return false
		}
	}
	return true
}
