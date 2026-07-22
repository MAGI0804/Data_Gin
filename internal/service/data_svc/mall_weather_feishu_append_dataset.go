package data_svc

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrMallWeatherFeishuAppendCheckpointConflict = errors.New(
	"mall weather feishu append dataset: checkpoint conflicts with rendered batch",
)

type mallWeatherFeishuAppendDatasetPager interface {
	Page(context.Context, data_dao.MallWeatherExportDataPageRequest) (*data_dao.MallWeatherExportDataPage, error)
}

type mallWeatherFeishuAppendCheckpointStore interface {
	FindLatestWeatherBatch(context.Context, uint, uint, string, int) (*model.DeliveryLog, error)
	ReconcileWeatherBatchSuccess(context.Context, uint, string, int64, int64, time.Time) error
}

type mallWeatherFeishuAppendBatchRunner interface {
	Execute(context.Context, mallWeatherFeishuAppendBatchRequest) (mallWeatherFeishuAppendBatchResult, error)
}

type mallWeatherFeishuAppendDatasetRequest struct {
	RunID       uint
	TraceID     string
	Destination *MallWeatherFeishuResolvedDestination
	Profile     MallWeatherExportProfileDTO
	Dataset     requestbody.MallWeatherExportDataset
	Filter      data_dao.MallWeatherExportEstimateFilter
	SnapshotAt  time.Time
	GridRows    int64
}

type mallWeatherFeishuAppendDatasetResult struct {
	DatasetKind   string
	BatchCount    int
	RecordCount   int64
	CellCount     int64
	LastCursor    uint
	LastRemoteRow int64
}

type mallWeatherFeishuAppendDatasetRunner struct {
	pager       mallWeatherFeishuAppendDatasetPager
	sheets      mallWeatherFeishuRangeReader
	checkpoints mallWeatherFeishuAppendCheckpointStore
	batches     mallWeatherFeishuAppendBatchRunner
	now         func() time.Time
}

func newMallWeatherFeishuAppendDatasetRunner(
	pager mallWeatherFeishuAppendDatasetPager,
	sheets mallWeatherFeishuRangeReader,
	checkpoints mallWeatherFeishuAppendCheckpointStore,
	batches mallWeatherFeishuAppendBatchRunner,
	now func() time.Time,
) (*mallWeatherFeishuAppendDatasetRunner, error) {
	if pager == nil || sheets == nil || checkpoints == nil || batches == nil || now == nil {
		return nil, errors.New("mall weather feishu append dataset: invalid runner configuration")
	}
	return &mallWeatherFeishuAppendDatasetRunner{
		pager: pager, sheets: sheets, checkpoints: checkpoints, batches: batches, now: now,
	}, nil
}

// Run must execute while the caller owns the destination execution lock. The
// lock covers the initial append-row scan and every subsequent append so two
// application instances cannot allocate the same remote rows.
func (runner *mallWeatherFeishuAppendDatasetRunner) Run(
	ctx context.Context,
	request mallWeatherFeishuAppendDatasetRequest,
) (mallWeatherFeishuAppendDatasetResult, error) {
	result := mallWeatherFeishuAppendDatasetResult{DatasetKind: request.Dataset.Kind}
	columns, fields, asOfUTC, err := validateMallWeatherFeishuAppendDatasetRequest(ctx, runner, request)
	if err != nil {
		return result, err
	}
	sheetID := request.Destination.SheetIDs[request.Dataset.Kind]
	appendRow, err := findMallWeatherFeishuAppendRow(
		ctx,
		runner.sheets,
		request.Destination.SpreadsheetToken,
		sheetID,
		request.GridRows,
	)
	if err != nil {
		return result, fmt.Errorf("mall weather feishu append dataset: locate append row: %w", err)
	}
	initialAppendRow := appendRow
	var afterID uint
	var previousCompletedRowEnd int64
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
			return result, fmt.Errorf("mall weather feishu append dataset: read dataset page: %w", err)
		}
		nextAfterID, err := validateMallWeatherExportPage(page, afterID)
		if err != nil {
			return result, fmt.Errorf("mall weather feishu append dataset: validate dataset page: %w", err)
		}
		if len(page.Rows) == 0 {
			return result, nil
		}
		batch, err := renderMallWeatherFeishuBatch(request.Profile, request.Dataset, page.Rows)
		if err != nil {
			return result, fmt.Errorf("mall weather feishu append dataset: render batch: %w", err)
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
			return result, fmt.Errorf("mall weather feishu append dataset: load batch checkpoint: %w", err)
		}
		completedRowEnd, attempt, shouldAppend, err := runner.resolveMallWeatherFeishuAppendCheckpoint(
			ctx,
			request,
			batchNo,
			columns,
			batch,
			checkpoint,
			initialAppendRow,
			previousCompletedRowEnd,
		)
		if err != nil {
			return result, err
		}
		if shouldAppend {
			batchResult, executeErr := runner.batches.Execute(ctx, mallWeatherFeishuAppendBatchRequest{
				RunID: request.RunID, TraceID: request.TraceID, Destination: request.Destination,
				DatasetKind: request.Dataset.Kind, BatchNo: batchNo, Attempt: attempt,
				RowStart: appendRow, Batch: batch,
			})
			if executeErr != nil {
				return result, fmt.Errorf("mall weather feishu append dataset: execute batch: %w", executeErr)
			}
			if !validMallWeatherFeishuAppendDatasetBatchResult(batchResult, batchNo, appendRow, batch, columns) {
				return result, errors.New("mall weather feishu append dataset: invalid batch result")
			}
			completedRowEnd = batchResult.RowEnd
			appendRow = batchResult.RowEnd + 1
		} else if completedRowEnd >= appendRow {
			appendRow = completedRowEnd + 1
		}
		previousCompletedRowEnd = completedRowEnd
		result.BatchCount++
		result.RecordCount += int64(len(batch.Rows))
		result.CellCount += int64(len(batch.Rows) * columns)
		result.LastCursor = nextAfterID
		result.LastRemoteRow = completedRowEnd
		afterID = nextAfterID
		if !page.HasMore {
			return result, nil
		}
	}
}

func validateMallWeatherFeishuAppendDatasetRequest(
	ctx context.Context,
	runner *mallWeatherFeishuAppendDatasetRunner,
	request mallWeatherFeishuAppendDatasetRequest,
) (int, []string, *time.Time, error) {
	if ctx == nil || runner == nil || runner.pager == nil || runner.sheets == nil || runner.checkpoints == nil ||
		runner.batches == nil || runner.now == nil || request.Destination == nil {
		return 0, nil, nil, errors.New("mall weather feishu append dataset: invalid request")
	}
	_, datasetAllowed := mallWeatherFeishuDatasetKinds[request.Dataset.Kind]
	destinationValid := request.Destination.DestinationID != 0 &&
		request.Destination.Code != "" && request.Destination.Config.WriteMode == "append" &&
		request.Destination.Config.BatchRows >= 1 &&
		request.Destination.Config.BatchRows <= maxMallWeatherFeishuBatchRows &&
		request.Destination.SpreadsheetToken != "" &&
		request.Destination.SheetIDs[request.Dataset.Kind] != "" &&
		request.Destination.Config.SheetIDEnvMapping[request.Dataset.Kind] != ""
	profileValid := request.Profile.ID != 0 && request.Profile.Version != 0 && request.Profile.Enabled &&
		request.Profile.Code != "" && request.Profile.Code == request.Destination.Config.ProfileCode
	if request.RunID == 0 || uuid.Validate(request.TraceID) != nil ||
		!destinationValid || !profileValid || !datasetAllowed || request.Dataset.SplitBy != "" ||
		!mallWeatherFeishuProfileHasDataset(request.Profile, request.Dataset) ||
		request.SnapshotAt.IsZero() || request.GridRows < 1 || request.GridRows > maxMallWeatherFeishuSheetRow {
		return 0, nil, nil, errors.New("mall weather feishu append dataset: invalid request")
	}
	columns, err := mallWeatherExportRenderColumns(request.Dataset)
	if err != nil || len(columns) == 0 || len(columns) > maxMallWeatherFeishuColumns {
		return 0, nil, nil, errors.New("mall weather feishu append dataset: invalid columns")
	}
	fields := make([]string, len(columns))
	for index, column := range columns {
		fields[index] = column.Field
	}
	var asOfUTC *time.Time
	if request.Dataset.AsOf != "" {
		parsed, parseErr := time.Parse(time.RFC3339Nano, request.Dataset.AsOf)
		if parseErr != nil {
			return 0, nil, nil, errors.New("mall weather feishu append dataset: invalid dataset as-of")
		}
		parsed = parsed.UTC()
		asOfUTC = &parsed
	}
	return len(columns), fields, asOfUTC, nil
}

func mallWeatherFeishuProfileHasDataset(
	profile MallWeatherExportProfileDTO,
	dataset requestbody.MallWeatherExportDataset,
) bool {
	matches := 0
	for _, configured := range profile.Datasets {
		if configured.Kind == dataset.Kind {
			matches++
			if !reflect.DeepEqual(configured, dataset) {
				return false
			}
		}
	}
	return matches == 1
}

func mallWeatherFeishuDatasetLatest(dataset requestbody.MallWeatherExportDataset) bool {
	return dataset.Latest != nil && *dataset.Latest
}

func (runner *mallWeatherFeishuAppendDatasetRunner) resolveMallWeatherFeishuAppendCheckpoint(
	ctx context.Context,
	request mallWeatherFeishuAppendDatasetRequest,
	batchNo int,
	columns int,
	batch mallWeatherFeishuRenderedBatch,
	checkpoint *model.DeliveryLog,
	initialAppendRow int64,
	previousCompletedRowEnd int64,
) (rowEnd int64, attempt int, shouldAppend bool, err error) {
	if checkpoint == nil {
		return 0, 1, true, nil
	}
	if err := validateMallWeatherFeishuAppendCheckpoint(
		request,
		batchNo,
		columns,
		batch,
		checkpoint,
	); err != nil {
		return 0, 0, false, err
	}
	switch checkpoint.Status {
	case "success":
		if checkpoint.RowEnd >= initialAppendRow ||
			(previousCompletedRowEnd > 0 && checkpoint.RowStart != previousCompletedRowEnd+1) {
			return 0, 0, false, ErrMallWeatherFeishuAppendCheckpointConflict
		}
		return checkpoint.RowEnd, checkpoint.Attempt, false, nil
	case "failed":
		if checkpoint.Attempt == int(^uint(0)>>1) {
			return 0, 0, false, ErrMallWeatherFeishuAppendCheckpointConflict
		}
		return 0, checkpoint.Attempt + 1, true, nil
	case "running", "unknown":
		if checkpoint.RowEnd >= initialAppendRow ||
			(previousCompletedRowEnd > 0 && checkpoint.RowStart != previousCompletedRowEnd+1) {
			return 0, 0, false, ErrMallWeatherFeishuAppendCheckpointConflict
		}
		matched, verifyErr := verifyMallWeatherFeishuAppendCheckpoint(
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
			return 0, 0, false, fmt.Errorf("mall weather feishu append dataset: verify uncertain checkpoint: %w", verifyErr)
		}
		if !matched {
			return 0, 0, false, ErrMallWeatherFeishuAppendCheckpointConflict
		}
		finishedAt := runner.now().UTC()
		if finishedAt.IsZero() {
			return 0, 0, false, errors.New("mall weather feishu append dataset: invalid reconciliation time")
		}
		reconcileCtx, cancel := context.WithTimeout(
			context.WithoutCancel(ctx),
			mallWeatherFeishuCheckpointTimeout,
		)
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
			return 0, 0, false, fmt.Errorf(
				"%w: reconcile verified checkpoint",
				ErrMallWeatherFeishuAppendStateUnknown,
			)
		}
		return checkpoint.RowEnd, checkpoint.Attempt, false, nil
	default:
		return 0, 0, false, ErrMallWeatherFeishuAppendCheckpointConflict
	}
}

func validMallWeatherFeishuAppendDatasetBatchResult(
	result mallWeatherFeishuAppendBatchResult,
	batchNo int,
	rowStart int64,
	batch mallWeatherFeishuRenderedBatch,
	columns int,
) bool {
	return result.BatchNo == batchNo && result.RowStart == rowStart &&
		result.RowEnd == rowStart+int64(len(batch.Rows))-1 &&
		result.RecordCount == len(batch.Rows) && result.CellCount == len(batch.Rows)*columns
}

func validateMallWeatherFeishuAppendCheckpoint(
	request mallWeatherFeishuAppendDatasetRequest,
	batchNo int,
	columns int,
	batch mallWeatherFeishuRenderedBatch,
	checkpoint *model.DeliveryLog,
) error {
	if checkpoint == nil || checkpoint.ID == 0 || checkpoint.RunID != request.RunID ||
		checkpoint.DestinationID != request.Destination.DestinationID ||
		checkpoint.DatasetKind != request.Dataset.Kind || checkpoint.BatchNo != batchNo ||
		checkpoint.RequestChecksum != batch.Checksum || checkpoint.Attempt < 1 ||
		checkpoint.RowStart < 2 || checkpoint.RowEnd < checkpoint.RowStart ||
		checkpoint.RowEnd > maxMallWeatherFeishuSheetRow ||
		checkpoint.RowEnd-checkpoint.RowStart+1 != int64(len(batch.Rows)) ||
		checkpoint.RecordCount != len(batch.Rows) || checkpoint.CellCount != len(batch.Rows)*columns {
		return ErrMallWeatherFeishuAppendCheckpointConflict
	}
	return nil
}
