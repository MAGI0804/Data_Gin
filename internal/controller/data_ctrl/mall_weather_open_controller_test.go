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

func TestMallWeatherControllerOpenRealtimeReturnsCurrentSnapshot(t *testing.T) {
	var gotActor, gotMall uint
	var gotTimeZone string
	service := fakeMallWeatherControllerService{currentRealtime: func(
		_ context.Context,
		actor, mallID uint,
		timeZone string,
	) (*data_svc.MallWeatherCurrentRealtimeResult, error) {
		gotActor, gotMall, gotTimeZone = actor, mallID, timeZone
		snapshotAt := time.Date(2026, 7, 29, 4, 0, 0, 0, time.UTC)
		return &data_svc.MallWeatherCurrentRealtimeResult{
			Realtime: &data_svc.MallWeatherRealtimeDTO{
				SnapshotAtUTC:           snapshotAt,
				SnapshotAtLocal:         "2026-07-29T12:00:00+08:00",
				ProviderServerTimeUTC:   snapshotAt,
				ProviderServerTimeLocal: "2026-07-29T12:00:00+08:00",
				FetchedAtUTC:            snapshotAt,
				FetchedAtLocal:          "2026-07-29T12:00:00+08:00",
				QualityStatus:           "VALID",
				QualityWarnings:         []data_svc.MallWeatherWarningDTO{},
			},
			Meta: data_svc.MallWeatherQueryMeta{TimeZone: "Asia/Shanghai", FreshnessStatus: "FRESH"},
		}, nil
	}}
	recorder := performOpenMallWeatherRequest(
		t, service, "/api/open/weather/realtime", `{"mallId":7,"timeZone":"Asia/Shanghai"}`,
	)
	response := recorder.Body.String()
	if recorder.Code != http.StatusOK || gotActor != 17 || gotMall != 7 || gotTimeZone != "Asia/Shanghai" ||
		!strings.Contains(response, `"snapshotAtUtc":"2026-07-29 04:00:00"`) ||
		!strings.Contains(response, `"snapshotAtLocal":"2026-07-29 12:00:00"`) ||
		strings.Contains(response, `"items"`) || strings.Contains(response, `"pagination"`) {
		t.Fatalf("status=%d actor=%d mall=%d timeZone=%q body=%s", recorder.Code, gotActor, gotMall, gotTimeZone, response)
	}
}

func TestMallWeatherControllerOpenRealtimeRejectsHistoricalQueryFields(t *testing.T) {
	calls := 0
	service := fakeMallWeatherControllerService{currentRealtime: func(
		context.Context,
		uint,
		uint,
		string,
	) (*data_svc.MallWeatherCurrentRealtimeResult, error) {
		calls++
		return nil, nil
	}}
	tests := []string{
		`{"mallId":7,"start":"2026-07-29 00:00:00"}`,
		`{"mallId":7,"end":"2026-07-30 00:00:00"}`,
		`{"mallId":7,"latest":true}`,
		`{"mallId":7,"asOf":"2026-07-29 00:00:00"}`,
		`{"mallId":7,"qualityStatus":"VALID"}`,
		`{"mallId":7,"cursor":"abc"}`,
		`{"mallId":7,"pageSize":50}`,
	}
	for _, body := range tests {
		recorder := performOpenMallWeatherRequest(t, service, "/api/open/weather/realtime", body)
		if recorder.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d body=%s request=%s", recorder.Code, recorder.Body.String(), body)
		}
	}
	if calls != 0 {
		t.Fatalf("service calls=%d", calls)
	}
}

func TestMallWeatherControllerOpenRealtimeReturnsNullWhenUnavailable(t *testing.T) {
	service := fakeMallWeatherControllerService{currentRealtime: func(
		context.Context,
		uint,
		uint,
		string,
	) (*data_svc.MallWeatherCurrentRealtimeResult, error) {
		return &data_svc.MallWeatherCurrentRealtimeResult{
			Meta: data_svc.MallWeatherQueryMeta{TimeZone: "Asia/Shanghai", FreshnessStatus: "UNAVAILABLE"},
		}, nil
	}}
	recorder := performOpenMallWeatherRequest(t, service, "/api/open/weather/realtime", `{"mallId":7}`)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"realtime":null`) ||
		!strings.Contains(recorder.Body.String(), `"freshnessStatus":"UNAVAILABLE"`) {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestMallWeatherControllerOpenRealtimeRejectsConflictingPathAndBodyMallID(t *testing.T) {
	calls := 0
	service := fakeMallWeatherControllerService{currentRealtime: func(
		context.Context,
		uint,
		uint,
		string,
	) (*data_svc.MallWeatherCurrentRealtimeResult, error) {
		calls++
		return nil, nil
	}}
	recorder := performOpenMallWeatherRequest(
		t, service, "/api/open/weather/malls/7/realtime", `{"mallId":8}`,
	)
	if recorder.Code != http.StatusUnprocessableEntity || calls != 0 {
		t.Fatalf("status=%d calls=%d body=%s", recorder.Code, calls, recorder.Body.String())
	}
}

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
		"mallId":7,
		"start":"2026-07-22 08:00:00",
		"end":"2026-07-23 08:00:00",
		"timeZone":"Asia/Shanghai",
		"latest":false,
		"asOf":"2026-07-22 09:00:00",
		"qualityStatus":"valid",
		"cursor":"abc",
		"pageSize":25
	}`
	recorder := performOpenMallWeatherRequest(t, service, "/api/open/weather/hourly", body)
	if recorder.Code != http.StatusOK || gotActor != 17 || gotMall != 7 {
		t.Fatalf("status=%d actor=%d mall=%d body=%s", recorder.Code, gotActor, gotMall, recorder.Body.String())
	}
	if gotRequest.Latest || gotRequest.PageSize != 25 || gotRequest.TimeZone != "Asia/Shanghai" ||
		gotRequest.QualityStatus != "valid" || gotRequest.Cursor != "abc" || gotRequest.AsOfUTC == nil ||
		!gotRequest.IncludeTotals || !gotRequest.StartUTC.Equal(time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)) {
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
	body := `{"mallId":7,"start":"2026-07-22 00:00:00","end":"2026-07-23 00:00:00"}`
	recorder := performOpenMallWeatherRequest(t, service, "/api/open/weather/hourly", body)
	if recorder.Code != http.StatusOK || !gotRequest.Latest {
		t.Fatalf("status=%d request=%+v body=%s", recorder.Code, gotRequest, recorder.Body.String())
	}
}

func TestMallWeatherControllerOpenHourlyRejectsConflictingPathAndBodyMallID(t *testing.T) {
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
	body := `{"mallId":8,"start":"2026-07-22 00:00:00","end":"2026-07-23 00:00:00"}`
	recorder := performOpenMallWeatherRequest(t, service, "/api/open/weather/malls/7/hourly", body)
	if recorder.Code != http.StatusUnprocessableEntity || calls != 0 {
		t.Fatalf("status=%d calls=%d body=%s", recorder.Code, calls, recorder.Body.String())
	}
}

func TestMallWeatherControllerOpenHourlyFormatsResponseDateTimes(t *testing.T) {
	service := fakeMallWeatherControllerService{hourly: func(
		context.Context,
		uint,
		uint,
		requestbody.MallWeatherHourlyQueryRequest,
	) (*data_svc.MallWeatherHourlyResult, error) {
		return &data_svc.MallWeatherHourlyResult{Items: []data_svc.MallWeatherHourlyDTO{{
			ForecastTimeUTC:   time.Date(2026, 7, 22, 1, 2, 3, 0, time.UTC),
			ForecastTimeLocal: "2026-07-22T09:02:03+08:00",
		}}}, nil
	}}
	body := `{"mallId":7,"start":"2026-07-22 00:00:00","end":"2026-07-23 00:00:00"}`
	recorder := performOpenMallWeatherRequest(t, service, "/api/open/weather/hourly", body)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"forecastTimeUtc":"2026-07-22 01:02:03"`) ||
		!strings.Contains(recorder.Body.String(), `"forecastTimeLocal":"2026-07-22 09:02:03"`) {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
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
		`{"mallId":7,"start":"2026-07-22 00:00:00","end":"2026-07-23 00:00:00","unknown":true}`,
		`{"mallId":7,"start":"bad","end":"2026-07-23 00:00:00"}`,
		`{"mallId":7,"start":"2026-07-22 00:00:00","end":"2026-07-23 00:00:00","pageSize":0}`,
		`{"start":"2026-07-22 00:00:00","end":"2026-07-23 00:00:00"}`,
	}
	for _, body := range tests {
		recorder := performOpenMallWeatherRequest(t, service, "/api/open/weather/hourly", body)
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
	router.POST("/api/open/weather/realtime", controller.OpenRealtime)
	router.POST("/api/open/weather/malls/:id/realtime", controller.OpenRealtime)
	router.POST("/api/open/weather/hourly", controller.OpenHourly)
	router.POST("/api/open/weather/malls/:id/hourly", controller.OpenHourly)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	return recorder
}
