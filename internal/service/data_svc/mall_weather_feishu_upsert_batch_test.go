package data_svc

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"

	"gin-biz-web-api/connector/feishu"
	"gin-biz-web-api/pkg/providerhttp"
)

func TestMallWeatherFeishuUpsertBatchExecutorAppendsAcknowledgedRows(t *testing.T) {
	t.Parallel()
	events := make([]string, 0, 3)
	request := validMallWeatherFeishuUpsertBatchRequest(t, "append", 12, 13)
	sheets := &fakeMallWeatherFeishuUpsertSheets{
		events: &events,
		appendResult: &feishu.SheetWriteResult{
			Revision: 19, UpdatedRowStart: 12, UpdatedRowEnd: 13,
			UpdatedRows: 2, UpdatedColumns: 2, UpdatedCells: 4,
		},
	}
	logs := &fakeMallWeatherFeishuBatchLogStore{events: &events, createID: 51}
	executor, err := newMallWeatherFeishuUpsertBatchExecutor(sheets, logs, sequentialMallWeatherFeishuNow())
	if err != nil {
		t.Fatalf("newMallWeatherFeishuUpsertBatchExecutor() error=%v", err)
	}

	result, err := executor.Execute(t.Context(), request)
	if err != nil {
		t.Fatalf("Execute() error=%v", err)
	}
	if !slices.Equal(events, []string{"create", "append", "finish"}) || result.DeliveryLogID != 51 ||
		result.Mode != "append" || result.BatchNo != 2 || result.RecordCount != 2 || result.CellCount != 4 ||
		result.Revision != 19 || len(result.Rows) != 2 || logs.created == nil || logs.created.Status != "running" ||
		logs.created.RowStart != 12 || logs.created.RowEnd != 13 || logs.created.RequestChecksum == "" ||
		logs.finished.Status != "success" || !logs.finished.Success || logs.finished.HTTPStatus != http.StatusOK {
		t.Fatalf("events=%v result=%+v sheets=%+v logs=%+v", events, result, sheets, logs)
	}
	if sheets.appendCalls != 1 || sheets.appendWrite.Range.SheetID != "sheet-hourly-secret" ||
		sheets.appendWrite.Range.StartRow != 12 || sheets.appendWrite.Range.EndRow != 13 ||
		len(sheets.appendWrite.Rows) != 2 {
		t.Fatalf("sheets=%+v", sheets)
	}
}

func TestMallWeatherFeishuUpsertBatchExecutorRejectsUncertainAppendAcknowledgement(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		result *feishu.SheetWriteResult
	}{
		{name: "missing acknowledgement"},
		{name: "mismatched acknowledgement", result: &feishu.SheetWriteResult{
			Revision: 1, UpdatedRowStart: 12, UpdatedRowEnd: 12,
			UpdatedRows: 1, UpdatedColumns: 2, UpdatedCells: 2,
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			sheets := &fakeMallWeatherFeishuUpsertSheets{appendResult: test.result}
			logs := &fakeMallWeatherFeishuBatchLogStore{createID: 51}
			executor, err := newMallWeatherFeishuUpsertBatchExecutor(sheets, logs, sequentialMallWeatherFeishuNow())
			if err != nil {
				t.Fatalf("newMallWeatherFeishuUpsertBatchExecutor() error=%v", err)
			}

			_, err = executor.Execute(t.Context(), validMallWeatherFeishuUpsertBatchRequest(t, "append", 12, 13))
			if !errors.Is(err, ErrMallWeatherFeishuUpsertStateUnknown) || sheets.appendCalls != 1 ||
				logs.finished.Status != "unknown" {
				t.Fatalf("Execute() error=%v sheets=%+v finish=%+v", err, sheets, logs.finished)
			}
		})
	}
}

func TestMallWeatherFeishuUpsertBatchExecutorMergesAndVerifiesUpdateRanges(t *testing.T) {
	t.Parallel()
	request := validMallWeatherFeishuUpsertBatchRequest(t, "update", 12, 13, 15)
	sheets := &fakeMallWeatherFeishuUpsertSheets{
		updateResult: &feishu.SheetWriteResult{Revision: 23},
		readResults: []*feishu.SheetValues{
			{Rows: [][]feishu.SheetCell{request.Rows[0].Cells, request.Rows[1].Cells}},
			{Rows: [][]feishu.SheetCell{request.Rows[2].Cells}},
		},
	}
	logs := &fakeMallWeatherFeishuBatchLogStore{createID: 52}
	executor, err := newMallWeatherFeishuUpsertBatchExecutor(sheets, logs, sequentialMallWeatherFeishuNow())
	if err != nil {
		t.Fatalf("newMallWeatherFeishuUpsertBatchExecutor() error=%v", err)
	}

	result, err := executor.Execute(t.Context(), request)
	if err != nil || result.Revision != 23 || logs.finished.Status != "success" ||
		len(sheets.updateWrites) != 2 || len(sheets.reads) != 2 {
		t.Fatalf("Execute() result=%+v error=%v sheets=%+v finish=%+v", result, err, sheets, logs.finished)
	}
	if first, second := sheets.updateWrites[0].Range, sheets.updateWrites[1].Range; first.StartRow != 12 || first.EndRow != 13 || second.StartRow != 15 || second.EndRow != 15 ||
		len(sheets.updateWrites[0].Rows) != 2 || len(sheets.updateWrites[1].Rows) != 1 {
		t.Fatalf("writes=%+v", sheets.updateWrites)
	}
}

func TestMallWeatherFeishuUpsertBatchExecutorRecoversUncertainUpdate(t *testing.T) {
	t.Parallel()
	request := validMallWeatherFeishuUpsertBatchRequest(t, "update", 12)
	sheets := &fakeMallWeatherFeishuUpsertSheets{
		updateErr: &feishu.SheetsError{
			Class: providerhttp.ErrorClassProvider, Retryable: true, HTTPCode: http.StatusServiceUnavailable,
		},
		readResults: []*feishu.SheetValues{{Rows: [][]feishu.SheetCell{request.Rows[0].Cells}}},
	}
	logs := &fakeMallWeatherFeishuBatchLogStore{createID: 53}
	executor, err := newMallWeatherFeishuUpsertBatchExecutor(sheets, logs, sequentialMallWeatherFeishuNow())
	if err != nil {
		t.Fatalf("newMallWeatherFeishuUpsertBatchExecutor() error=%v", err)
	}

	if _, err := executor.Execute(t.Context(), request); err != nil || logs.finished.Status != "success" ||
		!logs.finished.Success || logs.finished.HTTPStatus != http.StatusServiceUnavailable || len(sheets.reads) != 1 {
		t.Fatalf("Execute() error=%v sheets=%+v finish=%+v", err, sheets, logs.finished)
	}
}

func TestMallWeatherFeishuUpsertBatchExecutorFailsClosedWhenUpdateVerificationFails(t *testing.T) {
	t.Parallel()
	request := validMallWeatherFeishuUpsertBatchRequest(t, "update", 12)
	sheets := &fakeMallWeatherFeishuUpsertSheets{
		updateResult: &feishu.SheetWriteResult{Revision: 24},
		readErr:      errors.New("read unavailable"),
	}
	logs := &fakeMallWeatherFeishuBatchLogStore{createID: 57}
	executor, err := newMallWeatherFeishuUpsertBatchExecutor(sheets, logs, sequentialMallWeatherFeishuNow())
	if err != nil {
		t.Fatalf("newMallWeatherFeishuUpsertBatchExecutor() error=%v", err)
	}

	_, err = executor.Execute(t.Context(), request)
	if !errors.Is(err, ErrMallWeatherFeishuUpsertStateUnknown) || logs.finished.Status != "unknown" ||
		logs.finished.ResponseSummary != "fixed range checksum read failed" || len(sheets.reads) != 1 {
		t.Fatalf("Execute() error=%v sheets=%+v finish=%+v", err, sheets, logs.finished)
	}
}

func TestMallWeatherFeishuUpsertBatchExecutorClassifiesDefiniteFailures(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		writeErr      *feishu.SheetsError
		wantRetryable bool
	}{
		{
			name: "rate limit is retryable",
			writeErr: &feishu.SheetsError{
				Class: providerhttp.ErrorClassRateLimited, Retryable: true, HTTPCode: http.StatusTooManyRequests,
			},
			wantRetryable: true,
		},
		{
			name: "forbidden is permanent",
			writeErr: &feishu.SheetsError{
				Class: providerhttp.ErrorClassAuth, HTTPCode: http.StatusForbidden,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			sheets := &fakeMallWeatherFeishuUpsertSheets{updateErr: test.writeErr}
			logs := &fakeMallWeatherFeishuBatchLogStore{createID: 54}
			executor, err := newMallWeatherFeishuUpsertBatchExecutor(sheets, logs, sequentialMallWeatherFeishuNow())
			if err != nil {
				t.Fatalf("newMallWeatherFeishuUpsertBatchExecutor() error=%v", err)
			}

			_, err = executor.Execute(t.Context(), validMallWeatherFeishuUpsertBatchRequest(t, "update", 12))
			var executionErr *MallWeatherFeishuExecutionError
			if !errors.As(err, &executionErr) || executionErr == nil ||
				executionErr.Retryable != test.wantRetryable || logs.finished.Status != "failed" || len(sheets.reads) != 0 {
				t.Fatalf("Execute() error=%v sheets=%+v finish=%+v", err, sheets, logs.finished)
			}
		})
	}
}

func TestMallWeatherFeishuUpsertBatchExecutorFailsClosedWhenCheckpointFinishFails(t *testing.T) {
	t.Parallel()
	request := validMallWeatherFeishuUpsertBatchRequest(t, "append", 12)
	sheets := &fakeMallWeatherFeishuUpsertSheets{appendResult: &feishu.SheetWriteResult{
		Revision: 1, UpdatedRowStart: 12, UpdatedRowEnd: 12,
		UpdatedRows: 1, UpdatedColumns: 2, UpdatedCells: 2,
	}}
	logs := &fakeMallWeatherFeishuBatchLogStore{
		createID: 55, finishErr: errors.New("database unavailable"),
	}
	executor, err := newMallWeatherFeishuUpsertBatchExecutor(sheets, logs, sequentialMallWeatherFeishuNow())
	if err != nil {
		t.Fatalf("newMallWeatherFeishuUpsertBatchExecutor() error=%v", err)
	}

	_, err = executor.Execute(t.Context(), request)
	if !errors.Is(err, ErrMallWeatherFeishuUpsertStateUnknown) || sheets.appendCalls != 1 {
		t.Fatalf("Execute() error=%v sheets=%+v logs=%+v", err, sheets, logs)
	}
}

func TestMallWeatherFeishuUpsertBatchExecutorRejectsInvalidRowsBeforeSideEffects(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		mode   string
		rows   []int64
		mutate func(*mallWeatherFeishuUpsertBatchRequest)
	}{
		{name: "append rows are not contiguous", mode: "append", rows: []int64{12, 14}},
		{name: "business key is duplicated", mode: "update", rows: []int64{12, 13}, mutate: func(request *mallWeatherFeishuUpsertBatchRequest) {
			request.Rows[1].BusinessKey = request.Rows[0].BusinessKey
		}},
		{name: "row number is duplicated", mode: "update", rows: []int64{12, 13}, mutate: func(request *mallWeatherFeishuUpsertBatchRequest) {
			request.Rows[1].RowNumber = request.Rows[0].RowNumber
		}},
		{name: "checksum is inconsistent", mode: "update", rows: []int64{12}, mutate: func(request *mallWeatherFeishuUpsertBatchRequest) {
			request.Rows[0].Checksum = strings.Repeat("0", 64)
		}},
		{name: "business key prefix is missing", mode: "update", rows: []int64{12}, mutate: func(request *mallWeatherFeishuUpsertBatchRequest) {
			request.Rows[0].BusinessKey = strings.Repeat("a", 64)
		}},
		{name: "business key digest is uppercase", mode: "update", rows: []int64{12}, mutate: func(request *mallWeatherFeishuUpsertBatchRequest) {
			request.Rows[0].BusinessKey = "sha256:" + strings.Repeat("A", 64)
		}},
		{name: "dataset kind is not allowed", mode: "update", rows: []int64{12}, mutate: func(request *mallWeatherFeishuUpsertBatchRequest) {
			request.DatasetKind = "custom"
			request.Destination.SheetIDs["custom"] = "sheet-custom-secret"
			request.Destination.Config.SheetIDEnvMapping["custom"] = "FEISHU_CUSTOM_SHEET_ID"
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := validMallWeatherFeishuUpsertBatchRequest(t, test.mode, test.rows...)
			if test.mutate != nil {
				test.mutate(&request)
			}
			sheets := &fakeMallWeatherFeishuUpsertSheets{}
			logs := &fakeMallWeatherFeishuBatchLogStore{createID: 56}
			executor, err := newMallWeatherFeishuUpsertBatchExecutor(sheets, logs, sequentialMallWeatherFeishuNow())
			if err != nil {
				t.Fatalf("newMallWeatherFeishuUpsertBatchExecutor() error=%v", err)
			}

			if _, err := executor.Execute(t.Context(), request); err == nil || logs.createCalls != 0 ||
				sheets.appendCalls != 0 || sheets.updateCalls != 0 || len(sheets.reads) != 0 {
				t.Fatalf("Execute() error=%v sheets=%+v logs=%+v", err, sheets, logs)
			}
		})
	}
}

type fakeMallWeatherFeishuUpsertSheets struct {
	events       *[]string
	appendResult *feishu.SheetWriteResult
	appendErr    error
	updateResult *feishu.SheetWriteResult
	updateErr    error
	readResults  []*feishu.SheetValues
	readErr      error
	appendCalls  int
	updateCalls  int
	appendWrite  feishu.SheetWriteRange
	updateWrites []feishu.SheetWriteRange
	reads        []feishu.SheetRange
}

func (sheets *fakeMallWeatherFeishuUpsertSheets) AppendValues(
	_ context.Context,
	_ string,
	write feishu.SheetWriteRange,
) (*feishu.SheetWriteResult, error) {
	sheets.appendCalls++
	sheets.appendWrite = write
	if sheets.events != nil {
		*sheets.events = append(*sheets.events, "append")
	}
	return sheets.appendResult, sheets.appendErr
}

func (sheets *fakeMallWeatherFeishuUpsertSheets) BatchUpdateValues(
	_ context.Context,
	_ string,
	writes []feishu.SheetWriteRange,
) (*feishu.SheetWriteResult, error) {
	sheets.updateCalls++
	sheets.updateWrites = append([]feishu.SheetWriteRange(nil), writes...)
	if sheets.events != nil {
		*sheets.events = append(*sheets.events, "update")
	}
	return sheets.updateResult, sheets.updateErr
}

func (sheets *fakeMallWeatherFeishuUpsertSheets) ReadRange(
	_ context.Context,
	_ string,
	readRange feishu.SheetRange,
) (*feishu.SheetValues, error) {
	index := len(sheets.reads)
	sheets.reads = append(sheets.reads, readRange)
	if sheets.events != nil {
		*sheets.events = append(*sheets.events, "read")
	}
	if sheets.readErr != nil {
		return nil, sheets.readErr
	}
	if index >= len(sheets.readResults) {
		return nil, errors.New("missing fake read result")
	}
	return sheets.readResults[index], nil
}

func validMallWeatherFeishuUpsertBatchRequest(
	t *testing.T,
	mode string,
	rowNumbers ...int64,
) mallWeatherFeishuUpsertBatchRequest {
	t.Helper()
	input := validMallWeatherFeishuDryRunPlanInput()
	input.Destination.Config.WriteMode = "upsert"
	input.Destination.Config.BatchRows = 100
	rows := make([]mallWeatherFeishuUpsertWriteRow, len(rowNumbers))
	for index, rowNumber := range rowNumbers {
		cells := []feishu.SheetCell{
			{Type: feishu.SheetCellString, Text: "mall-" + string(rune('a'+index))},
			{Type: feishu.SheetCellNumber, Number: "1"},
		}
		checksum, err := checksumMallWeatherFeishuRows([][]feishu.SheetCell{cells}, 1, len(cells))
		if err != nil {
			t.Fatalf("checksumMallWeatherFeishuRows() error=%v", err)
		}
		rows[index] = mallWeatherFeishuUpsertWriteRow{
			BusinessKey: "sha256:" + strings.Repeat(string(rune('a'+index)), 64),
			RowNumber:   rowNumber,
			Checksum:    checksum,
			Cells:       cells,
		}
	}
	return mallWeatherFeishuUpsertBatchRequest{
		RunID: 23, TraceID: "11111111-1111-4111-8111-111111111111", Destination: input.Destination,
		DatasetKind: "hourly", BatchNo: 2, Attempt: 1, Mode: mode, Rows: rows,
	}
}
