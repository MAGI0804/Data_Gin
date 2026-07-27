package data_svc

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"
	"time"
)

func TestBuildMallWeatherCapacityPlanEstimatesProviderAndStoragePressure(t *testing.T) {
	t.Parallel()

	plan, err := BuildMallWeatherCapacityPlan(MallWeatherCapacityPlanInput{
		MallCount: 5, ProviderQPS: 2.5, HourlySteps: 360, DailySteps: 15, LifeIndexDays: 15, AlertsPerMall: 3,
	})
	if err != nil {
		t.Fatalf("BuildMallWeatherCapacityPlan() error=%v", err)
	}
	if plan.WeatherV26ProviderRequestsPerDay != 840 || plan.LifeIndexV3ProviderRequestsPerDay != 0 ||
		plan.ProviderRequests != 840 || plan.ProviderDrainSeconds != 336 ||
		plan.MinimumQPSForOneHourDrain != float64(840)/3600 {
		t.Fatalf("provider plan=%+v", plan)
	}
	if plan.TotalDatabaseRows != 2570 || plan.TotalDatabaseBatches != 35 || plan.FeishuBatchRows != defaultMallWeatherFeishuBatchRows ||
		plan.TotalFeishuBatches != 16 {
		t.Fatalf("plan totals=%+v", plan)
	}
	wantDatasets := []MallWeatherCapacityDatasetPlan{
		{Kind: "realtime", Rows: 5, DatabaseBatches: 5, FeishuBatches: 1},
		{Kind: "minutely", Rows: 600, DatabaseBatches: 5, FeishuBatches: 3},
		{Kind: "hourly", Rows: 1800, DatabaseBatches: 10, FeishuBatches: 9},
		{Kind: "daily", Rows: 75, DatabaseBatches: 5, FeishuBatches: 1},
		{Kind: "alerts", Rows: 15, DatabaseBatches: 5, FeishuBatches: 1},
		{Kind: "life_indices", Rows: 75, DatabaseBatches: 5, FeishuBatches: 1},
	}
	if !reflect.DeepEqual(plan.Datasets, wantDatasets) {
		t.Fatalf("datasets=%+v want %+v", plan.Datasets, wantDatasets)
	}
}

func TestBuildMallWeatherCapacityPlanUsesConfiguredFeishuBatchRows(t *testing.T) {
	t.Parallel()

	plan, err := BuildMallWeatherCapacityPlan(MallWeatherCapacityPlanInput{
		MallCount: 2, ProviderQPS: 1, HourlySteps: 24, DailySteps: 1, LifeIndexDays: 1, AlertsPerMall: 0, FeishuBatchRows: 50,
	})
	if err != nil {
		t.Fatalf("BuildMallWeatherCapacityPlan() error=%v", err)
	}
	if plan.FeishuBatchRows != 50 || plan.TotalFeishuBatches != 9 {
		t.Fatalf("plan=%+v", plan)
	}
}

func TestBuildMallWeatherCapacityPlanRejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*MallWeatherCapacityPlanInput)
	}{
		{name: "missing mall count", mutate: func(input *MallWeatherCapacityPlanInput) { input.MallCount = 0 }},
		{name: "missing qps", mutate: func(input *MallWeatherCapacityPlanInput) { input.ProviderQPS = 0 }},
		{name: "nan qps", mutate: func(input *MallWeatherCapacityPlanInput) { input.ProviderQPS = math.NaN() }},
		{name: "infinite qps", mutate: func(input *MallWeatherCapacityPlanInput) { input.ProviderQPS = math.Inf(1) }},
		{name: "too many hourly steps", mutate: func(input *MallWeatherCapacityPlanInput) { input.HourlySteps = 361 }},
		{name: "too many daily steps", mutate: func(input *MallWeatherCapacityPlanInput) { input.DailySteps = 16 }},
		{name: "too many life days", mutate: func(input *MallWeatherCapacityPlanInput) { input.LifeIndexDays = 16 }},
		{name: "too many alerts", mutate: func(input *MallWeatherCapacityPlanInput) { input.AlertsPerMall = 257 }},
		{name: "too many feishu rows", mutate: func(input *MallWeatherCapacityPlanInput) { input.FeishuBatchRows = 501 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := MallWeatherCapacityPlanInput{
				MallCount: 1, ProviderQPS: 1, HourlySteps: 1, DailySteps: 1, LifeIndexDays: 1,
			}
			test.mutate(&input)
			if _, err := BuildMallWeatherCapacityPlan(input); !errors.Is(err, ErrMallWeatherInvalidCapacityPlan) {
				t.Fatalf("BuildMallWeatherCapacityPlan() error=%v want %v", err, ErrMallWeatherInvalidCapacityPlan)
			}
		})
	}
}

func TestMallWeatherCapacityPlanServiceRequiresConfigPermission(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 23, 9, 0, 0, 0, time.UTC)
	permissions := &recordingMallPermissionChecker{allowed: map[string]bool{
		PermissionWeatherConfigManage: true,
	}}
	service, err := newMallWeatherCapacityPlanService(permissions, func() time.Time { return now })
	if err != nil {
		t.Fatalf("newMallWeatherCapacityPlanService() error=%v", err)
	}
	plan, err := service.Plan(context.Background(), 17, MallWeatherCapacityPlanInput{
		MallCount: 1, ProviderQPS: 1, HourlySteps: 24, DailySteps: 1, LifeIndexDays: 1,
	})
	if err != nil {
		t.Fatalf("Plan() error=%v", err)
	}
	if plan.ProviderRequests == 0 || !reflect.DeepEqual(permissions.requested, []string{PermissionWeatherConfigManage}) {
		t.Fatalf("plan=%+v permissions=%v", plan, permissions.requested)
	}
}

func TestMallWeatherCapacityPlanServiceRejectsUnauthorizedActor(t *testing.T) {
	t.Parallel()

	service, err := newMallWeatherCapacityPlanService(
		&recordingMallPermissionChecker{allowed: map[string]bool{}},
		time.Now,
	)
	if err != nil {
		t.Fatalf("newMallWeatherCapacityPlanService() error=%v", err)
	}
	_, err = service.Plan(context.Background(), 17, MallWeatherCapacityPlanInput{
		MallCount: 1, ProviderQPS: 1, HourlySteps: 24, DailySteps: 1, LifeIndexDays: 1,
	})
	if !errors.Is(err, ErrMallForbidden) {
		t.Fatalf("Plan() error=%v, want ErrMallForbidden", err)
	}
	if _, err = service.Plan(context.Background(), 0, MallWeatherCapacityPlanInput{}); !errors.Is(err, ErrMallForbidden) {
		t.Fatalf("Plan() anonymous error=%v, want ErrMallForbidden", err)
	}
}
