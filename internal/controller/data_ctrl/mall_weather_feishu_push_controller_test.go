package data_ctrl

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gin-biz-web-api/constant"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/internal/service/data_svc"

	"github.com/gin-gonic/gin"
)

func TestMallWeatherFeishuPushControllerReturnsDryRunPlan(t *testing.T) {
	service := fakeMallWeatherFeishuPushControllerService{dryRun: func(
		_ context.Context,
		actor uint,
		request requestbody.MallWeatherFeishuPushRequest,
	) (*data_svc.MallWeatherFeishuDryRunResult, error) {
		if actor != 17 || request.DestinationID != 8 || request.ProfileID != 9 ||
			request.ExpectedProfileVersion == nil || *request.ExpectedProfileVersion != 3 {
			t.Fatalf("actor=%d request=%+v", actor, request)
		}
		return &data_svc.MallWeatherFeishuDryRunResult{
			DestinationID: 8, ProfileID: 9, ProfileVersion: 3, CanExecute: true,
			Datasets: []data_svc.MallWeatherFeishuDatasetDryRunPlan{}, Warnings: []string{},
		}, nil
	}}
	recorder := performMallWeatherFeishuPushRequest(
		t,
		service,
		`{"destinationId":8,"profileId":9,"expectedProfileVersion":3}`,
	)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"canExecute":true`) {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestMallWeatherFeishuPushControllerCreatesIdempotentRun(t *testing.T) {
	service := fakeMallWeatherFeishuPushControllerService{create: func(
		_ context.Context,
		actor uint,
		key string,
		request requestbody.MallWeatherFeishuPushRequest,
	) (*data_svc.MallWeatherFeishuPushCreateResult, bool, error) {
		if actor != 17 || key != "feishu-request-1234" || request.DestinationID != 8 || request.ProfileID != 9 {
			t.Fatalf("actor=%d key=%q request=%+v", actor, key, request)
		}
		return &data_svc.MallWeatherFeishuPushCreateResult{
			RunID: 41, Status: "PENDING", DestinationID: 8, ProfileID: 9, ProfileVersion: 3,
		}, true, nil
	}}
	recorder := performMallWeatherFeishuPushRouteRequest(
		t,
		service,
		http.MethodPost,
		"/api/v1/weather-sheet-pushes",
		`{"destinationId":8,"profileId":9}`,
		"feishu-request-1234",
	)
	if recorder.Code != http.StatusAccepted || recorder.Header().Get("Idempotency-Replayed") != "true" ||
		!strings.Contains(recorder.Body.String(), `"runId":41`) ||
		strings.Contains(recorder.Body.String(), "feishu-request-1234") {
		t.Fatalf("status=%d headers=%v body=%s", recorder.Code, recorder.Header(), recorder.Body.String())
	}
}

func TestMallWeatherFeishuPushControllerGetsRunAndRejectsInvalidID(t *testing.T) {
	calls := 0
	service := fakeMallWeatherFeishuPushControllerService{get: func(
		_ context.Context,
		actor uint,
		runID uint,
	) (*data_svc.MallWeatherFeishuPushRunDTO, error) {
		calls++
		if actor != 17 || runID != 41 {
			t.Fatalf("actor=%d runID=%d", actor, runID)
		}
		return &data_svc.MallWeatherFeishuPushRunDTO{RunID: runID, Status: "RUNNING"}, nil
	}}
	recorder := performMallWeatherFeishuPushRouteRequest(
		t, service, http.MethodGet, "/api/v1/weather-sheet-pushes/41", "", "",
	)
	if recorder.Code != http.StatusOK || calls != 1 || !strings.Contains(recorder.Body.String(), `"status":"RUNNING"`) {
		t.Fatalf("status=%d calls=%d body=%s", recorder.Code, calls, recorder.Body.String())
	}
	recorder = performMallWeatherFeishuPushRouteRequest(
		t, service, http.MethodGet, "/api/v1/weather-sheet-pushes/not-a-number", "", "",
	)
	if recorder.Code != http.StatusUnprocessableEntity || calls != 1 {
		t.Fatalf("invalid status=%d calls=%d body=%s", recorder.Code, calls, recorder.Body.String())
	}
}

func TestMallWeatherFeishuPushControllerReportsDisabledFeature(t *testing.T) {
	calls := 0
	service := fakeMallWeatherFeishuPushControllerService{dryRun: func(
		context.Context,
		uint,
		requestbody.MallWeatherFeishuPushRequest,
	) (*data_svc.MallWeatherFeishuDryRunResult, error) {
		calls++
		return nil, data_svc.ErrMallWeatherFeishuDisabled
	}}
	recorder := performMallWeatherFeishuPushRequest(t, service, `{"destinationId":8,"profileId":9}`)
	if recorder.Code != http.StatusForbidden || calls != 1 || strings.Contains(recorder.Body.String(), "disabled") {
		t.Fatalf("status=%d calls=%d body=%s", recorder.Code, calls, recorder.Body.String())
	}
}

func TestMallWeatherFeishuPushControllerRejectsUnknownFieldsAndLeaksNoErrors(t *testing.T) {
	calls := 0
	service := fakeMallWeatherFeishuPushControllerService{dryRun: func(
		context.Context,
		uint,
		requestbody.MallWeatherFeishuPushRequest,
	) (*data_svc.MallWeatherFeishuDryRunResult, error) {
		calls++
		return nil, errors.New("token=must-not-leak")
	}}
	recorder := performMallWeatherFeishuPushRequest(t, service, `{"destinationId":8,"profileId":9,"secret":"x"}`)
	if recorder.Code != http.StatusUnprocessableEntity || calls != 0 || strings.Contains(recorder.Body.String(), "secret") {
		t.Fatalf("status=%d calls=%d body=%s", recorder.Code, calls, recorder.Body.String())
	}
	recorder = performMallWeatherFeishuPushRequest(t, service, `{"destinationId":8,"profileId":9}`)
	if recorder.Code != http.StatusInternalServerError || calls != 1 ||
		strings.Contains(recorder.Body.String(), "token") || strings.Contains(recorder.Body.String(), "must-not-leak") {
		t.Fatalf("status=%d calls=%d body=%s", recorder.Code, calls, recorder.Body.String())
	}
}

func performMallWeatherFeishuPushRequest(
	t *testing.T,
	service MallWeatherFeishuPushServiceAPI,
	body string,
) *httptest.ResponseRecorder {
	return performMallWeatherFeishuPushRouteRequest(
		t,
		service,
		http.MethodPost,
		"/api/v1/weather-sheet-pushes/dry-run",
		body,
		"",
	)
}

func performMallWeatherFeishuPushRouteRequest(
	t *testing.T,
	service MallWeatherFeishuPushServiceAPI,
	method string,
	path string,
	body string,
	idempotencyKey string,
) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(constant.CurrentUserID, "17")
		c.Next()
	})
	controller := NewMallWeatherFeishuPushControllerWithService(service)
	router.POST("/api/v1/weather-sheet-pushes", controller.Create)
	router.GET("/api/v1/weather-sheet-pushes/:run_id", controller.Get)
	router.POST("/api/v1/weather-sheet-pushes/dry-run", controller.DryRun)
	request := httptest.NewRequest(
		method,
		path,
		strings.NewReader(body),
	)
	request.Header.Set("Content-Type", "application/json")
	if idempotencyKey != "" {
		request.Header.Set("Idempotency-Key", idempotencyKey)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

type fakeMallWeatherFeishuPushControllerService struct {
	create func(
		context.Context,
		uint,
		string,
		requestbody.MallWeatherFeishuPushRequest,
	) (*data_svc.MallWeatherFeishuPushCreateResult, bool, error)
	get    func(context.Context, uint, uint) (*data_svc.MallWeatherFeishuPushRunDTO, error)
	dryRun func(
		context.Context,
		uint,
		requestbody.MallWeatherFeishuPushRequest,
	) (*data_svc.MallWeatherFeishuDryRunResult, error)
}

func (service fakeMallWeatherFeishuPushControllerService) Create(
	ctx context.Context,
	actor uint,
	key string,
	request requestbody.MallWeatherFeishuPushRequest,
) (*data_svc.MallWeatherFeishuPushCreateResult, bool, error) {
	if service.create == nil {
		panic("unexpected Create call")
	}
	return service.create(ctx, actor, key, request)
}

func (service fakeMallWeatherFeishuPushControllerService) Get(
	ctx context.Context,
	actor uint,
	runID uint,
) (*data_svc.MallWeatherFeishuPushRunDTO, error) {
	if service.get == nil {
		panic("unexpected Get call")
	}
	return service.get(ctx, actor, runID)
}

func (service fakeMallWeatherFeishuPushControllerService) DryRun(
	ctx context.Context,
	actor uint,
	request requestbody.MallWeatherFeishuPushRequest,
) (*data_svc.MallWeatherFeishuDryRunResult, error) {
	return service.dryRun(ctx, actor, request)
}
