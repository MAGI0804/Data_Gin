package data_ctrl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gin-biz-web-api/internal/reportoracle"
	"gin-biz-web-api/internal/requestbody"
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

func TestReportInputQueryControllerDefinitionLifecycleContract(t *testing.T) {
	service := &fakeReportInputQueryService{}
	controller := NewReportInputQueryControllerWithService(service)
	router := reportControllerRouter()
	router.POST("/definitions", controller.CreateDefinition)
	router.PUT("/definitions/:id", controller.UpdateDefinition)
	router.DELETE("/definitions/:id", controller.DeleteDefinition)
	router.POST("/definition-tests", controller.TestDefinitionDraft)

	create := httptest.NewRecorder()
	router.ServeHTTP(create, httptest.NewRequest(http.MethodPost, "/definitions", strings.NewReader(`{"name":"stores","selectSql":"SELECT id, name FROM stores","enabled":true}`)))
	if create.Code != http.StatusCreated || service.createRequest.Name != "stores" || service.actor != 17 {
		t.Fatalf("create=%d %s service=%#v", create.Code, create.Body, service)
	}

	update := httptest.NewRecorder()
	router.ServeHTTP(update, httptest.NewRequest(http.MethodPut, "/definitions/7", strings.NewReader(`{"name":"stores","selectSql":"SELECT id, name FROM stores","enabled":false,"expectedLockVersion":2}`)))
	if update.Code != http.StatusOK || service.definitionID != 7 || service.updateRequest.ExpectedLockVersion != 2 {
		t.Fatalf("update=%d %s service=%#v", update.Code, update.Body, service)
	}

	deleted := httptest.NewRecorder()
	router.ServeHTTP(deleted, httptest.NewRequest(http.MethodDelete, "/definitions/7?expectedLockVersion=3", nil))
	if deleted.Code != http.StatusOK || service.deleteLockVersion != 3 {
		t.Fatalf("delete=%d %s service=%#v", deleted.Code, deleted.Body, service)
	}

	testDraft := httptest.NewRecorder()
	router.ServeHTTP(testDraft, httptest.NewRequest(http.MethodPost, "/definition-tests", strings.NewReader(`{"selectSql":"SELECT id, name FROM stores","name":"上海店"}`)))
	if testDraft.Code != http.StatusOK || service.testRequest.Name != "上海店" {
		t.Fatalf("test=%d %s service=%#v", testDraft.Code, testDraft.Body, service)
	}

	invalid := httptest.NewRecorder()
	router.ServeHTTP(invalid, httptest.NewRequest(http.MethodPost, "/definitions", strings.NewReader(`{"name":"stores","selectSql":"SELECT id, name FROM stores","enabled":true,"unknown":1}`)))
	if invalid.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid=%d %s", invalid.Code, invalid.Body)
	}
}

type fakeReportInputQueryService struct {
	options           *data_svc.ReportInputOptionListDTO
	actor             uint
	reportID          uint
	conditionCode     string
	name              string
	definitionID      uint
	createRequest     requestbody.ReportInputQueryDefinitionSaveRequest
	updateRequest     requestbody.ReportInputQueryDefinitionSaveRequest
	deleteLockVersion uint64
	testRequest       requestbody.ReportInputQueryDefinitionTestRequest
}

func (*fakeReportInputQueryService) List(context.Context, uint) (*data_svc.ReportInputQueryListDTO, error) {
	return &data_svc.ReportInputQueryListDTO{Items: []string{"stores"}}, nil
}

func (service *fakeReportInputQueryService) Options(_ context.Context, actor, reportID uint, conditionCode, name string) (*data_svc.ReportInputOptionListDTO, error) {
	service.actor, service.reportID, service.conditionCode, service.name = actor, reportID, conditionCode, name
	return service.options, nil
}

func (*fakeReportInputQueryService) ListDefinitions(context.Context, uint) (*data_svc.ReportInputQueryDefinitionListDTO, error) {
	return &data_svc.ReportInputQueryDefinitionListDTO{}, nil
}

func (*fakeReportInputQueryService) GetDefinition(context.Context, uint, uint) (*data_svc.ReportInputQueryDefinitionDTO, error) {
	return &data_svc.ReportInputQueryDefinitionDTO{}, nil
}

func (service *fakeReportInputQueryService) CreateDefinition(_ context.Context, actor uint, request requestbody.ReportInputQueryDefinitionSaveRequest) (*data_svc.ReportInputQueryDefinitionDTO, error) {
	service.actor, service.createRequest = actor, request
	return &data_svc.ReportInputQueryDefinitionDTO{ID: 7, Name: request.Name, SelectSQL: request.SelectSQL, Enabled: request.Enabled, LockVersion: 1}, nil
}

func (service *fakeReportInputQueryService) UpdateDefinition(_ context.Context, actor, definitionID uint, request requestbody.ReportInputQueryDefinitionSaveRequest) (*data_svc.ReportInputQueryDefinitionDTO, error) {
	service.actor, service.definitionID, service.updateRequest = actor, definitionID, request
	return &data_svc.ReportInputQueryDefinitionDTO{ID: definitionID, Name: request.Name, SelectSQL: request.SelectSQL, Enabled: request.Enabled, LockVersion: request.ExpectedLockVersion + 1}, nil
}

func (service *fakeReportInputQueryService) DeleteDefinition(_ context.Context, actor, definitionID uint, expected uint64) (*data_svc.ReportInputQueryDefinitionDeleteDTO, error) {
	service.actor, service.definitionID, service.deleteLockVersion = actor, definitionID, expected
	return &data_svc.ReportInputQueryDefinitionDeleteDTO{ID: definitionID}, nil
}

func (*fakeReportInputQueryService) TestDefinition(context.Context, uint, uint, requestbody.ReportInputQueryDefinitionTestRequest) (*data_svc.ReportInputQueryTestDTO, error) {
	return &data_svc.ReportInputQueryTestDTO{}, nil
}

func (service *fakeReportInputQueryService) TestDefinitionDraft(_ context.Context, actor uint, request requestbody.ReportInputQueryDefinitionTestRequest) (*data_svc.ReportInputQueryTestDTO, error) {
	service.actor, service.testRequest = actor, request
	return &data_svc.ReportInputQueryTestDTO{Status: "SUCCESS", Items: []reportoracle.InputOption{}}, nil
}

func containsAll(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if !strings.Contains(value, candidate) {
			return false
		}
	}
	return true
}
