package data_ctrl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"gin-biz-web-api/constant"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/internal/service/data_svc"
)

func TestReportDatasourceControllerUsesStrictJSONAndRedactedResponse(t *testing.T) {
	service := &fakeReportDatasourceService{created: &data_svc.ReportDatasourceDTO{ID: 7, Code: "report_oracle", Name: "报表库", Driver: "ORACLE", HasPassword: true}}
	controller := NewReportDatasourceControllerWithService(service)
	router := datasourceControllerRouter()
	router.POST("/report-datasources", controller.Create)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/report-datasources", strings.NewReader(`{"code":"report_oracle","unknown":"secret"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnprocessableEntity || service.createCalls != 0 {
		t.Fatalf("unknown field response = %d %s calls=%d", recorder.Code, recorder.Body, service.createCalls)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/report-datasources", strings.NewReader(`{"code":"report_oracle","password":"secret-password"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated || service.actor != 17 || service.request.Password != "secret-password" {
		t.Fatalf("create response = %d %s service=%#v", recorder.Code, recorder.Body, service)
	}
	if strings.Contains(recorder.Body.String(), "secret-password") || strings.Contains(recorder.Body.String(), "ciphertext") || strings.Contains(recorder.Body.String(), "key-v1") {
		t.Fatalf("response leaked credentials: %s", recorder.Body)
	}
}

func datasourceControllerRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set(constant.CurrentUserID, "17"); c.Next() })
	return router
}

type fakeReportDatasourceService struct {
	actor       uint
	request     requestbody.ReportDatasourceSaveRequest
	createCalls int
	created     *data_svc.ReportDatasourceDTO
}

func (*fakeReportDatasourceService) List(context.Context, uint) ([]data_svc.ReportDatasourceDTO, error) {
	return nil, nil
}
func (*fakeReportDatasourceService) Get(context.Context, uint, uint) (*data_svc.ReportDatasourceDTO, error) {
	return nil, nil
}
func (service *fakeReportDatasourceService) Create(_ context.Context, actor uint, request requestbody.ReportDatasourceSaveRequest) (*data_svc.ReportDatasourceDTO, error) {
	service.actor, service.request = actor, request
	service.createCalls++
	return service.created, nil
}
func (*fakeReportDatasourceService) Update(context.Context, uint, uint, requestbody.ReportDatasourceSaveRequest) (*data_svc.ReportDatasourceDTO, error) {
	return nil, nil
}
func (*fakeReportDatasourceService) Test(context.Context, uint, uint) (*data_svc.ReportDatasourceTestDTO, error) {
	return nil, nil
}
