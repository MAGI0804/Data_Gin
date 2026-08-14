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
	"gin-biz-web-api/internal/reportoracle"
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

func TestReportDatasourceControllerTestsConnectionDraft(t *testing.T) {
	service := &fakeReportDatasourceService{connectionTest: &data_svc.ReportDatasourceTestDTO{Status: "SUCCESS", Message: "Oracle 连接测试成功"}}
	controller := NewReportDatasourceControllerWithService(service)
	router := datasourceControllerRouter()
	router.POST("/report-datasource-connection-tests", controller.TestConnection)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/report-datasource-connection-tests", strings.NewReader(`{"host":"oracle.internal","port":1521,"serviceName":"REPORT","username":"report_user","password":"secret"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || service.connectionTestCalls != 1 || service.connectionRequest.Password != "secret" || service.actor != 17 {
		t.Fatalf("connection test response=%d %s service=%#v", recorder.Code, recorder.Body, service)
	}
	if strings.Contains(recorder.Body.String(), "secret") {
		t.Fatalf("connection test response leaked password: %s", recorder.Body)
	}
}

func TestReportDatasourceControllerMapsCredentialConfigurationFailure(t *testing.T) {
	code, message := classifyReportDatasourceError(data_svc.ErrReportDatasourceCredentialUnavailable)
	if code.HttpStatusCode() != http.StatusServiceUnavailable || !strings.Contains(message, "凭据加密配置") {
		t.Fatalf("classifyReportDatasourceError() = %d, %q", code.HttpStatusCode(), message)
	}
}

func TestWriteReportDatasourceErrorRecordsPrivateCauseWithoutLeakingResponse(t *testing.T) {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	writeReportDatasourceError(context, errors.New("database password=secret unavailable"))
	if recorder.Code != http.StatusInternalServerError || len(context.Errors.ByType(gin.ErrorTypePrivate)) != 1 {
		t.Fatalf("response=%d errors=%v", recorder.Code, context.Errors)
	}
	if strings.Contains(recorder.Body.String(), "password") || strings.Contains(recorder.Body.String(), "secret") {
		t.Fatalf("response leaked private cause: %s", recorder.Body)
	}
}

func TestReportDatasourceControllerQueriesProcedureCatalogAndSignature(t *testing.T) {
	service := &fakeReportDatasourceService{}
	controller := NewReportDatasourceControllerWithService(service)
	router := datasourceControllerRouter()
	router.GET("/report-datasources/:id/procedures", controller.ListProcedures)
	router.GET("/report-datasources/:id/procedure-signature", controller.GetProcedureSignature)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/report-datasources/9/procedures?owner=REPORT&search=DAILY&after=cursor&limit=25", nil)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || service.datasourceID != 9 || service.procedureQuery.Owner != "REPORT" || service.procedureQuery.Search != "DAILY" || service.procedureQuery.After != "cursor" || service.procedureQuery.Limit != 25 {
		t.Fatalf("catalog response=%d %s service=%+v", recorder.Code, recorder.Body, service)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/report-datasources/9/procedure-signature?owner=REPORT&package=PKG_SALES&name=BUILD_DAILY&overload=2", nil)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || service.procedureRef.Owner != "REPORT" || service.procedureRef.Package != "PKG_SALES" || service.procedureRef.Name != "BUILD_DAILY" || service.procedureRef.Overload != "2" {
		t.Fatalf("signature response=%d %s ref=%+v", recorder.Code, recorder.Body, service.procedureRef)
	}
}

func datasourceControllerRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set(constant.CurrentUserID, "17"); c.Next() })
	return router
}

type fakeReportDatasourceService struct {
	actor               uint
	request             requestbody.ReportDatasourceSaveRequest
	createCalls         int
	created             *data_svc.ReportDatasourceDTO
	connectionTest      *data_svc.ReportDatasourceTestDTO
	connectionRequest   requestbody.ReportDatasourceConnectionTestRequest
	connectionTestCalls int
	datasourceID        uint
	procedureQuery      data_svc.ReportProcedureCatalogQuery
	procedureRef        reportoracle.ProcedureRef
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
func (service *fakeReportDatasourceService) TestConnection(_ context.Context, actor uint, request requestbody.ReportDatasourceConnectionTestRequest) (*data_svc.ReportDatasourceTestDTO, error) {
	service.actor, service.connectionRequest = actor, request
	service.connectionTestCalls++
	return service.connectionTest, nil
}
func (service *fakeReportDatasourceService) ListProcedures(_ context.Context, _ uint, datasourceID uint, query data_svc.ReportProcedureCatalogQuery) (*data_svc.ReportProcedurePageDTO, error) {
	service.datasourceID, service.procedureQuery = datasourceID, query
	return &data_svc.ReportProcedurePageDTO{}, nil
}
func (service *fakeReportDatasourceService) GetProcedureSignature(_ context.Context, _ uint, datasourceID uint, ref reportoracle.ProcedureRef) (*data_svc.ReportProcedureSignatureDTO, error) {
	service.datasourceID, service.procedureRef = datasourceID, ref
	return &data_svc.ReportProcedureSignatureDTO{}, nil
}
