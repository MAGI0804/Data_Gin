package data_svc

import (
	"context"
	"errors"

	"gin-biz-web-api/connector/feishu"
)

const mallWeatherFeishuAppendScanRows = int64(5_000)

type mallWeatherFeishuRangeReader interface {
	ReadRange(context.Context, string, feishu.SheetRange) (*feishu.SheetValues, error)
}

// findMallWeatherFeishuAppendRow follows Feishu's values_append rule: the
// first blank cell in the first column of the requested range is the append
// position. The caller must hold the destination execution lock throughout
// this scan and the subsequent append calls.
func findMallWeatherFeishuAppendRow(
	ctx context.Context,
	sheets mallWeatherFeishuRangeReader,
	spreadsheetToken string,
	sheetID string,
	gridRows int64,
) (int64, error) {
	if ctx == nil || sheets == nil || spreadsheetToken == "" || sheetID == "" ||
		gridRows < 1 || gridRows > maxMallWeatherFeishuSheetRow {
		return 0, errors.New("mall weather feishu append cursor: invalid input")
	}
	if gridRows < 2 {
		return 2, nil
	}
	for startRow := int64(2); startRow <= gridRows; {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		endRow := min(startRow+mallWeatherFeishuAppendScanRows-1, gridRows)
		values, err := sheets.ReadRange(ctx, spreadsheetToken, feishu.SheetRange{
			SheetID: sheetID, StartRow: startRow, EndRow: endRow, StartColumn: 1, EndColumn: 1,
		})
		if err != nil {
			return 0, err
		}
		if values == nil || int64(len(values.Rows)) > endRow-startRow+1 {
			return 0, errors.New("mall weather feishu append cursor: invalid range response")
		}
		for index, row := range values.Rows {
			if mallWeatherFeishuFirstCellIsBlank(row) {
				return startRow + int64(index), nil
			}
		}
		if int64(len(values.Rows)) < endRow-startRow+1 {
			return startRow + int64(len(values.Rows)), nil
		}
		startRow = endRow + 1
	}
	if gridRows == maxMallWeatherFeishuSheetRow {
		return 0, errors.New("mall weather feishu append cursor: sheet row limit reached")
	}
	return gridRows + 1, nil
}

func mallWeatherFeishuFirstCellIsBlank(row []feishu.SheetCell) bool {
	if len(row) == 0 {
		return true
	}
	cell := row[0]
	return cell.Type == feishu.SheetCellBlank || (cell.Type == feishu.SheetCellString && cell.Text == "")
}
