package job

import (
	"encoding/json"

	"github.com/hibiken/asynq"
)

const TypeDeliveryTaskRun = "delivery:task:run"

type DeliveryTaskRunPayload struct {
	TaskID uint `json:"task_id"`
}

func NewDeliveryTaskRunTask(payload DeliveryTaskRunPayload) (*asynq.Task, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(
		TypeDeliveryTaskRun,
		data,
		asynq.Queue(YouzanSyncQueueName),
		asynq.MaxRetry(3),
	), nil
}
