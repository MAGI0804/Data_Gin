package data_svc

import (
	"context"
	"errors"

	"gin-biz-web-api/connector/feishu"
)

// findMallWeatherFeishuLastDataRow scans the first column from the grid tail
// toward the header. Unlike append cursor discovery, it tolerates blank gaps
// left by an interrupted stale-tail cleanup. The caller must hold the
// destination execution lock for the scan and subsequent overwrite.
func findMallWeatherFeishuLastDataRow(
	ctx context.Context,
	sheets mallWeatherFeishuRangeReader,
	spreadsheetToken string,
	sheetID string,
	gridRows int64,
) (int64, error) {
	if ctx == nil || sheets == nil || spreadsheetToken == "" || sheetID == "" ||
		gridRows < 1 || gridRows > maxMallWeatherFeishuSheetRow {
		return 0, errors.New("mall weather feishu last row: invalid input")
	}
	if gridRows < 2 {
		return 1, nil
	}
	for endRow := gridRows; endRow >= 2; {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		startRow := max(int64(2), endRow-mallWeatherFeishuAppendScanRows+1)
		values, err := sheets.ReadRange(ctx, spreadsheetToken, feishu.SheetRange{
			SheetID: sheetID, StartRow: startRow, EndRow: endRow, StartColumn: 1, EndColumn: 1,
		})
		if err != nil {
			return 0, err
		}
		if values == nil || int64(len(values.Rows)) > endRow-startRow+1 {
			return 0, errors.New("mall weather feishu last row: invalid range response")
		}
		for index := len(values.Rows) - 1; index >= 0; index-- {
			if !mallWeatherFeishuFirstCellIsBlank(values.Rows[index]) {
				return startRow + int64(index), nil
			}
		}
		if startRow == 2 {
			break
		}
		endRow = startRow - 1
	}
	return 1, nil
}
