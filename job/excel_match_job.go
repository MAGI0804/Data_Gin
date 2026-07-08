package job

import (
	"encoding/json"

	"github.com/hibiken/asynq"
)

const TypeExcelMatchExport = "excel:match_export"

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
