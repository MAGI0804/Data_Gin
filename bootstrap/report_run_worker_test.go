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

func TestReportWorkerQueuesAreEnabledAtomically(t *testing.T) {
	configured := map[string]int{"default": 3, job.ReportQueueName: 99}
	disabled := reportWorkerQueues(configured, false, 2)
	if _, exists := disabled[job.ReportQueueName]; exists {
		t.Fatalf("disabled queues contain report: %#v", disabled)
	}
	enabled := reportWorkerQueues(configured, true, 2)
	if enabled[job.ReportQueueName] != 2 || configured[job.ReportQueueName] != 99 {
		t.Fatalf("enabled=%#v configured=%#v", enabled, configured)
	}
}

type fakeReportRunProcessor struct {
	runID uint
	err   error
}

func (processor *fakeReportRunProcessor) Process(_ context.Context, runID uint, _ bool) error {
	processor.runID = runID
	return processor.err
}
