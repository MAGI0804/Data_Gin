package data_svc

import (
	"errors"
	"strings"
	"testing"

	"gin-biz-web-api/connector/feishu"
)

func TestVerifyMallWeatherFeishuAppendCheckpointMatchesNormalizedRemoteRows(t *testing.T) {
	t.Parallel()
	requestRows := [][]feishu.SheetCell{
		{{Type: feishu.SheetCellString, Text: "mall-a"}, {Type: feishu.SheetCellNumber, Number: "1"}, {Type: feishu.SheetCellString, Text: ""}},
		{{Type: feishu.SheetCellString, Text: "mall-b"}, {Type: feishu.SheetCellBoolean, Boolean: true}, {Type: feishu.SheetCellBlank}},
	}
	checksum, err := checksumMallWeatherFeishuRows(requestRows, 2, 3)
	if err != nil {
		t.Fatalf("checksumMallWeatherFeishuRows() error=%v", err)
	}
	reader := &fakeMallWeatherFeishuRangeReader{responses: []*feishu.SheetValues{{Rows: [][]feishu.SheetCell{
		{{Type: feishu.SheetCellString, Text: "mall-a"}, {Type: feishu.SheetCellNumber, Number: "1.0"}},
		{{Type: feishu.SheetCellString, Text: "mall-b"}, {Type: feishu.SheetCellBoolean, Boolean: true}},
	}}}}
	matched, err := verifyMallWeatherFeishuAppendCheckpoint(
		t.Context(), reader, "spreadsheet_abc", "sheet_hourly", 12, 13, 3, checksum,
	)
	if err != nil || !matched || len(reader.ranges) != 1 || reader.ranges[0].StartRow != 12 ||
		reader.ranges[0].EndRow != 13 || reader.ranges[0].EndColumn != 3 {
		t.Fatalf("matched=%v error=%v reader=%+v", matched, err, reader)
	}
}

func TestVerifyMallWeatherFeishuAppendCheckpointDetectsMismatch(t *testing.T) {
	t.Parallel()
	rows := [][]feishu.SheetCell{{{Type: feishu.SheetCellString, Text: "expected"}}}
	checksum, err := checksumMallWeatherFeishuRows(rows, 1, 1)
	if err != nil {
		t.Fatalf("checksumMallWeatherFeishuRows() error=%v", err)
	}
	reader := &fakeMallWeatherFeishuRangeReader{responses: []*feishu.SheetValues{{Rows: [][]feishu.SheetCell{{
		{Type: feishu.SheetCellString, Text: "different"},
	}}}}}
	matched, err := verifyMallWeatherFeishuAppendCheckpoint(
		t.Context(), reader, "spreadsheet_abc", "sheet_hourly", 2, 2, 1, checksum,
	)
	if err != nil || matched {
		t.Fatalf("matched=%v error=%v", matched, err)
	}
}

func TestVerifyMallWeatherFeishuAppendCheckpointFailsClosed(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		reader   *fakeMallWeatherFeishuRangeReader
		checksum string
	}{
		{name: "invalid checksum", reader: &fakeMallWeatherFeishuRangeReader{}, checksum: "invalid"},
		{name: "missing response", reader: &fakeMallWeatherFeishuRangeReader{responses: []*feishu.SheetValues{nil}}, checksum: strings.Repeat("a", 64)},
		{name: "read failure", reader: &fakeMallWeatherFeishuRangeReader{err: errors.New("read unavailable")}, checksum: strings.Repeat("a", 64)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := verifyMallWeatherFeishuAppendCheckpoint(
				t.Context(), test.reader, "spreadsheet_abc", "sheet_hourly", 2, 2, 1, test.checksum,
			); err == nil {
				t.Fatal("verifyMallWeatherFeishuAppendCheckpoint() error=nil")
			}
		})
	}
}
