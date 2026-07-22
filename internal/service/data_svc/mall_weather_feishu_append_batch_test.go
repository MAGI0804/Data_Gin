package data_svc

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"gin-biz-web-api/connector/feishu"
	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/providerhttp"
)

func TestMallWeatherFeishuAppendBatchExecutorCheckpointsSuccess(t *testing.T) {
	t.Parallel()
	events := make([]string, 0, 3)
	sheets := &fakeMallWeatherFeishuAppender{
		events: &events,
		result: &feishu.SheetWriteResult{
			Revision: 17, UpdatedRowStart: 12, UpdatedRowEnd: 13,
			UpdatedRows: 2, UpdatedColumns: 2, UpdatedCells: 4,
		},
	}
	logs := &fakeMallWeatherFeishuBatchLogStore{events: &events, createID: 41}
	executor, err := newMallWeatherFeishuAppendBatchExecutor(sheets, logs, sequentialMallWeatherFeishuNow())
	if err != nil {
		t.Fatalf("newMallWeatherFeishuAppendBatchExecutor() error=%v", err)
	}
	result, err := executor.Execute(t.Context(), validMallWeatherFeishuAppendBatchRequest())
	if err != nil {
		t.Fatalf("Execute() error=%v", err)
	}
	if !slices.Equal(events, []string{"create", "append", "finish"}) || result.DeliveryLogID != 41 ||
		result.BatchNo != 2 || result.RowStart != 12 || result.RowEnd != 13 || result.RecordCount != 2 ||
		result.CellCount != 4 || result.Revision != 17 || logs.created == nil || logs.created.Status != "running" ||
		logs.created.RequestBody != "" || logs.created.ResponseBody != "" || logs.created.RequestChecksum != strings.Repeat("a", 64) ||
		logs.finished.Status != "success" || !logs.finished.Success || logs.finished.HTTPStatus != http.StatusOK ||
		logs.finished.RowStart != 12 || logs.finished.RowEnd != 13 {
		t.Fatalf("events=%v result=%+v sheets=%+v logs=%+v", events, result, sheets, logs)
	}
	if sheets.write.Range.SheetID != "sheet-hourly-secret" || sheets.write.Range.StartRow != 12 ||
		sheets.write.Range.EndRow != 13 || sheets.write.Range.EndColumn != 2 || len(sheets.write.Rows) != 2 {
		t.Fatalf("write=%+v", sheets.write)
	}
}

func TestMallWeatherFeishuAppendBatchExecutorClassifiesRemoteOutcomes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		sheetsErr   *feishu.SheetsError
		wantStatus  string
		wantUnknown bool
	}{
		{
			name: "forbidden is definite", sheetsErr: &feishu.SheetsError{
				Class: providerhttp.ErrorClassAuth, HTTPCode: http.StatusForbidden,
			}, wantStatus: "failed",
		},
		{
			name: "rate limit is definite", sheetsErr: &feishu.SheetsError{
				Class: providerhttp.ErrorClassRateLimited, Retryable: true, HTTPCode: http.StatusTooManyRequests,
			}, wantStatus: "failed",
		},
		{
			name: "provider response is unknown", sheetsErr: &feishu.SheetsError{
				Class: providerhttp.ErrorClassProvider, Retryable: true, HTTPCode: http.StatusServiceUnavailable,
			}, wantStatus: "unknown", wantUnknown: true,
		},
		{
			name: "transport is unknown", sheetsErr: &feishu.SheetsError{
				Class: providerhttp.ErrorClassTransport, Retryable: true,
			}, wantStatus: "unknown", wantUnknown: true,
		},
		{
			name: "malformed success response is unknown", sheetsErr: &feishu.SheetsError{
				Class: providerhttp.ErrorClassResponse, HTTPCode: http.StatusOK,
			}, wantStatus: "unknown", wantUnknown: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			sheets := &fakeMallWeatherFeishuAppender{err: test.sheetsErr}
			logs := &fakeMallWeatherFeishuBatchLogStore{createID: 41}
			executor, err := newMallWeatherFeishuAppendBatchExecutor(sheets, logs, sequentialMallWeatherFeishuNow())
			if err != nil {
				t.Fatalf("newMallWeatherFeishuAppendBatchExecutor() error=%v", err)
			}
			_, err = executor.Execute(t.Context(), validMallWeatherFeishuAppendBatchRequest())
			if err == nil || logs.finished.Status != test.wantStatus ||
				errors.Is(err, ErrMallWeatherFeishuAppendStateUnknown) != test.wantUnknown ||
				strings.Contains(logs.finished.SafeError, "spreadsheet") {
				t.Fatalf("Execute() error=%v finish=%+v", err, logs.finished)
			}
		})
	}
}

func TestMallWeatherFeishuAppendBatchExecutorFailsClosedAroundCheckpoint(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		createErr       error
		finishErr       error
		acknowledgement *feishu.SheetWriteResult
		wantAppend      int
		wantUnknown     bool
	}{
		{name: "checkpoint create fails", createErr: errors.New("database unavailable"), wantAppend: 0},
		{
			name: "success checkpoint finish fails", finishErr: errors.New("database unavailable"),
			acknowledgement: &feishu.SheetWriteResult{
				Revision: 1, UpdatedRowStart: 12, UpdatedRowEnd: 13,
				UpdatedRows: 2, UpdatedColumns: 2, UpdatedCells: 4,
			}, wantAppend: 1, wantUnknown: true,
		},
		{name: "acknowledgement missing", wantAppend: 1, wantUnknown: true},
		{
			name: "acknowledgement dimensions mismatch",
			acknowledgement: &feishu.SheetWriteResult{
				Revision: 1, UpdatedRowStart: 12, UpdatedRowEnd: 12,
				UpdatedRows: 1, UpdatedColumns: 2, UpdatedCells: 2,
			}, wantAppend: 1, wantUnknown: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			sheets := &fakeMallWeatherFeishuAppender{result: test.acknowledgement}
			logs := &fakeMallWeatherFeishuBatchLogStore{createID: 41, createErr: test.createErr, finishErr: test.finishErr}
			executor, err := newMallWeatherFeishuAppendBatchExecutor(sheets, logs, sequentialMallWeatherFeishuNow())
			if err != nil {
				t.Fatalf("newMallWeatherFeishuAppendBatchExecutor() error=%v", err)
			}
			_, err = executor.Execute(t.Context(), validMallWeatherFeishuAppendBatchRequest())
			if err == nil || sheets.calls != test.wantAppend ||
				errors.Is(err, ErrMallWeatherFeishuAppendStateUnknown) != test.wantUnknown {
				t.Fatalf("Execute() error=%v sheets=%+v logs=%+v", err, sheets, logs)
			}
		})
	}
}

func TestMallWeatherFeishuAppendBatchExecutorPersistsCanceledOutcome(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	sheets := &fakeMallWeatherFeishuAppender{
		before: cancel,
		err:    &feishu.SheetsError{Class: providerhttp.ErrorClassCanceled},
	}
	logs := &fakeMallWeatherFeishuBatchLogStore{createID: 41}
	executor, err := newMallWeatherFeishuAppendBatchExecutor(sheets, logs, sequentialMallWeatherFeishuNow())
	if err != nil {
		t.Fatalf("newMallWeatherFeishuAppendBatchExecutor() error=%v", err)
	}
	_, err = executor.Execute(ctx, validMallWeatherFeishuAppendBatchRequest())
	if !errors.Is(err, ErrMallWeatherFeishuAppendStateUnknown) || logs.finishContextErr != nil ||
		logs.finished.Status != "unknown" {
		t.Fatalf("Execute() error=%v finishContextErr=%v finish=%+v", err, logs.finishContextErr, logs.finished)
	}
}

func TestMallWeatherFeishuAppendBatchExecutorRejectsInvalidRequestBeforeSideEffects(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		mutate func(*mallWeatherFeishuAppendBatchRequest)
	}{
		{name: "ragged row", mutate: func(request *mallWeatherFeishuAppendBatchRequest) {
			request.Batch.Rows[1] = request.Batch.Rows[1][:1]
		}},
		{name: "blank first column", mutate: func(request *mallWeatherFeishuAppendBatchRequest) {
			request.Batch.Rows[0][0] = feishu.SheetCell{Type: feishu.SheetCellBlank}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			sheets := &fakeMallWeatherFeishuAppender{}
			logs := &fakeMallWeatherFeishuBatchLogStore{createID: 41}
			executor, err := newMallWeatherFeishuAppendBatchExecutor(sheets, logs, sequentialMallWeatherFeishuNow())
			if err != nil {
				t.Fatalf("newMallWeatherFeishuAppendBatchExecutor() error=%v", err)
			}
			request := validMallWeatherFeishuAppendBatchRequest()
			test.mutate(&request)
			if _, err := executor.Execute(t.Context(), request); err == nil || logs.createCalls != 0 || sheets.calls != 0 {
				t.Fatalf("Execute() error=%v sheets=%+v logs=%+v", err, sheets, logs)
			}
		})
	}
}

type fakeMallWeatherFeishuAppender struct {
	events *([]string)
	result *feishu.SheetWriteResult
	err    error
	before func()
	calls  int
	write  feishu.SheetWriteRange
}

func (appender *fakeMallWeatherFeishuAppender) AppendValues(
	_ context.Context,
	_ string,
	write feishu.SheetWriteRange,
) (*feishu.SheetWriteResult, error) {
	appender.calls++
	appender.write = write
	if appender.events != nil {
		*appender.events = append(*appender.events, "append")
	}
	if appender.before != nil {
		appender.before()
	}
	return appender.result, appender.err
}

type fakeMallWeatherFeishuBatchLogStore struct {
	events           *([]string)
	createID         uint
	createErr        error
	finishErr        error
	createCalls      int
	created          *model.DeliveryLog
	finished         data_dao.DeliveryLogBatchFinish
	finishContextErr error
}

func (store *fakeMallWeatherFeishuBatchLogStore) Create(
	_ context.Context,
	log *model.DeliveryLog,
) (uint, error) {
	store.createCalls++
	copy := *log
	store.created = &copy
	if store.events != nil {
		*store.events = append(*store.events, "create")
	}
	return store.createID, store.createErr
}

func (store *fakeMallWeatherFeishuBatchLogStore) FinishWeatherBatch(
	ctx context.Context,
	_ uint,
	finish data_dao.DeliveryLogBatchFinish,
) error {
	store.finished = finish
	store.finishContextErr = ctx.Err()
	if store.events != nil {
		*store.events = append(*store.events, "finish")
	}
	return store.finishErr
}

func validMallWeatherFeishuAppendBatchRequest() mallWeatherFeishuAppendBatchRequest {
	input := validMallWeatherFeishuDryRunPlanInput()
	input.Destination.Config.WriteMode = "append"
	input.Destination.Config.UniqueKeyFields = nil
	return mallWeatherFeishuAppendBatchRequest{
		RunID: 23, TraceID: "11111111-1111-4111-8111-111111111111", Destination: input.Destination,
		DatasetKind: "hourly", BatchNo: 2, Attempt: 1, RowStart: 12,
		Batch: mallWeatherFeishuRenderedBatch{
			Rows: [][]feishu.SheetCell{
				{{Type: feishu.SheetCellString, Text: "mall-a"}, {Type: feishu.SheetCellNumber, Number: "1"}},
				{{Type: feishu.SheetCellString, Text: "mall-b"}, {Type: feishu.SheetCellNumber, Number: "2"}},
			},
			Checksum: strings.Repeat("a", 64), FirstCursor: 7, LastCursor: 8,
		},
	}
}

func sequentialMallWeatherFeishuNow() func() time.Time {
	current := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	return func() time.Time {
		value := current
		current = current.Add(time.Second)
		return value
	}
}
