package data_svc

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"time"

	"gin-biz-web-api/connector/feishu"
	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/model"

	"github.com/google/uuid"
)

const maxMallWeatherFeishuUpsertBatchCells = maxMallWeatherFeishuBatchRows * maxMallWeatherFeishuColumns

type mallWeatherFeishuUpsertScannerRunner interface {
	Ensure(context.Context, mallWeatherFeishuUpsertScanRequest) (mallWeatherFeishuUpsertScanResult, error)
}

type mallWeatherFeishuUpsertMappingWriter interface {
	FindByBusinessKeys(context.Context, uint, string, []string) (map[string]model.MallWeatherSheetRow, error)
	UpsertMappings(context.Context, uint, string, string, []data_dao.MallWeatherSheetRowMapping, time.Time) error
	MarkUninitialized(context.Context, uint, string) error
	MarkInitialized(context.Context, uint, string, string, string, time.Time) error
}

type mallWeatherFeishuUpsertBatchRunner interface {
	Execute(context.Context, mallWeatherFeishuUpsertBatchRequest) (mallWeatherFeishuUpsertBatchResult, error)
}

type mallWeatherFeishuUpsertDatasetRequest struct {
	RunID       uint
	TraceID     string
	Destination *MallWeatherFeishuResolvedDestination
	Profile     MallWeatherExportProfileDTO
	Dataset     requestbody.MallWeatherExportDataset
	Filter      data_dao.MallWeatherExportEstimateFilter
	SnapshotAt  time.Time
	GridRows    int64
}

type mallWeatherFeishuUpsertDatasetResult struct {
	DatasetKind   string
	BatchCount    int
	RecordCount   int64
	CellCount     int64
	SkippedCount  int64
	UpdatedCount  int64
	AppendedCount int64
	LastCursor    uint
	LastRemoteRow int64
}

type mallWeatherFeishuUpsertDatasetRunner struct {
	pager    mallWeatherFeishuDatasetPager
	scanner  mallWeatherFeishuUpsertScannerRunner
	mappings mallWeatherFeishuUpsertMappingWriter
	batches  mallWeatherFeishuUpsertBatchRunner
	now      func() time.Time
}

func newMallWeatherFeishuUpsertDatasetRunner(
	pager mallWeatherFeishuDatasetPager,
	scanner mallWeatherFeishuUpsertScannerRunner,
	mappings mallWeatherFeishuUpsertMappingWriter,
	batches mallWeatherFeishuUpsertBatchRunner,
	now func() time.Time,
) (*mallWeatherFeishuUpsertDatasetRunner, error) {
	if pager == nil || scanner == nil || mappings == nil || batches == nil || now == nil {
		return nil, errors.New("mall weather feishu upsert dataset: invalid runner configuration")
	}
	return &mallWeatherFeishuUpsertDatasetRunner{
		pager: pager, scanner: scanner, mappings: mappings, batches: batches, now: now,
	}, nil
}

// Run must execute while the caller owns the destination lock. Before the
// first remote write it removes the schema marker; if the process crashes, the
// next run must rescan the remote sheet before deciding whether to update or
// append.
func (runner *mallWeatherFeishuUpsertDatasetRunner) Run(
	ctx context.Context,
	request mallWeatherFeishuUpsertDatasetRequest,
) (mallWeatherFeishuUpsertDatasetResult, error) {
	result := mallWeatherFeishuUpsertDatasetResult{DatasetKind: request.Dataset.Kind, LastRemoteRow: 1}
	columns, fields, uniqueKeys, sheetEnv, schemaChecksum, asOfUTC, err :=
		validateMallWeatherFeishuUpsertDatasetRequest(ctx, runner, request)
	if err != nil {
		return result, err
	}
	scan, err := runner.scanner.Ensure(ctx, mallWeatherFeishuUpsertScanRequest{
		Destination: request.Destination, Dataset: request.Dataset, GridRows: request.GridRows,
	})
	if err != nil {
		return result, fmt.Errorf("mall weather feishu upsert dataset: ensure mapping scan: %w", err)
	}
	nextAppendRow := scan.LastDataRow + 1
	if nextAppendRow < 2 {
		nextAppendRow = 2
	}
	result.LastRemoteRow = max(scan.LastDataRow, int64(1))
	var afterID uint
	batchNo := 1
	markerRemoved := false
	for {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		page, err := runner.pager.Page(ctx, data_dao.MallWeatherExportDataPageRequest{
			Kind: request.Dataset.Kind, Fields: fields, Filter: request.Filter,
			Latest: mallWeatherFeishuDatasetLatest(request.Dataset), AsOfUTC: asOfUTC,
			AfterID: afterID, Limit: request.Destination.Config.BatchRows, SnapshotAt: request.SnapshotAt.UTC(),
		})
		if err != nil {
			return result, fmt.Errorf("mall weather feishu upsert dataset: read dataset page: %w", err)
		}
		nextAfterID, err := validateMallWeatherExportPage(page, afterID)
		if err != nil {
			return result, fmt.Errorf("mall weather feishu upsert dataset: validate dataset page: %w", err)
		}
		if len(page.Rows) == 0 {
			return result, runner.markMallWeatherFeishuUpsertInitialized(
				ctx,
				request,
				sheetEnv,
				schemaChecksum,
				markerRemoved,
			)
		}
		renderedBatch, err := renderMallWeatherFeishuBatch(request.Profile, request.Dataset, page.Rows)
		if err != nil {
			return result, fmt.Errorf("mall weather feishu upsert dataset: render batch: %w", err)
		}
		renderedRows, err := buildMallWeatherFeishuUpsertRows(columns, uniqueKeys, renderedBatch)
		if err != nil {
			return result, fmt.Errorf("mall weather feishu upsert dataset: derive row keys: %w", err)
		}
		existing, err := runner.mappings.FindByBusinessKeys(
			ctx,
			request.Destination.DestinationID,
			request.Dataset.Kind,
			mallWeatherFeishuUpsertBusinessKeys(renderedRows),
		)
		if err != nil {
			return result, fmt.Errorf("mall weather feishu upsert dataset: load mappings: %w", err)
		}
		updateRows, appendRows, skipped, err := mallWeatherFeishuUpsertPlanWrites(
			renderedRows,
			existing,
			&nextAppendRow,
		)
		if err != nil {
			return result, err
		}
		for _, plan := range []struct {
			mode string
			rows []mallWeatherFeishuUpsertWriteRow
		}{
			{mode: "update", rows: updateRows},
			{mode: "append", rows: appendRows},
		} {
			writeBatches, err := mallWeatherFeishuUpsertWriteBatches(
				plan.mode,
				plan.rows,
				len(columns),
				request.Destination.Config.BatchRows,
			)
			if err != nil {
				return result, err
			}
			for _, rows := range writeBatches {
				if !markerRemoved {
					if err := runner.mappings.MarkUninitialized(
						ctx,
						request.Destination.DestinationID,
						request.Dataset.Kind,
					); err != nil {
						return result, fmt.Errorf("mall weather feishu upsert dataset: mark uninitialized: %w", err)
					}
					markerRemoved = true
				}
				batchResult, executeErr := runner.batches.Execute(ctx, mallWeatherFeishuUpsertBatchRequest{
					RunID: request.RunID, TraceID: request.TraceID, Destination: request.Destination,
					DatasetKind: request.Dataset.Kind, BatchNo: batchNo, Attempt: 1, Mode: plan.mode, Rows: rows,
				})
				if executeErr != nil {
					return result, fmt.Errorf("mall weather feishu upsert dataset: execute batch: %w", executeErr)
				}
				if !validMallWeatherFeishuUpsertDatasetBatchResult(batchResult, batchNo, plan.mode, rows, len(columns)) {
					return result, errors.New("mall weather feishu upsert dataset: invalid batch result")
				}
				if err := runner.persistMallWeatherFeishuUpsertMappings(
					ctx,
					request,
					sheetEnv,
					batchResult.Rows,
				); err != nil {
					return result, err
				}
				result.BatchCount++
				if plan.mode == "update" {
					result.UpdatedCount += int64(len(rows))
				} else {
					result.AppendedCount += int64(len(rows))
					result.LastRemoteRow = max(result.LastRemoteRow, rows[len(rows)-1].RowNumber)
				}
				batchNo++
			}
		}
		result.RecordCount += int64(len(renderedRows))
		result.CellCount += int64(len(renderedRows) * len(columns))
		result.SkippedCount += int64(skipped)
		result.LastCursor = nextAfterID
		afterID = nextAfterID
		if !page.HasMore {
			return result, runner.markMallWeatherFeishuUpsertInitialized(
				ctx,
				request,
				sheetEnv,
				schemaChecksum,
				markerRemoved,
			)
		}
	}
}

func validateMallWeatherFeishuUpsertDatasetRequest(
	ctx context.Context,
	runner *mallWeatherFeishuUpsertDatasetRunner,
	request mallWeatherFeishuUpsertDatasetRequest,
) ([]requestbody.MallWeatherExportColumn, []string, []string, string, string, *time.Time, error) {
	if ctx == nil || runner == nil || runner.pager == nil || runner.scanner == nil || runner.mappings == nil ||
		runner.batches == nil || runner.now == nil || request.Destination == nil {
		return nil, nil, nil, "", "", nil, errors.New("mall weather feishu upsert dataset: invalid request")
	}
	_, datasetAllowed := mallWeatherFeishuDatasetKinds[request.Dataset.Kind]
	destinationValid := request.Destination.DestinationID != 0 &&
		request.Destination.Code != "" && request.Destination.Config.WriteMode == "upsert" &&
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
		return nil, nil, nil, "", "", nil, errors.New("mall weather feishu upsert dataset: invalid request")
	}
	columns, err := mallWeatherExportRenderColumns(request.Dataset)
	uniqueKeys := request.Destination.Config.UniqueKeyFields[request.Dataset.Kind]
	if err != nil || len(columns) == 0 || len(columns) > maxMallWeatherFeishuColumns || len(uniqueKeys) == 0 {
		return nil, nil, nil, "", "", nil, errors.New("mall weather feishu upsert dataset: invalid columns")
	}
	if err := validateMallWeatherFeishuPlannedUniqueKeys("upsert", columns, uniqueKeys); err != nil {
		return nil, nil, nil, "", "", nil, errors.New("mall weather feishu upsert dataset: invalid unique keys")
	}
	fields := make([]string, len(columns))
	for index, column := range columns {
		fields[index] = column.Field
	}
	schemaChecksum, err := mallWeatherFeishuMappingSchemaChecksum(request.Destination, request.Dataset.Kind, columns)
	if err != nil {
		return nil, nil, nil, "", "", nil, err
	}
	var asOfUTC *time.Time
	if request.Dataset.AsOf != "" {
		parsed, parseErr := time.Parse(time.RFC3339Nano, request.Dataset.AsOf)
		if parseErr != nil {
			return nil, nil, nil, "", "", nil, errors.New("mall weather feishu upsert dataset: invalid dataset as-of")
		}
		parsed = parsed.UTC()
		asOfUTC = &parsed
	}
	return columns,
		fields,
		append([]string(nil), uniqueKeys...),
		request.Destination.Config.SheetIDEnvMapping[request.Dataset.Kind],
		schemaChecksum,
		asOfUTC,
		nil
}

func mallWeatherFeishuUpsertBusinessKeys(rows []mallWeatherFeishuUpsertRenderedRow) []string {
	keys := make([]string, len(rows))
	for index, row := range rows {
		keys[index] = row.BusinessKey
	}
	return keys
}

func mallWeatherFeishuUpsertPlanWrites(
	renderedRows []mallWeatherFeishuUpsertRenderedRow,
	existing map[string]model.MallWeatherSheetRow,
	nextAppendRow *int64,
) ([]mallWeatherFeishuUpsertWriteRow, []mallWeatherFeishuUpsertWriteRow, int, error) {
	if len(renderedRows) == 0 || existing == nil || nextAppendRow == nil || *nextAppendRow < 2 {
		return nil, nil, 0, errors.New("mall weather feishu upsert dataset: invalid write plan")
	}
	updateRows := make([]mallWeatherFeishuUpsertWriteRow, 0, len(renderedRows))
	appendRows := make([]mallWeatherFeishuUpsertWriteRow, 0, len(renderedRows))
	skipped := 0
	for _, rendered := range renderedRows {
		mapping, found := existing[rendered.BusinessKey]
		if found {
			if mapping.RowNumber < 2 || mapping.RowNumber > maxMallWeatherFeishuSheetRow {
				return nil, nil, 0, errors.New("mall weather feishu upsert dataset: invalid stored row mapping")
			}
			if mapping.Checksum == rendered.Checksum {
				skipped++
				continue
			}
			updateRows = append(updateRows, mallWeatherFeishuUpsertWriteRow{
				BusinessKey: rendered.BusinessKey,
				RowNumber:   mapping.RowNumber,
				Checksum:    rendered.Checksum,
				Cells:       append([]feishu.SheetCell(nil), rendered.Cells...),
			})
			continue
		}
		if *nextAppendRow > maxMallWeatherFeishuSheetRow {
			return nil, nil, 0, errors.New("mall weather feishu upsert dataset: sheet row limit reached")
		}
		appendRows = append(appendRows, mallWeatherFeishuUpsertWriteRow{
			BusinessKey: rendered.BusinessKey,
			RowNumber:   *nextAppendRow,
			Checksum:    rendered.Checksum,
			Cells:       append([]feishu.SheetCell(nil), rendered.Cells...),
		})
		*nextAppendRow++
	}
	return updateRows, appendRows, skipped, nil
}

func mallWeatherFeishuUpsertWriteBatches(
	mode string,
	rows []mallWeatherFeishuUpsertWriteRow,
	columns int,
	configuredRows int,
) ([][]mallWeatherFeishuUpsertWriteRow, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	if columns < 1 || columns > maxMallWeatherFeishuColumns || configuredRows < 1 ||
		configuredRows > maxMallWeatherFeishuBatchRows {
		return nil, errors.New("mall weather feishu upsert dataset: invalid batch dimensions")
	}
	maxRows := min(configuredRows, maxMallWeatherFeishuBatchRows, maxMallWeatherFeishuUpsertBatchCells/columns)
	if maxRows < 1 {
		return nil, errors.New("mall weather feishu upsert dataset: invalid batch cell limit")
	}
	sortedRows := slices.Clone(rows)
	sort.Slice(sortedRows, func(left, right int) bool { return sortedRows[left].RowNumber < sortedRows[right].RowNumber })
	if mode == "append" {
		return mallWeatherFeishuUpsertAppendWriteBatches(sortedRows, maxRows), nil
	}
	if mode != "update" {
		return nil, errors.New("mall weather feishu upsert dataset: invalid batch mode")
	}
	return mallWeatherFeishuUpsertUpdateWriteBatches(sortedRows, maxRows), nil
}

func mallWeatherFeishuUpsertAppendWriteBatches(
	rows []mallWeatherFeishuUpsertWriteRow,
	maxRows int,
) [][]mallWeatherFeishuUpsertWriteRow {
	batches := make([][]mallWeatherFeishuUpsertWriteRow, 0, (len(rows)+maxRows-1)/maxRows)
	for start := 0; start < len(rows); start += maxRows {
		end := min(start+maxRows, len(rows))
		batches = append(batches, slices.Clone(rows[start:end]))
	}
	return batches
}

func mallWeatherFeishuUpsertUpdateWriteBatches(
	rows []mallWeatherFeishuUpsertWriteRow,
	maxRows int,
) [][]mallWeatherFeishuUpsertWriteRow {
	batches := make([][]mallWeatherFeishuUpsertWriteRow, 0, (len(rows)+maxRows-1)/maxRows)
	start := 0
	rangeCount := 0
	for index, row := range rows {
		newRange := index == start || row.RowNumber != rows[index-1].RowNumber+1
		if index > start && (index-start == maxRows ||
			(newRange && rangeCount == maxMallWeatherFeishuUpsertUpdateRanges)) {
			batches = append(batches, slices.Clone(rows[start:index]))
			start = index
			rangeCount = 0
			newRange = true
		}
		if newRange {
			rangeCount++
		}
	}
	if start < len(rows) {
		batches = append(batches, slices.Clone(rows[start:]))
	}
	return batches
}

func validMallWeatherFeishuUpsertDatasetBatchResult(
	result mallWeatherFeishuUpsertBatchResult,
	batchNo int,
	mode string,
	rows []mallWeatherFeishuUpsertWriteRow,
	columns int,
) bool {
	if result.BatchNo != batchNo || result.Mode != mode || result.RecordCount != len(rows) ||
		result.CellCount != len(rows)*columns || len(result.Rows) != len(rows) {
		return false
	}
	for index, row := range rows {
		got := result.Rows[index]
		if got.BusinessKey != row.BusinessKey || got.RowNumber != row.RowNumber || got.Checksum != row.Checksum ||
			!slices.Equal(got.Cells, row.Cells) {
			return false
		}
	}
	return true
}

func (runner *mallWeatherFeishuUpsertDatasetRunner) persistMallWeatherFeishuUpsertMappings(
	ctx context.Context,
	request mallWeatherFeishuUpsertDatasetRequest,
	sheetEnv string,
	rows []mallWeatherFeishuUpsertWriteRow,
) error {
	if len(rows) == 0 {
		return errors.New("mall weather feishu upsert dataset: invalid empty mapping update")
	}
	syncedAt := runner.now().UTC()
	if syncedAt.IsZero() {
		return errors.New("mall weather feishu upsert dataset: invalid mapping sync time")
	}
	mappings := make([]data_dao.MallWeatherSheetRowMapping, len(rows))
	for index, row := range rows {
		mappings[index] = data_dao.MallWeatherSheetRowMapping{
			BusinessKey: row.BusinessKey,
			RowNumber:   row.RowNumber,
			Checksum:    row.Checksum,
		}
	}
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), mallWeatherFeishuCheckpointTimeout)
	defer cancel()
	if err := runner.mappings.UpsertMappings(
		persistCtx,
		request.Destination.DestinationID,
		request.Dataset.Kind,
		sheetEnv,
		mappings,
		syncedAt,
	); err != nil {
		return fmt.Errorf("%w: persist upsert mappings", ErrMallWeatherFeishuUpsertStateUnknown)
	}
	return nil
}

func (runner *mallWeatherFeishuUpsertDatasetRunner) markMallWeatherFeishuUpsertInitialized(
	ctx context.Context,
	request mallWeatherFeishuUpsertDatasetRequest,
	sheetEnv string,
	schemaChecksum string,
	markerRemoved bool,
) error {
	if !markerRemoved {
		return nil
	}
	initializedAt := runner.now().UTC()
	if initializedAt.IsZero() {
		return errors.New("mall weather feishu upsert dataset: invalid initialization time")
	}
	markCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), mallWeatherFeishuCheckpointTimeout)
	defer cancel()
	if err := runner.mappings.MarkInitialized(
		markCtx,
		request.Destination.DestinationID,
		request.Dataset.Kind,
		sheetEnv,
		schemaChecksum,
		initializedAt,
	); err != nil {
		return fmt.Errorf("mall weather feishu upsert dataset: mark initialized: %w", err)
	}
	return nil
}
