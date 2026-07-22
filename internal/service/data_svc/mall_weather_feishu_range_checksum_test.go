package data_svc

import (
	"testing"

	"gin-biz-web-api/connector/feishu"
)

func TestVerifyMallWeatherFeishuRangeChecksumSupportsFixedRanges(t *testing.T) {
	t.Parallel()
	rows := [][]feishu.SheetCell{{
		{Type: feishu.SheetCellString, Text: "mall-a"},
		{Type: feishu.SheetCellNumber, Number: "1"},
	}}
	checksum, err := checksumMallWeatherFeishuRows(rows, 1, 2)
	if err != nil {
		t.Fatalf("checksumMallWeatherFeishuRows() error=%v", err)
	}
	reader := &fakeMallWeatherFeishuRangeReader{responses: []*feishu.SheetValues{{Rows: [][]feishu.SheetCell{{
		{Type: feishu.SheetCellString, Text: "mall-a"},
		{Type: feishu.SheetCellNumber, Number: "1.0"},
	}}}}}
	matched, err := verifyMallWeatherFeishuRangeChecksum(
		t.Context(), reader, "spreadsheet_abc", "sheet_hourly", 1, 1, 2, checksum,
	)
	if err != nil || !matched || len(reader.ranges) != 1 || reader.ranges[0].StartRow != 1 ||
		reader.ranges[0].EndRow != 1 || reader.ranges[0].EndColumn != 2 {
		t.Fatalf("matched=%v error=%v reader=%+v", matched, err, reader)
	}
}
