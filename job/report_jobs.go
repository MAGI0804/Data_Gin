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
	TypeReportRun              = "report:run"
	TypeReportExport           = "report:export"
	TypeReportExportCleanup    = "report:export_cleanup"
	TypeReportResultCleanup    = "report:result_cleanup"
	ReportQueueName            = "report"
	ReportExportQueueName      = "report_export"
	ReportCleanupQueueName     = "report_cleanup"
	ReportRunTimeout           = 35 * time.Minute
	ReportExportTimeout        = 6 * time.Hour
	ReportExportCleanupTimeout = 30 * time.Minute
	ReportExportCleanupUnique  = 50 * time.Minute
	ReportExportCleanupCron    = "37 * * * *"
	ReportResultCleanupCron    = "7,17,27,37,47,57 * * * *"
	ReportRunFailureMaxRetry   = 3
	// Snapshot runs wait behind the report-scoped Oracle result table lock.
	// Business failures are capped separately; this allows about 12 hours.
	ReportRunRetryDelay        = 15 * time.Second
	ReportRunMaxRetry          = 2880
	ReportRunSnapshotWaitLimit = ReportRunMaxRetry * ReportRunRetryDelay
	ReportExportMaxRetry       = 3
)

type ReportRunTaskPayload struct {
	RunID uint `json:"run_id"`
}

type ReportExportTaskPayload struct {
	ExportID uint `json:"export_id"`
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

func DecodeReportExportTaskPayload(payload []byte) (ReportExportTaskPayload, error) {
	var decoded ReportExportTaskPayload
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil {
		return ReportExportTaskPayload{}, fmt.Errorf("report export task: invalid payload")
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); err != io.EOF || decoded.ExportID == 0 {
		return ReportExportTaskPayload{}, fmt.Errorf("report export task: invalid payload")
	}
	return decoded, nil
}

func NewReportExportTask(payload []byte) (*asynq.Task, error) {
	if _, err := DecodeReportExportTaskPayload(payload); err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeReportExport, append([]byte(nil), payload...), asynq.Queue(ReportExportQueueName)), nil
}

func NewReportExportCleanupTask() (*asynq.Task, error) {
	return asynq.NewTask(
		TypeReportExportCleanup,
		[]byte(`{}`),
		asynq.Queue(ReportCleanupQueueName),
		asynq.Timeout(ReportExportCleanupTimeout),
		asynq.MaxRetry(5),
		asynq.Unique(ReportExportCleanupUnique),
	), nil
}

func DecodeReportExportCleanupTaskPayload(payload []byte) error {
	return decodeEmptyReportTaskPayload(payload, "report export cleanup task")
}

func NewReportResultCleanupTask() (*asynq.Task, error) {
	return asynq.NewTask(
		TypeReportResultCleanup,
		[]byte(`{}`),
		asynq.Queue(ReportCleanupQueueName),
		asynq.Timeout(ReportExportCleanupTimeout),
		asynq.MaxRetry(5),
		asynq.Unique(9*time.Minute),
	), nil
}

func DecodeReportResultCleanupTaskPayload(payload []byte) error {
	return decodeEmptyReportTaskPayload(payload, "report result cleanup task")
}

func decodeEmptyReportTaskPayload(payload []byte, label string) error {
	var decoded map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil || decoded == nil || len(decoded) != 0 {
		return fmt.Errorf("%s: invalid payload", label)
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("%s: invalid payload", label)
	}
	return nil
}

func ReportOutboxTaskDefinitions(maxRetry int) []OutboxTaskDefinition {
	return []OutboxTaskDefinition{{
		TaskType: TypeReportRun, Queue: ReportQueueName, MaxRetry: maxRetry, Timeout: ReportRunTimeout,
		RecoverTaskIDConflict: true, Build: NewReportRunTask,
	}}
}

// ReportExportOutboxTaskDefinitions is registered only when the matching
// export worker is enabled, so rolling deployments never publish tasks to an
// instance without a handler.
func ReportExportOutboxTaskDefinitions(maxRetry int) []OutboxTaskDefinition {
	return []OutboxTaskDefinition{{
		TaskType: TypeReportExport, Queue: ReportExportQueueName, MaxRetry: maxRetry, Timeout: ReportExportTimeout,
		RecoverTaskIDConflict: true, Build: NewReportExportTask,
	}}
}
