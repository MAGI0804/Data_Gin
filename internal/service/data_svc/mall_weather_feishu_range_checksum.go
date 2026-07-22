package data_svc

import (
	"context"
	"encoding/hex"
	"errors"

	"gin-biz-web-api/connector/feishu"
)

func verifyMallWeatherFeishuRangeChecksum(
	ctx context.Context,
	sheets mallWeatherFeishuRangeReader,
	spreadsheetToken string,
	sheetID string,
	rowStart int64,
	rowEnd int64,
	columns int,
	expectedChecksum string,
) (bool, error) {
	checksumBytes, checksumErr := hex.DecodeString(expectedChecksum)
	if ctx == nil || sheets == nil || spreadsheetToken == "" || sheetID == "" || rowStart < 1 ||
		rowEnd < rowStart || rowEnd > maxMallWeatherFeishuSheetRow ||
		columns < 1 || columns > maxMallWeatherFeishuColumns || checksumErr != nil || len(checksumBytes) != 32 {
		return false, errors.New("mall weather feishu range checksum: invalid input")
	}
	rows := rowEnd - rowStart + 1
	if rows > int64(maxMallWeatherFeishuBatchRows) {
		return false, errors.New("mall weather feishu range checksum: invalid input")
	}
	values, err := sheets.ReadRange(ctx, spreadsheetToken, feishu.SheetRange{
		SheetID: sheetID, StartRow: rowStart, EndRow: rowEnd, StartColumn: 1, EndColumn: int64(columns),
	})
	if err != nil {
		return false, err
	}
	if values == nil {
		return false, errors.New("mall weather feishu range checksum: response is missing")
	}
	actualChecksum, err := checksumMallWeatherFeishuRows(values.Rows, int(rows), columns)
	if err != nil {
		return false, err
	}
	return actualChecksum == expectedChecksum, nil
}
