package data_svc

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"testing"

	"gin-biz-web-api/connector/feishu"
	"gin-biz-web-api/pkg/providerhttp"
)

func TestMallWeatherFeishuOverwriteBatchExecutorVerifiesSuccess(t *testing.T) {
	t.Parallel()
	events := make([]string, 0, 4)
	request := validMallWeatherFeishuOverwriteBatchRequest(t)
	sheets := &fakeMallWeatherFeishuOverwriteSheets{
		events:      &events,
		writeResult: &feishu.SheetWriteResult{Revision: 17},
		readResult:  &feishu.SheetValues{Rows: request.Batch.Rows},
	}
	logs := &fakeMallWeatherFeishuBatchLogStore{events: &events, createID: 41}
	executor, err := newMallWeatherFeishuOverwriteBatchExecutor(sheets, logs, sequentialMallWeatherFeishuNow())
	if err != nil {
		t.Fatalf("newMallWeatherFeishuOverwriteBatchExecutor() error=%v", err)
	}
	result, err := executor.Execute(t.Context(), request)
	if err != nil {
		t.Fatalf("Execute() error=%v", err)
	}
	if !slices.Equal(events, []string{"create", "overwrite", "read", "finish"}) ||
		result.DeliveryLogID != 41 || result.BatchNo != 2 || result.RowStart != 12 || result.RowEnd != 13 ||
		result.RecordCount != 2 || result.CellCount != 4 || result.Revision != 17 || logs.created == nil ||
		logs.created.Status != "running" || logs.created.RequestBody != "" || logs.created.ResponseBody != "" ||
		logs.finished.Status != "success" || !logs.finished.Success || logs.finished.HTTPStatus != http.StatusOK {
		t.Fatalf("events=%v result=%+v sheets=%+v logs=%+v", events, result, sheets, logs)
	}
	if len(sheets.writes) != 1 || len(sheets.writes[0]) != 1 ||
		sheets.writes[0][0].Range.SheetID != "sheet-hourly-secret" ||
		sheets.writes[0][0].Range.StartRow != 12 || sheets.writes[0][0].Range.EndRow != 13 ||
		len(sheets.reads) != 1 || sheets.reads[0] != sheets.writes[0][0].Range {
		t.Fatalf("writes=%+v reads=%+v", sheets.writes, sheets.reads)
	}
}

func TestMallWeatherFeishuOverwriteBatchExecutorReconcilesUncertainWrite(t *testing.T) {
	t.Parallel()
	request := validMallWeatherFeishuOverwriteBatchRequest(t)
	sheets := &fakeMallWeatherFeishuOverwriteSheets{
		writeErr: &feishu.SheetsError{
			Class: providerhttp.ErrorClassProvider, Retryable: true, HTTPCode: http.StatusServiceUnavailable,
		},
		readResult: &feishu.SheetValues{Rows: request.Batch.Rows},
	}
	logs := &fakeMallWeatherFeishuBatchLogStore{createID: 41}
	executor, err := newMallWeatherFeishuOverwriteBatchExecutor(sheets, logs, sequentialMallWeatherFeishuNow())
	if err != nil {
		t.Fatalf("newMallWeatherFeishuOverwriteBatchExecutor() error=%v", err)
	}
	if _, err := executor.Execute(t.Context(), request); err != nil || logs.finished.Status != "success" ||
		!logs.finished.Success || logs.finished.HTTPStatus != http.StatusServiceUnavailable || len(sheets.reads) != 1 {
		t.Fatalf("Execute() error=%v sheets=%+v finish=%+v", err, sheets, logs.finished)
	}
}

func TestMallWeatherFeishuOverwriteBatchExecutorClassifiesFailures(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		writeErr    error
		readResult  *feishu.SheetValues
		readErr     error
		wantStatus  string
		wantReads   int
		wantUnknown bool
	}{
		{
			name: "forbidden is definite",
			writeErr: &feishu.SheetsError{
				Class: providerhttp.ErrorClassAuth, HTTPCode: http.StatusForbidden,
			},
			wantStatus: "failed",
		},
		{
			name: "uncertain write mismatches",
			writeErr: &feishu.SheetsError{
				Class: providerhttp.ErrorClassTransport, Retryable: true,
			},
			readResult: &feishu.SheetValues{Rows: [][]feishu.SheetCell{{
				{Type: feishu.SheetCellString, Text: "different"},
				{Type: feishu.SheetCellNumber, Number: "1"},
			}}},
			wantStatus: "unknown", wantReads: 1, wantUnknown: true,
		},
		{
			name:       "acknowledged write cannot be read",
			readErr:    errors.New("read unavailable"),
			wantStatus: "unknown", wantReads: 1, wantUnknown: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := validMallWeatherFeishuOverwriteBatchRequest(t)
			sheets := &fakeMallWeatherFeishuOverwriteSheets{
				writeResult: &feishu.SheetWriteResult{Revision: 1},
				writeErr:    test.writeErr, readResult: test.readResult, readErr: test.readErr,
			}
			logs := &fakeMallWeatherFeishuBatchLogStore{createID: 41}
			executor, err := newMallWeatherFeishuOverwriteBatchExecutor(sheets, logs, sequentialMallWeatherFeishuNow())
			if err != nil {
				t.Fatalf("newMallWeatherFeishuOverwriteBatchExecutor() error=%v", err)
			}
			_, err = executor.Execute(t.Context(), request)
			if err == nil || logs.finished.Status != test.wantStatus || len(sheets.reads) != test.wantReads ||
				errors.Is(err, ErrMallWeatherFeishuOverwriteStateUnknown) != test.wantUnknown {
				t.Fatalf("Execute() error=%v sheets=%+v finish=%+v", err, sheets, logs.finished)
			}
		})
	}
}

func TestMallWeatherFeishuOverwriteBatchExecutorFailsClosedAroundCheckpoint(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		createErr   error
		finishErr   error
		wantWrites  int
		wantUnknown bool
	}{
		{name: "checkpoint create fails", createErr: errors.New("database unavailable")},
		{name: "checkpoint finish fails", finishErr: errors.New("database unavailable"), wantWrites: 1, wantUnknown: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := validMallWeatherFeishuOverwriteBatchRequest(t)
			sheets := &fakeMallWeatherFeishuOverwriteSheets{
				writeResult: &feishu.SheetWriteResult{Revision: 1},
				readResult:  &feishu.SheetValues{Rows: request.Batch.Rows},
			}
			logs := &fakeMallWeatherFeishuBatchLogStore{
				createID: 41, createErr: test.createErr, finishErr: test.finishErr,
			}
			executor, err := newMallWeatherFeishuOverwriteBatchExecutor(sheets, logs, sequentialMallWeatherFeishuNow())
			if err != nil {
				t.Fatalf("newMallWeatherFeishuOverwriteBatchExecutor() error=%v", err)
			}
			_, err = executor.Execute(t.Context(), request)
			if err == nil || len(sheets.writes) != test.wantWrites ||
				errors.Is(err, ErrMallWeatherFeishuOverwriteStateUnknown) != test.wantUnknown {
				t.Fatalf("Execute() error=%v sheets=%+v logs=%+v", err, sheets, logs)
			}
		})
	}
}

func TestMallWeatherFeishuOverwriteBatchExecutorPersistsCanceledOutcome(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	request := validMallWeatherFeishuOverwriteBatchRequest(t)
	sheets := &fakeMallWeatherFeishuOverwriteSheets{
		beforeWrite: cancel,
		writeErr:    &feishu.SheetsError{Class: providerhttp.ErrorClassCanceled},
		readErr:     context.Canceled,
	}
	logs := &fakeMallWeatherFeishuBatchLogStore{createID: 41}
	executor, err := newMallWeatherFeishuOverwriteBatchExecutor(sheets, logs, sequentialMallWeatherFeishuNow())
	if err != nil {
		t.Fatalf("newMallWeatherFeishuOverwriteBatchExecutor() error=%v", err)
	}
	_, err = executor.Execute(ctx, request)
	if !errors.Is(err, ErrMallWeatherFeishuOverwriteStateUnknown) || logs.finishContextErr != nil ||
		logs.finished.Status != "unknown" {
		t.Fatalf("Execute() error=%v finishContextErr=%v finish=%+v", err, logs.finishContextErr, logs.finished)
	}
}

func TestMallWeatherFeishuOverwriteBatchExecutorRejectsInvalidRequestBeforeSideEffects(t *testing.T) {
	t.Parallel()
	request := validMallWeatherFeishuOverwriteBatchRequest(t)
	request.Batch.Rows[0][0] = feishu.SheetCell{Type: feishu.SheetCellBlank}
	sheets := &fakeMallWeatherFeishuOverwriteSheets{}
	logs := &fakeMallWeatherFeishuBatchLogStore{createID: 41}
	executor, err := newMallWeatherFeishuOverwriteBatchExecutor(sheets, logs, sequentialMallWeatherFeishuNow())
	if err != nil {
		t.Fatalf("newMallWeatherFeishuOverwriteBatchExecutor() error=%v", err)
	}
	if _, err := executor.Execute(t.Context(), request); err == nil || logs.createCalls != 0 || len(sheets.writes) != 0 {
		t.Fatalf("Execute() error=%v sheets=%+v logs=%+v", err, sheets, logs)
	}
}

type fakeMallWeatherFeishuOverwriteSheets struct {
	events      *[]string
	writeResult *feishu.SheetWriteResult
	writeErr    error
	readResult  *feishu.SheetValues
	readErr     error
	beforeWrite func()
	writes      [][]feishu.SheetWriteRange
	reads       []feishu.SheetRange
}

func (sheets *fakeMallWeatherFeishuOverwriteSheets) BatchUpdateValues(
	_ context.Context,
	_ string,
	writes []feishu.SheetWriteRange,
) (*feishu.SheetWriteResult, error) {
	sheets.writes = append(sheets.writes, append([]feishu.SheetWriteRange(nil), writes...))
	if sheets.events != nil {
		*sheets.events = append(*sheets.events, "overwrite")
	}
	if sheets.beforeWrite != nil {
		sheets.beforeWrite()
	}
	return sheets.writeResult, sheets.writeErr
}

func (sheets *fakeMallWeatherFeishuOverwriteSheets) ReadRange(
	_ context.Context,
	_ string,
	readRange feishu.SheetRange,
) (*feishu.SheetValues, error) {
	sheets.reads = append(sheets.reads, readRange)
	if sheets.events != nil {
		*sheets.events = append(*sheets.events, "read")
	}
	return sheets.readResult, sheets.readErr
}

func validMallWeatherFeishuOverwriteBatchRequest(t *testing.T) mallWeatherFeishuOverwriteBatchRequest {
	t.Helper()
	appendRequest := validMallWeatherFeishuAppendBatchRequest()
	appendRequest.Destination.Config.WriteMode = "overwrite_range"
	appendRequest.Batch.Checksum = ""
	checksum, err := checksumMallWeatherFeishuRows(
		appendRequest.Batch.Rows,
		len(appendRequest.Batch.Rows),
		len(appendRequest.Batch.Rows[0]),
	)
	if err != nil {
		t.Fatalf("checksumMallWeatherFeishuRows() error=%v", err)
	}
	appendRequest.Batch.Checksum = checksum
	return mallWeatherFeishuOverwriteBatchRequest{
		RunID: appendRequest.RunID, TraceID: appendRequest.TraceID, Destination: appendRequest.Destination,
		DatasetKind: appendRequest.DatasetKind, BatchNo: appendRequest.BatchNo, Attempt: appendRequest.Attempt,
		RowStart: appendRequest.RowStart, Batch: appendRequest.Batch,
	}
}
