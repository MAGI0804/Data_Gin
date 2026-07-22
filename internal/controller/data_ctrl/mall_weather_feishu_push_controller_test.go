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
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(constant.CurrentUserID, "17")
		c.Next()
	})
	controller := NewMallWeatherFeishuPushControllerWithService(service)
	router.POST("/api/v1/weather-sheet-pushes/dry-run", controller.DryRun)
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/weather-sheet-pushes/dry-run",
		strings.NewReader(body),
	)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

type fakeMallWeatherFeishuPushControllerService struct {
	dryRun func(
		context.Context,
		uint,
		requestbody.MallWeatherFeishuPushRequest,
	) (*data_svc.MallWeatherFeishuDryRunResult, error)
}

func (service fakeMallWeatherFeishuPushControllerService) DryRun(
	ctx context.Context,
	actor uint,
	request requestbody.MallWeatherFeishuPushRequest,
) (*data_svc.MallWeatherFeishuDryRunResult, error) {
	return service.dryRun(ctx, actor, request)
}
