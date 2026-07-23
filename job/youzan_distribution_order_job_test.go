package job

import (
	"encoding/json"
	"testing"
	"time"

	"gin-biz-web-api/pkg/youzan"
)

func TestYouzanDistributionPreviousDayRangeUsesShanghaiCalendarDay(t *testing.T) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 16, 1, 10, 0, 0, location)

	startTime, endTime := YouzanDistributionPreviousDayRange(now)
	if startTime != "2026-07-15 00:00:00" || endTime != "2026-07-15 23:59:59" {
		t.Fatalf("range = %s ~ %s", startTime, endTime)
	}
}

func TestNewYouzanDistributionOrderSyncTaskRejectsHalfRange(t *testing.T) {
	_, err := NewYouzanDistributionOrderSyncTask(YouzanDistributionOrderSyncPayload{StartTime: "2026-07-05 00:00:00"})
	if err == nil {
		t.Fatal("expected an error when only start_time is provided")
	}
}

func TestNewYouzanDistributionOrderSyncTaskNormalizesTimeFilter(t *testing.T) {
	tests := []struct {
		name       string
		input      youzan.OrderTimeFilter
		expected   youzan.OrderTimeFilter
		shouldFail bool
	}{
		{name: "default created", expected: youzan.OrderTimeFilterCreated},
		{name: "created", input: youzan.OrderTimeFilterCreated, expected: youzan.OrderTimeFilterCreated},
		{name: "success", input: youzan.OrderTimeFilterSuccess, expected: youzan.OrderTimeFilterSuccess},
		{name: "unsupported", input: "pay", shouldFail: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			task, err := NewYouzanDistributionOrderSyncTask(YouzanDistributionOrderSyncPayload{TimeFilter: test.input})
			if test.shouldFail {
				if err == nil {
					t.Fatal("NewYouzanDistributionOrderSyncTask() error = nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("NewYouzanDistributionOrderSyncTask() error = %v", err)
			}
			var payload YouzanDistributionOrderSyncPayload
			if err := json.Unmarshal(task.Payload(), &payload); err != nil {
				t.Fatalf("decode task payload: %v", err)
			}
			if payload.TimeFilter != test.expected {
				t.Errorf("time_filter = %q, want %q", payload.TimeFilter, test.expected)
			}
		})
	}
}
