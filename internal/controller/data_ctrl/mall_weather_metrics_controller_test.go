package data_ctrl

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"gin-biz-web-api/constant"
	"gin-biz-web-api/internal/service/data_svc"

	"github.com/gin-gonic/gin"
)

func TestMallWeatherMetricsControllerReturnsSnapshot(t *testing.T) {
	service := fakeMallWeatherMetricsReader{
		snapshot: func(_ context.Context, actorID uint) (*data_svc.MallWeatherMetricsResult, error) {
			if actorID != 17 {
				t.Fatalf("actorID=%d, want 17", actorID)
			}
			return &data_svc.MallWeatherMetricsResult{
				Definitions: []data_svc.MallWeatherMetricDefinition{{
					Name:   data_svc.MallWeatherMetricFeishuRowsTotal,
					Labels: []string{"status"},
				}},
				Counters: []data_svc.MallWeatherMetricCounterSample{{
					Name:   data_svc.MallWeatherMetricFeishuRowsTotal,
					Labels: map[string]string{"status": "success"},
					Value:  9,
				}},
				Gauges: []data_svc.MallWeatherMetricGaugeSample{{
					Name:   data_svc.MallWeatherMetricDataAgeSeconds,
					Labels: map[string]string{"kind": "full"},
					Value:  12.5,
				}},
				Alerts: []data_svc.MallWeatherOperationalAlert{{
					Code:      "MALL_WEATHER_QUEUE_LAG_HIGH",
					Severity:  "WARNING",
					Status:    "FIRING",
					Metric:    data_svc.MallWeatherMetricQueueLagSeconds,
					Labels:    map[string]string{"kind": "full"},
					Value:     75,
					Threshold: 60,
				}},
				Summary: data_svc.MallWeatherAlertSummary{
					Total:   1,
					Warning: 1,
					ByStatus: []data_svc.MallWeatherAlertStatusSum{{
						Status: "FIRING",
						Count:  1,
					}},
				},
			}, nil
		},
	}

	recorder := performMallWeatherMetricsRequest(t, service, 17)
	body := recorder.Body.String()
	if recorder.Code != http.StatusOK ||
		!strings.Contains(body, `"definitions":[`) ||
		!strings.Contains(body, `"counters":[`) ||
		!strings.Contains(body, `"gauges":[`) ||
		!strings.Contains(body, `"alerts":[`) ||
		!strings.Contains(body, `"summary":`) ||
		!strings.Contains(body, `"warning":1`) ||
		!strings.Contains(body, `"code":"MALL_WEATHER_QUEUE_LAG_HIGH"`) ||
		!strings.Contains(body, `"value":9`) ||
		!strings.Contains(body, `"value":12.5`) {
		t.Fatalf("status=%d body=%s", recorder.Code, body)
	}
}

func TestMallWeatherMetricsControllerUsesSafeErrors(t *testing.T) {
	service := fakeMallWeatherMetricsReader{
		snapshot: func(context.Context, uint) (*data_svc.MallWeatherMetricsResult, error) {
			return nil, errors.New("redis password=secret")
		},
	}

	recorder := performMallWeatherMetricsRequest(t, service, 17)
	body := recorder.Body.String()
	if recorder.Code != http.StatusInternalServerError ||
		strings.Contains(body, "password") ||
		strings.Contains(body, "secret") {
		t.Fatalf("status=%d body=%s", recorder.Code, body)
	}
}

func TestMallWeatherMetricsControllerRejectsAnonymousActor(t *testing.T) {
	calls := 0
	service := fakeMallWeatherMetricsReader{
		snapshot: func(_ context.Context, actorID uint) (*data_svc.MallWeatherMetricsResult, error) {
			calls++
			if actorID != 0 {
				t.Fatalf("actorID=%d, want 0", actorID)
			}
			return nil, data_svc.ErrMallForbidden
		},
	}

	recorder := performMallWeatherMetricsRequest(t, service, 0)
	if recorder.Code != http.StatusForbidden || calls != 1 {
		t.Fatalf("status=%d calls=%d body=%s", recorder.Code, calls, recorder.Body.String())
	}
}

func performMallWeatherMetricsRequest(
	t *testing.T,
	service MallWeatherMetricsReader,
	actorID uint,
) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(constant.CurrentUserID, strconv.FormatUint(uint64(actorID), 10))
		c.Next()
	})
	controller := NewMallWeatherMetricsControllerWithService(service)
	router.GET("/api/v1/mall-weather/metrics", controller.Snapshot)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/mall-weather/metrics", nil))
	return recorder
}

type fakeMallWeatherMetricsReader struct {
	snapshot func(context.Context, uint) (*data_svc.MallWeatherMetricsResult, error)
}

func (service fakeMallWeatherMetricsReader) Snapshot(
	ctx context.Context,
	actorID uint,
) (*data_svc.MallWeatherMetricsResult, error) {
	if service.snapshot == nil {
		panic("unexpected Snapshot call")
	}
	return service.snapshot(ctx, actorID)
}
