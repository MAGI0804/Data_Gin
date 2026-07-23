package job

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gin-biz-web-api/pkg/youzan"

	"github.com/hibiken/asynq"
)

const TypeYouzanDistributionOrderSync = "youzan:distribution:orders:sync"

type YouzanDistributionOrderSyncPayload struct {
	TimeFilter youzan.OrderTimeFilter `json:"time_filter,omitempty"`
	StartTime  string                 `json:"start_time"`
	EndTime    string                 `json:"end_time"`
}

func NewYouzanDistributionOrderSyncTask(payload YouzanDistributionOrderSyncPayload) (*asynq.Task, error) {
	timeFilter, err := youzan.ParseOrderTimeFilter(string(payload.TimeFilter))
	if err != nil {
		return nil, err
	}
	payload.TimeFilter = timeFilter
	payload.StartTime = strings.TrimSpace(payload.StartTime)
	payload.EndTime = strings.TrimSpace(payload.EndTime)
	if (payload.StartTime == "") != (payload.EndTime == "") {
		return nil, fmt.Errorf("youzan distribution order sync requires both start_time and end_time")
	}
	if payload.StartTime != "" {
		if _, _, err := validateYouzanDistributionRange(payload.StartTime, payload.EndTime); err != nil {
			return nil, err
		}
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(
		TypeYouzanDistributionOrderSync,
		data,
		asynq.Queue(DefaultQueueName),
		asynq.MaxRetry(3),
	), nil
}

func ResolveYouzanDistributionOrderTimeFilter(payload YouzanDistributionOrderSyncPayload) (youzan.OrderTimeFilter, error) {
	return youzan.ParseOrderTimeFilter(string(payload.TimeFilter))
}

func ResolveYouzanDistributionOrderRange(payload YouzanDistributionOrderSyncPayload, now time.Time) (string, string) {
	if strings.TrimSpace(payload.StartTime) != "" && strings.TrimSpace(payload.EndTime) != "" {
		return strings.TrimSpace(payload.StartTime), strings.TrimSpace(payload.EndTime)
	}
	return YouzanDistributionPreviousDayRange(now)
}

func YouzanDistributionPreviousDayRange(now time.Time) (string, string) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		location = time.FixedZone("CST", 8*60*60)
	}
	localNow := now.In(location)
	previousDay := time.Date(localNow.Year(), localNow.Month(), localNow.Day()-1, 0, 0, 0, 0, location)
	return previousDay.Format("2006-01-02 15:04:05"), previousDay.Add(24*time.Hour - time.Second).Format("2006-01-02 15:04:05")
}

func validateYouzanDistributionRange(startTime, endTime string) (time.Time, time.Time, error) {
	const layout = "2006-01-02 15:04:05"
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		location = time.FixedZone("CST", 8*60*60)
	}
	start, err := time.ParseInLocation(layout, startTime, location)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("start_time must use %s", layout)
	}
	end, err := time.ParseInLocation(layout, endTime, location)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("end_time must use %s", layout)
	}
	if start.After(end) {
		return time.Time{}, time.Time{}, fmt.Errorf("start_time must not be after end_time")
	}
	return start, end, nil
}
