package bootstrap

import (
	"context"
	"errors"
	"testing"

	"gin-biz-web-api/internal/service/data_svc"
	"gin-biz-web-api/job"

	"github.com/hibiken/asynq"
)

func TestMallWeatherFeishuHandlerProcessesStrictPayload(t *testing.T) {
	t.Parallel()
	processor := &fakeMallWeatherFeishuProcessor{}
	task := asynq.NewTask(job.TypeMallWeatherFeishu, []byte(`{"pipeline_run_id":17}`))

	if err := newMallWeatherFeishuHandler(processor)(context.Background(), task); err != nil {
		t.Fatalf("handler error=%v", err)
	}
	if processor.calls != 1 || processor.pipelineRunID != 17 || !processor.retryAllowed {
		t.Fatalf("processor=%+v", processor)
	}
}

func TestMallWeatherFeishuHandlerSkipsInvalidAndPermanentFailures(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		task      *asynq.Task
		processor *fakeMallWeatherFeishuProcessor
	}{
		{
			name:      "invalid payload",
			task:      asynq.NewTask(job.TypeMallWeatherFeishu, []byte(`{"pipeline_run_id":0}`)),
			processor: &fakeMallWeatherFeishuProcessor{},
		},
		{
			name:      "wrong task type",
			task:      asynq.NewTask(job.TypeMallWeatherExport, []byte(`{"pipeline_run_id":17}`)),
			processor: &fakeMallWeatherFeishuProcessor{},
		},
		{
			name:      "permanent process failure",
			task:      asynq.NewTask(job.TypeMallWeatherFeishu, []byte(`{"pipeline_run_id":17}`)),
			processor: &fakeMallWeatherFeishuProcessor{err: data_svc.ErrMallWeatherFeishuProcessNonRetryable},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := newMallWeatherFeishuHandler(test.processor)(context.Background(), test.task)
			if !errors.Is(err, asynq.SkipRetry) {
				t.Fatalf("handler error=%v", err)
			}
		})
	}
}

type fakeMallWeatherFeishuProcessor struct {
	err           error
	pipelineRunID uint
	retryAllowed  bool
	calls         int
}

func (processor *fakeMallWeatherFeishuProcessor) Process(
	_ context.Context,
	pipelineRunID uint,
	retryAllowed bool,
) error {
	processor.calls++
	processor.pipelineRunID = pipelineRunID
	processor.retryAllowed = retryAllowed
	return processor.err
}

var _ mallWeatherFeishuProcessor = (*fakeMallWeatherFeishuProcessor)(nil)
