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

func TestMallWeatherExportProfileControllerCreatesProfile(t *testing.T) {
	service := fakeMallWeatherExportProfileControllerService{
		save: func(
			_ context.Context,
			actor uint,
			request requestbody.MallWeatherExportProfileSaveRequest,
		) (*data_svc.MallWeatherExportProfileDTO, bool, error) {
			if actor != 17 || request.Code != "mall_weather_full" || len(request.Datasets) != 1 {
				t.Fatalf("actor=%d request=%+v", actor, request)
			}
			return &data_svc.MallWeatherExportProfileDTO{ID: 9, Code: request.Code, Version: 1}, true, nil
		},
	}
	body := `{"code":"mall_weather_full","name":"全量导出",` +
		`"fileNameTemplate":"weather.xlsx","datasets":[{"kind":"hourly","sheetName":"小时"}]}`
	recorder := performMallWeatherExportProfileRequest(
		t,
		service,
		http.MethodPost,
		"/api/v1/weather-export-profiles",
		body,
	)
	if recorder.Code != http.StatusCreated || !strings.Contains(recorder.Body.String(), `"code":"mall_weather_full"`) {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestMallWeatherExportProfileControllerListsProfiles(t *testing.T) {
	service := fakeMallWeatherExportProfileControllerService{
		list: func(
			_ context.Context,
			actor uint,
			enabled *bool,
			cursor string,
			pageSize int,
		) (*data_svc.MallWeatherExportProfileListResult, error) {
			if actor != 17 || enabled == nil || !*enabled || cursor != "next" || pageSize != 25 {
				t.Fatalf("actor=%d enabled=%v cursor=%q pageSize=%d", actor, enabled, cursor, pageSize)
			}
			return &data_svc.MallWeatherExportProfileListResult{
				Items:      []data_svc.MallWeatherExportProfileDTO{{ID: 9, Code: "mall_weather_full"}},
				Pagination: data_svc.MallWeatherPagination{PageSize: 25},
			}, nil
		},
	}
	path := "/api/v1/weather-export-profiles?enabled=true&cursor=next&pageSize=25"
	recorder := performMallWeatherExportProfileRequest(t, service, http.MethodGet, path, "")
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"pageSize":25`) {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestMallWeatherExportProfileControllerRejectsUnknownJSONField(t *testing.T) {
	calls := 0
	service := fakeMallWeatherExportProfileControllerService{
		save: func(
			context.Context,
			uint,
			requestbody.MallWeatherExportProfileSaveRequest,
		) (*data_svc.MallWeatherExportProfileDTO, bool, error) {
			calls++
			return nil, false, nil
		},
	}
	body := `{"code":"mall_weather_full","secret":"x"}`
	recorder := performMallWeatherExportProfileRequest(
		t,
		service,
		http.MethodPost,
		"/api/v1/weather-export-profiles",
		body,
	)
	leakedSecret := strings.Contains(recorder.Body.String(), "secret")
	if recorder.Code != http.StatusUnprocessableEntity || calls != 0 || leakedSecret {
		t.Fatalf("status=%d calls=%d body=%s", recorder.Code, calls, recorder.Body.String())
	}
}

func TestMallWeatherExportProfileControllerMapsSafeErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{name: "forbidden", err: data_svc.ErrMallForbidden, wantStatus: http.StatusForbidden},
		{name: "conflict", err: data_svc.ErrMallWeatherExportProfileConflict, wantStatus: http.StatusConflict},
		{name: "invalid", err: data_svc.ErrMallWeatherExportProfileInvalid, wantStatus: http.StatusUnprocessableEntity},
		{name: "internal", err: errors.New("database password=secret"), wantStatus: http.StatusInternalServerError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := fakeMallWeatherExportProfileControllerService{
				list: func(context.Context, uint, *bool, string, int) (*data_svc.MallWeatherExportProfileListResult, error) {
					return nil, test.err
				},
			}
			recorder := performMallWeatherExportProfileRequest(t, service, http.MethodGet, "/api/v1/weather-export-profiles", "")
			if recorder.Code != test.wantStatus || strings.Contains(recorder.Body.String(), "password") ||
				strings.Contains(recorder.Body.String(), "secret") {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func performMallWeatherExportProfileRequest(
	t *testing.T,
	service MallWeatherExportProfileServiceAPI,
	method string,
	path string,
	body string,
) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(constant.CurrentUserID, "17")
		c.Next()
	})
	controller := NewMallWeatherExportProfileControllerWithService(service)
	router.POST("/api/v1/weather-export-profiles", controller.Save)
	router.GET("/api/v1/weather-export-profiles", controller.List)
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

type fakeMallWeatherExportProfileControllerService struct {
	save func(
		context.Context,
		uint,
		requestbody.MallWeatherExportProfileSaveRequest,
	) (*data_svc.MallWeatherExportProfileDTO, bool, error)
	list func(
		context.Context,
		uint,
		*bool,
		string,
		int,
	) (*data_svc.MallWeatherExportProfileListResult, error)
}

func (service fakeMallWeatherExportProfileControllerService) Save(
	ctx context.Context,
	actor uint,
	request requestbody.MallWeatherExportProfileSaveRequest,
) (*data_svc.MallWeatherExportProfileDTO, bool, error) {
	if service.save == nil {
		panic("unexpected Save call")
	}
	return service.save(ctx, actor, request)
}

func (service fakeMallWeatherExportProfileControllerService) List(
	ctx context.Context,
	actor uint,
	enabled *bool,
	cursor string,
	pageSize int,
) (*data_svc.MallWeatherExportProfileListResult, error) {
	if service.list == nil {
		panic("unexpected List call")
	}
	return service.list(ctx, actor, enabled, cursor, pageSize)
}
