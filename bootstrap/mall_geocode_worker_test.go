package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"gin-biz-web-api/internal/service/data_svc"
	"gin-biz-web-api/job"

	"github.com/hibiken/asynq"
)

func TestMallGeocodeHandlerDecodesValidatedPayload(t *testing.T) {
	processor := &fakeMallGeocodeProcessor{}
	handler := newMallGeocodeHandler(processor)
	payload := job.MallGeocodeTaskPayload{MallID: 7, MallVersion: 3, AddressHash: strings.Repeat("a", 64)}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if err := handler.ProcessTask(context.Background(), asynq.NewTask(job.TypeMallGeocode, data)); err != nil {
		t.Fatalf("ProcessTask() error = %v", err)
	}
	if processor.calls != 1 || processor.payload != payload {
		t.Fatalf("calls=%d payload=%+v", processor.calls, processor.payload)
	}
}

func TestMallGeocodeHandlerControlsRetryPolicy(t *testing.T) {
	tests := []struct {
		name      string
		payload   string
		processor mallGeocodeProcessor
		wantSkip  bool
	}{
		{name: "invalid payload", payload: `{"mall_id":7}`, processor: &fakeMallGeocodeProcessor{}, wantSkip: true},
		{name: "non retryable provider failure", payload: validGeocodeTaskJSON(t), processor: &fakeMallGeocodeProcessor{err: &data_svc.MallGeocodeProcessError{}}, wantSkip: true},
		{name: "retryable provider failure", payload: validGeocodeTaskJSON(t), processor: &fakeMallGeocodeProcessor{err: &data_svc.MallGeocodeProcessError{Retryable: true}}, wantSkip: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := newMallGeocodeHandler(test.processor).ProcessTask(
				context.Background(),
				asynq.NewTask(job.TypeMallGeocode, []byte(test.payload)),
			)
			if err == nil || errors.Is(err, asynq.SkipRetry) != test.wantSkip {
				t.Fatalf("ProcessTask() error = %v, wantSkip=%v", err, test.wantSkip)
			}
		})
	}
}

type fakeMallGeocodeProcessor struct {
	calls   int
	payload job.MallGeocodeTaskPayload
	err     error
}

func (processor *fakeMallGeocodeProcessor) Process(_ context.Context, payload job.MallGeocodeTaskPayload) error {
	processor.calls++
	processor.payload = payload
	return processor.err
}

func validGeocodeTaskJSON(t *testing.T) string {
	t.Helper()
	data, err := json.Marshal(job.MallGeocodeTaskPayload{MallID: 7, MallVersion: 3, AddressHash: strings.Repeat("a", 64)})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return string(data)
}
