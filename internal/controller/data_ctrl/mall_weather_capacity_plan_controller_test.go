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

func TestMallWeatherCapacityPlanControllerReturnsPlan(t *testing.T) {
	service := fakeMallWeatherCapacityPlanService{
		plan: func(_ context.Context, actorID uint, input data_svc.MallWeatherCapacityPlanInput) (*data_svc.MallWeatherCapacityPlan, error) {
			if actorID != 17 {
				t.Fatalf("actorID=%d, want 17", actorID)
			}
			if input.MallCount != 5 || input.ProviderQPS != 2.5 || input.HourlySteps != 360 ||
				input.DailySteps != 15 || input.LifeIndexDays != 15 || input.AlertsPerMall != 3 ||
				input.FeishuBatchRows != 200 {
				t.Fatalf("input=%+v", input)
			}
			return &data_svc.MallWeatherCapacityPlan{
				MallCount:                         input.MallCount,
				ProviderQPS:                       input.ProviderQPS,
				WeatherV26ProviderRequestsPerDay:  840,
				LifeIndexV3ProviderRequestsPerDay: 120,
				ProviderRequests:                  960,
				ProviderDrainSeconds:              384,
				MinimumQPSForOneHourDrain:         float64(960) / 3600,
				TotalDatabaseRows:                 2570,
				TotalDatabaseBatches:              35,
				FeishuBatchRows:                   input.FeishuBatchRows,
				TotalFeishuBatches:                16,
				Datasets: []data_svc.MallWeatherCapacityDatasetPlan{{
					Kind: "hourly", Rows: 1800, DatabaseBatches: 10, FeishuBatches: 9,
				}},
			}, nil
		},
	}

	recorder := performMallWeatherCapacityPlanRequest(t, service, 17, "?mallCount=5&providerQps=2.5&hourlySteps=360&dailySteps=15&lifeIndexDays=15&alertsPerMall=3&feishuBatchRows=200")
	body := recorder.Body.String()
	if recorder.Code != http.StatusOK ||
		!strings.Contains(body, `"providerRequests":960`) ||
		!strings.Contains(body, `"totalDatabaseRows":2570`) ||
		!strings.Contains(body, `"kind":"hourly"`) {
		t.Fatalf("status=%d body=%s", recorder.Code, body)
	}
}

func TestMallWeatherCapacityPlanControllerDefaultsFullProfileParameters(t *testing.T) {
	service := fakeMallWeatherCapacityPlanService{
		plan: func(_ context.Context, _ uint, input data_svc.MallWeatherCapacityPlanInput) (*data_svc.MallWeatherCapacityPlan, error) {
			if input.MallCount != 1000 || input.ProviderQPS != 20 ||
				input.HourlySteps != 360 || input.DailySteps != 15 || input.LifeIndexDays != 15 ||
				input.AlertsPerMall != 0 || input.FeishuBatchRows != 0 {
				t.Fatalf("input=%+v", input)
			}
			return &data_svc.MallWeatherCapacityPlan{MallCount: input.MallCount, ProviderQPS: input.ProviderQPS}, nil
		},
	}

	recorder := performMallWeatherCapacityPlanRequest(t, service, 17, "?mall_count=1000&provider_qps=20")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestMallWeatherCapacityPlanControllerRejectsInvalidQuery(t *testing.T) {
	calls := 0
	service := fakeMallWeatherCapacityPlanService{
		plan: func(context.Context, uint, data_svc.MallWeatherCapacityPlanInput) (*data_svc.MallWeatherCapacityPlan, error) {
			calls++
			return nil, nil
		},
	}

	recorder := performMallWeatherCapacityPlanRequest(t, service, 17, "?mallCount=bad&providerQps=2.5&hourlySteps=360&dailySteps=15&lifeIndexDays=15")
	if recorder.Code != http.StatusUnprocessableEntity || calls != 0 {
		t.Fatalf("status=%d calls=%d body=%s", recorder.Code, calls, recorder.Body.String())
	}

	recorder = performMallWeatherCapacityPlanRequest(t, service, 17, "?mallCount=5&mall_count=6&providerQps=2.5")
	if recorder.Code != http.StatusUnprocessableEntity || calls != 0 {
		t.Fatalf("conflict status=%d calls=%d body=%s", recorder.Code, calls, recorder.Body.String())
	}
}

func TestMallWeatherCapacityPlanControllerUsesSafeErrors(t *testing.T) {
	service := fakeMallWeatherCapacityPlanService{
		plan: func(context.Context, uint, data_svc.MallWeatherCapacityPlanInput) (*data_svc.MallWeatherCapacityPlan, error) {
			return nil, errors.New("capacity database password=secret")
		},
	}

	recorder := performMallWeatherCapacityPlanRequest(t, service, 17, "?mallCount=5&providerQps=2.5&hourlySteps=360&dailySteps=15&lifeIndexDays=15")
	body := recorder.Body.String()
	if recorder.Code != http.StatusInternalServerError ||
		strings.Contains(body, "password") ||
		strings.Contains(body, "secret") {
		t.Fatalf("status=%d body=%s", recorder.Code, body)
	}
}

func TestMallWeatherCapacityPlanControllerRejectsForbidden(t *testing.T) {
	service := fakeMallWeatherCapacityPlanService{
		plan: func(context.Context, uint, data_svc.MallWeatherCapacityPlanInput) (*data_svc.MallWeatherCapacityPlan, error) {
			return nil, data_svc.ErrMallForbidden
		},
	}

	recorder := performMallWeatherCapacityPlanRequest(t, service, 0, "?mallCount=5&providerQps=2.5&hourlySteps=360&dailySteps=15&lifeIndexDays=15")
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func performMallWeatherCapacityPlanRequest(
	t *testing.T,
	service MallWeatherCapacityPlanServiceAPI,
	actorID uint,
	query string,
) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(constant.CurrentUserID, strconv.FormatUint(uint64(actorID), 10))
		c.Next()
	})
	controller := NewMallWeatherCapacityPlanControllerWithService(service)
	router.GET("/api/v1/mall-weather/capacity-plan", controller.Show)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(
		recorder,
		httptest.NewRequest(http.MethodGet, "/api/v1/mall-weather/capacity-plan"+query, nil),
	)
	return recorder
}

type fakeMallWeatherCapacityPlanService struct {
	plan func(context.Context, uint, data_svc.MallWeatherCapacityPlanInput) (*data_svc.MallWeatherCapacityPlan, error)
}

func (service fakeMallWeatherCapacityPlanService) Plan(
	ctx context.Context,
	actorID uint,
	input data_svc.MallWeatherCapacityPlanInput,
) (*data_svc.MallWeatherCapacityPlan, error) {
	if service.plan == nil {
		panic("unexpected Plan call")
	}
	return service.plan(ctx, actorID, input)
}
