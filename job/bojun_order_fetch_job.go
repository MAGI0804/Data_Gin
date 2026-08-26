package job

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hibiken/asynq"
)

const TypeBojunOrderFetch = "bojun:order:fetch"

const bojunOrderFetchUniqueTTL = 30 * time.Minute

type BojunOrderFetchPayload struct {
	SourceMode string `json:"source_mode"`
	StartTime  string `json:"start_time"`
	EndTime    string `json:"end_time"`
}

func NewBojunOrderFetchTask(payload BojunOrderFetchPayload) (*asynq.Task, error) {
	payload.SourceMode = strings.ToLower(strings.TrimSpace(payload.SourceMode))
	if payload.SourceMode == "" {
		payload.SourceMode = "api"
	}
	if payload.SourceMode != "api" && payload.SourceMode != "oracle" {
		return nil, fmt.Errorf("bojun order fetch source_mode must be api or oracle")
	}
	payload.StartTime = strings.TrimSpace(payload.StartTime)
	payload.EndTime = strings.TrimSpace(payload.EndTime)
	if payload.StartTime == "" || payload.EndTime == "" {
		return nil, fmt.Errorf("bojun order fetch requires start_time and end_time")
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(
		TypeBojunOrderFetch,
		data,
		asynq.Queue(DefaultQueueName),
		asynq.MaxRetry(1),
		asynq.Unique(bojunOrderFetchUniqueTTL),
	), nil
}
