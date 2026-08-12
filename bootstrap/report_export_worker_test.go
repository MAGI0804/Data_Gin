package bootstrap

import (
	"context"
	"errors"
	"testing"

	"gin-biz-web-api/internal/service/data_svc"
	"gin-biz-web-api/job"

	"github.com/hibiken/asynq"
)

func TestReportExportHandlerDecodesStrictPayload(t *testing.T) {
	processor := &fakeReportExportProcessor{}
	if err := newReportExportHandler(processor)(t.Context(), asynq.NewTask(job.TypeReportExport, []byte(`{"export_id":41}`))); err != nil {
		t.Fatalf("handler error=%v", err)
	}
	if processor.exportID != 41 {
		t.Fatalf("export id=%d", processor.exportID)
	}
	bad := asynq.NewTask(job.TypeReportExport, []byte(`{"export_id":41,"secret":"x"}`))
	if err := newReportExportHandler(processor)(t.Context(), bad); !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("invalid payload error=%v", err)
	}
}

func TestReportExportHandlerSkipsPermanentFailures(t *testing.T) {
	processor := &fakeReportExportProcessor{err: data_svc.ErrReportExportProcessNonRetryable}
	err := newReportExportHandler(processor)(t.Context(), asynq.NewTask(job.TypeReportExport, []byte(`{"export_id":41}`)))
	if !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("handler error=%v", err)
	}
}

type fakeReportExportProcessor struct {
	exportID uint
	err      error
}

func (processor *fakeReportExportProcessor) Process(_ context.Context, exportID uint, _ bool) error {
	processor.exportID = exportID
	return processor.err
}
