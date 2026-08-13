package bootstrap

import (
	"context"
	"errors"
	"testing"

	"gin-biz-web-api/internal/service/data_svc"
	"gin-biz-web-api/job"

	"github.com/hibiken/asynq"
)

func TestReportResultCleanupHandlerRunsCleaner(t *testing.T) {
	cleaner := &fakeReportResultCleaner{}
	task, err := job.NewReportResultCleanupTask()
	if err != nil {
		t.Fatalf("NewReportResultCleanupTask() error=%v", err)
	}
	if err := newReportResultCleanupHandler(cleaner).ProcessTask(t.Context(), task); err != nil {
		t.Fatalf("ProcessTask() error=%v", err)
	}
	if cleaner.calls != 1 {
		t.Fatalf("cleaner calls=%d", cleaner.calls)
	}
}

func TestReportResultCleanupHandlerSkipsInvalidTask(t *testing.T) {
	err := newReportResultCleanupHandler(&fakeReportResultCleaner{}).ProcessTask(t.Context(), asynq.NewTask(job.TypeReportResultCleanup, []byte(`{"extra":true}`)))
	if !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("ProcessTask() error=%v", err)
	}
}

type fakeReportResultCleaner struct{ calls int }

func (cleaner *fakeReportResultCleaner) Cleanup(context.Context) (data_svc.ReportResultCleanupResult, error) {
	cleaner.calls++
	return data_svc.ReportResultCleanupResult{}, nil
}
