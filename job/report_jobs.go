package job

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/hibiken/asynq"
)

const (
	TypeReportRun    = "report:run"
	ReportQueueName  = "report"
	ReportRunTimeout = 35 * time.Minute
)

type ReportRunTaskPayload struct {
	RunID uint `json:"run_id"`
}

func DecodeReportRunTaskPayload(payload []byte) (ReportRunTaskPayload, error) {
	var decoded ReportRunTaskPayload
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil {
		return ReportRunTaskPayload{}, fmt.Errorf("report run task: invalid payload")
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		return ReportRunTaskPayload{}, fmt.Errorf("report run task: invalid payload")
	}
	if decoded.RunID == 0 {
		return ReportRunTaskPayload{}, fmt.Errorf("report run task: run id is required")
	}
	return decoded, nil
}

func NewReportRunTask(payload []byte) (*asynq.Task, error) {
	if _, err := DecodeReportRunTaskPayload(payload); err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeReportRun, append([]byte(nil), payload...), asynq.Queue(ReportQueueName)), nil
}

func ReportOutboxTaskDefinitions(maxRetry int) []OutboxTaskDefinition {
	return []OutboxTaskDefinition{{
		TaskType: TypeReportRun, Queue: ReportQueueName, MaxRetry: maxRetry,
		Timeout: ReportRunTimeout, Build: NewReportRunTask,
	}}
}
