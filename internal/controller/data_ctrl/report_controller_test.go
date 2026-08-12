package data_ctrl

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"gin-biz-web-api/constant"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/internal/service/data_svc"
)

func TestReportControllerCreateUsesActorAndStrictJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &fakeReportControllerService{createResult: &data_svc.ReportDraftDTO{ID: 9}}
	controller := NewReportControllerWithService(service)
	router := reportControllerRouter()
	router.POST("/reports", controller.Create)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/reports", strings.NewReader(`{"code":"sales","unknown":true}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnprocessableEntity || service.createCalls != 0 {
		t.Fatalf("unknown field response = %d %s, calls=%d", recorder.Code, recorder.Body, service.createCalls)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/reports", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated || service.actor != 17 || service.createCalls != 1 {
		t.Fatalf("create response = %d %s actor=%d calls=%d", recorder.Code, recorder.Body, service.actor, service.createCalls)
	}
}

func TestReportControllerErrorContract(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{name: "not found", err: data_svc.ErrReportNotFound, wantStatus: http.StatusNotFound},
		{name: "conflict", err: data_svc.ErrReportConflict, wantStatus: http.StatusConflict},
		{name: "invalid", err: data_svc.ErrReportInvalid, wantStatus: http.StatusUnprocessableEntity},
		{name: "publication invalid", err: data_svc.ErrReportPublicationInvalid, wantStatus: http.StatusUnprocessableEntity},
		{name: "internal", err: errors.New("database password=secret"), wantStatus: http.StatusInternalServerError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			controller := NewReportControllerWithService(&fakeReportControllerService{getErr: test.err})
			router := reportControllerRouter()
			router.GET("/reports/:id", controller.Get)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/reports/9", nil))
			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body)
			}
			if strings.Contains(recorder.Body.String(), "password") || strings.Contains(recorder.Body.String(), "secret") {
				t.Fatalf("internal error leaked: %s", recorder.Body)
			}
		})
	}
}

func TestReportControllerListAndUpdateParseBoundaries(t *testing.T) {
	service := &fakeReportControllerService{listResult: &data_svc.ReportDraftListDTO{}, updateResult: &data_svc.ReportDraftDTO{ID: 7}}
	controller := NewReportControllerWithService(service)
	router := reportControllerRouter()
	router.GET("/reports", controller.List)
	router.PUT("/reports/:id", controller.Update)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/reports?afterId=4&limit=20&category=finance&search=sales", nil))
	if recorder.Code != http.StatusOK || service.afterID != 4 || service.limit != 20 || service.category != "finance" || service.search != "sales" {
		t.Fatalf("list response = %d %s service=%#v", recorder.Code, recorder.Body, service)
	}

	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/reports?limit=bad", nil))
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad list status = %d body=%s", recorder.Code, recorder.Body)
	}

	recorder = httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/reports/7", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || service.reportID != 7 || service.updateCalls != 1 {
		t.Fatalf("update response = %d %s id=%d calls=%d", recorder.Code, recorder.Body, service.reportID, service.updateCalls)
	}
}

func TestReportControllerPublishUsesActorAndLockVersion(t *testing.T) {
	service := &fakeReportControllerService{}
	publishService := &fakeReportPublishService{result: &data_svc.ReportPublicationDTO{DefinitionID: 7}}
	controller := NewReportControllerWithServices(service, publishService)
	router := reportControllerRouter()
	router.POST("/reports/:id/publish", controller.Publish)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/reports/7/publish", strings.NewReader(`{"expectedLockVersion":3}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || publishService.actor != 17 || publishService.reportID != 7 || publishService.lockVersion != 3 {
		t.Fatalf("publish response = %d %s service=%#v", recorder.Code, recorder.Body, publishService)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/reports/7/publish", strings.NewReader(`{"expectedLockVersion":0}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnprocessableEntity || publishService.calls != 1 {
		t.Fatalf("invalid publish response = %d %s calls=%d", recorder.Code, recorder.Body, publishService.calls)
	}
}

func TestReportControllerCreateRunUsesActorParametersAndStrictJSON(t *testing.T) {
	draftService := &fakeReportControllerService{}
	runService := &fakeReportRunService{result: &data_svc.ReportRunDTO{ID: 31}}
	controller := NewReportControllerWithAllServices(draftService, nil, runService)
	router := reportControllerRouter()
	router.POST("/reports/:id/runs", controller.CreateRun)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/reports/9/runs", strings.NewReader(`{"parameters":{"storeCode":"S001"},"refreshNonce":"refresh-1"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted || runService.actor != 17 || runService.reportID != 9 ||
		string(runService.request.Parameters["storeCode"]) != `"S001"` || runService.request.RefreshNonce != "refresh-1" {
		t.Fatalf("create run response = %d %s service=%#v", recorder.Code, recorder.Body, runService)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/reports/9/runs", strings.NewReader(`{"parameters":{},"unknown":true}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnprocessableEntity || runService.calls != 1 {
		t.Fatalf("invalid run response = %d %s calls=%d", recorder.Code, recorder.Body, runService.calls)
	}
}

func TestReportControllerCreateRunMapsDeniedAndInvalid(t *testing.T) {
	for _, test := range []struct {
		name       string
		err        error
		wantStatus int
	}{
		{name: "denied", err: data_svc.ErrReportRunDenied, wantStatus: http.StatusForbidden},
		{name: "invalid", err: data_svc.ErrReportRunInvalid, wantStatus: http.StatusUnprocessableEntity},
	} {
		t.Run(test.name, func(t *testing.T) {
			runService := &fakeReportRunService{err: test.err}
			controller := NewReportControllerWithAllServices(&fakeReportControllerService{}, nil, runService)
			router := reportControllerRouter()
			router.POST("/reports/:id/runs", controller.CreateRun)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/reports/9/runs", strings.NewReader(`{"parameters":{}}`))
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(recorder, request)
			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body)
			}
		})
	}
}

func reportControllerRouter() *gin.Engine {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(constant.CurrentUserID, "17")
		c.Next()
	})
	return router
}

type fakeReportControllerService struct {
	actor        uint
	reportID     uint
	afterID      uint
	limit        int
	category     string
	search       string
	createCalls  int
	updateCalls  int
	createResult *data_svc.ReportDraftDTO
	listResult   *data_svc.ReportDraftListDTO
	updateResult *data_svc.ReportDraftDTO
	getErr       error
}

type fakeReportPublishService struct {
	actor       uint
	reportID    uint
	lockVersion uint64
	calls       int
	result      *data_svc.ReportPublicationDTO
}

type fakeReportRunService struct {
	actor    uint
	reportID uint
	calls    int
	request  requestbody.ReportRunCreateRequest
	result   *data_svc.ReportRunDTO
	err      error
}

func (service *fakeReportRunService) Create(_ context.Context, actor, reportID uint, request requestbody.ReportRunCreateRequest) (*data_svc.ReportRunDTO, error) {
	service.actor, service.reportID, service.request = actor, reportID, request
	service.calls++
	return service.result, service.err
}

func (service *fakeReportPublishService) Publish(_ context.Context, actor, reportID uint, lockVersion uint64) (*data_svc.ReportPublicationDTO, error) {
	service.actor, service.reportID, service.lockVersion = actor, reportID, lockVersion
	service.calls++
	return service.result, nil
}

func (service *fakeReportControllerService) Create(_ context.Context, actor uint, _ requestbody.ReportDraftSaveRequest) (*data_svc.ReportDraftDTO, error) {
	service.actor = actor
	service.createCalls++
	return service.createResult, nil
}

func (service *fakeReportControllerService) Get(_ context.Context, actor, reportID uint) (*data_svc.ReportDraftDTO, error) {
	service.actor = actor
	service.reportID = reportID
	return nil, service.getErr
}

func (service *fakeReportControllerService) List(_ context.Context, actor, afterID uint, limit int, category, search string) (*data_svc.ReportDraftListDTO, error) {
	service.actor = actor
	service.afterID = afterID
	service.limit = limit
	service.category = category
	service.search = search
	return service.listResult, nil
}

func (service *fakeReportControllerService) Update(_ context.Context, actor, reportID uint, _ requestbody.ReportDraftSaveRequest) (*data_svc.ReportDraftDTO, error) {
	service.actor = actor
	service.reportID = reportID
	service.updateCalls++
	return service.updateResult, nil
}
