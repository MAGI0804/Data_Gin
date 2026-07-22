package data_svc

import (
	"context"
	"errors"
	"testing"

	"gin-biz-web-api/connector/feishu"
	"gin-biz-web-api/internal/requestbody"
)

func TestEnsureMallWeatherFeishuHeadersWritesEmptyHeaderAndVerifies(t *testing.T) {
	t.Parallel()
	destination, profile := validMallWeatherFeishuHeaderWriterInput()
	sheets := &fakeMallWeatherFeishuHeaderSheets{
		reads: map[string][]*feishu.SheetValues{
			"sheet-hourly-secret": {
				{Revision: 7, Rows: [][]feishu.SheetCell{}},
				matchedMallWeatherFeishuHeader(),
			},
		},
	}
	outcomes, err := ensureMallWeatherFeishuHeaders(t.Context(), destination, profile, sheets)
	if err != nil {
		t.Fatalf("ensureMallWeatherFeishuHeaders() error=%v", err)
	}
	if len(outcomes) != 1 || outcomes[0].DatasetKind != "hourly" || outcomes[0].Action != "WRITE" ||
		len(sheets.readCalls) != 2 || len(sheets.writeCalls) != 1 || len(sheets.writeCalls[0]) != 1 {
		t.Fatalf("outcomes=%+v sheets=%+v", outcomes, sheets)
	}
	write := sheets.writeCalls[0][0]
	if write.Range.SheetID != "sheet-hourly-secret" || write.Range.StartRow != 1 || write.Range.EndRow != 1 ||
		write.Range.StartColumn != 1 || write.Range.EndColumn != 3 || len(write.Rows) != 1 ||
		len(write.Rows[0]) != 3 || write.Rows[0][0].Text != "Mall" || write.Rows[0][2].Text != "Issued" {
		t.Fatalf("write=%+v", write)
	}
}

func TestEnsureMallWeatherFeishuHeadersSkipsMatchedHeader(t *testing.T) {
	t.Parallel()
	destination, profile := validMallWeatherFeishuHeaderWriterInput()
	sheets := &fakeMallWeatherFeishuHeaderSheets{reads: map[string][]*feishu.SheetValues{
		"sheet-hourly-secret": {matchedMallWeatherFeishuHeader()},
	}}
	outcomes, err := ensureMallWeatherFeishuHeaders(t.Context(), destination, profile, sheets)
	if err != nil || len(outcomes) != 0 || len(sheets.readCalls) != 1 || len(sheets.writeCalls) != 0 {
		t.Fatalf("outcomes=%+v error=%v sheets=%+v", outcomes, err, sheets)
	}
}

func TestEnsureMallWeatherFeishuHeadersBlocksMismatchBeforeAnyWrite(t *testing.T) {
	t.Parallel()
	destination, profile := validMallWeatherFeishuHeaderWriterInput()
	addMallWeatherFeishuRealtimeHeaderDataset(destination, &profile)
	sheets := &fakeMallWeatherFeishuHeaderSheets{reads: map[string][]*feishu.SheetValues{
		"sheet-hourly-secret": {{Rows: [][]feishu.SheetCell{}}},
		"sheet-realtime-secret": {{Rows: [][]feishu.SheetCell{{
			{Type: feishu.SheetCellString, Text: "Wrong Mall"},
			{Type: feishu.SheetCellString, Text: "Snapshot"},
		}}}},
	}}
	_, err := ensureMallWeatherFeishuHeaders(t.Context(), destination, profile, sheets)
	if !errors.Is(err, ErrMallWeatherFeishuHeaderConflict) || len(sheets.readCalls) != 2 ||
		len(sheets.writeCalls) != 0 {
		t.Fatalf("ensureMallWeatherFeishuHeaders() error=%v sheets=%+v", err, sheets)
	}
}

func TestEnsureMallWeatherFeishuHeadersRewritesOnlyWhenEnabled(t *testing.T) {
	t.Parallel()
	destination, profile := validMallWeatherFeishuHeaderWriterInput()
	destination.Config.AllowHeaderRewrite = true
	mismatch := &feishu.SheetValues{Rows: [][]feishu.SheetCell{{
		{Type: feishu.SheetCellString, Text: "Mall"},
		{Type: feishu.SheetCellString, Text: "Wrong"},
		{Type: feishu.SheetCellString, Text: "Issued"},
	}}}
	sheets := &fakeMallWeatherFeishuHeaderSheets{reads: map[string][]*feishu.SheetValues{
		"sheet-hourly-secret": {mismatch, matchedMallWeatherFeishuHeader()},
	}}
	outcomes, err := ensureMallWeatherFeishuHeaders(t.Context(), destination, profile, sheets)
	if err != nil || len(outcomes) != 1 || outcomes[0].Action != "REWRITE" ||
		len(sheets.writeCalls) != 1 || len(sheets.readCalls) != 2 {
		t.Fatalf("outcomes=%+v error=%v sheets=%+v", outcomes, err, sheets)
	}
}

func TestEnsureMallWeatherFeishuHeadersFailsClosedAfterWrite(t *testing.T) {
	t.Parallel()
	destination, profile := validMallWeatherFeishuHeaderWriterInput()
	verificationMismatch := &feishu.SheetValues{Rows: [][]feishu.SheetCell{{
		{Type: feishu.SheetCellString, Text: "Unexpected"},
		{Type: feishu.SheetCellString, Text: "Forecast"},
		{Type: feishu.SheetCellString, Text: "Issued"},
	}}}
	tests := []struct {
		name      string
		sheets    *fakeMallWeatherFeishuHeaderSheets
		wantWrite bool
	}{
		{
			name: "missing acknowledgement",
			sheets: &fakeMallWeatherFeishuHeaderSheets{
				reads:  map[string][]*feishu.SheetValues{"sheet-hourly-secret": {{Rows: [][]feishu.SheetCell{}}}},
				nilAck: true,
			},
			wantWrite: true,
		},
		{
			name: "verification mismatch",
			sheets: &fakeMallWeatherFeishuHeaderSheets{reads: map[string][]*feishu.SheetValues{
				"sheet-hourly-secret": {{Rows: [][]feishu.SheetCell{}}, verificationMismatch},
			}},
			wantWrite: true,
		},
		{
			name: "write failure",
			sheets: &fakeMallWeatherFeishuHeaderSheets{
				reads:    map[string][]*feishu.SheetValues{"sheet-hourly-secret": {{Rows: [][]feishu.SheetCell{}}}},
				writeErr: errors.New("write unavailable"),
			},
			wantWrite: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := ensureMallWeatherFeishuHeaders(t.Context(), destination, profile, test.sheets); err == nil {
				t.Fatal("ensureMallWeatherFeishuHeaders() error=nil")
			}
			if got := len(test.sheets.writeCalls) == 1; got != test.wantWrite {
				t.Fatalf("write calls=%d wantWrite=%v", len(test.sheets.writeCalls), test.wantWrite)
			}
		})
	}
}

func TestEnsureMallWeatherFeishuHeadersRejectsInvalidInputBeforeRemoteRead(t *testing.T) {
	t.Parallel()
	destination, profile := validMallWeatherFeishuHeaderWriterInput()
	profile.Datasets = append(profile.Datasets, profile.Datasets[0])
	sheets := &fakeMallWeatherFeishuHeaderSheets{}
	if _, err := ensureMallWeatherFeishuHeaders(t.Context(), destination, profile, sheets); err == nil {
		t.Fatal("ensureMallWeatherFeishuHeaders() accepted duplicate dataset")
	}
	if len(sheets.readCalls) != 0 || len(sheets.writeCalls) != 0 {
		t.Fatalf("remote calls occurred: %+v", sheets)
	}
}

type fakeMallWeatherFeishuHeaderSheets struct {
	reads      map[string][]*feishu.SheetValues
	readErrors map[string]error
	writeErr   error
	nilAck     bool
	readCalls  []feishu.SheetRange
	writeCalls [][]feishu.SheetWriteRange
}

func (sheets *fakeMallWeatherFeishuHeaderSheets) ReadRange(
	_ context.Context,
	_ string,
	readRange feishu.SheetRange,
) (*feishu.SheetValues, error) {
	sheets.readCalls = append(sheets.readCalls, readRange)
	if err := sheets.readErrors[readRange.SheetID]; err != nil {
		return nil, err
	}
	values := sheets.reads[readRange.SheetID]
	if len(values) == 0 {
		return nil, errors.New("unexpected header read")
	}
	sheets.reads[readRange.SheetID] = values[1:]
	return values[0], nil
}

func (sheets *fakeMallWeatherFeishuHeaderSheets) BatchUpdateValues(
	_ context.Context,
	_ string,
	writes []feishu.SheetWriteRange,
) (*feishu.SheetWriteResult, error) {
	sheets.writeCalls = append(sheets.writeCalls, append([]feishu.SheetWriteRange(nil), writes...))
	if sheets.writeErr != nil {
		return nil, sheets.writeErr
	}
	if sheets.nilAck {
		return nil, nil
	}
	return &feishu.SheetWriteResult{Revision: 9}, nil
}

func validMallWeatherFeishuHeaderWriterInput() (*MallWeatherFeishuResolvedDestination, MallWeatherExportProfileDTO) {
	input := validMallWeatherFeishuDryRunPlanInput()
	return input.Destination, input.Profile
}

func addMallWeatherFeishuRealtimeHeaderDataset(
	destination *MallWeatherFeishuResolvedDestination,
	profile *MallWeatherExportProfileDTO,
) {
	profile.Datasets = append(profile.Datasets, requestbody.MallWeatherExportDataset{
		Kind: "realtime",
		Columns: []requestbody.MallWeatherExportColumn{
			{Field: "mall_code", Title: "Mall"},
			{Field: "snapshot_at", Title: "Snapshot"},
		},
	})
	destination.SheetIDs["realtime"] = "sheet-realtime-secret"
	destination.Config.SheetIDEnvMapping["realtime"] = "FEISHU_WEATHER_REALTIME_SHEET_ID"
}
