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
	TypeOfficePush                 = "office:push"
	TypeOfficePushSchedule         = "office:push:schedule"
	OfficePushQueueName            = "delivery"
	OfficePushScheduleCron         = "* * * * *"
	OfficePushTaskTimeout          = 2 * time.Hour
	OfficePushScheduleTaskTimeout  = 50 * time.Second
	OfficePushScheduleUniquePeriod = 50 * time.Second
	OfficePushTaskMaxRetry         = 3
)

type OfficePushTaskPayload struct {
	RunID uint `json:"run_id"`
}

func NewOfficePushScheduleTask() (*asynq.Task, error) {
	payload := []byte(`{}`)
	if err := DecodeOfficePushScheduleTaskPayload(payload); err != nil {
		return nil, err
	}
	return asynq.NewTask(
		TypeOfficePushSchedule,
		payload,
		asynq.Queue(OfficePushQueueName),
		asynq.Timeout(OfficePushScheduleTaskTimeout),
	), nil
}

func DecodeOfficePushScheduleTaskPayload(payload []byte) error {
	var decoded map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil {
		return fmt.Errorf("office push schedule task: invalid payload")
	}
	if decoded == nil || len(decoded) != 0 {
		return fmt.Errorf("office push schedule task: invalid payload")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("office push schedule task: invalid payload")
	}
	return nil
}

func DecodeOfficePushTaskPayload(payload []byte) (OfficePushTaskPayload, error) {
	var decoded OfficePushTaskPayload
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil {
		return OfficePushTaskPayload{}, fmt.Errorf("office push task: invalid payload")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF || decoded.RunID == 0 {
		return OfficePushTaskPayload{}, fmt.Errorf("office push task: invalid payload")
	}
	return decoded, nil
}

func NewOfficePushTask(payload []byte) (*asynq.Task, error) {
	if _, err := DecodeOfficePushTaskPayload(payload); err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeOfficePush, append([]byte(nil), payload...), asynq.Queue(OfficePushQueueName)), nil
}

func OfficePushOutboxTaskDefinitions() []OutboxTaskDefinition {
	return []OutboxTaskDefinition{{
		TaskType: TypeOfficePush, Queue: OfficePushQueueName, MaxRetry: OfficePushTaskMaxRetry,
		Timeout: OfficePushTaskTimeout, Build: NewOfficePushTask,
	}}
}
