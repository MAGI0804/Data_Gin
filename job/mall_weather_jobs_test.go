package job

import (
	"strings"
	"testing"

	"github.com/hibiken/asynq"
)

func TestMallWeatherTaskConstructorsUseNonSensitivePayloads(t *testing.T) {
	tests := []struct {
		name     string
		taskType string
		queue    string
		create   func() (*asynq.Task, error)
	}{
		{"geocode", TypeMallGeocode, MallWeatherQueueName, func() (*asynq.Task, error) { return NewMallGeocodeTask(MallTaskPayload{MallID: 11}) }},
		{"fast", TypeMallWeatherFast, MallWeatherQueueName, func() (*asynq.Task, error) { return NewMallWeatherFastTask(MallTaskPayload{MallID: 11}) }},
		{"full", TypeMallWeatherFull, MallWeatherQueueName, func() (*asynq.Task, error) { return NewMallWeatherFullTask(MallTaskPayload{MallID: 11}) }},
		{"life index", TypeMallWeatherLifeIndex, MallWeatherQueueName, func() (*asynq.Task, error) { return NewMallWeatherLifeIndexTask(MallTaskPayload{MallID: 11}) }},
		{"repair", TypeMallWeatherRepair, MallWeatherQueueName, func() (*asynq.Task, error) { return NewMallWeatherRepairTask(MallTaskPayload{MallID: 11}) }},
		{"manual", TypeMallWeatherManual, MallWeatherQueueName, func() (*asynq.Task, error) { return NewMallWeatherManualTask(MallTaskPayload{MallID: 11}) }},
		{"export", TypeMallWeatherExport, MallExportQueueName, func() (*asynq.Task, error) {
			return NewMallWeatherExportTask(MallWeatherExportTaskPayload{ExportJobID: 22})
		}},
		{"feishu", TypeMallWeatherFeishu, MallDeliveryQueueName, func() (*asynq.Task, error) {
			return NewMallWeatherFeishuTask(MallWeatherFeishuTaskPayload{PipelineRunID: 33})
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task, err := tt.create()
			if err != nil {
				t.Fatalf("create task: %v", err)
			}
			if task.Type() != tt.taskType {
				t.Fatalf("task type = %q, want %q", task.Type(), tt.taskType)
			}
			queue, ok := ExpectedMallWeatherQueue(task.Type())
			if !ok || queue != tt.queue {
				t.Fatalf("queue = %q, %v, want %q, true", queue, ok, tt.queue)
			}

			payload := strings.ToLower(string(task.Payload()))
			for _, forbidden := range []string{"key", "secret", "token", "password", "url"} {
				if strings.Contains(payload, forbidden) {
					t.Fatalf("payload contains forbidden credential or URL marker %q: %s", forbidden, payload)
				}
			}
		})
	}
}

func TestNewMallWeatherTaskRejectsInvalidPayload(t *testing.T) {
	tests := []struct {
		name     string
		taskType string
		payload  string
	}{
		{"unknown type", "mall:weather:unknown", `{"mall_id":1}`},
		{"missing id", TypeMallWeatherFast, `{"mall_id":0}`},
		{"credential field", TypeMallWeatherFast, `{"mall_id":1,"app_secret":"do-not-queue"}`},
		{"URL field", TypeMallWeatherFast, `{"mall_id":1,"provider_url":"https://example.invalid"}`},
		{"multiple values", TypeMallWeatherFast, `{"mall_id":1}{"mall_id":2}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewMallWeatherTask(tt.taskType, []byte(tt.payload)); err == nil {
				t.Fatal("NewMallWeatherTask() error = nil")
			}
		})
	}
}

func TestMallWeatherTaskTypesReturnsCopy(t *testing.T) {
	types := MallWeatherTaskTypes()
	types[0] = "changed"
	if MallWeatherTaskTypes()[0] == "changed" {
		t.Fatal("MallWeatherTaskTypes returned shared storage")
	}
}
