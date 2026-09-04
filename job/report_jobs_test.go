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
	registry, err := NewOutboxTaskRegistry(ReportOutboxTaskDefinitions(ReportRunMaxRetry)...)
	if err != nil {
		t.Fatalf("NewOutboxTaskRegistry() error = %v", err)
	}
	task, options, err := registry.Resolve(model.AsyncJobOutbox{
		TaskKey: "report:run:run-uuid", TaskType: TypeReportRun,
		QueueName: ReportQueueName, PayloadJSON: model.JSONText(`{"run_id":31}`),
	})
	if err != nil || task.Type() != TypeReportRun || options.TaskID != "report:run:run-uuid" ||
		!options.RecoverTaskIDConflict || options.Queue != ReportQueueName ||
		options.Timeout != ReportRunTimeout || options.MaxRetry != ReportRunMaxRetry {
		t.Fatalf("Resolve() task=%#v options=%#v error=%v", task, options, err)
	}
	if options.Timeout != 35*time.Minute {
		t.Fatalf("report timeout = %v", options.Timeout)
	}
}

func TestReportExportTaskAcceptsOnlyExportID(t *testing.T) {
	valid, err := NewReportExportTask([]byte(`{"export_id":41}`))
	if err != nil || valid.Type() != TypeReportExport {
		t.Fatalf("NewReportExportTask() task=%#v error=%v", valid, err)
	}
	for _, payload := range []string{`{"export_id":0}`, `{"export_id":41,"password":"secret"}`, `{"export_id":41}{}`} {
		if _, err := NewReportExportTask([]byte(payload)); err == nil {
			t.Fatalf("NewReportExportTask() accepted %s", payload)
		}
	}
}

func TestOutboxRegistryResolvesReportExport(t *testing.T) {
	registry, err := NewOutboxTaskRegistry(ReportExportOutboxTaskDefinitions(ReportExportMaxRetry)...)
	if err != nil {
		t.Fatalf("NewOutboxTaskRegistry() error = %v", err)
	}
	task, options, err := registry.Resolve(model.AsyncJobOutbox{
		TaskKey: "report:export:export-uuid", TaskType: TypeReportExport,
		QueueName: ReportExportQueueName, PayloadJSON: model.JSONText(`{"export_id":41}`),
	})
	if err != nil || task.Type() != TypeReportExport || !options.RecoverTaskIDConflict ||
		options.Timeout != ReportExportTimeout || options.MaxRetry != ReportExportMaxRetry {
		t.Fatalf("Resolve() task=%#v options=%#v error=%v", task, options, err)
	}
}

func TestReportExportCleanupTaskIsStrictAndScheduledForCleanupQueue(t *testing.T) {
	task, err := NewReportExportCleanupTask()
	if err != nil {
		t.Fatalf("NewReportExportCleanupTask() error=%v", err)
	}
	if task.Type() != TypeReportExportCleanup || string(task.Payload()) != `{}` {
		t.Fatalf("cleanup task type=%q payload=%q", task.Type(), task.Payload())
	}
	if err := DecodeReportExportCleanupTaskPayload([]byte(`{"secret":"x"}`)); err == nil {
		t.Fatal("DecodeReportExportCleanupTaskPayload() accepted unknown field")
	}
	if ReportExportCleanupCron != "37 * * * *" || ReportExportCleanupTimeout != 30*time.Minute {
		t.Fatalf("cleanup schedule=%q timeout=%v", ReportExportCleanupCron, ReportExportCleanupTimeout)
	}
}

func TestReportResultCleanupTaskIsStrictAndScheduledForCleanupQueue(t *testing.T) {
	task, err := NewReportResultCleanupTask()
	if err != nil {
		t.Fatalf("NewReportResultCleanupTask() error=%v", err)
	}
	if task.Type() != TypeReportResultCleanup || string(task.Payload()) != `{}` {
		t.Fatalf("result cleanup task type=%q payload=%q", task.Type(), task.Payload())
	}
	if err := DecodeReportResultCleanupTaskPayload([]byte(`{"extra":true}`)); err == nil {
		t.Fatal("DecodeReportResultCleanupTaskPayload() accepted unknown field")
	}
}
