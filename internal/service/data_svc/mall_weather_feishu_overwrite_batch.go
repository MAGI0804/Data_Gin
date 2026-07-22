package data_svc

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"gin-biz-web-api/connector/feishu"
	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/model"

	"github.com/google/uuid"
)

var ErrMallWeatherFeishuOverwriteStateUnknown = errors.New(
	"mall weather feishu overwrite: remote state is unknown",
)

type mallWeatherFeishuOverwriteSheets interface {
	BatchUpdateValues(context.Context, string, []feishu.SheetWriteRange) (*feishu.SheetWriteResult, error)
	ReadRange(context.Context, string, feishu.SheetRange) (*feishu.SheetValues, error)
}

type mallWeatherFeishuOverwriteBatchRequest struct {
	RunID       uint
	TraceID     string
	Destination *MallWeatherFeishuResolvedDestination
	DatasetKind string
	BatchNo     int
	Attempt     int
	RowStart    int64
	Batch       mallWeatherFeishuRenderedBatch
}

type mallWeatherFeishuOverwriteClearBatchRequest struct {
	RunID       uint
	TraceID     string
	Destination *MallWeatherFeishuResolvedDestination
	DatasetKind string
	BatchNo     int
	Attempt     int
	RowStart    int64
	RowEnd      int64
	Columns     int
}

type mallWeatherFeishuOverwriteBatchResult struct {
	DeliveryLogID uint
	BatchNo       int
	RowStart      int64
	RowEnd        int64
	RecordCount   int
	CellCount     int
	Revision      int64
}

type mallWeatherFeishuOverwriteBatchExecutor struct {
	sheets mallWeatherFeishuOverwriteSheets
	logs   mallWeatherFeishuBatchLogStore
	now    func() time.Time
}

func newMallWeatherFeishuOverwriteBatchExecutor(
	sheets mallWeatherFeishuOverwriteSheets,
	logs mallWeatherFeishuBatchLogStore,
	now func() time.Time,
) (*mallWeatherFeishuOverwriteBatchExecutor, error) {
	if sheets == nil || logs == nil || now == nil {
		return nil, errors.New("mall weather feishu overwrite: invalid executor configuration")
	}
	return &mallWeatherFeishuOverwriteBatchExecutor{sheets: sheets, logs: logs, now: now}, nil
}

func (executor *mallWeatherFeishuOverwriteBatchExecutor) Execute(
	ctx context.Context,
	request mallWeatherFeishuOverwriteBatchRequest,
) (mallWeatherFeishuOverwriteBatchResult, error) {
	rowEnd, recordCount, cellCount, err := validateMallWeatherFeishuOverwriteBatchRequest(ctx, request)
	if executor == nil || executor.sheets == nil || executor.logs == nil || executor.now == nil || err != nil {
		return mallWeatherFeishuOverwriteBatchResult{}, errors.New("mall weather feishu overwrite: invalid batch request")
	}
	return executor.executeFixedRange(ctx, request, rowEnd, recordCount, cellCount)
}

func (executor *mallWeatherFeishuOverwriteBatchExecutor) Clear(
	ctx context.Context,
	request mallWeatherFeishuOverwriteClearBatchRequest,
) (mallWeatherFeishuOverwriteBatchResult, error) {
	rows, cells, err := validateMallWeatherFeishuOverwriteClearBatchRequest(ctx, request)
	if executor == nil || executor.sheets == nil || executor.logs == nil || executor.now == nil || err != nil {
		return mallWeatherFeishuOverwriteBatchResult{}, errors.New("mall weather feishu overwrite: invalid clear batch request")
	}
	blankRows := make([][]feishu.SheetCell, rows)
	for rowIndex := range blankRows {
		blankRows[rowIndex] = make([]feishu.SheetCell, request.Columns)
		for columnIndex := range blankRows[rowIndex] {
			blankRows[rowIndex][columnIndex].Type = feishu.SheetCellBlank
		}
	}
	checksum, err := checksumMallWeatherFeishuRows(blankRows, rows, request.Columns)
	if err != nil {
		return mallWeatherFeishuOverwriteBatchResult{}, err
	}
	return executor.executeFixedRange(ctx, mallWeatherFeishuOverwriteBatchRequest{
		RunID: request.RunID, TraceID: request.TraceID, Destination: request.Destination,
		DatasetKind: request.DatasetKind, BatchNo: request.BatchNo, Attempt: request.Attempt,
		RowStart: request.RowStart,
		Batch: mallWeatherFeishuRenderedBatch{
			Rows: blankRows, Checksum: checksum, FirstCursor: 1, LastCursor: 1,
		},
	}, request.RowEnd, rows, cells)
}

func (executor *mallWeatherFeishuOverwriteBatchExecutor) executeFixedRange(
	ctx context.Context,
	request mallWeatherFeishuOverwriteBatchRequest,
	rowEnd int64,
	recordCount int,
	cellCount int,
) (mallWeatherFeishuOverwriteBatchResult, error) {
	var result mallWeatherFeishuOverwriteBatchResult
	startedAt := executor.now().UTC()
	if startedAt.IsZero() {
		return result, errors.New("mall weather feishu overwrite: invalid start time")
	}
	log := &model.DeliveryLog{
		TraceID: request.TraceID, RunID: request.RunID, DestinationID: request.Destination.DestinationID,
		SourceCode: "mall_weather", DestinationCode: request.Destination.Code,
		DestinationName: request.Destination.Code,
		BusinessKey:     request.DatasetKind + ":batch:" + strconv.Itoa(request.BatchNo),
		DatasetKind:     request.DatasetKind, BatchNo: request.BatchNo, RowStart: request.RowStart, RowEnd: rowEnd,
		RecordCount: recordCount, CellCount: cellCount, RequestChecksum: request.Batch.Checksum,
		Status: "running", Attempt: request.Attempt, RetryCount: request.Attempt - 1,
		StartedAt: &model.TimeNormal{Time: startedAt},
	}
	logID, err := executor.logs.Create(ctx, log)
	if err != nil || logID == 0 {
		return result, fmt.Errorf(
			"mall weather feishu overwrite: create batch checkpoint: %w",
			nonNilMallWeatherFeishuError(err),
		)
	}
	result = mallWeatherFeishuOverwriteBatchResult{
		DeliveryLogID: logID, BatchNo: request.BatchNo, RowStart: request.RowStart, RowEnd: rowEnd,
		RecordCount: recordCount, CellCount: cellCount,
	}
	write := feishu.SheetWriteRange{
		Range: feishu.SheetRange{
			SheetID:  request.Destination.SheetIDs[request.DatasetKind],
			StartRow: request.RowStart, EndRow: rowEnd, StartColumn: 1,
			EndColumn: int64(len(request.Batch.Rows[0])),
		},
		Rows: request.Batch.Rows,
	}
	acknowledgement, writeErr := executor.sheets.BatchUpdateValues(
		ctx,
		request.Destination.SpreadsheetToken,
		[]feishu.SheetWriteRange{write},
	)
	if acknowledgement != nil {
		result.Revision = acknowledgement.Revision
	}
	shouldVerify := writeErr == nil || mallWeatherFeishuOverwriteOutcomeUnknown(writeErr)
	matched := false
	var verifyErr error
	if shouldVerify {
		matched, verifyErr = verifyMallWeatherFeishuRangeChecksum(
			ctx,
			executor.sheets,
			request.Destination.SpreadsheetToken,
			request.Destination.SheetIDs[request.DatasetKind],
			request.RowStart,
			rowEnd,
			len(request.Batch.Rows[0]),
			request.Batch.Checksum,
		)
	}
	finish := mallWeatherFeishuOverwriteFinish(writeErr, shouldVerify, matched, verifyErr, executor.now().UTC())
	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), mallWeatherFeishuCheckpointTimeout)
	finishErr := executor.logs.FinishWeatherBatch(finishCtx, logID, finish)
	cancel()
	if finishErr != nil {
		return result, fmt.Errorf("%w: persist batch outcome", ErrMallWeatherFeishuOverwriteStateUnknown)
	}
	switch finish.Status {
	case "success":
		return result, nil
	case "failed":
		return result, writeErr
	default:
		return result, ErrMallWeatherFeishuOverwriteStateUnknown
	}
}

func validateMallWeatherFeishuOverwriteBatchRequest(
	ctx context.Context,
	request mallWeatherFeishuOverwriteBatchRequest,
) (int64, int, int, error) {
	_, datasetAllowed := mallWeatherFeishuDatasetKinds[request.DatasetKind]
	if ctx == nil || request.RunID == 0 || uuid.Validate(request.TraceID) != nil || request.Destination == nil ||
		request.Destination.DestinationID == 0 || request.Destination.Code == "" ||
		request.Destination.Config.WriteMode != "overwrite_range" || request.Destination.SpreadsheetToken == "" ||
		!datasetAllowed || request.Destination.SheetIDs[request.DatasetKind] == "" ||
		request.Destination.Config.SheetIDEnvMapping[request.DatasetKind] == "" || request.BatchNo < 1 ||
		request.Attempt < 1 || request.RowStart < 2 || request.RowStart > maxMallWeatherFeishuSheetRow ||
		len(request.Batch.Rows) == 0 || len(request.Batch.Rows) > maxMallWeatherFeishuBatchRows ||
		len(request.Batch.Checksum) != 64 {
		return 0, 0, 0, errors.New("invalid batch")
	}
	checksum, err := hex.DecodeString(request.Batch.Checksum)
	if err != nil || len(checksum) != 32 || request.Batch.FirstCursor == 0 ||
		request.Batch.LastCursor < request.Batch.FirstCursor {
		return 0, 0, 0, errors.New("invalid batch checkpoint")
	}
	columns := len(request.Batch.Rows[0])
	if columns == 0 || columns > maxMallWeatherFeishuColumns || len(request.Batch.Rows) > math.MaxInt/columns {
		return 0, 0, 0, errors.New("invalid batch dimensions")
	}
	for _, row := range request.Batch.Rows {
		if len(row) != columns || mallWeatherFeishuFirstCellIsBlank(row) {
			return 0, 0, 0, errors.New("invalid batch dimensions")
		}
	}
	rowCount := int64(len(request.Batch.Rows))
	if request.RowStart > math.MaxInt64-rowCount+1 {
		return 0, 0, 0, errors.New("invalid batch row range")
	}
	rowEnd := request.RowStart + rowCount - 1
	if rowEnd > maxMallWeatherFeishuSheetRow {
		return 0, 0, 0, errors.New("invalid batch row range")
	}
	return rowEnd, len(request.Batch.Rows), len(request.Batch.Rows) * columns, nil
}

func validateMallWeatherFeishuOverwriteClearBatchRequest(
	ctx context.Context,
	request mallWeatherFeishuOverwriteClearBatchRequest,
) (int, int, error) {
	_, datasetAllowed := mallWeatherFeishuDatasetKinds[request.DatasetKind]
	if ctx == nil || request.RunID == 0 || uuid.Validate(request.TraceID) != nil || request.Destination == nil ||
		request.Destination.DestinationID == 0 || request.Destination.Code == "" ||
		request.Destination.Config.WriteMode != "overwrite_range" || request.Destination.SpreadsheetToken == "" ||
		!datasetAllowed || request.Destination.SheetIDs[request.DatasetKind] == "" ||
		request.Destination.Config.SheetIDEnvMapping[request.DatasetKind] == "" || request.BatchNo < 1 ||
		request.Attempt < 1 || request.RowStart < 2 || request.RowEnd < request.RowStart ||
		request.RowEnd > maxMallWeatherFeishuSheetRow || request.Columns < 1 ||
		request.Columns > maxMallWeatherFeishuColumns {
		return 0, 0, errors.New("invalid clear batch")
	}
	rowCount := request.RowEnd - request.RowStart + 1
	if rowCount > int64(maxMallWeatherFeishuBatchRows) || rowCount > int64(math.MaxInt) ||
		rowCount > int64(math.MaxInt/request.Columns) {
		return 0, 0, errors.New("invalid clear batch dimensions")
	}
	rows := int(rowCount)
	return rows, rows * request.Columns, nil
}

func mallWeatherFeishuOverwriteOutcomeUnknown(err error) bool {
	var sheetsErr *feishu.SheetsError
	return errors.As(err, &sheetsErr) && mallWeatherFeishuAppendOutcomeUnknown(sheetsErr)
}

func mallWeatherFeishuOverwriteFinish(
	writeErr error,
	verified bool,
	matched bool,
	verifyErr error,
	finishedAt time.Time,
) data_dao.DeliveryLogBatchFinish {
	finish := data_dao.DeliveryLogBatchFinish{FinishedAt: finishedAt}
	var sheetsErr *feishu.SheetsError
	if errors.As(writeErr, &sheetsErr) {
		finish.HTTPStatus = sheetsErr.HTTPCode
		finish.FeishuCode = sheetsErr.Code
	}
	if verified && verifyErr == nil && matched {
		finish.Status = "success"
		finish.Success = true
		finish.ResponseSummary = "fixed range checksum matched"
		if writeErr == nil {
			finish.HTTPStatus = http.StatusOK
		}
		return finish
	}
	if writeErr != nil && !mallWeatherFeishuOverwriteOutcomeUnknown(writeErr) {
		finish.Status = "failed"
		finish.SafeError = "feishu overwrite failed"
		finish.ResponseSummary = "feishu overwrite rejected"
		if sheetsErr != nil {
			finish.ResponseSummary = fmt.Sprintf("class=%s retryable=%t", sheetsErr.Class, sheetsErr.Retryable)
		}
		return finish
	}
	finish.Status = "unknown"
	finish.SafeError = "feishu overwrite outcome is unknown"
	switch {
	case verifyErr != nil:
		finish.ResponseSummary = "fixed range checksum read failed"
	case verified && !matched:
		finish.ResponseSummary = "fixed range checksum mismatched"
	default:
		finish.ResponseSummary = "feishu overwrite acknowledgement is uncertain"
	}
	return finish
}
