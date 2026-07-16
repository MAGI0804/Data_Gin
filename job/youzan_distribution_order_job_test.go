package job

import (
	"testing"
	"time"
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
