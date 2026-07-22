package data_svc

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"gin-biz-web-api/connector/feishu"
	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/providerhttp"

	"github.com/google/uuid"
)

const (
	mallWeatherFeishuCheckpointTimeout = 5 * time.Second
	maxMallWeatherFeishuSheetRow       = int64(10_000_000)
)

var ErrMallWeatherFeishuAppendStateUnknown = errors.New("mall weather feishu append: remote state is unknown")

type mallWeatherFeishuAppender interface {
	AppendValues(context.Context, string, feishu.SheetWriteRange) (*feishu.SheetWriteResult, error)
}

type mallWeatherFeishuBatchLogStore interface {
	Create(context.Context, *model.DeliveryLog) (uint, error)
	FinishWeatherBatch(context.Context, uint, data_dao.DeliveryLogBatchFinish) error
}

type mallWeatherFeishuAppendBatchRequest struct {
	RunID       uint
	TraceID     string
	Destination *MallWeatherFeishuResolvedDestination
	DatasetKind string
	BatchNo     int
	Attempt     int
	RowStart    int64
	Batch       mallWeatherFeishuRenderedBatch
}

type mallWeatherFeishuAppendBatchResult struct {
	DeliveryLogID uint
	BatchNo       int
	RowStart      int64
	RowEnd        int64
	RecordCount   int
	CellCount     int
	Revision      int64
}

type mallWeatherFeishuAppendBatchExecutor struct {
	sheets mallWeatherFeishuAppender
	logs   mallWeatherFeishuBatchLogStore
	now    func() time.Time
}

func newMallWeatherFeishuAppendBatchExecutor(
	sheets mallWeatherFeishuAppender,
	logs mallWeatherFeishuBatchLogStore,
	now func() time.Time,
) (*mallWeatherFeishuAppendBatchExecutor, error) {
	if sheets == nil || logs == nil || now == nil {
		return nil, errors.New("mall weather feishu append: invalid executor configuration")
	}
	return &mallWeatherFeishuAppendBatchExecutor{sheets: sheets, logs: logs, now: now}, nil
}

func (executor *mallWeatherFeishuAppendBatchExecutor) Execute(
	ctx context.Context,
	request mallWeatherFeishuAppendBatchRequest,
) (mallWeatherFeishuAppendBatchResult, error) {
	var result mallWeatherFeishuAppendBatchResult
	rowEnd, recordCount, cellCount, err := validateMallWeatherFeishuAppendBatchRequest(ctx, request)
	if executor == nil || executor.sheets == nil || executor.logs == nil || executor.now == nil || err != nil {
		return result, errors.New("mall weather feishu append: invalid batch request")
	}
	startedAt := executor.now().UTC()
	if startedAt.IsZero() {
		return result, errors.New("mall weather feishu append: invalid start time")
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
		return result, fmt.Errorf("mall weather feishu append: create batch checkpoint: %w", nonNilMallWeatherFeishuError(err))
	}
	result = mallWeatherFeishuAppendBatchResult{
		DeliveryLogID: logID, BatchNo: request.BatchNo, RowStart: request.RowStart, RowEnd: rowEnd,
		RecordCount: recordCount, CellCount: cellCount,
	}
	write := feishu.SheetWriteRange{
		Range: feishu.SheetRange{
			SheetID: request.Destination.SheetIDs[request.DatasetKind], StartRow: request.RowStart, EndRow: rowEnd,
			StartColumn: 1, EndColumn: int64(len(request.Batch.Rows[0])),
		},
		Rows: request.Batch.Rows,
	}
	acknowledgement, appendErr := executor.sheets.AppendValues(ctx, request.Destination.SpreadsheetToken, write)
	if appendErr == nil && !validMallWeatherFeishuAppendAcknowledgement(
		acknowledgement,
		recordCount,
		len(request.Batch.Rows[0]),
		cellCount,
	) {
		acknowledgement = nil
	}
	finish := mallWeatherFeishuAppendFinish(acknowledgement, appendErr, executor.now().UTC())
	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), mallWeatherFeishuCheckpointTimeout)
	finishErr := executor.logs.FinishWeatherBatch(finishCtx, logID, finish)
	cancel()
	if finishErr != nil {
		return result, fmt.Errorf("%w: persist batch outcome", ErrMallWeatherFeishuAppendStateUnknown)
	}
	if appendErr != nil {
		if finish.Status == "unknown" {
			return result, fmt.Errorf("%w: %v", ErrMallWeatherFeishuAppendStateUnknown, appendErr)
		}
		return result, appendErr
	}
	if acknowledgement == nil {
		return result, ErrMallWeatherFeishuAppendStateUnknown
	}
	result.Revision = acknowledgement.Revision
	result.RowStart = acknowledgement.UpdatedRowStart
	result.RowEnd = acknowledgement.UpdatedRowEnd
	return result, nil
}

func validMallWeatherFeishuAppendAcknowledgement(
	acknowledgement *feishu.SheetWriteResult,
	rows int,
	columns int,
	cells int,
) bool {
	if acknowledgement == nil || rows < 1 || columns < 1 || cells < 1 ||
		acknowledgement.UpdatedRows != int64(rows) || acknowledgement.UpdatedColumns != int64(columns) ||
		acknowledgement.UpdatedCells != int64(cells) || acknowledgement.UpdatedRowStart < 2 ||
		acknowledgement.UpdatedRowStart > maxMallWeatherFeishuSheetRow {
		return false
	}
	return acknowledgement.UpdatedRowEnd == acknowledgement.UpdatedRowStart+int64(rows)-1 &&
		acknowledgement.UpdatedRowEnd <= maxMallWeatherFeishuSheetRow
}

func validateMallWeatherFeishuAppendBatchRequest(
	ctx context.Context,
	request mallWeatherFeishuAppendBatchRequest,
) (int64, int, int, error) {
	_, datasetAllowed := mallWeatherFeishuDatasetKinds[request.DatasetKind]
	if ctx == nil || request.RunID == 0 || uuid.Validate(request.TraceID) != nil || request.Destination == nil ||
		request.Destination.DestinationID == 0 || request.Destination.Code == "" ||
		request.Destination.Config.WriteMode != "append" || request.Destination.SpreadsheetToken == "" ||
		!datasetAllowed || request.Destination.SheetIDs[request.DatasetKind] == "" ||
		request.Destination.Config.SheetIDEnvMapping[request.DatasetKind] == "" || request.BatchNo < 1 ||
		request.Attempt < 1 || request.RowStart < 2 || request.RowStart > maxMallWeatherFeishuSheetRow ||
		len(request.Batch.Rows) == 0 ||
		len(request.Batch.Rows) > maxMallWeatherFeishuBatchRows || len(request.Batch.Checksum) != 64 {
		return 0, 0, 0, errors.New("invalid batch")
	}
	checksum, err := hex.DecodeString(request.Batch.Checksum)
	if err != nil || len(checksum) != 32 || request.Batch.FirstCursor == 0 ||
		request.Batch.LastCursor < request.Batch.FirstCursor {
		return 0, 0, 0, errors.New("invalid batch checkpoint")
	}
	columns := len(request.Batch.Rows[0])
	if columns == 0 || columns > maxMallWeatherExportColumns || len(request.Batch.Rows) > math.MaxInt/columns {
		return 0, 0, 0, errors.New("invalid batch dimensions")
	}
	for _, row := range request.Batch.Rows {
		if len(row) != columns {
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

func mallWeatherFeishuAppendFinish(
	acknowledgement *feishu.SheetWriteResult,
	appendErr error,
	finishedAt time.Time,
) data_dao.DeliveryLogBatchFinish {
	finish := data_dao.DeliveryLogBatchFinish{FinishedAt: finishedAt}
	if appendErr == nil && acknowledgement != nil {
		finish.Status = "success"
		finish.Success = true
		finish.RowStart = acknowledgement.UpdatedRowStart
		finish.RowEnd = acknowledgement.UpdatedRowEnd
		finish.HTTPStatus = 200
		finish.ResponseSummary = "feishu append acknowledged"
		return finish
	}
	finish.Status = "failed"
	finish.SafeError = "feishu append failed"
	finish.ResponseSummary = "feishu append rejected"
	var sheetsErr *feishu.SheetsError
	if errors.As(appendErr, &sheetsErr) {
		finish.HTTPStatus = sheetsErr.HTTPCode
		finish.FeishuCode = sheetsErr.Code
		finish.ResponseSummary = fmt.Sprintf("class=%s retryable=%t", sheetsErr.Class, sheetsErr.Retryable)
		if mallWeatherFeishuAppendOutcomeUnknown(sheetsErr) {
			finish.Status = "unknown"
			finish.SafeError = "feishu append outcome is unknown"
		}
	} else if appendErr == nil {
		finish.Status = "unknown"
		finish.SafeError = "feishu append acknowledgement is missing"
		finish.ResponseSummary = "feishu append acknowledgement missing"
	}
	return finish
}

func mallWeatherFeishuAppendOutcomeUnknown(err *feishu.SheetsError) bool {
	if err == nil {
		return true
	}
	return err.HTTPCode >= 500 || err.Class == providerhttp.ErrorClassCanceled ||
		err.Class == providerhttp.ErrorClassTimeout || err.Class == providerhttp.ErrorClassTransport ||
		err.Class == providerhttp.ErrorClassResponse || err.Class == providerhttp.ErrorClassProvider
}

func nonNilMallWeatherFeishuError(err error) error {
	if err != nil {
		return err
	}
	return errors.New("checkpoint id is missing")
}
