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
	TypeOfficePush         = "office:push"
	OfficePushQueueName    = "delivery"
	OfficePushTaskTimeout  = 2 * time.Hour
	OfficePushTaskMaxRetry = 3
)

type OfficePushTaskPayload struct {
	RunID uint `json:"run_id"`
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
