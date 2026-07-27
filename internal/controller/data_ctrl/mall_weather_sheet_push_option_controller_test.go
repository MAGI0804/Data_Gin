package data_ctrl

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gin-biz-web-api/constant"
	"gin-biz-web-api/internal/service/data_svc"

	"github.com/gin-gonic/gin"
)

func TestMallWeatherSheetPushOptionControllerListsSafeOptions(t *testing.T) {
	service := fakeMallWeatherSheetPushOptionControllerService{
		list: func(_ context.Context, actorUserID uint) (*data_svc.MallWeatherSheetPushOptionResult, error) {
			if actorUserID != 17 {
				t.Fatalf("actorUserID=%d, want 17", actorUserID)
			}
			return &data_svc.MallWeatherSheetPushOptionResult{
				Items: []data_svc.MallWeatherSheetPushOption{{
					DestinationID:  7,
					Name:           "天气看板",
					Code:           "weather_board",
					ProfileID:      9,
					ProfileCode:    "mall_weather_full",
					ProfileVersion: 3,
				}},
			}, nil
		},
	}
	recorder := performMallWeatherSheetPushOptionRequest(t, service)
	body := recorder.Body.String()
	if recorder.Code != http.StatusOK ||
		!strings.Contains(body, `"destinationId":7`) ||
		!strings.Contains(body, `"profileVersion":3`) {
		t.Fatalf("status=%d body=%s", recorder.Code, body)
	}
	for _, forbidden := range []string{"config_json", "configJson", "spreadsheetTokenEnv", "enabled"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("response leaked %q: %s", forbidden, body)
		}
	}
}

func TestMallWeatherSheetPushOptionControllerMapsSafeErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{name: "forbidden", err: data_svc.ErrMallForbidden, wantStatus: http.StatusForbidden},
		{name: "internal", err: errors.New("database password=secret"), wantStatus: http.StatusInternalServerError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := fakeMallWeatherSheetPushOptionControllerService{
				list: func(context.Context, uint) (*data_svc.MallWeatherSheetPushOptionResult, error) {
					return nil, test.err
				},
			}
			recorder := performMallWeatherSheetPushOptionRequest(t, service)
			body := recorder.Body.String()
			if recorder.Code != test.wantStatus || strings.Contains(body, "password") || strings.Contains(body, "secret") {
				t.Fatalf("status=%d body=%s", recorder.Code, body)
			}
		})
	}
}

func performMallWeatherSheetPushOptionRequest(
	t *testing.T,
	service MallWeatherSheetPushOptionServiceAPI,
) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(constant.CurrentUserID, "17")
		c.Next()
	})
	controller := NewMallWeatherSheetPushOptionControllerWithService(service)
	router.GET("/api/v1/weather-sheet-push-options", controller.List)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(
		recorder,
		httptest.NewRequest(http.MethodGet, "/api/v1/weather-sheet-push-options", nil),
	)
	return recorder
}

type fakeMallWeatherSheetPushOptionControllerService struct {
	list func(context.Context, uint) (*data_svc.MallWeatherSheetPushOptionResult, error)
}

func (service fakeMallWeatherSheetPushOptionControllerService) List(
	ctx context.Context,
	actorUserID uint,
) (*data_svc.MallWeatherSheetPushOptionResult, error) {
	if service.list == nil {
		panic("unexpected List call")
	}
	return service.list(ctx, actorUserID)
}
