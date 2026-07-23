package data_svc

import (
	"context"
	"errors"
	"testing"
	"time"

	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/model"

	"github.com/google/uuid"
)

func TestMallWeatherFeishuProcessorCompletesOwnedRun(t *testing.T) {
	store := newFakeMallWeatherFeishuRunStore(data_dao.MallWeatherFeishuRunDispositionAcquired)
	executor := &fakeMallWeatherFeishuRunExecutor{
		result:          MallWeatherFeishuExecutionResult{SuccessCount: 3},
		progressSuccess: 2,
	}
	processor := newTestMallWeatherFeishuProcessor(t, store, executor)

	if err := processor.Process(t.Context(), 41, true); err != nil {
		t.Fatalf("Process() error=%v", err)
	}
	if executor.calls != 1 || store.progressCalls != 1 || store.releaseCalls != 0 || store.finishCalls != 1 {
		t.Fatalf("unexpected calls executor=%d progress=%d release=%d finish=%d", executor.calls, store.progressCalls, store.releaseCalls, store.finishCalls)
	}
	if store.finish.Status != "success" || store.finish.SuccessCount != 3 || store.finish.FailedCount != 0 {
		t.Fatalf("unexpected finish=%+v", store.finish)
	}
}

func TestMallWeatherFeishuProcessorRecordsTerminalFeishuMetrics(t *testing.T) {
	tests := []struct {
		name    string
		result  MallWeatherFeishuExecutionResult
		err     error
		want    []fakeMallWeatherMetricCounter
		wantErr bool
	}{
		{
			name:   "success rows",
			result: MallWeatherFeishuExecutionResult{SuccessCount: 3},
			want: []fakeMallWeatherMetricCounter{
				{name: MallWeatherMetricFeishuRowsTotal, labels: map[string]string{"status": mallWeatherMetricStatusSuccess}, value: 3},
				{name: MallWeatherMetricFeishuRunsTotal, labels: map[string]string{"status": mallWeatherMetricStatusSuccess}, value: 1},
			},
		},
		{
			name: "partial rows only records confirmed success count",
			result: MallWeatherFeishuExecutionResult{
				SuccessCount: 8,
				FailedCount:  1,
			},
			err: &MallWeatherFeishuExecutionError{
				Retryable: false, SafeMessage: "部分飞书数据集推送失败", cause: errors.New("header conflict"),
			},
			want: []fakeMallWeatherMetricCounter{
				{name: MallWeatherMetricFeishuRowsTotal, labels: map[string]string{"status": mallWeatherMetricStatusSuccess}, value: 8},
				{name: MallWeatherMetricFeishuRunsTotal, labels: map[string]string{"status": mallWeatherMetricStatusPartialSuccess}, value: 1},
			},
			wantErr: true,
		},
		{
			name: "failed execution without confirmed rows",
			result: MallWeatherFeishuExecutionResult{
				FailedCount: 2,
			},
			err: &MallWeatherFeishuExecutionError{
				Retryable: false, SafeMessage: "飞书推送失败", cause: errors.New("permission denied"),
			},
			want: []fakeMallWeatherMetricCounter{{
				name: MallWeatherMetricFeishuRunsTotal, labels: map[string]string{"status": mallWeatherMetricStatusFailed}, value: 1,
			}},
			wantErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := newFakeMallWeatherFeishuRunStore(data_dao.MallWeatherFeishuRunDispositionAcquired)
			metrics := &fakeMallWeatherMetricRecorder{}
			processor := newTestMallWeatherFeishuProcessorWithMetrics(
				t,
				store,
				&fakeMallWeatherFeishuRunExecutor{result: test.result, err: test.err},
				metrics,
			)

			err := processor.Process(t.Context(), 41, true)
			if (err != nil) != test.wantErr {
				t.Fatalf("Process() error=%v wantErr=%t", err, test.wantErr)
			}
			assertFakeMallWeatherMetricCounters(t, metrics.counters, test.want)
		})
	}
}

func TestMallWeatherFeishuProcessorDoesNotRecordMetricsForRetryRelease(t *testing.T) {
	store := newFakeMallWeatherFeishuRunStore(data_dao.MallWeatherFeishuRunDispositionAcquired)
	metrics := &fakeMallWeatherMetricRecorder{}
	processor := newTestMallWeatherFeishuProcessorWithMetrics(
		t,
		store,
		&fakeMallWeatherFeishuRunExecutor{
			result: MallWeatherFeishuExecutionResult{SuccessCount: 3, FailedCount: 1},
			err:    errors.New("sheets unavailable"),
		},
		metrics,
	)

	err := processor.Process(t.Context(), 41, true)
	if err == nil || store.releaseCalls != 1 || len(metrics.counters) != 0 {
		t.Fatalf("Process() error=%v release=%d metrics=%+v", err, store.releaseCalls, metrics.counters)
	}
}

func TestMallWeatherFeishuProcessorSkipsBusyOrTerminalRun(t *testing.T) {
	for _, disposition := range []data_dao.MallWeatherFeishuRunDisposition{
		data_dao.MallWeatherFeishuRunDispositionBusy,
		data_dao.MallWeatherFeishuRunDispositionTerminal,
	} {
		t.Run(dispositionName(disposition), func(t *testing.T) {
			store := newFakeMallWeatherFeishuRunStore(disposition)
			executor := &fakeMallWeatherFeishuRunExecutor{}
			processor := newTestMallWeatherFeishuProcessor(t, store, executor)

			if err := processor.Process(t.Context(), 41, true); err != nil {
				t.Fatalf("Process() error=%v", err)
			}
			if executor.calls != 0 || store.finishCalls != 0 || store.releaseCalls != 0 {
				t.Fatalf("unexpected side effects executor=%d finish=%d release=%d", executor.calls, store.finishCalls, store.releaseCalls)
			}
		})
	}
}

func TestMallWeatherFeishuProcessorClassifiesExecutionFailures(t *testing.T) {
	tests := []struct {
		name          string
		retryAllowed  bool
		executionErr  error
		wantRelease   int
		wantFinish    int
		wantStatus    string
		wantSkipRetry bool
	}{
		{
			name:         "retryable failure releases lease",
			retryAllowed: true,
			executionErr: errors.New("sheets unavailable"),
			wantRelease:  1,
		},
		{
			name:          "final retryable failure is terminal",
			retryAllowed:  false,
			executionErr:  errors.New("sheets unavailable"),
			wantFinish:    1,
			wantStatus:    "failed",
			wantSkipRetry: true,
		},
		{
			name:         "permanent failure is terminal",
			retryAllowed: true,
			executionErr: &MallWeatherFeishuExecutionError{
				Retryable: false, SafeMessage: "飞书目标配置无效", cause: errors.New("invalid snapshot"),
			},
			wantFinish:    1,
			wantStatus:    "failed",
			wantSkipRetry: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := newFakeMallWeatherFeishuRunStore(data_dao.MallWeatherFeishuRunDispositionAcquired)
			executor := &fakeMallWeatherFeishuRunExecutor{err: test.executionErr}
			processor := newTestMallWeatherFeishuProcessor(t, store, executor)

			err := processor.Process(t.Context(), 41, test.retryAllowed)
			if err == nil {
				t.Fatal("Process() returned nil error")
			}
			if errors.Is(err, ErrMallWeatherFeishuProcessNonRetryable) != test.wantSkipRetry {
				t.Fatalf("Process() error=%v wantSkipRetry=%v", err, test.wantSkipRetry)
			}
			if store.releaseCalls != test.wantRelease || store.finishCalls != test.wantFinish {
				t.Fatalf("release=%d finish=%d", store.releaseCalls, store.finishCalls)
			}
			if test.wantStatus != "" && store.finish.Status != test.wantStatus {
				t.Fatalf("finish status=%q want=%q", store.finish.Status, test.wantStatus)
			}
		})
	}
}

func TestMallWeatherFeishuProcessorReportsPartialFailure(t *testing.T) {
	store := newFakeMallWeatherFeishuRunStore(data_dao.MallWeatherFeishuRunDispositionAcquired)
	executor := &fakeMallWeatherFeishuRunExecutor{
		result: MallWeatherFeishuExecutionResult{SuccessCount: 8, FailedCount: 1},
		err: &MallWeatherFeishuExecutionError{
			Retryable: false, SafeMessage: "部分飞书数据集推送失败", cause: errors.New("header conflict"),
		},
	}
	processor := newTestMallWeatherFeishuProcessor(t, store, executor)

	err := processor.Process(t.Context(), 41, true)
	if !errors.Is(err, ErrMallWeatherFeishuProcessNonRetryable) {
		t.Fatalf("Process() error=%v", err)
	}
	if store.finish.Status != "partial_success" || store.finish.SuccessCount != 8 || store.finish.FailedCount != 1 ||
		store.finish.SafeError != "部分飞书数据集推送失败" {
		t.Fatalf("unexpected finish=%+v", store.finish)
	}
}

func TestMallWeatherFeishuProcessorReleasesLeaseWhenSuccessStateCannotPersist(t *testing.T) {
	store := newFakeMallWeatherFeishuRunStore(data_dao.MallWeatherFeishuRunDispositionAcquired)
	store.finishErr = errors.New("database unavailable")
	processor := newTestMallWeatherFeishuProcessor(t, store, &fakeMallWeatherFeishuRunExecutor{
		result: MallWeatherFeishuExecutionResult{SuccessCount: 3},
	})

	err := processor.Process(t.Context(), 41, true)
	if err == nil || errors.Is(err, ErrMallWeatherFeishuProcessNonRetryable) {
		t.Fatalf("Process() error=%v", err)
	}
	if store.finishCalls != 1 || store.releaseCalls != 1 {
		t.Fatalf("finish=%d release=%d", store.finishCalls, store.releaseCalls)
	}
}

func TestMallWeatherFeishuProcessorStopsWhenLeaseIsLost(t *testing.T) {
	store := newFakeMallWeatherFeishuRunStore(data_dao.MallWeatherFeishuRunDispositionAcquired)
	store.heartbeatErr = data_dao.ErrMallWeatherFeishuRunLeaseLost
	executor := &fakeMallWeatherFeishuRunExecutor{blockUntilCanceled: true}
	processor := newTestMallWeatherFeishuProcessor(t, store, executor)
	processor.heartbeatInterval = time.Millisecond

	if err := processor.Process(t.Context(), 41, true); err != nil {
		t.Fatalf("Process() error=%v", err)
	}
	if store.heartbeatCalls == 0 || store.finishCalls != 0 || store.releaseCalls != 0 {
		t.Fatalf("heartbeat=%d finish=%d release=%d", store.heartbeatCalls, store.finishCalls, store.releaseCalls)
	}
}

func TestMallWeatherFeishuProcessorRejectsInvalidConfiguration(t *testing.T) {
	processor := &MallWeatherFeishuProcessor{}
	if err := processor.Process(t.Context(), 41, true); !errors.Is(err, ErrMallWeatherFeishuProcessNonRetryable) {
		t.Fatalf("Process() error=%v", err)
	}
	if _, err := newMallWeatherFeishuProcessor(
		newFakeMallWeatherFeishuRunStore(data_dao.MallWeatherFeishuRunDispositionAcquired),
		&fakeMallWeatherFeishuRunExecutor{},
		time.Now,
		uuid.NewString,
		noopMallWeatherMetricRecorder{},
		time.Minute,
		time.Minute,
		time.Second,
	); err == nil {
		t.Fatal("newMallWeatherFeishuProcessor() accepted heartbeat equal to stale duration")
	}
	if _, err := newMallWeatherFeishuProcessor(
		newFakeMallWeatherFeishuRunStore(data_dao.MallWeatherFeishuRunDispositionAcquired),
		&fakeMallWeatherFeishuRunExecutor{},
		time.Now,
		uuid.NewString,
		nil,
		2*time.Minute,
		time.Minute,
		time.Second,
	); err == nil {
		t.Fatal("newMallWeatherFeishuProcessor() accepted nil metrics recorder")
	}
}

func newTestMallWeatherFeishuProcessor(
	t *testing.T,
	store mallWeatherFeishuRunStore,
	executor mallWeatherFeishuRunExecutor,
) *MallWeatherFeishuProcessor {
	t.Helper()
	return newTestMallWeatherFeishuProcessorWithMetrics(t, store, executor, noopMallWeatherMetricRecorder{})
}

func newTestMallWeatherFeishuProcessorWithMetrics(
	t *testing.T,
	store mallWeatherFeishuRunStore,
	executor mallWeatherFeishuRunExecutor,
	metrics mallWeatherMetricRecorder,
) *MallWeatherFeishuProcessor {
	t.Helper()
	processor, err := newMallWeatherFeishuProcessor(
		store,
		executor,
		func() time.Time { return time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC) },
		uuid.NewString,
		metrics,
		2*time.Hour,
		time.Hour,
		time.Second,
	)
	if err != nil {
		t.Fatalf("newMallWeatherFeishuProcessor() error=%v", err)
	}
	return processor
}

type fakeMallWeatherMetricCounter struct {
	name   string
	labels map[string]string
	value  int64
}

type fakeMallWeatherMetricRecorder struct {
	counters []fakeMallWeatherMetricCounter
}

func (recorder *fakeMallWeatherMetricRecorder) AddCounter(
	name string,
	labels map[string]string,
	value int64,
) {
	recorder.counters = append(recorder.counters, fakeMallWeatherMetricCounter{
		name:   name,
		labels: labels,
		value:  value,
	})
}

func assertFakeMallWeatherMetricCounters(
	t *testing.T,
	got []fakeMallWeatherMetricCounter,
	want []fakeMallWeatherMetricCounter,
) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("metric counters=%+v want %+v", got, want)
	}
	for index := range got {
		if got[index].name != want[index].name || got[index].value != want[index].value {
			t.Fatalf("metric counters=%+v want %+v", got, want)
		}
		for key, value := range want[index].labels {
			if got[index].labels[key] != value {
				t.Fatalf("metric counters=%+v want %+v", got, want)
			}
		}
		if len(got[index].labels) != len(want[index].labels) {
			t.Fatalf("metric counters=%+v want %+v", got, want)
		}
	}
}

type fakeMallWeatherFeishuRunStore struct {
	lease          *data_dao.MallWeatherFeishuRunLease
	heartbeatErr   error
	heartbeatCalls int
	progressCalls  int
	finishCalls    int
	releaseCalls   int
	finish         data_dao.MallWeatherFeishuRunFinish
	finishErr      error
}

func newFakeMallWeatherFeishuRunStore(
	disposition data_dao.MallWeatherFeishuRunDisposition,
) *fakeMallWeatherFeishuRunStore {
	return &fakeMallWeatherFeishuRunStore{lease: &data_dao.MallWeatherFeishuRunLease{
		Disposition: disposition,
		Record: data_dao.MallWeatherFeishuRunRecord{
			Pipeline: model.PipelineRun{BaseModel: model.BaseModel{ID: 41}},
			Detail: model.MallWeatherFeishuRun{
				BaseModel: model.BaseModel{ID: 42}, PipelineRunID: 41,
			},
		},
	}}
}

func (store *fakeMallWeatherFeishuRunStore) BeginRun(
	_ context.Context,
	_ uint,
	runToken string,
	_ time.Time,
	_ time.Duration,
) (*data_dao.MallWeatherFeishuRunLease, error) {
	lease := *store.lease
	lease.RunToken = runToken
	return &lease, nil
}

func (store *fakeMallWeatherFeishuRunStore) HeartbeatRun(
	context.Context,
	uint,
	string,
	time.Time,
) error {
	store.heartbeatCalls++
	return store.heartbeatErr
}

func (store *fakeMallWeatherFeishuRunStore) UpdateRunProgress(
	context.Context,
	uint,
	string,
	data_dao.MallWeatherFeishuRunProgress,
) error {
	store.progressCalls++
	return nil
}

func (store *fakeMallWeatherFeishuRunStore) FinishRun(
	_ context.Context,
	_ uint,
	_ string,
	finish data_dao.MallWeatherFeishuRunFinish,
) error {
	store.finishCalls++
	store.finish = finish
	return store.finishErr
}

func (store *fakeMallWeatherFeishuRunStore) ReleaseRunForRetry(
	context.Context,
	uint,
	string,
	time.Time,
) error {
	store.releaseCalls++
	return nil
}

type fakeMallWeatherFeishuRunExecutor struct {
	result             MallWeatherFeishuExecutionResult
	err                error
	progressSuccess    int
	blockUntilCanceled bool
	calls              int
}

func (executor *fakeMallWeatherFeishuRunExecutor) Execute(
	ctx context.Context,
	_ data_dao.MallWeatherFeishuRunRecord,
	progress func(int, int) error,
) (MallWeatherFeishuExecutionResult, error) {
	executor.calls++
	if executor.blockUntilCanceled {
		<-ctx.Done()
		return executor.result, ctx.Err()
	}
	if executor.progressSuccess > 0 {
		if err := progress(executor.progressSuccess, 0); err != nil {
			return executor.result, err
		}
	}
	return executor.result, executor.err
}

func dispositionName(disposition data_dao.MallWeatherFeishuRunDisposition) string {
	switch disposition {
	case data_dao.MallWeatherFeishuRunDispositionBusy:
		return "busy"
	case data_dao.MallWeatherFeishuRunDispositionTerminal:
		return "terminal"
	default:
		return "unknown"
	}
}
