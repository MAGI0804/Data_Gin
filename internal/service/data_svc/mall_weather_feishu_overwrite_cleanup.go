package data_svc

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gin-biz-web-api/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrMallWeatherFeishuOverwriteCleanupCheckpointConflict = errors.New(
	"mall weather feishu overwrite cleanup: checkpoint conflicts with stale range",
)

type mallWeatherFeishuOverwriteClearBatchRunner interface {
	Clear(context.Context, mallWeatherFeishuOverwriteClearBatchRequest) (mallWeatherFeishuOverwriteBatchResult, error)
}

type mallWeatherFeishuOverwriteCleanupRequest struct {
	RunID        uint
	TraceID      string
	Destination  *MallWeatherFeishuResolvedDestination
	DatasetKind  string
	StartBatchNo int
	RowStart     int64
	RowEnd       int64
	Columns      int
}

type mallWeatherFeishuOverwriteCleanupResult struct {
	DatasetKind string
	BatchCount  int
	ClearedRows int64
	CellCount   int64
	LastRow     int64
}

type mallWeatherFeishuOverwriteCleanupRunner struct {
	sheets      mallWeatherFeishuRangeReader
	checkpoints mallWeatherFeishuBatchCheckpointStore
	batches     mallWeatherFeishuOverwriteClearBatchRunner
	now         func() time.Time
}

func newMallWeatherFeishuOverwriteCleanupRunner(
	sheets mallWeatherFeishuRangeReader,
	checkpoints mallWeatherFeishuBatchCheckpointStore,
	batches mallWeatherFeishuOverwriteClearBatchRunner,
	now func() time.Time,
) (*mallWeatherFeishuOverwriteCleanupRunner, error) {
	if sheets == nil || checkpoints == nil || batches == nil || now == nil {
		return nil, errors.New("mall weather feishu overwrite cleanup: invalid runner configuration")
	}
	return &mallWeatherFeishuOverwriteCleanupRunner{
		sheets: sheets, checkpoints: checkpoints, batches: batches, now: now,
	}, nil
}

// Run must execute under the same destination lock as the preceding overwrite
// dataset run. Callers only invoke it after every replacement data batch has
// been verified, so the old dashboard is never cleared first.
func (runner *mallWeatherFeishuOverwriteCleanupRunner) Run(
	ctx context.Context,
	request mallWeatherFeishuOverwriteCleanupRequest,
) (mallWeatherFeishuOverwriteCleanupResult, error) {
	result := mallWeatherFeishuOverwriteCleanupResult{DatasetKind: request.DatasetKind}
	if err := validateMallWeatherFeishuOverwriteCleanupRequest(ctx, runner, request); err != nil {
		return result, err
	}
	rowStart := request.RowStart
	for batchNo := request.StartBatchNo; rowStart <= request.RowEnd; batchNo++ {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		rowEnd := min(rowStart+int64(request.Destination.Config.BatchRows)-1, request.RowEnd)
		rowCount := int(rowEnd - rowStart + 1)
		checksum, err := checksumMallWeatherFeishuRows(nil, rowCount, request.Columns)
		if err != nil {
			return result, fmt.Errorf("mall weather feishu overwrite cleanup: checksum blank range: %w", err)
		}
		checkpoint, err := runner.checkpoints.FindLatestWeatherBatch(
			ctx,
			request.RunID,
			request.Destination.DestinationID,
			request.DatasetKind,
			batchNo,
		)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			checkpoint = nil
		} else if err != nil {
			return result, fmt.Errorf("mall weather feishu overwrite cleanup: load batch checkpoint: %w", err)
		}
		attempt, shouldClear, err := runner.resolveMallWeatherFeishuOverwriteCleanupCheckpoint(
			ctx,
			request,
			batchNo,
			rowStart,
			rowEnd,
			rowCount,
			checksum,
			checkpoint,
		)
		if err != nil {
			return result, err
		}
		if shouldClear {
			batchResult, clearErr := runner.batches.Clear(ctx, mallWeatherFeishuOverwriteClearBatchRequest{
				RunID: request.RunID, TraceID: request.TraceID, Destination: request.Destination,
				DatasetKind: request.DatasetKind, BatchNo: batchNo, Attempt: attempt,
				RowStart: rowStart, RowEnd: rowEnd, Columns: request.Columns,
			})
			if clearErr != nil {
				return result, fmt.Errorf("mall weather feishu overwrite cleanup: clear batch: %w", clearErr)
			}
			if !validMallWeatherFeishuOverwriteCleanupBatchResult(
				batchResult,
				batchNo,
				rowStart,
				rowEnd,
				rowCount,
				request.Columns,
			) {
				return result, errors.New("mall weather feishu overwrite cleanup: invalid batch result")
			}
		}
		result.BatchCount++
		result.ClearedRows += int64(rowCount)
		result.CellCount += int64(rowCount * request.Columns)
		result.LastRow = rowEnd
		rowStart = rowEnd + 1
	}
	return result, nil
}

func validateMallWeatherFeishuOverwriteCleanupRequest(
	ctx context.Context,
	runner *mallWeatherFeishuOverwriteCleanupRunner,
	request mallWeatherFeishuOverwriteCleanupRequest,
) error {
	if ctx == nil || runner == nil || runner.sheets == nil || runner.checkpoints == nil || runner.batches == nil ||
		runner.now == nil || request.RunID == 0 || uuid.Validate(request.TraceID) != nil || request.Destination == nil ||
		request.Destination.DestinationID == 0 || request.Destination.Code == "" ||
		request.Destination.Config.WriteMode != "overwrite_range" ||
		request.Destination.Config.BatchRows < 1 ||
		request.Destination.Config.BatchRows > maxMallWeatherFeishuBatchRows ||
		request.Destination.SpreadsheetToken == "" || request.StartBatchNo < 1 || request.RowStart < 2 ||
		request.RowEnd < request.RowStart || request.RowEnd > maxMallWeatherFeishuSheetRow ||
		request.Columns < 1 || request.Columns > maxMallWeatherFeishuColumns {
		return errors.New("mall weather feishu overwrite cleanup: invalid request")
	}
	if _, allowed := mallWeatherFeishuDatasetKinds[request.DatasetKind]; !allowed ||
		request.Destination.SheetIDs[request.DatasetKind] == "" ||
		request.Destination.Config.SheetIDEnvMapping[request.DatasetKind] == "" {
		return errors.New("mall weather feishu overwrite cleanup: invalid request")
	}
	return nil
}

func (runner *mallWeatherFeishuOverwriteCleanupRunner) resolveMallWeatherFeishuOverwriteCleanupCheckpoint(
	ctx context.Context,
	request mallWeatherFeishuOverwriteCleanupRequest,
	batchNo int,
	rowStart int64,
	rowEnd int64,
	rowCount int,
	checksum string,
	checkpoint *model.DeliveryLog,
) (attempt int, shouldClear bool, err error) {
	if checkpoint == nil {
		return 1, true, nil
	}
	if err := validateMallWeatherFeishuOverwriteCleanupCheckpoint(
		request,
		batchNo,
		rowStart,
		rowEnd,
		rowCount,
		checksum,
		checkpoint,
	); err != nil {
		return 0, false, err
	}
	switch checkpoint.Status {
	case "failed":
		return nextMallWeatherFeishuOverwriteAttempt(checkpoint.Attempt)
	case "success", "running", "unknown":
		matched, verifyErr := verifyMallWeatherFeishuRangeChecksum(
			ctx,
			runner.sheets,
			request.Destination.SpreadsheetToken,
			request.Destination.SheetIDs[request.DatasetKind],
			rowStart,
			rowEnd,
			request.Columns,
			checksum,
		)
		if verifyErr != nil {
			return 0, false, fmt.Errorf("mall weather feishu overwrite cleanup: verify checkpoint: %w", verifyErr)
		}
		if !matched {
			return nextMallWeatherFeishuOverwriteAttempt(checkpoint.Attempt)
		}
		if checkpoint.Status == "success" {
			return checkpoint.Attempt, false, nil
		}
		finishedAt := runner.now().UTC()
		if finishedAt.IsZero() {
			return 0, false, errors.New("mall weather feishu overwrite cleanup: invalid reconciliation time")
		}
		reconcileCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), mallWeatherFeishuCheckpointTimeout)
		reconcileErr := runner.checkpoints.ReconcileWeatherBatchSuccess(
			reconcileCtx,
			checkpoint.ID,
			checksum,
			rowStart,
			rowEnd,
			finishedAt,
		)
		cancel()
		if reconcileErr != nil {
			return 0, false, fmt.Errorf(
				"%w: reconcile verified cleanup checkpoint",
				ErrMallWeatherFeishuOverwriteStateUnknown,
			)
		}
		return checkpoint.Attempt, false, nil
	default:
		return 0, false, ErrMallWeatherFeishuOverwriteCleanupCheckpointConflict
	}
}

func validateMallWeatherFeishuOverwriteCleanupCheckpoint(
	request mallWeatherFeishuOverwriteCleanupRequest,
	batchNo int,
	rowStart int64,
	rowEnd int64,
	rowCount int,
	checksum string,
	checkpoint *model.DeliveryLog,
) error {
	if checkpoint == nil || checkpoint.ID == 0 || checkpoint.RunID != request.RunID ||
		checkpoint.DestinationID != request.Destination.DestinationID ||
		checkpoint.DatasetKind != request.DatasetKind || checkpoint.BatchNo != batchNo ||
		checkpoint.RequestChecksum != checksum || checkpoint.Attempt < 1 || checkpoint.RowStart != rowStart ||
		checkpoint.RowEnd != rowEnd || checkpoint.RecordCount != rowCount ||
		checkpoint.CellCount != rowCount*request.Columns {
		return ErrMallWeatherFeishuOverwriteCleanupCheckpointConflict
	}
	return nil
}

func validMallWeatherFeishuOverwriteCleanupBatchResult(
	result mallWeatherFeishuOverwriteBatchResult,
	batchNo int,
	rowStart int64,
	rowEnd int64,
	rowCount int,
	columns int,
) bool {
	return result.BatchNo == batchNo && result.RowStart == rowStart && result.RowEnd == rowEnd &&
		result.RecordCount == rowCount && result.CellCount == rowCount*columns
}
