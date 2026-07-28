package data_ctrl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"gin-biz-web-api/constant"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/internal/service/data_svc"

	"github.com/gin-gonic/gin"
)

func TestMallWeatherControllerOpenHourlyParsesJSON(t *testing.T) {
	var gotActor, gotMall uint
	var gotRequest requestbody.MallWeatherHourlyQueryRequest
	service := fakeMallWeatherControllerService{hourly: func(
		_ context.Context,
		actor, mallID uint,
		request requestbody.MallWeatherHourlyQueryRequest,
	) (*data_svc.MallWeatherHourlyResult, error) {
		gotActor, gotMall, gotRequest = actor, mallID, request
		return &data_svc.MallWeatherHourlyResult{
			Items:      []data_svc.MallWeatherHourlyDTO{},
			Pagination: data_svc.MallWeatherPagination{PageSize: 25},
		}, nil
	}}
	body := `{
		"start":"2026-07-22T08:00:00+08:00",
		"end":"2026-07-23T08:00:00+08:00",
		"timeZone":"Asia/Shanghai",
		"latest":false,
		"asOf":"2026-07-22T09:00:00+08:00",
		"qualityStatus":"valid",
		"cursor":"abc",
		"pageSize":25
	}`
	recorder := performOpenMallWeatherRequest(t, service, "/api/open/weather/malls/7/hourly", body)
	if recorder.Code != http.StatusOK || gotActor != 17 || gotMall != 7 {
		t.Fatalf("status=%d actor=%d mall=%d body=%s", recorder.Code, gotActor, gotMall, recorder.Body.String())
	}
	if gotRequest.Latest || gotRequest.PageSize != 25 || gotRequest.TimeZone != "Asia/Shanghai" ||
		gotRequest.QualityStatus != "valid" || gotRequest.Cursor != "abc" || gotRequest.AsOfUTC == nil ||
		!gotRequest.StartUTC.Equal(time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("request=%+v", gotRequest)
	}
}

func TestMallWeatherControllerOpenHourlyDefaultsLatest(t *testing.T) {
	var gotRequest requestbody.MallWeatherHourlyQueryRequest
	service := fakeMallWeatherControllerService{hourly: func(
		_ context.Context,
		_, _ uint,
		request requestbody.MallWeatherHourlyQueryRequest,
	) (*data_svc.MallWeatherHourlyResult, error) {
		gotRequest = request
		return &data_svc.MallWeatherHourlyResult{Items: []data_svc.MallWeatherHourlyDTO{}}, nil
	}}
	body := `{"start":"2026-07-22T00:00:00Z","end":"2026-07-23T00:00:00Z"}`
	recorder := performOpenMallWeatherRequest(t, service, "/api/open/weather/malls/7/hourly", body)
	if recorder.Code != http.StatusOK || !gotRequest.Latest {
		t.Fatalf("status=%d request=%+v body=%s", recorder.Code, gotRequest, recorder.Body.String())
	}
}

func TestMallWeatherControllerOpenHourlyRejectsInvalidJSON(t *testing.T) {
	calls := 0
	service := fakeMallWeatherControllerService{hourly: func(
		context.Context,
		uint,
		uint,
		requestbody.MallWeatherHourlyQueryRequest,
	) (*data_svc.MallWeatherHourlyResult, error) {
		calls++
		return nil, nil
	}}
	tests := []string{
		`{"start":"2026-07-22T00:00:00Z","end":"2026-07-23T00:00:00Z","unknown":true}`,
		`{"start":"bad","end":"2026-07-23T00:00:00Z"}`,
		`{"start":"2026-07-22T00:00:00Z","end":"2026-07-23T00:00:00Z","pageSize":0}`,
	}
	for _, body := range tests {
		recorder := performOpenMallWeatherRequest(t, service, "/api/open/weather/malls/7/hourly", body)
		if recorder.Code != http.StatusUnprocessableEntity || strings.Contains(recorder.Body.String(), "unknown") {
			t.Fatalf("status=%d response=%s request=%s", recorder.Code, recorder.Body.String(), body)
		}
	}
	if calls != 0 {
		t.Fatalf("service calls=%d", calls)
	}
}

func performOpenMallWeatherRequest(
	t *testing.T,
	service MallWeatherQueryService,
	path string,
	body string,
) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(constant.CurrentUserID, strconv.FormatUint(17, 10))
		c.Next()
	})
	controller := NewMallWeatherControllerWithService(service)
	router.POST("/api/open/weather/malls/:id/hourly", controller.OpenHourly)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	return recorder
}
