package bootstrap

import (
	"context"
	"errors"
	"testing"

	"gin-biz-web-api/job"

	"github.com/hibiken/asynq"
)

func TestExcelMatchCleanupHandlerRunsCleaner(t *testing.T) {
	cleaner := &fakeExcelMatchCleanupRunner{}
	task, err := job.NewExcelMatchCleanupTask()
	if err != nil {
		t.Fatalf("NewExcelMatchCleanupTask() error=%v", err)
	}
	if err := newExcelMatchCleanupHandler(cleaner).ProcessTask(t.Context(), task); err != nil {
		t.Fatalf("ProcessTask() error=%v", err)
	}
	if cleaner.calls != 1 {
		t.Fatalf("cleaner calls=%d", cleaner.calls)
	}
}

func TestExcelMatchCleanupHandlerSkipsInvalidPayload(t *testing.T) {
	err := newExcelMatchCleanupHandler(&fakeExcelMatchCleanupRunner{}).ProcessTask(
		t.Context(),
		asynq.NewTask(job.TypeExcelMatchCleanup, []byte(`{"secret":"x"}`)),
	)
	if !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("ProcessTask() error=%v", err)
	}
}

func TestExcelMatchCleanupHandlerReturnsRetryableCleanupError(t *testing.T) {
	wantErr := errors.New("temporary OSS failure")
	err := newExcelMatchCleanupHandler(&fakeExcelMatchCleanupRunner{err: wantErr}).ProcessTask(
		t.Context(),
		asynq.NewTask(job.TypeExcelMatchCleanup, []byte(`{}`)),
	)
	if !errors.Is(err, wantErr) || errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("ProcessTask() error=%v", err)
	}
}

type fakeExcelMatchCleanupRunner struct {
	calls int
	err   error
}

func (cleaner *fakeExcelMatchCleanupRunner) CleanupExpiredJobs(context.Context) error {
	cleaner.calls++
	return cleaner.err
}

var _ excelMatchCleanupRunner = (*fakeExcelMatchCleanupRunner)(nil)
