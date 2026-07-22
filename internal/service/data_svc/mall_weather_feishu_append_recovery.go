package data_svc

import (
	"context"
	"errors"
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
	if rowStart < 2 {
		return false, errors.New("mall weather feishu append recovery: invalid checkpoint")
	}
	return verifyMallWeatherFeishuRangeChecksum(
		ctx,
		sheets,
		spreadsheetToken,
		sheetID,
		rowStart,
		rowEnd,
		columns,
		expectedChecksum,
	)
}
