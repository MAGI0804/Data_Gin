package data_svc

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrMallWeatherFeishuOverwriteCheckpointConflict = errors.New(
	"mall weather feishu overwrite dataset: checkpoint conflicts with rendered batch",
)

type mallWeatherFeishuOverwriteBatchRunner interface {
	Execute(context.Context, mallWeatherFeishuOverwriteBatchRequest) (mallWeatherFeishuOverwriteBatchResult, error)
}

type mallWeatherFeishuOverwriteDatasetRequest struct {
	RunID       uint
	TraceID     string
	Destination *MallWeatherFeishuResolvedDestination
	Profile     MallWeatherExportProfileDTO
	Dataset     requestbody.MallWeatherExportDataset
	Filter      data_dao.MallWeatherExportEstimateFilter
	SnapshotAt  time.Time
	GridRows    int64
}

type mallWeatherFeishuOverwriteDatasetResult struct {
	DatasetKind     string
	BatchCount      int
	RecordCount     int64
	CellCount       int64
	LastCursor      uint
	DataLastRow     int64
	PreviousLastRow int64
	StaleRowStart   int64
	StaleRowEnd     int64
}

type mallWeatherFeishuOverwriteDatasetRunner struct {
	pager       mallWeatherFeishuDatasetPager
	sheets      mallWeatherFeishuRangeReader
	checkpoints mallWeatherFeishuBatchCheckpointStore
	batches     mallWeatherFeishuOverwriteBatchRunner
	now         func() time.Time
}

func newMallWeatherFeishuOverwriteDatasetRunner(
	pager mallWeatherFeishuDatasetPager,
	sheets mallWeatherFeishuRangeReader,
	checkpoints mallWeatherFeishuBatchCheckpointStore,
	batches mallWeatherFeishuOverwriteBatchRunner,
	now func() time.Time,
) (*mallWeatherFeishuOverwriteDatasetRunner, error) {
	if pager == nil || sheets == nil || checkpoints == nil || batches == nil || now == nil {
		return nil, errors.New("mall weather feishu overwrite dataset: invalid runner configuration")
	}
	return &mallWeatherFeishuOverwriteDatasetRunner{
		pager: pager, sheets: sheets, checkpoints: checkpoints, batches: batches, now: now,
	}, nil
}

// Run must execute while the caller owns the destination execution lock. It
// writes new data before reporting any stale tail range, so cleanup never
// clears the old dashboard before replacement rows have been verified.
func (runner *mallWeatherFeishuOverwriteDatasetRunner) Run(
	ctx context.Context,
	request mallWeatherFeishuOverwriteDatasetRequest,
) (mallWeatherFeishuOverwriteDatasetResult, error) {
	result := mallWeatherFeishuOverwriteDatasetResult{DatasetKind: request.Dataset.Kind, DataLastRow: 1}
	columns, fields, asOfUTC, err := validateMallWeatherFeishuOverwriteDatasetRequest(ctx, runner, request)
	if err != nil {
		return result, err
	}
	initialAppendRow, err := findMallWeatherFeishuAppendRow(
		ctx,
		runner.sheets,
		request.Destination.SpreadsheetToken,
		request.Destination.SheetIDs[request.Dataset.Kind],
		request.GridRows,
	)
	if err != nil {
		return result, fmt.Errorf("mall weather feishu overwrite dataset: locate previous data tail: %w", err)
	}
	result.PreviousLastRow = initialAppendRow - 1
	rowStart := int64(2)
	var afterID uint
	for batchNo := 1; ; batchNo++ {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		page, err := runner.pager.Page(ctx, data_dao.MallWeatherExportDataPageRequest{
			Kind: request.Dataset.Kind, Fields: fields, Filter: request.Filter,
			Latest: mallWeatherFeishuDatasetLatest(request.Dataset), AsOfUTC: asOfUTC,
			AfterID: afterID, Limit: request.Destination.Config.BatchRows, SnapshotAt: request.SnapshotAt.UTC(),
		})
		if err != nil {
			return result, fmt.Errorf("mall weather feishu overwrite dataset: read dataset page: %w", err)
		}
		nextAfterID, err := validateMallWeatherExportPage(page, afterID)
		if err != nil {
			return result, fmt.Errorf("mall weather feishu overwrite dataset: validate dataset page: %w", err)
		}
		if len(page.Rows) == 0 {
			setMallWeatherFeishuOverwriteStaleRange(&result, rowStart)
			return result, nil
		}
		batch, err := renderMallWeatherFeishuBatch(request.Profile, request.Dataset, page.Rows)
		if err != nil {
			return result, fmt.Errorf("mall weather feishu overwrite dataset: render batch: %w", err)
		}
		rowEnd := rowStart + int64(len(batch.Rows)) - 1
		if rowEnd > maxMallWeatherFeishuSheetRow {
			return result, errors.New("mall weather feishu overwrite dataset: sheet row limit reached")
		}
		checkpoint, err := runner.checkpoints.FindLatestWeatherBatch(
			ctx,
			request.RunID,
			request.Destination.DestinationID,
			request.Dataset.Kind,
			batchNo,
		)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			checkpoint = nil
		} else if err != nil {
			return result, fmt.Errorf("mall weather feishu overwrite dataset: load batch checkpoint: %w", err)
		}
		attempt, shouldWrite, err := runner.resolveMallWeatherFeishuOverwriteCheckpoint(
			ctx,
			request,
			batchNo,
			rowStart,
			rowEnd,
			columns,
			batch,
			checkpoint,
			initialAppendRow,
		)
		if err != nil {
			return result, err
		}
		if shouldWrite {
			batchResult, executeErr := runner.batches.Execute(ctx, mallWeatherFeishuOverwriteBatchRequest{
				RunID: request.RunID, TraceID: request.TraceID, Destination: request.Destination,
				DatasetKind: request.Dataset.Kind, BatchNo: batchNo, Attempt: attempt,
				RowStart: rowStart, Batch: batch,
			})
			if executeErr != nil {
				return result, fmt.Errorf("mall weather feishu overwrite dataset: execute batch: %w", executeErr)
			}
			if !validMallWeatherFeishuOverwriteDatasetBatchResult(
				batchResult,
				batchNo,
				rowStart,
				rowEnd,
				batch,
				columns,
			) {
				return result, errors.New("mall weather feishu overwrite dataset: invalid batch result")
			}
		}
		result.BatchCount++
		result.RecordCount += int64(len(batch.Rows))
		result.CellCount += int64(len(batch.Rows) * columns)
		result.LastCursor = nextAfterID
		result.DataLastRow = rowEnd
		afterID = nextAfterID
		rowStart = rowEnd + 1
		if !page.HasMore {
			setMallWeatherFeishuOverwriteStaleRange(&result, rowStart)
			return result, nil
		}
	}
}

func validateMallWeatherFeishuOverwriteDatasetRequest(
	ctx context.Context,
	runner *mallWeatherFeishuOverwriteDatasetRunner,
	request mallWeatherFeishuOverwriteDatasetRequest,
) (int, []string, *time.Time, error) {
	if ctx == nil || runner == nil || runner.pager == nil || runner.sheets == nil || runner.checkpoints == nil ||
		runner.batches == nil || runner.now == nil || request.Destination == nil {
		return 0, nil, nil, errors.New("mall weather feishu overwrite dataset: invalid request")
	}
	_, datasetAllowed := mallWeatherFeishuDatasetKinds[request.Dataset.Kind]
	destinationValid := request.Destination.DestinationID != 0 && request.Destination.Code != "" &&
		request.Destination.Config.WriteMode == "overwrite_range" &&
		request.Destination.Config.BatchRows >= 1 &&
		request.Destination.Config.BatchRows <= maxMallWeatherFeishuBatchRows &&
		request.Destination.SpreadsheetToken != "" && request.Destination.SheetIDs[request.Dataset.Kind] != "" &&
		request.Destination.Config.SheetIDEnvMapping[request.Dataset.Kind] != ""
	profileValid := request.Profile.ID != 0 && request.Profile.Version != 0 && request.Profile.Enabled &&
		request.Profile.Code != "" && request.Profile.Code == request.Destination.Config.ProfileCode
	if request.RunID == 0 || uuid.Validate(request.TraceID) != nil || !destinationValid || !profileValid ||
		!datasetAllowed || request.Dataset.SplitBy != "" ||
		!mallWeatherFeishuProfileHasDataset(request.Profile, request.Dataset) || request.SnapshotAt.IsZero() ||
		request.GridRows < 1 || request.GridRows > maxMallWeatherFeishuSheetRow {
		return 0, nil, nil, errors.New("mall weather feishu overwrite dataset: invalid request")
	}
	columns, err := mallWeatherExportRenderColumns(request.Dataset)
	if err != nil || len(columns) == 0 || len(columns) > maxMallWeatherFeishuColumns {
		return 0, nil, nil, errors.New("mall weather feishu overwrite dataset: invalid columns")
	}
	fields := make([]string, len(columns))
	for index, column := range columns {
		fields[index] = column.Field
	}
	var asOfUTC *time.Time
	if request.Dataset.AsOf != "" {
		parsed, parseErr := time.Parse(time.RFC3339Nano, request.Dataset.AsOf)
		if parseErr != nil {
			return 0, nil, nil, errors.New("mall weather feishu overwrite dataset: invalid dataset as-of")
		}
		parsed = parsed.UTC()
		asOfUTC = &parsed
	}
	return len(columns), fields, asOfUTC, nil
}

func (runner *mallWeatherFeishuOverwriteDatasetRunner) resolveMallWeatherFeishuOverwriteCheckpoint(
	ctx context.Context,
	request mallWeatherFeishuOverwriteDatasetRequest,
	batchNo int,
	rowStart int64,
	rowEnd int64,
	columns int,
	batch mallWeatherFeishuRenderedBatch,
	checkpoint *model.DeliveryLog,
	initialAppendRow int64,
) (attempt int, shouldWrite bool, err error) {
	if checkpoint == nil {
		return 1, true, nil
	}
	if err := validateMallWeatherFeishuOverwriteCheckpoint(
		request,
		batchNo,
		rowStart,
		rowEnd,
		columns,
		batch,
		checkpoint,
	); err != nil {
		return 0, false, err
	}
	if checkpoint.Status == "failed" {
		return nextMallWeatherFeishuOverwriteAttempt(checkpoint.Attempt)
	}
	if checkpoint.Status != "success" && checkpoint.Status != "running" && checkpoint.Status != "unknown" {
		return 0, false, ErrMallWeatherFeishuOverwriteCheckpointConflict
	}
	if checkpoint.Status == "success" && checkpoint.RowEnd < initialAppendRow {
		return checkpoint.Attempt, false, nil
	}
	matched, verifyErr := verifyMallWeatherFeishuRangeChecksum(
		ctx,
		runner.sheets,
		request.Destination.SpreadsheetToken,
		request.Destination.SheetIDs[request.Dataset.Kind],
		checkpoint.RowStart,
		checkpoint.RowEnd,
		columns,
		batch.Checksum,
	)
	if verifyErr != nil {
		return 0, false, fmt.Errorf("mall weather feishu overwrite dataset: verify checkpoint: %w", verifyErr)
	}
	if !matched {
		return nextMallWeatherFeishuOverwriteAttempt(checkpoint.Attempt)
	}
	if checkpoint.Status == "success" {
		return checkpoint.Attempt, false, nil
	}
	finishedAt := runner.now().UTC()
	if finishedAt.IsZero() {
		return 0, false, errors.New("mall weather feishu overwrite dataset: invalid reconciliation time")
	}
	reconcileCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), mallWeatherFeishuCheckpointTimeout)
	reconcileErr := runner.checkpoints.ReconcileWeatherBatchSuccess(
		reconcileCtx,
		checkpoint.ID,
		batch.Checksum,
		checkpoint.RowStart,
		checkpoint.RowEnd,
		finishedAt,
	)
	cancel()
	if reconcileErr != nil {
		return 0, false, fmt.Errorf(
			"%w: reconcile verified checkpoint",
			ErrMallWeatherFeishuOverwriteStateUnknown,
		)
	}
	return checkpoint.Attempt, false, nil
}

func nextMallWeatherFeishuOverwriteAttempt(previous int) (int, bool, error) {
	if previous < 1 || previous == int(^uint(0)>>1) {
		return 0, false, ErrMallWeatherFeishuOverwriteCheckpointConflict
	}
	return previous + 1, true, nil
}

func validateMallWeatherFeishuOverwriteCheckpoint(
	request mallWeatherFeishuOverwriteDatasetRequest,
	batchNo int,
	rowStart int64,
	rowEnd int64,
	columns int,
	batch mallWeatherFeishuRenderedBatch,
	checkpoint *model.DeliveryLog,
) error {
	if checkpoint == nil || checkpoint.ID == 0 || checkpoint.RunID != request.RunID ||
		checkpoint.DestinationID != request.Destination.DestinationID ||
		checkpoint.DatasetKind != request.Dataset.Kind || checkpoint.BatchNo != batchNo ||
		checkpoint.RequestChecksum != batch.Checksum || checkpoint.Attempt < 1 || checkpoint.RowStart != rowStart ||
		checkpoint.RowEnd != rowEnd || checkpoint.RecordCount != len(batch.Rows) ||
		checkpoint.CellCount != len(batch.Rows)*columns {
		return ErrMallWeatherFeishuOverwriteCheckpointConflict
	}
	return nil
}

func validMallWeatherFeishuOverwriteDatasetBatchResult(
	result mallWeatherFeishuOverwriteBatchResult,
	batchNo int,
	rowStart int64,
	rowEnd int64,
	batch mallWeatherFeishuRenderedBatch,
	columns int,
) bool {
	return result.BatchNo == batchNo && result.RowStart == rowStart && result.RowEnd == rowEnd &&
		result.RecordCount == len(batch.Rows) && result.CellCount == len(batch.Rows)*columns
}

func setMallWeatherFeishuOverwriteStaleRange(
	result *mallWeatherFeishuOverwriteDatasetResult,
	nextDataRow int64,
) {
	if result == nil || nextDataRow < 2 || nextDataRow > result.PreviousLastRow {
		return
	}
	result.StaleRowStart = nextDataRow
	result.StaleRowEnd = result.PreviousLastRow
}
