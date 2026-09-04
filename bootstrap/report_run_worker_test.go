package bootstrap

import (
	"context"
	"errors"
	"testing"

	"gin-biz-web-api/internal/service/data_svc"
	"gin-biz-web-api/job"

	"github.com/hibiken/asynq"
)

func TestReportRunHandlerDecodesStrictPayload(t *testing.T) {
	processor := &fakeReportRunProcessor{}
	task := asynq.NewTask(job.TypeReportRun, []byte(`{"run_id":31}`))
	if err := newReportRunHandler(processor)(t.Context(), task); err != nil {
		t.Fatalf("handler error = %v", err)
	}
	if processor.runID != 31 {
		t.Fatalf("run id = %d", processor.runID)
	}
	if processor.failureRetryLimit != job.ReportRunFailureMaxRetry {
		t.Fatalf("failure retry limit = %d", processor.failureRetryLimit)
	}
	if !processor.queueRetryAllowed {
		t.Fatal("queue retry should remain available without Asynq retry metadata")
	}
	bad := asynq.NewTask(job.TypeReportRun, []byte(`{"run_id":31,"secret":"x"}`))
	if err := newReportRunHandler(processor)(t.Context(), bad); !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("invalid payload error = %v", err)
	}
}

func TestReportRunHandlerSkipsPermanentFailures(t *testing.T) {
	processor := &fakeReportRunProcessor{err: data_svc.ErrReportRunProcessNonRetryable}
	err := newReportRunHandler(processor)(t.Context(), asynq.NewTask(job.TypeReportRun, []byte(`{"run_id":31}`)))
	if !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("handler error = %v", err)
	}
}

func TestReportWorkersUseDedicatedQueues(t *testing.T) {
	configured := map[string]int{
		"default":                  3,
		job.ReportQueueName:        99,
		job.ReportExportQueueName:  88,
		job.ReportCleanupQueueName: 77,
	}
	specs := queueJobServerSpecs(configured, 10, true, 2, 4, 5, 1)
	if len(specs) != 4 {
		t.Fatalf("server specs = %#v", specs)
	}
	if specs[0].name != "default" || specs[0].concurrency != 10 || len(specs[0].queues) != 1 || specs[0].queues["default"] != 3 {
		t.Fatalf("default server = %#v", specs[0])
	}
	if specs[1].name != "report run" || specs[1].concurrency != 4 || len(specs[1].queues) != 1 || specs[1].queues[job.ReportQueueName] != 2 {
		t.Fatalf("report run server = %#v", specs[1])
	}
	if specs[2].name != "report export" || specs[2].concurrency != 5 || len(specs[2].queues) != 1 || specs[2].queues[job.ReportExportQueueName] != 2 {
		t.Fatalf("report export server = %#v", specs[2])
	}
	if specs[3].name != "report cleanup" || specs[3].concurrency != 1 || len(specs[3].queues) != 1 || specs[3].queues[job.ReportCleanupQueueName] != 2 {
		t.Fatalf("report cleanup server = %#v", specs[3])
	}
	seen := make(map[string]string)
	for _, spec := range specs {
		for queue := range spec.queues {
			if owner, exists := seen[queue]; exists {
				t.Fatalf("queue %q is consumed by %q and %q", queue, owner, spec.name)
			}
			seen[queue] = spec.name
		}
	}
}

func TestQueueJobServerSpecsDoNotInventDefaultQueue(t *testing.T) {
	specs := queueJobServerSpecs(map[string]int{job.ReportQueueName: 2}, 10, true, 2, 4, 2, 1)
	if len(specs) != 3 || specs[0].name != "report run" || specs[1].name != "report export" || specs[2].name != "report cleanup" {
		t.Fatalf("server specs = %#v", specs)
	}
}

type fakeReportRunProcessor struct {
	runID             uint
	failureRetryLimit int
	queueRetryAllowed bool
	err               error
}

func (processor *fakeReportRunProcessor) Process(_ context.Context, runID uint, failureRetryLimit int, queueRetryAllowed bool) error {
	processor.runID = runID
	processor.failureRetryLimit = failureRetryLimit
	processor.queueRetryAllowed = queueRetryAllowed
	return processor.err
}
