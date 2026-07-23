package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"gin-biz-web-api/internal/service/data_svc"
)

func TestMallWeatherCapacityPlanCommandOutputsJSON(t *testing.T) {
	t.Parallel()

	command := NewMallWeatherCmd()
	var stdout bytes.Buffer
	command.SetOut(&stdout)
	command.SetErr(&bytes.Buffer{})
	command.SetArgs([]string{
		"capacity-plan",
		"--mall-count", "1000",
		"--provider-qps", "20",
		"--alerts-per-mall", "3",
	})

	if err := command.Execute(); err != nil {
		t.Fatalf("Execute() error=%v", err)
	}
	var plan data_svc.MallWeatherCapacityPlan
	if err := json.Unmarshal(stdout.Bytes(), &plan); err != nil {
		t.Fatalf("capacity plan output is not JSON: %v body=%s", err, stdout.String())
	}
	if plan.ProviderRequests != 192000 || plan.WeatherV26ProviderRequestsPerDay != 168000 ||
		plan.LifeIndexV3ProviderRequestsPerDay != 24000 || plan.ProviderDrainSeconds != 9600 ||
		plan.TotalDatabaseRows != 514000 || len(plan.Datasets) != 6 {
		t.Fatalf("plan=%+v", plan)
	}
}

func TestMallWeatherCapacityPlanCommandRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	command := NewMallWeatherCmd()
	command.SetOut(&bytes.Buffer{})
	command.SetErr(&bytes.Buffer{})
	command.SetArgs([]string{
		"capacity-plan",
		"--mall-count", "1000",
		"--provider-qps", "0",
	})

	err := command.Execute()
	if !errors.Is(err, data_svc.ErrMallWeatherInvalidCapacityPlan) {
		t.Fatalf("Execute() error=%v want %v", err, data_svc.ErrMallWeatherInvalidCapacityPlan)
	}
}

func TestMallWeatherCapacityPlanCommandRequiresCoreFlags(t *testing.T) {
	t.Parallel()

	command := NewMallWeatherCmd()
	command.SetOut(&bytes.Buffer{})
	command.SetErr(&bytes.Buffer{})
	command.SetArgs([]string{"capacity-plan", "--provider-qps", "20"})

	err := command.Execute()
	if err == nil || !strings.Contains(err.Error(), `required flag(s) "mall-count" not set`) {
		t.Fatalf("Execute() error=%v want missing mall-count", err)
	}
}
