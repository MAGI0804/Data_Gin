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
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/internal/service/data_svc"

	"github.com/gin-gonic/gin"
)

func TestMallWeatherControllerOverviewParsesContract(t *testing.T) {
	var gotActor, gotMall uint
	var gotTimeZone string
	service := fakeMallWeatherControllerService{overview: func(_ context.Context, actor, mallID uint, timeZone string) (*data_svc.MallWeatherOverviewResult, error) {
		gotActor, gotMall, gotTimeZone = actor, mallID, timeZone
		return &data_svc.MallWeatherOverviewResult{
			Minutely: []data_svc.MallWeatherMinutelyDTO{}, Hourly: []data_svc.MallWeatherHourlyDTO{}, Alerts: []data_svc.MallWeatherAlertDTO{},
		}, nil
	}}
	recorder := performMallWeatherRequest(t, service, "/api/v1/malls/7/weather/overview?timezone=Asia%2FShanghai")
	if recorder.Code != http.StatusOK || gotActor != 17 || gotMall != 7 || gotTimeZone != "Asia/Shanghai" ||
		!strings.Contains(recorder.Body.String(), `"minutely":[]`) {
		t.Fatalf("status=%d actor=%d mall=%d timezone=%q body=%s", recorder.Code, gotActor, gotMall, gotTimeZone, recorder.Body.String())
	}
}

func TestMallWeatherControllerOverviewRejectsInvalidBoundaryValues(t *testing.T) {
	calls := 0
	service := fakeMallWeatherControllerService{overview: func(context.Context, uint, uint, string) (*data_svc.MallWeatherOverviewResult, error) {
		calls++
		return nil, nil
	}}
	paths := []string{
		"/api/v1/malls/0/weather/overview",
		"/api/v1/malls/7/weather/overview?timeZone=UTC&timezone=Asia%2FShanghai",
	}
	for _, path := range paths {
		recorder := performMallWeatherRequest(t, service, path)
		if recorder.Code != http.StatusUnprocessableEntity || strings.Contains(recorder.Body.String(), "conflicting") {
			t.Fatalf("path=%s status=%d body=%s", path, recorder.Code, recorder.Body.String())
		}
	}
	if calls != 0 {
		t.Fatalf("service calls=%d", calls)
	}
}

func TestMallWeatherControllerRealtimeParsesSharedContract(t *testing.T) {
	var gotActor, gotMall uint
	var gotRequest requestbody.MallWeatherRealtimeQueryRequest
	service := fakeMallWeatherControllerService{realtime: func(_ context.Context, actor, mallID uint, request requestbody.MallWeatherRealtimeQueryRequest) (*data_svc.MallWeatherRealtimeResult, error) {
		gotActor, gotMall, gotRequest = actor, mallID, request
		return &data_svc.MallWeatherRealtimeResult{Items: []data_svc.MallWeatherRealtimeDTO{}, Pagination: data_svc.MallWeatherPagination{PageSize: 25}}, nil
	}}
	path := "/api/v1/malls/7/weather/realtime?start=2026-07-22T08%3A00%3A00%2B08%3A00&end=2026-07-23T08%3A00%3A00%2B08%3A00" +
		"&timezone=Asia%2FShanghai&latest=false&as_of=2026-07-22T09%3A00%3A00%2B08%3A00&quality_status=valid&page_size=25&cursor=abc"
	recorder := performMallWeatherRequest(t, service, path)
	if recorder.Code != http.StatusOK || gotActor != 17 || gotMall != 7 {
		t.Fatalf("status=%d actor=%d mall=%d body=%s", recorder.Code, gotActor, gotMall, recorder.Body.String())
	}
	if gotRequest.Latest || gotRequest.PageSize != 25 || gotRequest.TimeZone != "Asia/Shanghai" ||
		gotRequest.QualityStatus != "valid" || gotRequest.Cursor != "abc" || gotRequest.AsOfUTC == nil ||
		!gotRequest.StartUTC.Equal(time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("request=%+v", gotRequest)
	}
}

func TestMallWeatherControllerMinutelyParsesSharedContract(t *testing.T) {
	var gotRequest requestbody.MallWeatherMinutelyQueryRequest
	service := fakeMallWeatherControllerService{minutely: func(_ context.Context, _, _ uint, request requestbody.MallWeatherMinutelyQueryRequest) (*data_svc.MallWeatherMinutelyResult, error) {
		gotRequest = request
		return &data_svc.MallWeatherMinutelyResult{Items: []data_svc.MallWeatherMinutelyDTO{}, Pagination: data_svc.MallWeatherPagination{PageSize: 20}}, nil
	}}
	path := "/api/v1/malls/7/weather/minutely?start=2026-07-22T00:00:00Z&end=2026-07-22T02:00:00Z" +
		"&timeZone=Asia%2FShanghai&latest=true&qualityStatus=warning&pageSize=20"
	recorder := performMallWeatherRequest(t, service, path)
	if recorder.Code != http.StatusOK || !gotRequest.Latest || gotRequest.PageSize != 20 ||
		gotRequest.TimeZone != "Asia/Shanghai" || gotRequest.QualityStatus != "warning" {
		t.Fatalf("status=%d request=%+v body=%s", recorder.Code, gotRequest, recorder.Body.String())
	}
}

func TestMallWeatherControllerDailyParsesSharedContract(t *testing.T) {
	var gotRequest requestbody.MallWeatherDailyQueryRequest
	service := fakeMallWeatherControllerService{daily: func(_ context.Context, _, _ uint, request requestbody.MallWeatherDailyQueryRequest) (*data_svc.MallWeatherDailyResult, error) {
		gotRequest = request
		return &data_svc.MallWeatherDailyResult{Items: []data_svc.MallWeatherDailyDTO{}, Pagination: data_svc.MallWeatherPagination{PageSize: 15}}, nil
	}}
	path := "/api/v1/malls/7/weather/daily?start=2026-07-22T00:00:00Z&end=2026-08-06T00:00:00Z" +
		"&timezone=Asia%2FShanghai&latest=true&as_of=2026-07-22T01:00:00Z&quality_status=valid&page_size=15"
	recorder := performMallWeatherRequest(t, service, path)
	if recorder.Code != http.StatusOK || !gotRequest.Latest || gotRequest.PageSize != 15 ||
		gotRequest.TimeZone != "Asia/Shanghai" || gotRequest.QualityStatus != "valid" || gotRequest.AsOfUTC == nil {
		t.Fatalf("status=%d request=%+v body=%s", recorder.Code, gotRequest, recorder.Body.String())
	}
}

func TestMallWeatherControllerAlertsParsesSharedContract(t *testing.T) {
	var gotRequest requestbody.MallWeatherAlertQueryRequest
	service := fakeMallWeatherControllerService{alerts: func(_ context.Context, _, _ uint, request requestbody.MallWeatherAlertQueryRequest) (*data_svc.MallWeatherAlertResult, error) {
		gotRequest = request
		return &data_svc.MallWeatherAlertResult{Items: []data_svc.MallWeatherAlertDTO{}, Pagination: data_svc.MallWeatherPagination{PageSize: 20}}, nil
	}}
	path := "/api/v1/malls/7/weather/alerts?start=2026-07-01T00:00:00Z&end=2026-08-01T00:00:00Z" +
		"&timezone=Asia%2FShanghai&latest=false&asOf=2026-07-22T01:00:00Z&qualityStatus=warning&pageSize=20"
	recorder := performMallWeatherRequest(t, service, path)
	if recorder.Code != http.StatusOK || gotRequest.Latest || gotRequest.PageSize != 20 ||
		gotRequest.TimeZone != "Asia/Shanghai" || gotRequest.QualityStatus != "warning" || gotRequest.AsOfUTC == nil {
		t.Fatalf("status=%d request=%+v body=%s", recorder.Code, gotRequest, recorder.Body.String())
	}
}

func TestMallWeatherControllerLifeIndicesParsesSharedContract(t *testing.T) {
	var gotActor, gotMall uint
	var gotRequest requestbody.MallWeatherLifeIndexQueryRequest
	service := fakeMallWeatherControllerService{lifeIndices: func(_ context.Context, actor, mallID uint, request requestbody.MallWeatherLifeIndexQueryRequest) (*data_svc.MallWeatherLifeIndexResult, error) {
		gotActor, gotMall, gotRequest = actor, mallID, request
		return &data_svc.MallWeatherLifeIndexResult{Items: []data_svc.MallWeatherLifeIndexDTO{}, Pagination: data_svc.MallWeatherPagination{PageSize: 15}}, nil
	}}
	path := "/api/v1/malls/7/weather/life-indices?start=2026-07-22T00:00:00Z&end=2026-08-06T00:00:00Z" +
		"&timezone=Asia%2FShanghai&latest=false&as_of=2026-07-22T01:00:00Z&quality_status=warning&page_size=15&cursor=abc"
	recorder := performMallWeatherRequest(t, service, path)
	if recorder.Code != http.StatusOK || gotActor != 17 || gotMall != 7 || gotRequest.Latest || gotRequest.PageSize != 15 ||
		gotRequest.TimeZone != "Asia/Shanghai" || gotRequest.QualityStatus != "warning" || gotRequest.AsOfUTC == nil || gotRequest.Cursor != "abc" ||
		!strings.Contains(recorder.Body.String(), `"items":[]`) {
		t.Fatalf("status=%d actor=%d mall=%d request=%+v body=%s", recorder.Code, gotActor, gotMall, gotRequest, recorder.Body.String())
	}
}

func TestMallWeatherControllerHourlyParsesContract(t *testing.T) {
	var gotActor, gotMall uint
	var gotRequest requestbody.MallWeatherHourlyQueryRequest
	service := fakeMallWeatherControllerService{hourly: func(_ context.Context, actor, mallID uint, request requestbody.MallWeatherHourlyQueryRequest) (*data_svc.MallWeatherHourlyResult, error) {
		gotActor, gotMall, gotRequest = actor, mallID, request
		return &data_svc.MallWeatherHourlyResult{Items: []data_svc.MallWeatherHourlyDTO{}, Pagination: data_svc.MallWeatherPagination{PageSize: 25}}, nil
	}}
	path := "/api/v1/malls/7/weather/hourly?start=2026-07-22T08%3A00%3A00%2B08%3A00&end=2026-07-23T08%3A00%3A00%2B08%3A00" +
		"&timezone=Asia%2FShanghai&latest=false&as_of=2026-07-22T09%3A00%3A00%2B08%3A00&quality_status=valid&page_size=25&cursor=abc"
	recorder := performMallWeatherRequest(t, service, path)
	if recorder.Code != http.StatusOK || gotActor != 17 || gotMall != 7 {
		t.Fatalf("status=%d actor=%d mall=%d body=%s", recorder.Code, gotActor, gotMall, recorder.Body.String())
	}
	if gotRequest.Latest || gotRequest.PageSize != 25 || gotRequest.TimeZone != "Asia/Shanghai" ||
		gotRequest.QualityStatus != "valid" || gotRequest.Cursor != "abc" || gotRequest.AsOfUTC == nil ||
		!gotRequest.StartUTC.Equal(time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("request=%+v", gotRequest)
	}
}

func TestMallWeatherControllerHourlyRejectsInvalidBoundaryValues(t *testing.T) {
	calls := 0
	service := fakeMallWeatherControllerService{hourly: func(context.Context, uint, uint, requestbody.MallWeatherHourlyQueryRequest) (*data_svc.MallWeatherHourlyResult, error) {
		calls++
		return nil, nil
	}}
	paths := []string{
		"/api/v1/malls/0/weather/hourly?start=2026-07-22T00:00:00Z&end=2026-07-23T00:00:00Z",
		"/api/v1/malls/7/weather/hourly?end=2026-07-23T00:00:00Z",
		"/api/v1/malls/7/weather/hourly?start=bad&end=2026-07-23T00:00:00Z",
		"/api/v1/malls/7/weather/hourly?start=2026-07-22T00:00:00Z&end=2026-07-23T00:00:00Z&latest=maybe",
		"/api/v1/malls/7/weather/hourly?start=2026-07-22T00:00:00Z&end=2026-07-23T00:00:00Z&pageSize=10&page_size=20",
	}
	for _, path := range paths {
		recorder := performMallWeatherRequest(t, service, path)
		if recorder.Code != http.StatusUnprocessableEntity || strings.Contains(recorder.Body.String(), "invalid") {
			t.Fatalf("path=%s status=%d body=%s", path, recorder.Code, recorder.Body.String())
		}
	}
	if calls != 0 {
		t.Fatalf("service calls=%d", calls)
	}
}

func TestMallWeatherControllerHourlyUsesSafeErrors(t *testing.T) {
	service := fakeMallWeatherControllerService{hourly: func(context.Context, uint, uint, requestbody.MallWeatherHourlyQueryRequest) (*data_svc.MallWeatherHourlyResult, error) {
		return nil, errors.New("database password=secret")
	}}
	recorder := performMallWeatherRequest(t, service, "/api/v1/malls/7/weather/hourly?start=2026-07-22T00:00:00Z&end=2026-07-23T00:00:00Z")
	if recorder.Code != http.StatusInternalServerError || strings.Contains(recorder.Body.String(), "password") || strings.Contains(recorder.Body.String(), "secret") {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func performMallWeatherRequest(t *testing.T, service MallWeatherQueryService, path string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(constant.CurrentUserID, strconv.FormatUint(17, 10))
		c.Next()
	})
	controller := NewMallWeatherControllerWithService(service)
	router.GET("/api/v1/malls/:id/weather/overview", controller.Overview)
	router.GET("/api/v1/malls/:id/weather/realtime", controller.Realtime)
	router.GET("/api/v1/malls/:id/weather/minutely", controller.Minutely)
	router.GET("/api/v1/malls/:id/weather/hourly", controller.Hourly)
	router.GET("/api/v1/malls/:id/weather/daily", controller.Daily)
	router.GET("/api/v1/malls/:id/weather/alerts", controller.Alerts)
	router.GET("/api/v1/malls/:id/weather/life-indices", controller.LifeIndices)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	return recorder
}

type fakeMallWeatherControllerService struct {
	overview    func(context.Context, uint, uint, string) (*data_svc.MallWeatherOverviewResult, error)
	realtime    func(context.Context, uint, uint, requestbody.MallWeatherRealtimeQueryRequest) (*data_svc.MallWeatherRealtimeResult, error)
	minutely    func(context.Context, uint, uint, requestbody.MallWeatherMinutelyQueryRequest) (*data_svc.MallWeatherMinutelyResult, error)
	hourly      func(context.Context, uint, uint, requestbody.MallWeatherHourlyQueryRequest) (*data_svc.MallWeatherHourlyResult, error)
	daily       func(context.Context, uint, uint, requestbody.MallWeatherDailyQueryRequest) (*data_svc.MallWeatherDailyResult, error)
	alerts      func(context.Context, uint, uint, requestbody.MallWeatherAlertQueryRequest) (*data_svc.MallWeatherAlertResult, error)
	lifeIndices func(context.Context, uint, uint, requestbody.MallWeatherLifeIndexQueryRequest) (*data_svc.MallWeatherLifeIndexResult, error)
}

func (service fakeMallWeatherControllerService) Overview(ctx context.Context, actor, mallID uint, timeZone string) (*data_svc.MallWeatherOverviewResult, error) {
	if service.overview == nil {
		panic("unexpected Overview call")
	}
	return service.overview(ctx, actor, mallID, timeZone)
}

func (service fakeMallWeatherControllerService) Realtime(ctx context.Context, actor, mallID uint, request requestbody.MallWeatherRealtimeQueryRequest) (*data_svc.MallWeatherRealtimeResult, error) {
	if service.realtime == nil {
		panic("unexpected Realtime call")
	}
	return service.realtime(ctx, actor, mallID, request)
}

func (service fakeMallWeatherControllerService) Minutely(ctx context.Context, actor, mallID uint, request requestbody.MallWeatherMinutelyQueryRequest) (*data_svc.MallWeatherMinutelyResult, error) {
	if service.minutely == nil {
		panic("unexpected Minutely call")
	}
	return service.minutely(ctx, actor, mallID, request)
}

func (service fakeMallWeatherControllerService) Hourly(ctx context.Context, actor, mallID uint, request requestbody.MallWeatherHourlyQueryRequest) (*data_svc.MallWeatherHourlyResult, error) {
	if service.hourly == nil {
		panic("unexpected Hourly call")
	}
	return service.hourly(ctx, actor, mallID, request)
}

func (service fakeMallWeatherControllerService) Daily(ctx context.Context, actor, mallID uint, request requestbody.MallWeatherDailyQueryRequest) (*data_svc.MallWeatherDailyResult, error) {
	if service.daily == nil {
		panic("unexpected Daily call")
	}
	return service.daily(ctx, actor, mallID, request)
}

func (service fakeMallWeatherControllerService) Alerts(ctx context.Context, actor, mallID uint, request requestbody.MallWeatherAlertQueryRequest) (*data_svc.MallWeatherAlertResult, error) {
	if service.alerts == nil {
		panic("unexpected Alerts call")
	}
	return service.alerts(ctx, actor, mallID, request)
}

func (service fakeMallWeatherControllerService) LifeIndices(ctx context.Context, actor, mallID uint, request requestbody.MallWeatherLifeIndexQueryRequest) (*data_svc.MallWeatherLifeIndexResult, error) {
	if service.lifeIndices == nil {
		panic("unexpected LifeIndices call")
	}
	return service.lifeIndices(ctx, actor, mallID, request)
}
