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

func TestMallWeatherRefreshControllerReturnsAcceptedAndReplayHeader(t *testing.T) {
	var gotActor, gotMall uint
	var gotKey string
	var gotRequest requestbody.MallWeatherRefreshRequest
	service := fakeMallWeatherRefreshControllerService{refresh: func(_ context.Context, actor, mallID uint, key string, request requestbody.MallWeatherRefreshRequest) (*data_svc.MallWeatherRefreshResult, bool, error) {
		gotActor, gotMall, gotKey, gotRequest = actor, mallID, key, request
		return &data_svc.MallWeatherRefreshResult{JobID: 31, MallID: mallID, CorrelationID: "manual:opaque-123"}, true, nil
	}}
	recorder := performMallWeatherRefreshRequest(t, service, "7", `{"kinds":["V26_FULL","V3_LIFE_INDEX"],"force":false,"reason":"operator review"}`, "refresh-key-1234")
	if recorder.Code != http.StatusAccepted || recorder.Header().Get("Idempotency-Replayed") != "true" ||
		gotActor != 17 || gotMall != 7 || gotKey != "refresh-key-1234" || len(gotRequest.Kinds) != 2 ||
		!strings.Contains(recorder.Body.String(), `"jobId":31`) ||
		!strings.Contains(recorder.Body.String(), `"correlationId":"manual:opaque-123"`) {
		t.Fatalf("status=%d headers=%v actor=%d mall=%d key=%q request=%+v body=%s", recorder.Code, recorder.Header(), gotActor, gotMall, gotKey, gotRequest, recorder.Body.String())
	}
}

func TestMallWeatherRefreshControllerRejectsUnknownJSONFields(t *testing.T) {
	calls := 0
	service := fakeMallWeatherRefreshControllerService{refresh: func(context.Context, uint, uint, string, requestbody.MallWeatherRefreshRequest) (*data_svc.MallWeatherRefreshResult, bool, error) {
		calls++
		return nil, false, nil
	}}
	recorder := performMallWeatherRefreshRequest(t, service, "7", `{"kinds":["V26_FULL"],"force":false,"reason":"review","secret":"x"}`, "refresh-key-1234")
	if recorder.Code != http.StatusUnprocessableEntity || calls != 0 || strings.Contains(recorder.Body.String(), "secret") {
		t.Fatalf("status=%d calls=%d body=%s", recorder.Code, calls, recorder.Body.String())
	}
}

func TestMallWeatherRefreshControllerUsesSafeErrors(t *testing.T) {
	service := fakeMallWeatherRefreshControllerService{refresh: func(context.Context, uint, uint, string, requestbody.MallWeatherRefreshRequest) (*data_svc.MallWeatherRefreshResult, bool, error) {
		return nil, false, errors.New("database password=secret")
	}}
	recorder := performMallWeatherRefreshRequest(t, service, "7", `{"kinds":["V26_FULL"],"force":false,"reason":"review"}`, "refresh-key-1234")
	if recorder.Code != http.StatusInternalServerError || strings.Contains(recorder.Body.String(), "password") || strings.Contains(recorder.Body.String(), "secret") {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func performMallWeatherRefreshRequest(t *testing.T, service MallWeatherRefreshServiceAPI, mallID, body, key string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(constant.CurrentUserID, "17")
		c.Next()
	})
	controller := NewMallWeatherRefreshControllerWithService(service)
	router.POST("/api/v1/malls/:id/weather-refresh", controller.Refresh)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/malls/"+mallID+"/weather-refresh", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", key)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

type fakeMallWeatherRefreshControllerService struct {
	refresh func(context.Context, uint, uint, string, requestbody.MallWeatherRefreshRequest) (*data_svc.MallWeatherRefreshResult, bool, error)
}

func (service fakeMallWeatherRefreshControllerService) Refresh(ctx context.Context, actor, mallID uint, key string, request requestbody.MallWeatherRefreshRequest) (*data_svc.MallWeatherRefreshResult, bool, error) {
	if service.refresh == nil {
		panic("unexpected Refresh call")
	}
	return service.refresh(ctx, actor, mallID, key, request)
}
