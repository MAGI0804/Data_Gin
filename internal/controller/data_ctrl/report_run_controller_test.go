package data_ctrl

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"gin-biz-web-api/internal/service/data_svc"
	"gin-biz-web-api/model"
)

func TestReportRunControllerUsesActorAndResultQuery(t *testing.T) {
	service := &fakeReportRunQueryService{
		view: &data_svc.ReportRunViewDTO{ID: 31, Status: model.ReportRunStatusSucceeded},
		page: &data_svc.ReportResultPageDTO{},
	}
	controller := NewReportRunControllerWithService(service)
	router := reportControllerRouter()
	router.GET("/report-runs/:id", controller.Get)
	router.GET("/report-runs/:id/results", controller.Results)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/report-runs/31", nil))
	if recorder.Code != http.StatusOK || service.actor != 17 || service.runID != 31 {
		t.Fatalf("get response=%d body=%s service=%#v", recorder.Code, recorder.Body, service)
	}

	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/report-runs/31/results?cursor=signed&limit=20", nil))
	if recorder.Code != http.StatusOK || service.cursor != "signed" || service.limit != 20 {
		t.Fatalf("results response=%d body=%s service=%#v", recorder.Code, recorder.Body, service)
	}

	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/report-runs/31/results?limit=1001", nil))
	if recorder.Code != http.StatusUnprocessableEntity || service.resultCalls != 1 {
		t.Fatalf("invalid result response=%d body=%s calls=%d", recorder.Code, recorder.Body, service.resultCalls)
	}
}

func TestReportRunControllerCancellationStatusAndSafeErrors(t *testing.T) {
	service := &fakeReportRunQueryService{view: &data_svc.ReportRunViewDTO{ID: 31, Status: model.ReportRunStatusRunning, CancelRequested: true}}
	controller := NewReportRunControllerWithService(service)
	router := reportControllerRouter()
	router.POST("/report-runs/:id/cancel", controller.Cancel)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/report-runs/31/cancel", nil))
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("cancel response=%d body=%s", recorder.Code, recorder.Body)
	}

	service.err = errors.New("oracle password=secret")
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/report-runs/31/cancel", nil))
	if recorder.Code != http.StatusInternalServerError || containsSecret(recorder.Body.String()) {
		t.Fatalf("error response=%d body=%s", recorder.Code, recorder.Body)
	}
}

func containsSecret(value string) bool {
	return value != "" && (contains(value, "password") || contains(value, "secret"))
}

func contains(value, needle string) bool {
	for index := 0; index+len(needle) <= len(value); index++ {
		if value[index:index+len(needle)] == needle {
			return true
		}
	}
	return false
}

type fakeReportRunQueryService struct {
	actor       uint
	runID       uint
	cursor      string
	limit       int
	resultCalls int
	view        *data_svc.ReportRunViewDTO
	page        *data_svc.ReportResultPageDTO
	err         error
}

func (service *fakeReportRunQueryService) Get(_ context.Context, actor, runID uint) (*data_svc.ReportRunViewDTO, error) {
	service.actor, service.runID = actor, runID
	return service.view, service.err
}

func (service *fakeReportRunQueryService) Cancel(_ context.Context, actor, runID uint) (*data_svc.ReportRunViewDTO, error) {
	service.actor, service.runID = actor, runID
	return service.view, service.err
}

func (service *fakeReportRunQueryService) ReadResults(_ context.Context, actor, runID uint, cursor string, limit int) (*data_svc.ReportResultPageDTO, error) {
	service.actor, service.runID, service.cursor, service.limit = actor, runID, cursor, limit
	service.resultCalls++
	return service.page, service.err
}
