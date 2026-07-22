package data_svc

import (
	"errors"
	"testing"

	"gin-biz-web-api/connector/feishu"
)

func TestFindMallWeatherFeishuLastDataRowFindsDataAfterBlankGap(t *testing.T) {
	t.Parallel()
	reader := &fakeMallWeatherFeishuRangeReader{responses: []*feishu.SheetValues{{Rows: [][]feishu.SheetCell{
		{{Type: feishu.SheetCellString, Text: "new-a"}},
		{{Type: feishu.SheetCellString, Text: "new-b"}},
		{},
		{},
		{{Type: feishu.SheetCellString, Text: "stale-a"}},
		{{Type: feishu.SheetCellString, Text: "stale-b"}},
	}}}}
	row, err := findMallWeatherFeishuLastDataRow(
		t.Context(), reader, "spreadsheet_abc", "sheet_hourly", 10,
	)
	if err != nil || row != 7 || len(reader.ranges) != 1 || reader.ranges[0].StartRow != 2 ||
		reader.ranges[0].EndRow != 10 || reader.ranges[0].StartColumn != 1 || reader.ranges[0].EndColumn != 1 {
		t.Fatalf("row=%d error=%v reader=%+v", row, err, reader)
	}
}

func TestFindMallWeatherFeishuLastDataRowScansBackwardInBoundedPages(t *testing.T) {
	t.Parallel()
	reader := &fakeMallWeatherFeishuRangeReader{responses: []*feishu.SheetValues{
		{},
		{Rows: [][]feishu.SheetCell{
			{{Type: feishu.SheetCellString, Text: "occupied"}},
		}},
	}}
	gridRows := mallWeatherFeishuAppendScanRows + 20
	row, err := findMallWeatherFeishuLastDataRow(
		t.Context(), reader, "spreadsheet_abc", "sheet_hourly", gridRows,
	)
	if err != nil || row != 2 || len(reader.ranges) != 2 ||
		reader.ranges[0].StartRow != 21 || reader.ranges[0].EndRow != gridRows ||
		reader.ranges[1].StartRow != 2 || reader.ranges[1].EndRow != 20 {
		t.Fatalf("row=%d error=%v reader=%+v", row, err, reader)
	}
}

func TestFindMallWeatherFeishuLastDataRowHandlesEmptyAndFailure(t *testing.T) {
	t.Parallel()
	reader := &fakeMallWeatherFeishuRangeReader{responses: []*feishu.SheetValues{{}}}
	row, err := findMallWeatherFeishuLastDataRow(
		t.Context(), reader, "spreadsheet_abc", "sheet_hourly", 10,
	)
	if err != nil || row != 1 {
		t.Fatalf("row=%d error=%v", row, err)
	}
	wantErr := errors.New("read unavailable")
	if _, err := findMallWeatherFeishuLastDataRow(
		t.Context(), &fakeMallWeatherFeishuRangeReader{err: wantErr},
		"spreadsheet_abc", "sheet_hourly", 10,
	); !errors.Is(err, wantErr) {
		t.Fatalf("findMallWeatherFeishuLastDataRow() error=%v", err)
	}
}
