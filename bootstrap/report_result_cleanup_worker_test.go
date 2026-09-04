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

func TestReportResultCleanupHandlerRunsTargetedCleaner(t *testing.T) {
	cleaner := &fakeReportResultCleaner{}
	task, err := job.NewReportResultCleanupRunTask(24)
	if err != nil {
		t.Fatalf("NewReportResultCleanupRunTask() error=%v", err)
	}
	if err := newReportResultCleanupHandler(cleaner).ProcessTask(t.Context(), task); err != nil {
		t.Fatalf("ProcessTask() error=%v", err)
	}
	if cleaner.targetRunID != 24 || cleaner.calls != 0 {
		t.Fatalf("targetRunID=%d full cleanup calls=%d", cleaner.targetRunID, cleaner.calls)
	}
}

type fakeReportResultCleaner struct {
	calls       int
	targetRunID uint
}

func (cleaner *fakeReportResultCleaner) Cleanup(context.Context) (data_svc.ReportResultCleanupResult, error) {
	cleaner.calls++
	return data_svc.ReportResultCleanupResult{}, nil
}

func (cleaner *fakeReportResultCleaner) CleanupRun(_ context.Context, runID uint) (bool, error) {
	cleaner.targetRunID = runID
	return true, nil
}
