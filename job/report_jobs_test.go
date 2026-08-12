package job

import (
	"testing"
	"time"

	"gin-biz-web-api/model"
)

func TestReportRunTaskAcceptsOnlyRunID(t *testing.T) {
	for _, test := range []struct {
		name    string
		payload string
		valid   bool
	}{
		{name: "valid", payload: `{"run_id":31}`, valid: true},
		{name: "zero", payload: `{"run_id":0}`},
		{name: "secret smuggling", payload: `{"run_id":31,"password":"secret"}`},
		{name: "trailing json", payload: `{"run_id":31}{}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			task, err := NewReportRunTask([]byte(test.payload))
			if test.valid {
				if err != nil || task.Type() != TypeReportRun || string(task.Payload()) != test.payload {
					t.Fatalf("NewReportRunTask() task=%#v error=%v", task, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("NewReportRunTask() unexpectedly accepted %s", test.payload)
			}
		})
	}
}

func TestOutboxRegistryResolvesReportRun(t *testing.T) {
	registry, err := NewOutboxTaskRegistry(ReportOutboxTaskDefinitions(0)...)
	if err != nil {
		t.Fatalf("NewOutboxTaskRegistry() error = %v", err)
	}
	task, options, err := registry.Resolve(model.AsyncJobOutbox{
		TaskKey: "report:run:run-uuid", TaskType: TypeReportRun,
		QueueName: ReportQueueName, PayloadJSON: model.JSONText(`{"run_id":31}`),
	})
	if err != nil || task.Type() != TypeReportRun || options.TaskID != "report:run:run-uuid" ||
		options.Queue != ReportQueueName || options.Timeout != ReportRunTimeout || options.MaxRetry != 0 {
		t.Fatalf("Resolve() task=%#v options=%#v error=%v", task, options, err)
	}
	if options.Timeout != 35*time.Minute {
		t.Fatalf("report timeout = %v", options.Timeout)
	}
}
