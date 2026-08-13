package bootstrap

import (
	"context"
	"errors"
	"testing"

	"gin-biz-web-api/internal/service/data_svc"
	"gin-biz-web-api/job"

	"github.com/hibiken/asynq"
)

func TestReportExportCleanupHandlerRunsCleaner(t *testing.T) {
	cleaner := &fakeReportExportCleaner{}
	task, err := job.NewReportExportCleanupTask()
	if err != nil {
		t.Fatalf("NewReportExportCleanupTask() error=%v", err)
	}
	if err := newReportExportCleanupHandler(cleaner).ProcessTask(t.Context(), task); err != nil {
		t.Fatalf("ProcessTask() error=%v", err)
	}
	if cleaner.calls != 1 {
		t.Fatalf("cleaner calls=%d", cleaner.calls)
	}
}

func TestReportExportCleanupHandlerSkipsInvalidTask(t *testing.T) {
	cleaner := &fakeReportExportCleaner{}
	for _, task := range []*asynq.Task{
		asynq.NewTask(job.TypeReportExportCleanup, []byte(`{"secret":"x"}`)),
		asynq.NewTask(job.TypeReportExport, []byte(`{}`)),
	} {
		if err := newReportExportCleanupHandler(cleaner).ProcessTask(t.Context(), task); !errors.Is(err, asynq.SkipRetry) {
			t.Fatalf("ProcessTask() error=%v", err)
		}
	}
}

type fakeReportExportCleaner struct {
	calls int
	err   error
}

func (cleaner *fakeReportExportCleaner) Cleanup(context.Context) (data_svc.ReportExportCleanupResult, error) {
	cleaner.calls++
	return data_svc.ReportExportCleanupResult{Scanned: 1, Claimed: 1, Expired: 1, Deleted: 1}, cleaner.err
}

var _ reportExportCleaner = (*fakeReportExportCleaner)(nil)
