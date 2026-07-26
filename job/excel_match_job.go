package job

import (
	"bytes"
	"encoding/json"
	"errors"
	"time"

	"github.com/hibiken/asynq"
)

const (
	TypeExcelMatchExport  = "excel:match_export"
	TypeExcelMatchCleanup = "excel:match_cleanup"

	ExcelMatchCleanupCron = "7,22,37,52 * * * *"
)

type ExcelMatchExportPayload struct {
	JobID uint `json:"job_id"`
}

func NewExcelMatchExportTask(jobID uint) (*asynq.Task, error) {
	payload, err := json.Marshal(ExcelMatchExportPayload{JobID: jobID})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(
		TypeExcelMatchExport,
		payload,
		asynq.Queue(DefaultQueueName),
		asynq.MaxRetry(1),
	), nil
}

func NewExcelMatchCleanupTask() (*asynq.Task, error) {
	return asynq.NewTask(
		TypeExcelMatchCleanup,
		[]byte(`{}`),
		asynq.Queue(DefaultQueueName),
		asynq.MaxRetry(5),
		asynq.Timeout(12*time.Minute),
		asynq.Unique(14*time.Minute),
	), nil
}

func DecodeExcelMatchCleanupTaskPayload(payload []byte) error {
	if !bytes.Equal(bytes.TrimSpace(payload), []byte(`{}`)) {
		return errors.New("excel match cleanup payload must be an empty object")
	}
	return nil
}
