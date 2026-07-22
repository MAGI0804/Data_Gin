package data_svc

import (
	"context"
	"errors"
	"testing"

	"gin-biz-web-api/connector/feishu"
)

func TestFindMallWeatherFeishuAppendRowScansToFirstBlank(t *testing.T) {
	t.Parallel()
	reader := &fakeMallWeatherFeishuRangeReader{responses: []*feishu.SheetValues{
		{Rows: [][]feishu.SheetCell{
			{{Type: feishu.SheetCellString, Text: "mall-a"}},
			{{Type: feishu.SheetCellNumber, Number: "12"}},
			{},
			{{Type: feishu.SheetCellString, Text: "ignored-after-first-blank"}},
		}},
	}}
	row, err := findMallWeatherFeishuAppendRow(t.Context(), reader, "spreadsheet_abc", "sheet_hourly", 100)
	if err != nil || row != 4 || len(reader.ranges) != 1 || reader.ranges[0].StartRow != 2 ||
		reader.ranges[0].EndRow != 100 || reader.ranges[0].StartColumn != 1 || reader.ranges[0].EndColumn != 1 {
		t.Fatalf("row=%d error=%v reader=%+v", row, err, reader)
	}
}

func TestFindMallWeatherFeishuAppendRowScansBoundedPages(t *testing.T) {
	t.Parallel()
	firstPage := make([][]feishu.SheetCell, mallWeatherFeishuAppendScanRows)
	for index := range firstPage {
		firstPage[index] = []feishu.SheetCell{{Type: feishu.SheetCellString, Text: "occupied"}}
	}
	reader := &fakeMallWeatherFeishuRangeReader{responses: []*feishu.SheetValues{
		{Rows: firstPage},
		{Rows: [][]feishu.SheetCell{{{Type: feishu.SheetCellString, Text: "occupied"}}}},
	}}
	row, err := findMallWeatherFeishuAppendRow(
		t.Context(), reader, "spreadsheet_abc", "sheet_hourly", mallWeatherFeishuAppendScanRows+20,
	)
	if err != nil || row != mallWeatherFeishuAppendScanRows+3 || len(reader.ranges) != 2 ||
		reader.ranges[1].StartRow != mallWeatherFeishuAppendScanRows+2 {
		t.Fatalf("row=%d error=%v reader=%+v", row, err, reader)
	}
}

func TestFindMallWeatherFeishuAppendRowHandlesCapacityBoundaries(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		gridRows  int64
		responses []*feishu.SheetValues
		wantRow   int64
		wantError bool
	}{
		{name: "header-only grid", gridRows: 1, wantRow: 2},
		{
			name: "short response marks trailing blank", gridRows: 10,
			responses: []*feishu.SheetValues{{Rows: [][]feishu.SheetCell{
				{{Type: feishu.SheetCellString, Text: "occupied"}},
			}}},
			wantRow: 3,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reader := &fakeMallWeatherFeishuRangeReader{responses: test.responses}
			row, err := findMallWeatherFeishuAppendRow(
				t.Context(), reader, "spreadsheet_abc", "sheet_hourly", test.gridRows,
			)
			if row != test.wantRow || (err != nil) != test.wantError {
				t.Fatalf("row=%d error=%v", row, err)
			}
		})
	}
}

func TestFindMallWeatherFeishuAppendRowPropagatesReadFailure(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("read unavailable")
	reader := &fakeMallWeatherFeishuRangeReader{err: wantErr}
	if _, err := findMallWeatherFeishuAppendRow(
		t.Context(), reader, "spreadsheet_abc", "sheet_hourly", 100,
	); !errors.Is(err, wantErr) {
		t.Fatalf("findMallWeatherFeishuAppendRow() error=%v", err)
	}
}

type fakeMallWeatherFeishuRangeReader struct {
	responses []*feishu.SheetValues
	err       error
	ranges    []feishu.SheetRange
}

func (reader *fakeMallWeatherFeishuRangeReader) ReadRange(
	_ context.Context,
	_ string,
	readRange feishu.SheetRange,
) (*feishu.SheetValues, error) {
	reader.ranges = append(reader.ranges, readRange)
	if reader.err != nil {
		return nil, reader.err
	}
	if len(reader.responses) == 0 {
		return nil, errors.New("unexpected range read")
	}
	response := reader.responses[0]
	reader.responses = reader.responses[1:]
	return response, nil
}
