package data_svc

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gin-biz-web-api/connector/feishu"
	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/internal/requestbody"
)

type mallWeatherFeishuUpsertMappingStore interface {
	IsInitialized(context.Context, uint, string, string, string) (bool, error)
	ResetMappings(context.Context, uint, string) error
	CreateScannedMappings(
		context.Context,
		uint,
		string,
		string,
		[]data_dao.MallWeatherSheetRowMapping,
		time.Time,
	) error
	MarkInitialized(context.Context, uint, string, string, string, time.Time) error
}

type mallWeatherFeishuUpsertScanRequest struct {
	Destination *MallWeatherFeishuResolvedDestination
	Dataset     requestbody.MallWeatherExportDataset
	GridRows    int64
}

type mallWeatherFeishuUpsertScanResult struct {
	Initialized bool
	LastDataRow int64
	MappedRows  int64
}

type mallWeatherFeishuUpsertScanner struct {
	sheets   mallWeatherFeishuRangeReader
	mappings mallWeatherFeishuUpsertMappingStore
	now      func() time.Time
}

func newMallWeatherFeishuUpsertScanner(
	sheets mallWeatherFeishuRangeReader,
	mappings mallWeatherFeishuUpsertMappingStore,
	now func() time.Time,
) (*mallWeatherFeishuUpsertScanner, error) {
	if sheets == nil || mappings == nil || now == nil {
		return nil, errors.New("mall weather feishu upsert scan: invalid configuration")
	}
	return &mallWeatherFeishuUpsertScanner{sheets: sheets, mappings: mappings, now: now}, nil
}

// Ensure executes while the caller owns the destination lock. A missing or
// stale schema marker resets partial local mappings before scanning again, so
// a Worker crash can never make an incomplete scan appear initialized.
func (scanner *mallWeatherFeishuUpsertScanner) Ensure(
	ctx context.Context,
	request mallWeatherFeishuUpsertScanRequest,
) (mallWeatherFeishuUpsertScanResult, error) {
	var result mallWeatherFeishuUpsertScanResult
	columns, uniqueKeys, sheetID, sheetEnv, err := validateMallWeatherFeishuUpsertScanRequest(ctx, scanner, request)
	if err != nil {
		return result, err
	}
	schemaChecksum, err := mallWeatherFeishuMappingSchemaChecksum(request.Destination, request.Dataset.Kind, columns)
	if err != nil {
		return result, err
	}
	initialized, err := scanner.mappings.IsInitialized(
		ctx,
		request.Destination.DestinationID,
		request.Dataset.Kind,
		sheetEnv,
		schemaChecksum,
	)
	if err != nil {
		return result, fmt.Errorf("mall weather feishu upsert scan: inspect mapping state: %w", err)
	}
	lastRow, err := findMallWeatherFeishuLastDataRow(
		ctx,
		scanner.sheets,
		request.Destination.SpreadsheetToken,
		sheetID,
		request.GridRows,
	)
	if err != nil {
		return result, fmt.Errorf("mall weather feishu upsert scan: locate last row: %w", err)
	}
	result.LastDataRow = lastRow
	if initialized {
		result.Initialized = true
		return result, nil
	}
	if err := scanner.mappings.ResetMappings(
		ctx,
		request.Destination.DestinationID,
		request.Dataset.Kind,
	); err != nil {
		return result, fmt.Errorf("mall weather feishu upsert scan: reset partial mappings: %w", err)
	}
	for startRow := int64(2); startRow <= lastRow; {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		endRow := min(startRow+int64(request.Destination.Config.BatchRows)-1, lastRow)
		values, err := scanner.sheets.ReadRange(ctx, request.Destination.SpreadsheetToken, feishu.SheetRange{
			SheetID: sheetID, StartRow: startRow, EndRow: endRow,
			StartColumn: 1, EndColumn: int64(len(columns)),
		})
		if err != nil {
			return result, fmt.Errorf("mall weather feishu upsert scan: read mapping range: %w", err)
		}
		rows, rowNumbers, err := normalizeMallWeatherFeishuScannedRows(values, startRow, endRow, len(columns))
		if err != nil {
			return result, err
		}
		if len(rows) > 0 {
			rendered, err := buildMallWeatherFeishuUpsertRows(
				columns,
				uniqueKeys,
				mallWeatherFeishuRenderedBatch{Rows: rows},
			)
			if err != nil {
				return result, fmt.Errorf("mall weather feishu upsert scan: derive remote mappings: %w", err)
			}
			mappings := make([]data_dao.MallWeatherSheetRowMapping, len(rendered))
			for index, row := range rendered {
				mappings[index] = data_dao.MallWeatherSheetRowMapping{
					BusinessKey: row.BusinessKey,
					RowNumber:   rowNumbers[index],
					Checksum:    row.Checksum,
				}
			}
			if err := scanner.mappings.CreateScannedMappings(
				ctx,
				request.Destination.DestinationID,
				request.Dataset.Kind,
				sheetEnv,
				mappings,
				scanner.now().UTC(),
			); err != nil {
				return result, fmt.Errorf("mall weather feishu upsert scan: persist remote mappings: %w", err)
			}
			result.MappedRows += int64(len(mappings))
		}
		startRow = endRow + 1
	}
	initializedAt := scanner.now().UTC()
	if initializedAt.IsZero() {
		return result, errors.New("mall weather feishu upsert scan: invalid initialization time")
	}
	if err := scanner.mappings.MarkInitialized(
		ctx,
		request.Destination.DestinationID,
		request.Dataset.Kind,
		sheetEnv,
		schemaChecksum,
		initializedAt,
	); err != nil {
		return result, fmt.Errorf("mall weather feishu upsert scan: mark initialized: %w", err)
	}
	result.Initialized = true
	return result, nil
}

func validateMallWeatherFeishuUpsertScanRequest(
	ctx context.Context,
	scanner *mallWeatherFeishuUpsertScanner,
	request mallWeatherFeishuUpsertScanRequest,
) ([]requestbody.MallWeatherExportColumn, []string, string, string, error) {
	if ctx == nil || scanner == nil || scanner.sheets == nil || scanner.mappings == nil || scanner.now == nil ||
		request.Destination == nil || request.Destination.DestinationID == 0 ||
		request.Destination.Config.WriteMode != "upsert" || request.Destination.SpreadsheetToken == "" ||
		request.Destination.Config.BatchRows < 1 ||
		request.Destination.Config.BatchRows > maxMallWeatherFeishuBatchRows || request.Dataset.Kind == "" ||
		request.Dataset.SplitBy != "" || request.GridRows < 1 || request.GridRows > maxMallWeatherFeishuSheetRow {
		return nil, nil, "", "", errors.New("mall weather feishu upsert scan: invalid request")
	}
	columns, err := mallWeatherExportRenderColumns(request.Dataset)
	uniqueKeys := request.Destination.Config.UniqueKeyFields[request.Dataset.Kind]
	sheetID := request.Destination.SheetIDs[request.Dataset.Kind]
	sheetEnv := request.Destination.Config.SheetIDEnvMapping[request.Dataset.Kind]
	if err != nil || len(columns) == 0 || len(columns) > maxMallWeatherFeishuColumns || len(uniqueKeys) == 0 ||
		sheetID == "" || sheetEnv == "" {
		return nil, nil, "", "", errors.New("mall weather feishu upsert scan: invalid dataset mapping")
	}
	if err := validateMallWeatherFeishuPlannedUniqueKeys("upsert", columns, uniqueKeys); err != nil {
		return nil, nil, "", "", errors.New("mall weather feishu upsert scan: invalid unique keys")
	}
	return columns, uniqueKeys, sheetID, sheetEnv, nil
}

func normalizeMallWeatherFeishuScannedRows(
	values *feishu.SheetValues,
	startRow int64,
	endRow int64,
	columns int,
) ([][]feishu.SheetCell, []int64, error) {
	if values == nil || startRow < 2 || endRow < startRow || columns < 1 || columns > maxMallWeatherFeishuColumns ||
		int64(len(values.Rows)) > endRow-startRow+1 {
		return nil, nil, errors.New("mall weather feishu upsert scan: invalid range response")
	}
	rows := make([][]feishu.SheetCell, 0, len(values.Rows))
	rowNumbers := make([]int64, 0, len(values.Rows))
	for index, source := range values.Rows {
		if len(source) > columns {
			return nil, nil, errors.New("mall weather feishu upsert scan: remote row is too wide")
		}
		row := make([]feishu.SheetCell, columns)
		for column := range row {
			row[column].Type = feishu.SheetCellBlank
		}
		copy(row, source)
		blank, err := mallWeatherFeishuScannedRowIsBlank(row)
		if err != nil {
			return nil, nil, err
		}
		if blank {
			continue
		}
		if mallWeatherFeishuFirstCellIsBlank(row) {
			return nil, nil, errors.New("mall weather feishu upsert scan: non-empty row has a blank first cell")
		}
		rows = append(rows, row)
		rowNumbers = append(rowNumbers, startRow+int64(index))
	}
	return rows, rowNumbers, nil
}

func mallWeatherFeishuScannedRowIsBlank(row []feishu.SheetCell) (bool, error) {
	for _, cell := range row {
		canonical, err := canonicalMallWeatherFeishuCell(cell)
		if err != nil {
			return false, errors.New("mall weather feishu upsert scan: invalid remote cell")
		}
		if canonical.Type != feishu.SheetCellBlank {
			return false, nil
		}
	}
	return true, nil
}
