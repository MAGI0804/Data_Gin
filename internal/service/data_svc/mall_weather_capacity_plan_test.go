package data_svc

import (
	"errors"
	"math"
	"reflect"
	"testing"
)

func TestBuildMallWeatherCapacityPlanEstimatesProviderAndStoragePressure(t *testing.T) {
	t.Parallel()

	plan, err := BuildMallWeatherCapacityPlan(MallWeatherCapacityPlanInput{
		MallCount: 5, ProviderQPS: 2.5, HourlySteps: 360, DailySteps: 15, LifeIndexDays: 15, AlertsPerMall: 3,
	})
	if err != nil {
		t.Fatalf("BuildMallWeatherCapacityPlan() error=%v", err)
	}
	if plan.WeatherV26ProviderRequestsPerDay != 840 || plan.LifeIndexV3ProviderRequestsPerDay != 120 ||
		plan.ProviderRequests != 960 || plan.ProviderDrainSeconds != 384 ||
		plan.MinimumQPSForOneHourDrain != float64(960)/3600 {
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
