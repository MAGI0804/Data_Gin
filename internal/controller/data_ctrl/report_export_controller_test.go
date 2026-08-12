package data_ctrl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"gin-biz-web-api/constant"
	"gin-biz-web-api/internal/reportquery"
	"gin-biz-web-api/internal/service/data_svc"
	"gin-biz-web-api/model"

	"github.com/gin-gonic/gin"
)

func TestReportExportControllerCreateAndGet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &fakeReportExportControllerService{}
	controller := NewReportExportControllerWithServices(service, service)
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set(constant.CurrentUserID, "17") })
	router.POST("/runs/:id/export", controller.Create)
	router.GET("/exports/:id", controller.Get)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/runs/31/export", nil)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted || service.actor != 17 || service.runID != 31 {
		t.Fatalf("create status=%d service=%#v body=%s", recorder.Code, service, recorder.Body.String())
	}
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/exports/41", nil)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || service.exportID != 41 || recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("get status=%d service=%#v headers=%v", recorder.Code, service, recorder.Header())
	}
}

func TestReportExportControllerListValidatesAndPassesCursor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &fakeReportExportControllerService{}
	controller := NewReportExportControllerWithServices(service, service)
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set(constant.CurrentUserID, "17") })
	router.GET("/exports", controller.List)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/exports?afterId=55&limit=20&status=READY", nil)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || service.actor != 17 || service.afterID != 55 || service.limit != 20 || service.status != "READY" || recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("list status=%d service=%#v headers=%v body=%s", recorder.Code, service, recorder.Header(), recorder.Body)
	}

	for _, target := range []string{"/exports?afterId=0", "/exports?limit=101"} {
		recorder = httptest.NewRecorder()
		request = httptest.NewRequest(http.MethodGet, target, nil)
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusUnprocessableEntity {
			t.Fatalf("invalid list %s status=%d body=%s", target, recorder.Code, recorder.Body)
		}
	}
}

type fakeReportExportControllerService struct {
	actor    uint
	runID    uint
	exportID uint
	afterID  uint
	limit    int
	status   string
}

func (service *fakeReportExportControllerService) Create(_ context.Context, actor, runID uint, _ reportquery.Input) (*data_svc.ReportExportDTO, bool, error) {
	service.actor, service.runID = actor, runID
	return &data_svc.ReportExportDTO{ID: 41, RunID: runID, Status: model.ReportExportStatusPending}, false, nil
}

func (service *fakeReportExportControllerService) Get(_ context.Context, actor, exportID uint) (*data_svc.ReportExportViewDTO, error) {
	service.actor, service.exportID = actor, exportID
	return &data_svc.ReportExportViewDTO{ID: exportID, Status: model.ReportExportStatusPending}, nil
}

func (service *fakeReportExportControllerService) List(_ context.Context, actor, afterID uint, limit int, status string) (*data_svc.ReportExportListDTO, error) {
	service.actor, service.afterID, service.limit, service.status = actor, afterID, limit, status
	return &data_svc.ReportExportListDTO{}, nil
}

func (*fakeReportExportControllerService) Download(context.Context, uint, uint) (*data_svc.ReportExportDownloadDTO, error) {
	return &data_svc.ReportExportDownloadDTO{}, nil
}
