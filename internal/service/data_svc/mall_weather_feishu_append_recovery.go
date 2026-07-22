package data_svc

import (
	"context"
	"encoding/hex"
	"errors"

	"gin-biz-web-api/connector/feishu"
)

func verifyMallWeatherFeishuAppendCheckpoint(
	ctx context.Context,
	sheets mallWeatherFeishuRangeReader,
	spreadsheetToken string,
	sheetID string,
	rowStart int64,
	rowEnd int64,
	columns int,
	expectedChecksum string,
) (bool, error) {
	rows := rowEnd - rowStart + 1
	checksumBytes, checksumErr := hex.DecodeString(expectedChecksum)
	if ctx == nil || sheets == nil || spreadsheetToken == "" || sheetID == "" || rowStart < 2 ||
		rowEnd < rowStart || rowEnd > maxMallWeatherFeishuSheetRow || rows > int64(maxMallWeatherFeishuBatchRows) ||
		columns < 1 || columns > maxMallWeatherFeishuColumns || checksumErr != nil || len(checksumBytes) != 32 {
		return false, errors.New("mall weather feishu append recovery: invalid checkpoint")
	}
	values, err := sheets.ReadRange(ctx, spreadsheetToken, feishu.SheetRange{
		SheetID: sheetID, StartRow: rowStart, EndRow: rowEnd, StartColumn: 1, EndColumn: int64(columns),
	})
	if err != nil {
		return false, err
	}
	if values == nil {
		return false, errors.New("mall weather feishu append recovery: range response is missing")
	}
	actualChecksum, err := checksumMallWeatherFeishuRows(values.Rows, int(rows), columns)
	if err != nil {
		return false, err
	}
	return actualChecksum == expectedChecksum, nil
}
