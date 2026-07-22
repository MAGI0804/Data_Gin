package bootstrap

import (
	"context"
	"errors"
	"testing"

	"gin-biz-web-api/internal/service/data_svc"
	"gin-biz-web-api/job"

	"github.com/hibiken/asynq"
)

func TestMallWeatherExportHandlerDecodesAndProcessesTask(t *testing.T) {
	processor := &fakeMallWeatherExportProcessor{}
	task, err := job.NewMallWeatherExportTask(job.MallWeatherExportTaskPayload{ExportJobID: 17})
	if err != nil {
		t.Fatalf("NewMallWeatherExportTask() error=%v", err)
	}
	if err := newMallWeatherExportHandler(processor).ProcessTask(t.Context(), task); err != nil {
		t.Fatalf("ProcessTask() error=%v", err)
	}
	if processor.jobID != 17 || !processor.retryAllowed {
		t.Fatalf("processor=%+v", processor)
	}
}

func TestMallWeatherExportHandlerSkipsPermanentFailures(t *testing.T) {
	tests := []struct {
		name      string
		task      *asynq.Task
		processor mallWeatherExportProcessor
	}{
		{name: "invalid payload", task: asynq.NewTask(job.TypeMallWeatherExport, []byte(`{"export_job_id":0}`)), processor: &fakeMallWeatherExportProcessor{}},
		{name: "permanent process failure", task: asynq.NewTask(job.TypeMallWeatherExport, []byte(`{"export_job_id":17}`)), processor: &fakeMallWeatherExportProcessor{err: data_svc.ErrMallWeatherExportProcessNonRetryable}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := newMallWeatherExportHandler(tt.processor).ProcessTask(context.Background(), tt.task)
			if err == nil || !errors.Is(err, asynq.SkipRetry) {
				t.Fatalf("ProcessTask() error=%v", err)
			}
		})
	}
}

func TestMallWeatherExportCleanupHandlerRunsCleaner(t *testing.T) {
	cleaner := &fakeMallWeatherExportCleaner{}
	task, err := job.NewMallWeatherExportCleanupTask()
	if err != nil {
		t.Fatalf("NewMallWeatherExportCleanupTask() error=%v", err)
	}
	if err := newMallWeatherExportCleanupHandler(cleaner).ProcessTask(t.Context(), task); err != nil {
		t.Fatalf("ProcessTask() error=%v", err)
	}
	if cleaner.calls != 1 {
		t.Fatalf("cleaner calls=%d", cleaner.calls)
	}
}

func TestMallWeatherExportCleanupHandlerSkipsInvalidPayload(t *testing.T) {
	err := newMallWeatherExportCleanupHandler(&fakeMallWeatherExportCleaner{}).ProcessTask(
		t.Context(),
		asynq.NewTask(job.TypeMallWeatherExportCleanup, []byte(`{"secret":"x"}`)),
	)
	if !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("ProcessTask() error=%v", err)
	}
}

type fakeMallWeatherExportProcessor struct {
	jobID        uint
	retryAllowed bool
	err          error
}

func (processor *fakeMallWeatherExportProcessor) Process(
	_ context.Context,
	jobID uint,
	retryAllowed bool,
) error {
	processor.jobID = jobID
	processor.retryAllowed = retryAllowed
	return processor.err
}

var _ mallWeatherExportProcessor = (*fakeMallWeatherExportProcessor)(nil)

type fakeMallWeatherExportCleaner struct {
	calls int
	err   error
}

func (cleaner *fakeMallWeatherExportCleaner) Cleanup(
	context.Context,
) (data_svc.MallWeatherExportCleanupResult, error) {
	cleaner.calls++
	return data_svc.MallWeatherExportCleanupResult{Scanned: 2, Claimed: 1, Expired: 1, Deleted: 1}, cleaner.err
}

var _ mallWeatherExportCleaner = (*fakeMallWeatherExportCleaner)(nil)
