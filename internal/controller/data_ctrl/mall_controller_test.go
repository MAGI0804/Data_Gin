package data_ctrl

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"gin-biz-web-api/constant"
	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/internal/service/data_svc"

	"github.com/gin-gonic/gin"
)

func TestMallControllerCreateUsesJWTActorAndIdempotencyContract(t *testing.T) {
	service := &fakeMallService{
		create: func(_ context.Context, actorUserID uint, idempotencyKey string, request requestbody.MallCreateRequest) (*data_svc.MallCreateResult, bool, error) {
			if actorUserID != 17 || idempotencyKey != "84c2e4a0-1234-4567-8901-123456789012" {
				t.Fatalf("actor=%d idempotencyKey=%q", actorUserID, idempotencyKey)
			}
			if request.MallCode != "SH-001" || request.NameCN != "示例商场" {
				t.Fatalf("request = %+v", request)
			}
			return &data_svc.MallCreateResult{ID: 7, MallCode: "SH-001", CreatedAt: time.Unix(1, 0)}, true, nil
		},
	}
	recorder := performMallRequest(t, service, http.MethodPost, "/api/v1/malls", `{"mallCode":"SH-001","nameCn":"示例商场"}`, map[string]string{
		"Idempotency-Key": "84c2e4a0-1234-4567-8901-123456789012",
	})

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("Idempotency-Replayed") != "true" {
		t.Fatal("missing idempotency replay header")
	}
	if strings.Contains(recorder.Body.String(), "84c2e4a0") {
		t.Fatalf("response leaked idempotency key: %s", recorder.Body.String())
	}
}

func TestMallControllerRejectsUnsafeJSONBeforeService(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "unknown field", body: `{"mallCode":"SH-001","unknown":true}`},
		{name: "trailing value", body: `{"mallCode":"SH-001"} {}`},
		{name: "empty body", body: ``},
		{name: "oversized body", body: `{"mallCode":"` + strings.Repeat("a", int(maxMallRequestBodyBytes)) + `"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &fakeMallService{create: func(context.Context, uint, string, requestbody.MallCreateRequest) (*data_svc.MallCreateResult, bool, error) {
				t.Fatal("service called for invalid JSON")
				return nil, false, nil
			}}
			recorder := performMallRequest(t, service, http.MethodPost, "/api/v1/malls", test.body, nil)
			if recorder.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestMallControllerImportReturnsPerRowResult(t *testing.T) {
	service := &fakeMallService{importItems: func(_ context.Context, actor uint, key string, items []requestbody.MallCreateRequest) (*data_svc.MallImportResult, error) {
		if actor != 17 || key != "84c2e4a0-1234-4567-8901-123456789012" || len(items) != 2 {
			t.Fatalf("actor=%d key=%q items=%+v", actor, key, items)
		}
		return &data_svc.MallImportResult{
			Rows: []data_svc.MallImportRowResult{
				{Row: 1, Status: "CREATED", ReviewStatus: "PENDING_GEOCODE", Mall: &data_svc.MallCreateResult{ID: 1}},
				{Row: 2, Status: "FAILED", ErrorCode: "INVALID_INPUT"},
			},
			Created: 1,
			Failed:  1,
		}, nil
	}}
	recorder := performMallRequest(t, service, http.MethodPost, "/api/v1/malls/import", `{"items":[{"mallCode":"SH-001"},{"mallCode":"bad"}]}`, map[string]string{
		"Idempotency-Key": "84c2e4a0-1234-4567-8901-123456789012",
	})
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "PENDING_GEOCODE") || !strings.Contains(recorder.Body.String(), "INVALID_INPUT") {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestMallControllerMapsServiceErrorsWithoutLeakingDetails(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantStatus  int
		wantMessage string
	}{
		{name: "forbidden", err: data_svc.ErrMallForbidden, wantStatus: http.StatusForbidden},
		{name: "not found", err: data_dao.ErrMallNotFound, wantStatus: http.StatusNotFound},
		{name: "candidate not found", err: data_dao.ErrMallGeocodeCandidateNotFound, wantStatus: http.StatusNotFound},
		{name: "version conflict", err: data_dao.ErrMallVersionConflict, wantStatus: http.StatusConflict},
		{name: "invalid", err: data_svc.ErrMallInvalidInput, wantStatus: http.StatusUnprocessableEntity},
		{name: "weather disabled", err: errors.Join(errors.New("internal detail"), data_svc.ErrMallWeatherDisabled), wantStatus: http.StatusServiceUnavailable, wantMessage: "商场天气服务未启用，请联系管理员完成配置后重试"},
		{name: "internal", err: errors.New("database password=secret unavailable"), wantStatus: http.StatusInternalServerError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &fakeMallService{get: func(context.Context, uint, uint) (*data_svc.MallDTO, error) {
				return nil, test.err
			}}
			recorder := performMallRequest(t, service, http.MethodGet, "/api/v1/malls/7", "", nil)
			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
			}
			if strings.Contains(recorder.Body.String(), "password") || strings.Contains(recorder.Body.String(), "secret") {
				t.Fatalf("response leaked internal error: %s", recorder.Body.String())
			}
			if test.wantMessage != "" && !strings.Contains(recorder.Body.String(), test.wantMessage) {
				t.Fatalf("response missing safe message %q: %s", test.wantMessage, recorder.Body.String())
			}
			if test.wantStatus == http.StatusServiceUnavailable && !strings.Contains(recorder.Body.String(), `"code":100503`) {
				t.Fatalf("response missing service unavailable code: %s", recorder.Body.String())
			}
		})
	}
}

func TestMallControllerListParsesFilters(t *testing.T) {
	service := &fakeMallService{list: func(_ context.Context, actorUserID uint, request requestbody.MallListRequest) (*data_svc.MallListResult, error) {
		if actorUserID != 17 || request.AfterID != 8 || request.Limit != 25 || request.City != "上海" || request.Status != "active" || request.GeocodeStatus != "confirmed" {
			t.Fatalf("request = %+v actor=%d", request, actorUserID)
		}
		if request.WeatherEnabled == nil || !*request.WeatherEnabled {
			t.Fatalf("weatherEnabled = %v", request.WeatherEnabled)
		}
		return &data_svc.MallListResult{Items: []data_svc.MallDTO{{ID: 9}}}, nil
	}}
	recorder := performMallRequest(t, service, http.MethodGet, "/api/v1/malls?afterId=8&limit=25&city=%E4%B8%8A%E6%B5%B7&status=active&geocodeStatus=confirmed&weatherEnabled=true", "", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestMallControllerUpdateAndDeleteContracts(t *testing.T) {
	service := &fakeMallService{
		update: func(_ context.Context, actorUserID, mallID uint, request requestbody.MallPatchRequest) (*data_svc.MallDTO, error) {
			if actorUserID != 17 || mallID != 9 || request.ExpectedMallVersion != 4 || request.NameCN == nil || *request.NameCN != "新名称" {
				t.Fatalf("actor=%d mall=%d request=%+v", actorUserID, mallID, request)
			}
			return &data_svc.MallDTO{ID: mallID, Version: 5}, nil
		},
		delete: func(_ context.Context, actorUserID, mallID uint, expectedVersion uint64) error {
			if actorUserID != 17 || mallID != 9 || expectedVersion != 5 {
				t.Fatalf("actor=%d mall=%d version=%d", actorUserID, mallID, expectedVersion)
			}
			return nil
		},
	}
	updateRecorder := performMallRequest(t, service, http.MethodPatch, "/api/v1/malls/9", `{"expectedMallVersion":4,"nameCn":"新名称"}`, nil)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("update status = %d body=%s", updateRecorder.Code, updateRecorder.Body.String())
	}
	deleteRecorder := performMallRequest(t, service, http.MethodDelete, "/api/v1/malls/9?expectedMallVersion=5", "", nil)
	if deleteRecorder.Code != http.StatusNoContent || deleteRecorder.Body.Len() != 0 {
		t.Fatalf("delete status = %d body=%s", deleteRecorder.Code, deleteRecorder.Body.String())
	}
}

func TestMallControllerRejectsInvalidPathAndQueryValues(t *testing.T) {
	service := &fakeMallService{}
	requests := []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/api/v1/malls/0"},
		{method: http.MethodGet, path: "/api/v1/malls?limit=-1"},
		{method: http.MethodGet, path: "/api/v1/malls?weatherEnabled=sometimes"},
		{method: http.MethodDelete, path: "/api/v1/malls/9?expectedMallVersion=0"},
	}
	for _, request := range requests {
		t.Run(request.method+request.path, func(t *testing.T) {
			recorder := performMallRequest(t, service, request.method, request.path, "", nil)
			if recorder.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestMallControllerGeocodeContractsUseJWTActor(t *testing.T) {
	candidateID := uint(987)
	service := &fakeMallService{
		triggerGeocode: func(_ context.Context, actor, mallID uint, version uint64) (*data_svc.MallGeocodeTriggerResult, error) {
			if actor != 17 || mallID != 9 || version != 4 {
				t.Fatalf("trigger actor=%d mall=%d version=%d", actor, mallID, version)
			}
			return &data_svc.MallGeocodeTriggerResult{JobID: 12, MallID: mallID, MallVersion: 5}, nil
		},
		listGeocode: func(_ context.Context, actor, mallID uint) (*data_svc.MallGeocodeCandidatesResult, error) {
			if actor != 17 || mallID != 9 {
				t.Fatalf("list actor=%d mall=%d", actor, mallID)
			}
			return &data_svc.MallGeocodeCandidatesResult{MallID: mallID, RunID: 8}, nil
		},
		confirmGeocode: func(_ context.Context, actor, mallID uint, request requestbody.MallGeocodeConfirmRequest) (*data_svc.MallDTO, error) {
			if actor != 17 || mallID != 9 || request.CandidateID == nil || *request.CandidateID != candidateID || request.ExpectedMallVersion != 5 || !request.WeatherEnabled {
				t.Fatalf("confirm actor=%d mall=%d request=%+v", actor, mallID, request)
			}
			return &data_svc.MallDTO{ID: mallID, Version: 6}, nil
		},
	}

	trigger := performMallRequest(t, service, http.MethodPost, "/api/v1/malls/9/geocode", `{"expectedMallVersion":4}`, nil)
	if trigger.Code != http.StatusAccepted {
		t.Fatalf("trigger status=%d body=%s", trigger.Code, trigger.Body.String())
	}
	list := performMallRequest(t, service, http.MethodGet, "/api/v1/malls/9/geocode-candidates", "", nil)
	if list.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}
	confirm := performMallRequest(t, service, http.MethodPost, "/api/v1/malls/9/geocode-confirm", `{"candidateId":987,"expectedMallVersion":5,"weatherEnabled":true}`, nil)
	if confirm.Code != http.StatusOK {
		t.Fatalf("confirm status=%d body=%s", confirm.Code, confirm.Body.String())
	}
}

func TestMallControllerGeocodeConfirmRejectsClientAuditFields(t *testing.T) {
	service := &fakeMallService{confirmGeocode: func(context.Context, uint, uint, requestbody.MallGeocodeConfirmRequest) (*data_svc.MallDTO, error) {
		t.Fatal("service called for client-supplied audit field")
		return nil, nil
	}}
	recorder := performMallRequest(t, service, http.MethodPost, "/api/v1/malls/9/geocode-confirm", `{"candidateId":987,"expectedMallVersion":5,"weatherEnabled":true,"confirmedBy":99}`, nil)
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func performMallRequest(t *testing.T, service MallService, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(constant.CurrentUserID, strconv.FormatUint(17, 10))
		c.Next()
	})
	controller := NewMallControllerWithService(service)
	group := router.Group("/api/v1/malls")
	group.POST("", controller.Create)
	group.POST("/import", controller.Import)
	group.GET("", controller.List)
	group.GET("/:id", controller.Get)
	group.PATCH("/:id", controller.Update)
	group.DELETE("/:id", controller.Delete)
	group.POST("/:id/geocode", controller.TriggerGeocode)
	group.GET("/:id/geocode-candidates", controller.ListGeocodeCandidates)
	group.POST("/:id/geocode-confirm", controller.ConfirmGeocode)

	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

type fakeMallService struct {
	create         func(context.Context, uint, string, requestbody.MallCreateRequest) (*data_svc.MallCreateResult, bool, error)
	importItems    func(context.Context, uint, string, []requestbody.MallCreateRequest) (*data_svc.MallImportResult, error)
	get            func(context.Context, uint, uint) (*data_svc.MallDTO, error)
	list           func(context.Context, uint, requestbody.MallListRequest) (*data_svc.MallListResult, error)
	update         func(context.Context, uint, uint, requestbody.MallPatchRequest) (*data_svc.MallDTO, error)
	delete         func(context.Context, uint, uint, uint64) error
	triggerGeocode func(context.Context, uint, uint, uint64) (*data_svc.MallGeocodeTriggerResult, error)
	listGeocode    func(context.Context, uint, uint) (*data_svc.MallGeocodeCandidatesResult, error)
	confirmGeocode func(context.Context, uint, uint, requestbody.MallGeocodeConfirmRequest) (*data_svc.MallDTO, error)
}

func (service *fakeMallService) Import(ctx context.Context, actor uint, key string, items []requestbody.MallCreateRequest) (*data_svc.MallImportResult, error) {
	if service.importItems == nil {
		panic("unexpected Import call")
	}
	return service.importItems(ctx, actor, key, items)
}

func (service *fakeMallService) Create(ctx context.Context, actor uint, key string, request requestbody.MallCreateRequest) (*data_svc.MallCreateResult, bool, error) {
	if service.create == nil {
		panic("unexpected Create call")
	}
	return service.create(ctx, actor, key, request)
}

func (service *fakeMallService) Get(ctx context.Context, actor, mallID uint) (*data_svc.MallDTO, error) {
	if service.get == nil {
		panic("unexpected Get call")
	}
	return service.get(ctx, actor, mallID)
}

func (service *fakeMallService) List(ctx context.Context, actor uint, request requestbody.MallListRequest) (*data_svc.MallListResult, error) {
	if service.list == nil {
		panic("unexpected List call")
	}
	return service.list(ctx, actor, request)
}

func (service *fakeMallService) Update(ctx context.Context, actor, mallID uint, request requestbody.MallPatchRequest) (*data_svc.MallDTO, error) {
	if service.update == nil {
		panic("unexpected Update call")
	}
	return service.update(ctx, actor, mallID, request)
}

func (service *fakeMallService) Delete(ctx context.Context, actor, mallID uint, version uint64) error {
	if service.delete == nil {
		panic("unexpected Delete call")
	}
	return service.delete(ctx, actor, mallID, version)
}

func (service *fakeMallService) TriggerGeocode(ctx context.Context, actor, mallID uint, version uint64) (*data_svc.MallGeocodeTriggerResult, error) {
	if service.triggerGeocode == nil {
		panic("unexpected TriggerGeocode call")
	}
	return service.triggerGeocode(ctx, actor, mallID, version)
}

func (service *fakeMallService) ListGeocodeCandidates(ctx context.Context, actor, mallID uint) (*data_svc.MallGeocodeCandidatesResult, error) {
	if service.listGeocode == nil {
		panic("unexpected ListGeocodeCandidates call")
	}
	return service.listGeocode(ctx, actor, mallID)
}

func (service *fakeMallService) ConfirmGeocode(ctx context.Context, actor, mallID uint, request requestbody.MallGeocodeConfirmRequest) (*data_svc.MallDTO, error) {
	if service.confirmGeocode == nil {
		panic("unexpected ConfirmGeocode call")
	}
	return service.confirmGeocode(ctx, actor, mallID, request)
}
