package data_svc

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"gin-biz-web-api/connector/feishu"
	weatherdomain "gin-biz-web-api/internal/weather"
	"gin-biz-web-api/model"
)

func TestMallWeatherFeishuExecutorLocksAndAppendsAllData(t *testing.T) {
	record, resources := validMallWeatherFeishuExecutionRecordForMode(t, "append")
	locker := &fakeMallWeatherFeishuExecutionLocker{acquired: true}
	sheets := newFakeMallWeatherFeishuExecutionSheets()
	appendRunner := &fakeMallWeatherFeishuAppendDatasetExecutor{result: mallWeatherFeishuAppendDatasetResult{
		DatasetKind: "hourly", RecordCount: 5,
	}}
	executor := newTestMallWeatherFeishuExecutor(t, resources, sheets, locker, appendRunner,
		&fakeMallWeatherFeishuOverwriteDatasetExecutor{}, &fakeMallWeatherFeishuUpsertDatasetExecutor{},
		&fakeMallWeatherFeishuOverwriteCleanupExecutor{})
	var progressSuccess int

	result, err := executor.Execute(t.Context(), record, func(successCount, failedCount int) error {
		progressSuccess = successCount
		if failedCount != 0 {
			t.Fatalf("failedCount=%d", failedCount)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Execute() error=%v", err)
	}
	if result.SuccessCount != 5 || result.FailedCount != 0 || progressSuccess != 5 ||
		locker.key != "destination:17" || locker.lock.releaseCalls != 1 || sheets.inspectCalls != 1 ||
		sheets.headerWriteCalls != 1 || appendRunner.calls != 1 {
		t.Fatalf("unexpected result=%+v progress=%d locker=%+v sheets=%+v appendCalls=%d", result, progressSuccess, locker, sheets, appendRunner.calls)
	}
	if appendRunner.request.GridRows != 1_000 || appendRunner.request.RunID != 41 ||
		appendRunner.request.Destination.SpreadsheetToken != "spreadsheet-secret-token" {
		t.Fatalf("unexpected append request=%+v", appendRunner.request)
	}
}

func TestMallWeatherFeishuExecutorRunsUpsertDatasets(t *testing.T) {
	record, resources := validMallWeatherFeishuExecutionRecordForMode(t, "upsert")
	locker := &fakeMallWeatherFeishuExecutionLocker{acquired: true}
	sheets := newFakeMallWeatherFeishuExecutionSheets()
	upsertRunner := &fakeMallWeatherFeishuUpsertDatasetExecutor{result: mallWeatherFeishuUpsertDatasetResult{
		DatasetKind: "hourly", RecordCount: 7, SkippedCount: 2, UpdatedCount: 3, AppendedCount: 2,
	}}
	executor := newTestMallWeatherFeishuExecutor(
		t,
		resources,
		sheets,
		locker,
		&fakeMallWeatherFeishuAppendDatasetExecutor{},
		&fakeMallWeatherFeishuOverwriteDatasetExecutor{},
		upsertRunner,
		&fakeMallWeatherFeishuOverwriteCleanupExecutor{},
	)
	var progressSuccess int

	result, err := executor.Execute(t.Context(), record, func(successCount, failedCount int) error {
		progressSuccess = successCount
		if failedCount != 0 {
			t.Fatalf("failedCount=%d", failedCount)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Execute() error=%v", err)
	}
	if result.SuccessCount != 7 || result.FailedCount != 0 || progressSuccess != 7 ||
		upsertRunner.calls != 1 || sheets.inspectCalls != 1 || sheets.headerWriteCalls != 1 ||
		locker.lock.releaseCalls != 1 {
		t.Fatalf("result=%+v progress=%d upsert=%+v sheets=%+v locker=%+v", result, progressSuccess, upsertRunner, sheets, locker)
	}
	if upsertRunner.request.GridRows != 1_000 || upsertRunner.request.RunID != 41 ||
		upsertRunner.request.Destination.Config.WriteMode != "upsert" {
		t.Fatalf("unexpected upsert request=%+v", upsertRunner.request)
	}
}

func TestMallWeatherFeishuExecutorKeepsOverwriteCleanupInSameLock(t *testing.T) {
	record, resources := validMallWeatherFeishuExecutionRecordForMode(t, "overwrite_range")
	locker := &fakeMallWeatherFeishuExecutionLocker{acquired: true}
	overwriteRunner := &fakeMallWeatherFeishuOverwriteDatasetExecutor{result: mallWeatherFeishuOverwriteDatasetResult{
		DatasetKind: "hourly", BatchCount: 2, RecordCount: 4, StaleRowStart: 8, StaleRowEnd: 9,
	}}
	cleanupRunner := &fakeMallWeatherFeishuOverwriteCleanupExecutor{}
	executor := newTestMallWeatherFeishuExecutor(
		t,
		resources,
		newFakeMallWeatherFeishuExecutionSheets(),
		locker,
		&fakeMallWeatherFeishuAppendDatasetExecutor{},
		overwriteRunner,
		&fakeMallWeatherFeishuUpsertDatasetExecutor{},
		cleanupRunner,
	)

	result, err := executor.Execute(t.Context(), record, func(int, int) error { return nil })
	if err != nil {
		t.Fatalf("Execute() error=%v", err)
	}
	if result.SuccessCount != 4 || overwriteRunner.calls != 1 || cleanupRunner.calls != 1 ||
		cleanupRunner.request.StartBatchNo != 3 || cleanupRunner.request.RowStart != 8 ||
		cleanupRunner.request.RowEnd != 9 || locker.lock.releaseCalls != 1 {
		t.Fatalf("result=%+v overwriteCalls=%d cleanup=%+v release=%d", result, overwriteRunner.calls, cleanupRunner.request, locker.lock.releaseCalls)
	}
}

func TestMallWeatherFeishuExecutorNeverBypassesDestinationLock(t *testing.T) {
	tests := []struct {
		name     string
		acquired bool
		lockErr  error
	}{
		{name: "busy", acquired: false},
		{name: "redis unavailable", lockErr: errors.New("redis unavailable")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			record, resources := validMallWeatherFeishuExecutionRecordForMode(t, "append")
			sheets := newFakeMallWeatherFeishuExecutionSheets()
			executor := newTestMallWeatherFeishuExecutor(
				t,
				resources,
				sheets,
				&fakeMallWeatherFeishuExecutionLocker{acquired: test.acquired, err: test.lockErr},
				&fakeMallWeatherFeishuAppendDatasetExecutor{},
				&fakeMallWeatherFeishuOverwriteDatasetExecutor{},
				&fakeMallWeatherFeishuUpsertDatasetExecutor{},
				&fakeMallWeatherFeishuOverwriteCleanupExecutor{},
			)
			_, err := executor.Execute(t.Context(), record, func(int, int) error { return nil })
			assertMallWeatherFeishuExecutionError(t, err, true)
			if sheets.inspectCalls != 0 || sheets.headerWriteCalls != 0 {
				t.Fatalf("remote calls inspect=%d header=%d", sheets.inspectCalls, sheets.headerWriteCalls)
			}
		})
	}
}

func TestMallWeatherFeishuExecutorRejectsInvalidSnapshotBeforeLock(t *testing.T) {
	record, resources := validMallWeatherFeishuExecutionRecordForMode(t, "append")
	record.Detail.DestinationSnapshotJSON = model.JSONText(strings.TrimSuffix(
		string(record.Detail.DestinationSnapshotJSON),
		"}",
	) + `,"unknown":true}`)
	locker := &fakeMallWeatherFeishuExecutionLocker{acquired: true}
	executor := newTestMallWeatherFeishuExecutor(
		t,
		resources,
		newFakeMallWeatherFeishuExecutionSheets(),
		locker,
		&fakeMallWeatherFeishuAppendDatasetExecutor{},
		&fakeMallWeatherFeishuOverwriteDatasetExecutor{},
		&fakeMallWeatherFeishuUpsertDatasetExecutor{},
		&fakeMallWeatherFeishuOverwriteCleanupExecutor{},
	)

	_, err := executor.Execute(t.Context(), record, func(int, int) error { return nil })
	assertMallWeatherFeishuExecutionError(t, err, false)
	if locker.acquireCalls != 0 {
		t.Fatalf("lock acquire calls=%d", locker.acquireCalls)
	}
}

func TestMallWeatherFeishuExecutorRetriesSuccessfulWorkWhenLockReleaseFails(t *testing.T) {
	record, resources := validMallWeatherFeishuExecutionRecordForMode(t, "append")
	locker := &fakeMallWeatherFeishuExecutionLocker{
		acquired: true,
		lock:     fakeMallWeatherFeishuExecutionLock{err: errors.New("redis unavailable")},
	}
	executor := newTestMallWeatherFeishuExecutor(
		t,
		resources,
		newFakeMallWeatherFeishuExecutionSheets(),
		locker,
		&fakeMallWeatherFeishuAppendDatasetExecutor{result: mallWeatherFeishuAppendDatasetResult{RecordCount: 2}},
		&fakeMallWeatherFeishuOverwriteDatasetExecutor{},
		&fakeMallWeatherFeishuUpsertDatasetExecutor{},
		&fakeMallWeatherFeishuOverwriteCleanupExecutor{},
	)

	result, err := executor.Execute(t.Context(), record, func(int, int) error { return nil })
	assertMallWeatherFeishuExecutionError(t, err, true)
	if result.SuccessCount != 2 || locker.lock.releaseCalls != 1 {
		t.Fatalf("result=%+v release=%d", result, locker.lock.releaseCalls)
	}
}

func newTestMallWeatherFeishuExecutor(
	t *testing.T,
	resources mallWeatherFeishuResourceResolver,
	sheets mallWeatherFeishuExecutionSheets,
	locker weatherdomain.TaskLocker,
	appendRunner mallWeatherFeishuAppendDatasetExecutor,
	overwriteRunner mallWeatherFeishuOverwriteDatasetExecutor,
	upsertRunner mallWeatherFeishuUpsertDatasetExecutor,
	cleanupRunner mallWeatherFeishuOverwriteCleanupExecutor,
) *mallWeatherFeishuExecutor {
	t.Helper()
	executor, err := newMallWeatherFeishuExecutor(
		resources,
		sheets,
		locker,
		appendRunner,
		overwriteRunner,
		upsertRunner,
		cleanupRunner,
		time.Second,
	)
	if err != nil {
		t.Fatalf("newMallWeatherFeishuExecutor() error=%v", err)
	}
	return executor
}

func assertMallWeatherFeishuExecutionError(t *testing.T, err error, retryable bool) {
	t.Helper()
	var executionError *MallWeatherFeishuExecutionError
	if !errors.As(err, &executionError) || executionError == nil || executionError.Retryable != retryable {
		t.Fatalf("error=%v retryable=%v", err, retryable)
	}
}

type fakeMallWeatherFeishuExecutionLocker struct {
	acquired     bool
	err          error
	key          string
	acquireCalls int
	lock         fakeMallWeatherFeishuExecutionLock
}

func (locker *fakeMallWeatherFeishuExecutionLocker) Acquire(
	_ context.Context,
	key string,
) (weatherdomain.TaskLock, bool, error) {
	locker.acquireCalls++
	locker.key = key
	if locker.err != nil || !locker.acquired {
		return nil, locker.acquired, locker.err
	}
	return &locker.lock, true, nil
}

type fakeMallWeatherFeishuExecutionLock struct {
	err          error
	releaseCalls int
}

func (lock *fakeMallWeatherFeishuExecutionLock) Release(context.Context) error {
	lock.releaseCalls++
	return lock.err
}

type fakeMallWeatherFeishuExecutionSheets struct {
	inspectCalls     int
	headerWriteCalls int
	headers          map[string]feishu.SheetWriteRange
}

func newFakeMallWeatherFeishuExecutionSheets() *fakeMallWeatherFeishuExecutionSheets {
	return &fakeMallWeatherFeishuExecutionSheets{headers: make(map[string]feishu.SheetWriteRange)}
}

func (sheets *fakeMallWeatherFeishuExecutionSheets) Inspect(
	_ context.Context,
	spreadsheetToken string,
	_ []string,
) (*feishu.SpreadsheetMetadata, error) {
	sheets.inspectCalls++
	return &feishu.SpreadsheetMetadata{
		SpreadsheetToken: spreadsheetToken,
		Sheets: []feishu.SheetMetadata{{
			SheetID: "hourly-secret-sheet", ResourceType: "sheet",
			GridProperties: feishu.SheetGridProperties{RowCount: 1_000, ColumnCount: 100},
		}},
	}, nil
}

func (sheets *fakeMallWeatherFeishuExecutionSheets) ReadRange(
	_ context.Context,
	_ string,
	readRange feishu.SheetRange,
) (*feishu.SheetValues, error) {
	write, exists := sheets.headers[readRange.SheetID]
	if !exists {
		return &feishu.SheetValues{}, nil
	}
	return &feishu.SheetValues{Rows: write.Rows}, nil
}

func (sheets *fakeMallWeatherFeishuExecutionSheets) BatchUpdateValues(
	_ context.Context,
	_ string,
	writes []feishu.SheetWriteRange,
) (*feishu.SheetWriteResult, error) {
	sheets.headerWriteCalls++
	for _, write := range writes {
		sheets.headers[write.Range.SheetID] = write
	}
	return &feishu.SheetWriteResult{Revision: 1}, nil
}

type fakeMallWeatherFeishuAppendDatasetExecutor struct {
	result  mallWeatherFeishuAppendDatasetResult
	err     error
	request mallWeatherFeishuAppendDatasetRequest
	calls   int
}

func (runner *fakeMallWeatherFeishuAppendDatasetExecutor) Run(
	_ context.Context,
	request mallWeatherFeishuAppendDatasetRequest,
) (mallWeatherFeishuAppendDatasetResult, error) {
	runner.calls++
	runner.request = request
	return runner.result, runner.err
}

type fakeMallWeatherFeishuOverwriteDatasetExecutor struct {
	result  mallWeatherFeishuOverwriteDatasetResult
	err     error
	request mallWeatherFeishuOverwriteDatasetRequest
	calls   int
}

func (runner *fakeMallWeatherFeishuOverwriteDatasetExecutor) Run(
	_ context.Context,
	request mallWeatherFeishuOverwriteDatasetRequest,
) (mallWeatherFeishuOverwriteDatasetResult, error) {
	runner.calls++
	runner.request = request
	return runner.result, runner.err
}

type fakeMallWeatherFeishuUpsertDatasetExecutor struct {
	result  mallWeatherFeishuUpsertDatasetResult
	err     error
	request mallWeatherFeishuUpsertDatasetRequest
	calls   int
}

func (runner *fakeMallWeatherFeishuUpsertDatasetExecutor) Run(
	_ context.Context,
	request mallWeatherFeishuUpsertDatasetRequest,
) (mallWeatherFeishuUpsertDatasetResult, error) {
	runner.calls++
	runner.request = request
	return runner.result, runner.err
}

type fakeMallWeatherFeishuOverwriteCleanupExecutor struct {
	result  mallWeatherFeishuOverwriteCleanupResult
	err     error
	request mallWeatherFeishuOverwriteCleanupRequest
	calls   int
}

func (runner *fakeMallWeatherFeishuOverwriteCleanupExecutor) Run(
	_ context.Context,
	request mallWeatherFeishuOverwriteCleanupRequest,
) (mallWeatherFeishuOverwriteCleanupResult, error) {
	runner.calls++
	runner.request = request
	return runner.result, runner.err
}
