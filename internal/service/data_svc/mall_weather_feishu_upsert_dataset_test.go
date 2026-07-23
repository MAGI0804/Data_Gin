package data_svc

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/model"
)

func TestMallWeatherFeishuUpsertDatasetRunnerSkipsUpdatesAndAppends(t *testing.T) {
	t.Parallel()
	request := validMallWeatherFeishuUpsertDatasetRequest()
	page := &data_dao.MallWeatherExportDataPage{
		Rows: []data_dao.MallWeatherExportDataRow{
			mallWeatherFeishuUpsertDataRow(1, "M001", "2026-07-23T10:00:00Z", "sunny"),
			mallWeatherFeishuUpsertDataRow(2, "M002", "2026-07-23T10:00:00Z", "rain"),
			mallWeatherFeishuUpsertDataRow(3, "M003", "2026-07-23T10:00:00Z", "cloudy"),
		},
		NextAfterID: 3,
	}
	rendered := renderTestMallWeatherFeishuUpsertRows(t, request, page.Rows)
	mappings := &fakeMallWeatherFeishuUpsertDatasetMappings{existing: map[string]model.MallWeatherSheetRow{
		rendered[0].BusinessKey: {
			BaseModel: model.BaseModel{ID: 1}, DestinationID: 17, DatasetKind: "hourly",
			BusinessKey: rendered[0].BusinessKey, RowNumber: 2, Checksum: rendered[0].Checksum,
			LastSyncedAt: time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC),
		},
		rendered[1].BusinessKey: {
			BaseModel: model.BaseModel{ID: 2}, DestinationID: 17, DatasetKind: "hourly",
			BusinessKey: rendered[1].BusinessKey, RowNumber: 5, Checksum: rendered[0].Checksum,
			LastSyncedAt: time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC),
		},
	}}
	batches := &fakeMallWeatherFeishuUpsertDatasetBatchRunner{}
	runner := newTestMallWeatherFeishuUpsertDatasetRunner(
		t,
		&fakeMallWeatherFeishuAppendDatasetPager{pages: []*data_dao.MallWeatherExportDataPage{page}},
		&fakeMallWeatherFeishuUpsertDatasetScanner{result: mallWeatherFeishuUpsertScanResult{
			Initialized: true, LastDataRow: 5,
		}},
		mappings,
		batches,
	)

	result, err := runner.Run(t.Context(), request)
	if err != nil {
		t.Fatalf("Run() error=%v", err)
	}
	if result.BatchCount != 2 || result.RecordCount != 3 || result.SkippedCount != 1 ||
		result.UpdatedCount != 1 || result.AppendedCount != 1 || result.LastRemoteRow != 6 ||
		len(batches.requests) != 2 || len(mappings.upserted) != 2 ||
		mappings.markUninitializedCalls != 1 || mappings.markInitializedCalls != 1 {
		t.Fatalf("result=%+v batches=%+v mappings=%+v", result, batches.requests, mappings)
	}
	if batches.requests[0].Mode != "update" || batches.requests[0].Rows[0].RowNumber != 5 ||
		batches.requests[1].Mode != "append" || batches.requests[1].Rows[0].RowNumber != 6 {
		t.Fatalf("batch requests=%+v", batches.requests)
	}
	if len(mappings.events) != 5 || mappings.events[0] != "find" || mappings.events[1] != "unmark" ||
		mappings.events[2] != "upsert" || mappings.events[3] != "upsert" || mappings.events[4] != "mark" {
		t.Fatalf("events=%v", mappings.events)
	}
}

func TestMallWeatherFeishuUpsertDatasetRunnerLeavesMarkerWhenAllRowsSkip(t *testing.T) {
	t.Parallel()
	request := validMallWeatherFeishuUpsertDatasetRequest()
	page := &data_dao.MallWeatherExportDataPage{
		Rows: []data_dao.MallWeatherExportDataRow{
			mallWeatherFeishuUpsertDataRow(1, "M001", "2026-07-23T10:00:00Z", "sunny"),
		},
		NextAfterID: 1,
	}
	rendered := renderTestMallWeatherFeishuUpsertRows(t, request, page.Rows)
	mappings := &fakeMallWeatherFeishuUpsertDatasetMappings{existing: map[string]model.MallWeatherSheetRow{
		rendered[0].BusinessKey: {
			BaseModel: model.BaseModel{ID: 1}, DestinationID: 17, DatasetKind: "hourly",
			BusinessKey: rendered[0].BusinessKey, RowNumber: 2, Checksum: rendered[0].Checksum,
			LastSyncedAt: time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC),
		},
	}}
	batches := &fakeMallWeatherFeishuUpsertDatasetBatchRunner{}
	runner := newTestMallWeatherFeishuUpsertDatasetRunner(
		t,
		&fakeMallWeatherFeishuAppendDatasetPager{pages: []*data_dao.MallWeatherExportDataPage{page}},
		&fakeMallWeatherFeishuUpsertDatasetScanner{result: mallWeatherFeishuUpsertScanResult{
			Initialized: true, LastDataRow: 2,
		}},
		mappings,
		batches,
	)

	result, err := runner.Run(t.Context(), request)
	if err != nil || result.BatchCount != 0 || result.SkippedCount != 1 || len(batches.requests) != 0 ||
		mappings.markUninitializedCalls != 0 || mappings.markInitializedCalls != 0 || len(mappings.upserted) != 0 {
		t.Fatalf("Run() result=%+v error=%v batches=%+v mappings=%+v", result, err, batches.requests, mappings)
	}
}

func TestMallWeatherFeishuUpsertDatasetRunnerFailsUnknownWhenMappingPersistFails(t *testing.T) {
	t.Parallel()
	request := validMallWeatherFeishuUpsertDatasetRequest()
	page := &data_dao.MallWeatherExportDataPage{
		Rows: []data_dao.MallWeatherExportDataRow{
			mallWeatherFeishuUpsertDataRow(1, "M001", "2026-07-23T10:00:00Z", "sunny"),
		},
		NextAfterID: 1,
	}
	mappings := &fakeMallWeatherFeishuUpsertDatasetMappings{
		existing:  map[string]model.MallWeatherSheetRow{},
		upsertErr: errors.New("database unavailable"),
	}
	batches := &fakeMallWeatherFeishuUpsertDatasetBatchRunner{}
	runner := newTestMallWeatherFeishuUpsertDatasetRunner(
		t,
		&fakeMallWeatherFeishuAppendDatasetPager{pages: []*data_dao.MallWeatherExportDataPage{page}},
		&fakeMallWeatherFeishuUpsertDatasetScanner{result: mallWeatherFeishuUpsertScanResult{
			Initialized: true, LastDataRow: 1,
		}},
		mappings,
		batches,
	)

	result, err := runner.Run(t.Context(), request)
	if !errors.Is(err, ErrMallWeatherFeishuUpsertStateUnknown) || result.BatchCount != 0 ||
		len(batches.requests) != 1 || mappings.markUninitializedCalls != 1 || mappings.markInitializedCalls != 0 {
		t.Fatalf("Run() result=%+v error=%v batches=%+v mappings=%+v", result, err, batches.requests, mappings)
	}
}

func TestMallWeatherFeishuUpsertDatasetRunnerSplitsAppendRows(t *testing.T) {
	t.Parallel()
	request := validMallWeatherFeishuUpsertDatasetRequest()
	request.Destination.Config.BatchRows = 2
	page := &data_dao.MallWeatherExportDataPage{
		Rows: []data_dao.MallWeatherExportDataRow{
			mallWeatherFeishuUpsertDataRow(1, "M001", "2026-07-23T10:00:00Z", "sunny"),
			mallWeatherFeishuUpsertDataRow(2, "M002", "2026-07-23T10:00:00Z", "rain"),
			mallWeatherFeishuUpsertDataRow(3, "M003", "2026-07-23T10:00:00Z", "cloudy"),
		},
		NextAfterID: 3,
	}
	batches := &fakeMallWeatherFeishuUpsertDatasetBatchRunner{}
	runner := newTestMallWeatherFeishuUpsertDatasetRunner(
		t,
		&fakeMallWeatherFeishuAppendDatasetPager{pages: []*data_dao.MallWeatherExportDataPage{page}},
		&fakeMallWeatherFeishuUpsertDatasetScanner{result: mallWeatherFeishuUpsertScanResult{
			Initialized: true, LastDataRow: 1,
		}},
		&fakeMallWeatherFeishuUpsertDatasetMappings{existing: map[string]model.MallWeatherSheetRow{}},
		batches,
	)

	result, err := runner.Run(t.Context(), request)
	if err != nil || result.BatchCount != 2 || result.AppendedCount != 3 || len(batches.requests) != 2 ||
		len(batches.requests[0].Rows) != 2 || batches.requests[0].Rows[0].RowNumber != 2 ||
		batches.requests[0].Rows[1].RowNumber != 3 || len(batches.requests[1].Rows) != 1 ||
		batches.requests[1].Rows[0].RowNumber != 4 {
		t.Fatalf("Run() result=%+v error=%v batches=%+v", result, err, batches.requests)
	}
}

func TestMallWeatherFeishuUpsertWriteBatchesSplitsSparseUpdatesByRangeLimit(t *testing.T) {
	t.Parallel()
	rows := make([]mallWeatherFeishuUpsertWriteRow, maxMallWeatherFeishuUpsertUpdateRanges+1)
	for index := range rows {
		rows[index] = mallWeatherFeishuUpsertWriteRow{
			BusinessKey: "sha256:" + strings.Repeat(string(rune('a'+index%26)), 64),
			RowNumber:   int64(2 + index*2),
			Checksum:    strings.Repeat("a", 64),
			Cells:       testMallWeatherFeishuUpsertCells("M001", "2026-07-23T10:00:00Z", "sunny"),
		}
	}

	batches, err := mallWeatherFeishuUpsertWriteBatches("update", rows, 3, maxMallWeatherFeishuBatchRows)
	if err != nil || len(batches) != 2 || len(batches[0]) != maxMallWeatherFeishuUpsertUpdateRanges ||
		len(batches[1]) != 1 {
		t.Fatalf("batches=%+v error=%v", batches, err)
	}
}

func newTestMallWeatherFeishuUpsertDatasetRunner(
	t *testing.T,
	pager mallWeatherFeishuDatasetPager,
	scanner mallWeatherFeishuUpsertScannerRunner,
	mappings mallWeatherFeishuUpsertMappingWriter,
	batches mallWeatherFeishuUpsertBatchRunner,
) *mallWeatherFeishuUpsertDatasetRunner {
	t.Helper()
	runner, err := newMallWeatherFeishuUpsertDatasetRunner(
		pager,
		scanner,
		mappings,
		batches,
		func() time.Time { return time.Date(2026, 7, 23, 14, 0, 0, 0, time.UTC) },
	)
	if err != nil {
		t.Fatalf("newMallWeatherFeishuUpsertDatasetRunner() error=%v", err)
	}
	return runner
}

func validMallWeatherFeishuUpsertDatasetRequest() mallWeatherFeishuUpsertDatasetRequest {
	destination := testMallWeatherFeishuUpsertDestination()
	destination.Config.BatchRows = 10
	dataset := requestbody.MallWeatherExportDataset{
		Kind: "hourly", SheetName: "Hourly", Columns: testMallWeatherFeishuUpsertColumns(),
	}
	profile := MallWeatherExportProfileDTO{
		ID: 9, Code: destination.Config.ProfileCode, Version: 3, Enabled: true,
		TimeZone: "Asia/Shanghai", UnitSystem: "metric", DateFormat: "2006-01-02",
		DateTimeFormat: "2006-01-02 15:04:05", Datasets: []requestbody.MallWeatherExportDataset{dataset},
	}
	return mallWeatherFeishuUpsertDatasetRequest{
		RunID: 23, TraceID: "11111111-1111-4111-8111-111111111111",
		Destination: destination, Profile: profile, Dataset: dataset,
		SnapshotAt: time.Date(2026, 7, 23, 6, 0, 0, 0, time.UTC), GridRows: 10,
	}
}

func mallWeatherFeishuUpsertDataRow(cursor uint, mallCode, forecastTime, skycon string) data_dao.MallWeatherExportDataRow {
	return data_dao.MallWeatherExportDataRow{CursorID: cursor, Values: map[string]interface{}{
		"mall_code": mallCode, "forecast_time": forecastTime, "skycon": skycon,
	}}
}

func renderTestMallWeatherFeishuUpsertRows(
	t *testing.T,
	request mallWeatherFeishuUpsertDatasetRequest,
	rows []data_dao.MallWeatherExportDataRow,
) []mallWeatherFeishuUpsertRenderedRow {
	t.Helper()
	batch, err := renderMallWeatherFeishuBatch(request.Profile, request.Dataset, rows)
	if err != nil {
		t.Fatalf("renderMallWeatherFeishuBatch() error=%v", err)
	}
	rendered, err := buildMallWeatherFeishuUpsertRows(
		testMallWeatherFeishuUpsertColumns(),
		request.Destination.Config.UniqueKeyFields[request.Dataset.Kind],
		batch,
	)
	if err != nil {
		t.Fatalf("buildMallWeatherFeishuUpsertRows() error=%v", err)
	}
	return rendered
}

type fakeMallWeatherFeishuUpsertDatasetScanner struct {
	result   mallWeatherFeishuUpsertScanResult
	err      error
	requests []mallWeatherFeishuUpsertScanRequest
}

func (scanner *fakeMallWeatherFeishuUpsertDatasetScanner) Ensure(
	_ context.Context,
	request mallWeatherFeishuUpsertScanRequest,
) (mallWeatherFeishuUpsertScanResult, error) {
	scanner.requests = append(scanner.requests, request)
	return scanner.result, scanner.err
}

type fakeMallWeatherFeishuUpsertDatasetMappings struct {
	existing                 map[string]model.MallWeatherSheetRow
	upsertErr                error
	findRequests             [][]string
	upserted                 [][]data_dao.MallWeatherSheetRowMapping
	events                   []string
	markUninitializedCalls   int
	markInitializedCalls     int
	lastInitializationSchema string
}

func (store *fakeMallWeatherFeishuUpsertDatasetMappings) FindByBusinessKeys(
	_ context.Context,
	_ uint,
	_ string,
	keys []string,
) (map[string]model.MallWeatherSheetRow, error) {
	store.findRequests = append(store.findRequests, append([]string(nil), keys...))
	store.events = append(store.events, "find")
	result := make(map[string]model.MallWeatherSheetRow, len(keys))
	for _, key := range keys {
		if row, ok := store.existing[key]; ok {
			result[key] = row
		}
	}
	return result, nil
}

func (store *fakeMallWeatherFeishuUpsertDatasetMappings) UpsertMappings(
	_ context.Context,
	_ uint,
	_ string,
	_ string,
	mappings []data_dao.MallWeatherSheetRowMapping,
	_ time.Time,
) error {
	store.events = append(store.events, "upsert")
	store.upserted = append(store.upserted, append([]data_dao.MallWeatherSheetRowMapping(nil), mappings...))
	return store.upsertErr
}

func (store *fakeMallWeatherFeishuUpsertDatasetMappings) MarkUninitialized(context.Context, uint, string) error {
	store.markUninitializedCalls++
	store.events = append(store.events, "unmark")
	return nil
}

func (store *fakeMallWeatherFeishuUpsertDatasetMappings) MarkInitialized(
	_ context.Context,
	_ uint,
	_ string,
	_ string,
	schemaChecksum string,
	_ time.Time,
) error {
	store.markInitializedCalls++
	store.lastInitializationSchema = schemaChecksum
	store.events = append(store.events, "mark")
	return nil
}

type fakeMallWeatherFeishuUpsertDatasetBatchRunner struct {
	requests []mallWeatherFeishuUpsertBatchRequest
	err      error
}

func (runner *fakeMallWeatherFeishuUpsertDatasetBatchRunner) Execute(
	_ context.Context,
	request mallWeatherFeishuUpsertBatchRequest,
) (mallWeatherFeishuUpsertBatchResult, error) {
	runner.requests = append(runner.requests, request)
	if runner.err != nil {
		return mallWeatherFeishuUpsertBatchResult{}, runner.err
	}
	return mallWeatherFeishuUpsertBatchResult{
		Mode:        request.Mode,
		BatchNo:     request.BatchNo,
		RecordCount: len(request.Rows),
		CellCount:   len(request.Rows) * len(request.Rows[0].Cells),
		Rows:        append([]mallWeatherFeishuUpsertWriteRow(nil), request.Rows...),
	}, nil
}
