package data_svc

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"gin-biz-web-api/connector/feishu"
	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/model"

	"gorm.io/gorm"
)

func TestMallWeatherFeishuAppendDatasetRunnerPagesStableSnapshot(t *testing.T) {
	t.Parallel()
	request := validMallWeatherFeishuAppendDatasetRequest()
	latest := true
	request.Dataset.Latest = &latest
	request.Dataset.AsOf = "2026-07-23T03:00:00+08:00"
	request.Profile.Datasets[0] = request.Dataset
	request.Destination.Config.BatchRows = 2
	request.GridRows = 3
	pager := &fakeMallWeatherFeishuAppendDatasetPager{pages: []*data_dao.MallWeatherExportDataPage{
		{
			Rows: []data_dao.MallWeatherExportDataRow{
				mallWeatherFeishuAppendDataRow(1, "mall-a", 20),
				mallWeatherFeishuAppendDataRow(2, "mall-b", 21),
			},
			NextAfterID: 2,
			HasMore:     true,
		},
		{
			Rows:        []data_dao.MallWeatherExportDataRow{mallWeatherFeishuAppendDataRow(3, "mall-c", 22)},
			NextAfterID: 3,
		},
	}}
	sheets := &fakeMallWeatherFeishuAppendDatasetSheets{responses: []*feishu.SheetValues{{Rows: [][]feishu.SheetCell{
		{{Type: feishu.SheetCellString, Text: "occupied-a"}},
		{{Type: feishu.SheetCellString, Text: "occupied-b"}},
	}}}}
	checkpoints := &fakeMallWeatherFeishuAppendCheckpointStore{}
	batches := &fakeMallWeatherFeishuAppendBatchRunner{}
	runner := newTestMallWeatherFeishuAppendDatasetRunner(t, pager, sheets, checkpoints, batches)

	result, err := runner.Run(t.Context(), request)
	if err != nil {
		t.Fatalf("Run() error=%v", err)
	}
	if result.DatasetKind != "hourly" || result.BatchCount != 2 || result.RecordCount != 3 ||
		result.CellCount != 6 || result.LastCursor != 3 || result.LastRemoteRow != 6 || len(batches.requests) != 2 {
		t.Fatalf("result=%+v batches=%+v", result, batches.requests)
	}
	if batches.requests[0].BatchNo != 1 || batches.requests[0].Attempt != 1 || batches.requests[0].RowStart != 4 ||
		batches.requests[1].BatchNo != 2 || batches.requests[1].Attempt != 1 || batches.requests[1].RowStart != 6 {
		t.Fatalf("batch requests=%+v", batches.requests)
	}
	if len(pager.requests) != 2 || pager.requests[0].AfterID != 0 || pager.requests[1].AfterID != 2 ||
		pager.requests[0].Limit != 2 || !pager.requests[0].Latest || pager.requests[0].AsOfUTC == nil ||
		!pager.requests[0].AsOfUTC.Equal(time.Date(2026, 7, 22, 19, 0, 0, 0, time.UTC)) ||
		!pager.requests[0].SnapshotAt.Equal(request.SnapshotAt.UTC()) ||
		!slices.Equal(pager.requests[0].Fields, []string{"mall_code", "temperature_c"}) {
		t.Fatalf("page requests=%+v", pager.requests)
	}
}

func TestMallWeatherFeishuAppendDatasetRunnerRecoversCheckpoints(t *testing.T) {
	t.Parallel()
	request := validMallWeatherFeishuAppendDatasetRequest()
	request.Destination.Config.BatchRows = 2
	request.GridRows = 7
	pages := []*data_dao.MallWeatherExportDataPage{
		{Rows: []data_dao.MallWeatherExportDataRow{
			mallWeatherFeishuAppendDataRow(1, "mall-a", 20),
			mallWeatherFeishuAppendDataRow(2, "mall-b", 21),
		}, NextAfterID: 2, HasMore: true},
		{Rows: []data_dao.MallWeatherExportDataRow{
			mallWeatherFeishuAppendDataRow(3, "mall-c", 22),
			mallWeatherFeishuAppendDataRow(4, "mall-d", 23),
		}, NextAfterID: 4, HasMore: true},
		{Rows: []data_dao.MallWeatherExportDataRow{
			mallWeatherFeishuAppendDataRow(5, "mall-e", 24),
			mallWeatherFeishuAppendDataRow(6, "mall-f", 25),
		}, NextAfterID: 6},
	}
	firstBatch := renderTestMallWeatherFeishuAppendBatch(t, request, pages[0].Rows)
	secondBatch := renderTestMallWeatherFeishuAppendBatch(t, request, pages[1].Rows)
	thirdBatch := renderTestMallWeatherFeishuAppendBatch(t, request, pages[2].Rows)
	checkpoints := &fakeMallWeatherFeishuAppendCheckpointStore{logs: map[int]*model.DeliveryLog{
		1: mallWeatherFeishuAppendCheckpoint(request, 1, 1, "success", 2, firstBatch),
		2: mallWeatherFeishuAppendCheckpoint(request, 2, 1, "unknown", 4, secondBatch),
		3: mallWeatherFeishuAppendCheckpoint(request, 3, 2, "failed", 6, thirdBatch),
	}}
	scanRows := make([][]feishu.SheetCell, 6)
	for index := range scanRows {
		scanRows[index] = []feishu.SheetCell{{Type: feishu.SheetCellString, Text: "occupied"}}
	}
	sheets := &fakeMallWeatherFeishuAppendDatasetSheets{responses: []*feishu.SheetValues{
		{Rows: scanRows},
		{Rows: secondBatch.Rows},
	}}
	pager := &fakeMallWeatherFeishuAppendDatasetPager{pages: pages}
	batches := &fakeMallWeatherFeishuAppendBatchRunner{}
	runner := newTestMallWeatherFeishuAppendDatasetRunner(t, pager, sheets, checkpoints, batches)

	result, err := runner.Run(t.Context(), request)
	if err != nil {
		t.Fatalf("Run() error=%v", err)
	}
	if result.BatchCount != 3 || result.RecordCount != 6 || result.LastCursor != 6 || result.LastRemoteRow != 9 ||
		len(checkpoints.reconciled) != 1 || checkpoints.reconciled[0].id != checkpoints.logs[2].ID ||
		len(batches.requests) != 1 || batches.requests[0].BatchNo != 3 || batches.requests[0].Attempt != 3 ||
		batches.requests[0].RowStart != 8 {
		t.Fatalf("result=%+v checkpoints=%+v batches=%+v", result, checkpoints, batches.requests)
	}
	if len(sheets.ranges) != 2 || sheets.ranges[1].StartRow != 4 || sheets.ranges[1].EndRow != 5 {
		t.Fatalf("read ranges=%+v", sheets.ranges)
	}
}

func TestMallWeatherFeishuAppendDatasetRunnerFailsClosedOnCheckpointConflict(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		status        string
		checksum      string
		recoveryRows  [][]feishu.SheetCell
		wantReadCount int
	}{
		{name: "successful checksum changed", status: "success", checksum: strings.Repeat("f", 64), wantReadCount: 1},
		{name: "uncertain range mismatches", status: "unknown", recoveryRows: [][]feishu.SheetCell{{
			{Type: feishu.SheetCellString, Text: "different"},
			{Type: feishu.SheetCellNumber, Number: "20"},
		}}, wantReadCount: 2},
		{name: "unknown checkpoint status", status: "queued", wantReadCount: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := validMallWeatherFeishuAppendDatasetRequest()
			request.GridRows = 2
			page := &data_dao.MallWeatherExportDataPage{
				Rows:        []data_dao.MallWeatherExportDataRow{mallWeatherFeishuAppendDataRow(1, "mall-a", 20)},
				NextAfterID: 1,
			}
			batch := renderTestMallWeatherFeishuAppendBatch(t, request, page.Rows)
			checkpoint := mallWeatherFeishuAppendCheckpoint(request, 1, 1, test.status, 2, batch)
			if test.checksum != "" {
				checkpoint.RequestChecksum = test.checksum
			}
			responses := []*feishu.SheetValues{{Rows: [][]feishu.SheetCell{{
				{Type: feishu.SheetCellString, Text: "occupied"},
			}}}}
			if test.recoveryRows != nil {
				responses = append(responses, &feishu.SheetValues{Rows: test.recoveryRows})
			}
			sheets := &fakeMallWeatherFeishuAppendDatasetSheets{responses: responses}
			checkpoints := &fakeMallWeatherFeishuAppendCheckpointStore{logs: map[int]*model.DeliveryLog{1: checkpoint}}
			batches := &fakeMallWeatherFeishuAppendBatchRunner{}
			runner := newTestMallWeatherFeishuAppendDatasetRunner(
				t,
				&fakeMallWeatherFeishuAppendDatasetPager{pages: []*data_dao.MallWeatherExportDataPage{page}},
				sheets,
				checkpoints,
				batches,
			)

			result, err := runner.Run(t.Context(), request)
			if !errors.Is(err, ErrMallWeatherFeishuAppendCheckpointConflict) || result.BatchCount != 0 ||
				len(batches.requests) != 0 || len(checkpoints.reconciled) != 0 || len(sheets.ranges) != test.wantReadCount {
				t.Fatalf("Run() result=%+v error=%v sheets=%+v checkpoints=%+v batches=%+v", result, err, sheets, checkpoints, batches)
			}
		})
	}
}

func TestMallWeatherFeishuAppendDatasetRunnerDoesNotAdvanceFailedBatch(t *testing.T) {
	t.Parallel()
	request := validMallWeatherFeishuAppendDatasetRequest()
	request.GridRows = 1
	pager := &fakeMallWeatherFeishuAppendDatasetPager{pages: []*data_dao.MallWeatherExportDataPage{{
		Rows:        []data_dao.MallWeatherExportDataRow{mallWeatherFeishuAppendDataRow(1, "mall-a", 20)},
		NextAfterID: 1,
	}}}
	batches := &fakeMallWeatherFeishuAppendBatchRunner{err: errors.New("append unavailable")}
	runner := newTestMallWeatherFeishuAppendDatasetRunner(
		t,
		pager,
		&fakeMallWeatherFeishuAppendDatasetSheets{},
		&fakeMallWeatherFeishuAppendCheckpointStore{},
		batches,
	)
	result, err := runner.Run(t.Context(), request)
	if err == nil || result.BatchCount != 0 || result.RecordCount != 0 || result.LastCursor != 0 ||
		len(batches.requests) != 1 {
		t.Fatalf("Run() result=%+v error=%v batches=%+v", result, err, batches.requests)
	}
}

func TestMallWeatherFeishuAppendDatasetRunnerRejectsInvalidBatchResult(t *testing.T) {
	t.Parallel()
	request := validMallWeatherFeishuAppendDatasetRequest()
	request.GridRows = 1
	pager := &fakeMallWeatherFeishuAppendDatasetPager{pages: []*data_dao.MallWeatherExportDataPage{{
		Rows:        []data_dao.MallWeatherExportDataRow{mallWeatherFeishuAppendDataRow(1, "mall-a", 20)},
		NextAfterID: 1,
	}}}
	batches := &fakeMallWeatherFeishuAppendBatchRunner{rowEndOffset: 1}
	runner := newTestMallWeatherFeishuAppendDatasetRunner(
		t,
		pager,
		&fakeMallWeatherFeishuAppendDatasetSheets{},
		&fakeMallWeatherFeishuAppendCheckpointStore{},
		batches,
	)
	result, err := runner.Run(t.Context(), request)
	if err == nil || result.BatchCount != 0 || result.LastCursor != 0 || len(batches.requests) != 1 {
		t.Fatalf("Run() result=%+v error=%v batches=%+v", result, err, batches.requests)
	}
}

func TestMallWeatherFeishuAppendDatasetRunnerRejectsInvalidRequestBeforeSideEffects(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		mutate func(*mallWeatherFeishuAppendDatasetRequest)
	}{
		{name: "destination missing", mutate: func(request *mallWeatherFeishuAppendDatasetRequest) {
			request.Destination = nil
		}},
		{name: "dataset differs from profile snapshot", mutate: func(request *mallWeatherFeishuAppendDatasetRequest) {
			request.Dataset.SheetName = "different"
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := validMallWeatherFeishuAppendDatasetRequest()
			test.mutate(&request)
			pager := &fakeMallWeatherFeishuAppendDatasetPager{}
			sheets := &fakeMallWeatherFeishuAppendDatasetSheets{}
			batches := &fakeMallWeatherFeishuAppendBatchRunner{}
			runner := newTestMallWeatherFeishuAppendDatasetRunner(
				t,
				pager,
				sheets,
				&fakeMallWeatherFeishuAppendCheckpointStore{},
				batches,
			)
			if _, err := runner.Run(t.Context(), request); err == nil || len(pager.requests) != 0 ||
				len(sheets.ranges) != 0 || len(batches.requests) != 0 {
				t.Fatalf("Run() error=%v pager=%+v sheets=%+v batches=%+v", err, pager, sheets, batches)
			}
		})
	}
}

type fakeMallWeatherFeishuAppendDatasetPager struct {
	pages    []*data_dao.MallWeatherExportDataPage
	err      error
	requests []data_dao.MallWeatherExportDataPageRequest
}

func (pager *fakeMallWeatherFeishuAppendDatasetPager) Page(
	_ context.Context,
	request data_dao.MallWeatherExportDataPageRequest,
) (*data_dao.MallWeatherExportDataPage, error) {
	pager.requests = append(pager.requests, request)
	if pager.err != nil {
		return nil, pager.err
	}
	if len(pager.pages) == 0 {
		return nil, errors.New("unexpected dataset page")
	}
	page := pager.pages[0]
	pager.pages = pager.pages[1:]
	return page, nil
}

type fakeMallWeatherFeishuAppendDatasetSheets struct {
	responses []*feishu.SheetValues
	err       error
	ranges    []feishu.SheetRange
}

func (sheets *fakeMallWeatherFeishuAppendDatasetSheets) ReadRange(
	_ context.Context,
	_ string,
	readRange feishu.SheetRange,
) (*feishu.SheetValues, error) {
	sheets.ranges = append(sheets.ranges, readRange)
	if sheets.err != nil {
		return nil, sheets.err
	}
	if len(sheets.responses) == 0 {
		return nil, errors.New("unexpected range read")
	}
	response := sheets.responses[0]
	sheets.responses = sheets.responses[1:]
	return response, nil
}

type fakeMallWeatherFeishuAppendCheckpointStore struct {
	logs         map[int]*model.DeliveryLog
	findErr      error
	reconcileErr error
	reconciled   []mallWeatherFeishuAppendReconciliation
}

type mallWeatherFeishuAppendReconciliation struct {
	id       uint
	checksum string
	rowStart int64
	rowEnd   int64
}

func (store *fakeMallWeatherFeishuAppendCheckpointStore) FindLatestWeatherBatch(
	_ context.Context,
	_ uint,
	_ uint,
	_ string,
	batchNo int,
) (*model.DeliveryLog, error) {
	if store.findErr != nil {
		return nil, store.findErr
	}
	checkpoint := store.logs[batchNo]
	if checkpoint == nil {
		return nil, gorm.ErrRecordNotFound
	}
	copy := *checkpoint
	return &copy, nil
}

func (store *fakeMallWeatherFeishuAppendCheckpointStore) ReconcileWeatherBatchSuccess(
	_ context.Context,
	id uint,
	checksum string,
	rowStart int64,
	rowEnd int64,
	_ time.Time,
) error {
	store.reconciled = append(store.reconciled, mallWeatherFeishuAppendReconciliation{
		id: id, checksum: checksum, rowStart: rowStart, rowEnd: rowEnd,
	})
	return store.reconcileErr
}

type fakeMallWeatherFeishuAppendBatchRunner struct {
	requests     []mallWeatherFeishuAppendBatchRequest
	err          error
	rowEndOffset int64
}

func (runner *fakeMallWeatherFeishuAppendBatchRunner) Execute(
	_ context.Context,
	request mallWeatherFeishuAppendBatchRequest,
) (mallWeatherFeishuAppendBatchResult, error) {
	runner.requests = append(runner.requests, request)
	if runner.err != nil {
		return mallWeatherFeishuAppendBatchResult{}, runner.err
	}
	return mallWeatherFeishuAppendBatchResult{
		BatchNo: request.BatchNo, RowStart: request.RowStart,
		RowEnd:      request.RowStart + int64(len(request.Batch.Rows)) - 1 + runner.rowEndOffset,
		RecordCount: len(request.Batch.Rows), CellCount: len(request.Batch.Rows) * len(request.Batch.Rows[0]),
	}, nil
}

func newTestMallWeatherFeishuAppendDatasetRunner(
	t *testing.T,
	pager mallWeatherFeishuDatasetPager,
	sheets mallWeatherFeishuRangeReader,
	checkpoints mallWeatherFeishuBatchCheckpointStore,
	batches mallWeatherFeishuAppendBatchRunner,
) *mallWeatherFeishuAppendDatasetRunner {
	t.Helper()
	runner, err := newMallWeatherFeishuAppendDatasetRunner(
		pager,
		sheets,
		checkpoints,
		batches,
		func() time.Time { return time.Date(2026, 7, 23, 4, 0, 0, 0, time.UTC) },
	)
	if err != nil {
		t.Fatalf("newMallWeatherFeishuAppendDatasetRunner() error=%v", err)
	}
	return runner
}

func validMallWeatherFeishuAppendDatasetRequest() mallWeatherFeishuAppendDatasetRequest {
	input := validMallWeatherFeishuDryRunPlanInput()
	input.Destination.Config.WriteMode = "append"
	input.Destination.Config.UniqueKeyFields = nil
	input.Destination.Config.BatchRows = 2
	input.Profile.Code = input.Destination.Config.ProfileCode
	input.Profile.TimeZone = "Asia/Shanghai"
	input.Profile.UnitSystem = "metric"
	input.Profile.DateFormat = "2006-01-02"
	input.Profile.DateTimeFormat = "2006-01-02 15:04:05"
	dataset := requestbody.MallWeatherExportDataset{Kind: "hourly", Columns: []requestbody.MallWeatherExportColumn{
		{Field: "mall_code", Format: "text"},
		{Field: "temperature_c", Format: "decimal"},
	}}
	input.Profile.Datasets = []requestbody.MallWeatherExportDataset{dataset}
	return mallWeatherFeishuAppendDatasetRequest{
		RunID: 23, TraceID: "11111111-1111-4111-8111-111111111111",
		Destination: input.Destination, Profile: input.Profile, Dataset: dataset,
		SnapshotAt: time.Date(2026, 7, 23, 3, 30, 0, 0, time.UTC), GridRows: 1,
	}
}

func mallWeatherFeishuAppendDataRow(cursor uint, mallCode string, temperature float64) data_dao.MallWeatherExportDataRow {
	return data_dao.MallWeatherExportDataRow{CursorID: cursor, Values: map[string]interface{}{
		"mall_code": mallCode, "temperature_c": temperature,
	}}
}

func renderTestMallWeatherFeishuAppendBatch(
	t *testing.T,
	request mallWeatherFeishuAppendDatasetRequest,
	rows []data_dao.MallWeatherExportDataRow,
) mallWeatherFeishuRenderedBatch {
	t.Helper()
	batch, err := renderMallWeatherFeishuBatch(request.Profile, request.Dataset, rows)
	if err != nil {
		t.Fatalf("renderMallWeatherFeishuBatch() error=%v", err)
	}
	return batch
}

func mallWeatherFeishuAppendCheckpoint(
	request mallWeatherFeishuAppendDatasetRequest,
	batchNo int,
	attempt int,
	status string,
	rowStart int64,
	batch mallWeatherFeishuRenderedBatch,
) *model.DeliveryLog {
	return &model.DeliveryLog{
		BaseModel: model.BaseModel{ID: uint(100 + batchNo)},
		RunID:     request.RunID, DestinationID: request.Destination.DestinationID,
		DatasetKind: request.Dataset.Kind, BatchNo: batchNo, Attempt: attempt, Status: status,
		RowStart: rowStart, RowEnd: rowStart + int64(len(batch.Rows)) - 1,
		RecordCount: len(batch.Rows), CellCount: len(batch.Rows) * len(batch.Rows[0]),
		RequestChecksum: batch.Checksum,
	}
}
