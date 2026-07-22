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
	"gin-biz-web-api/model"
)

func TestMallWeatherFeishuOverwriteDatasetRunnerPagesFixedRanges(t *testing.T) {
	t.Parallel()
	request := validMallWeatherFeishuOverwriteDatasetRequest()
	request.Destination.Config.BatchRows = 2
	request.GridRows = 6
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
	sheets := &fakeMallWeatherFeishuAppendDatasetSheets{responses: []*feishu.SheetValues{{
		Rows: occupiedMallWeatherFeishuFirstColumnRows(5),
	}}}
	checkpoints := &fakeMallWeatherFeishuAppendCheckpointStore{}
	batches := &fakeMallWeatherFeishuOverwriteBatchRunner{}
	runner := newTestMallWeatherFeishuOverwriteDatasetRunner(t, pager, sheets, checkpoints, batches)

	result, err := runner.Run(t.Context(), request)
	if err != nil {
		t.Fatalf("Run() error=%v", err)
	}
	if result.DatasetKind != "hourly" || result.BatchCount != 2 || result.RecordCount != 3 ||
		result.CellCount != 6 || result.LastCursor != 3 || result.DataLastRow != 4 ||
		result.PreviousLastRow != 6 || result.StaleRowStart != 5 || result.StaleRowEnd != 6 ||
		len(batches.requests) != 2 {
		t.Fatalf("result=%+v batches=%+v", result, batches.requests)
	}
	if batches.requests[0].BatchNo != 1 || batches.requests[0].Attempt != 1 || batches.requests[0].RowStart != 2 ||
		batches.requests[1].BatchNo != 2 || batches.requests[1].Attempt != 1 || batches.requests[1].RowStart != 4 {
		t.Fatalf("batch requests=%+v", batches.requests)
	}
	if len(pager.requests) != 2 || pager.requests[0].AfterID != 0 || pager.requests[1].AfterID != 2 ||
		pager.requests[0].Limit != 2 || !pager.requests[0].SnapshotAt.Equal(request.SnapshotAt.UTC()) ||
		!slices.Equal(pager.requests[0].Fields, []string{"mall_code", "temperature_c"}) {
		t.Fatalf("page requests=%+v", pager.requests)
	}
}

func TestMallWeatherFeishuOverwriteDatasetRunnerRecoversAndRewritesCheckpoints(t *testing.T) {
	t.Parallel()
	request := validMallWeatherFeishuOverwriteDatasetRequest()
	request.Destination.Config.BatchRows = 1
	request.GridRows = 7
	pages := []*data_dao.MallWeatherExportDataPage{
		{Rows: []data_dao.MallWeatherExportDataRow{mallWeatherFeishuAppendDataRow(1, "mall-a", 20)}, NextAfterID: 1, HasMore: true},
		{Rows: []data_dao.MallWeatherExportDataRow{mallWeatherFeishuAppendDataRow(2, "mall-b", 21)}, NextAfterID: 2, HasMore: true},
		{Rows: []data_dao.MallWeatherExportDataRow{mallWeatherFeishuAppendDataRow(3, "mall-c", 22)}, NextAfterID: 3},
	}
	firstBatch := renderTestMallWeatherFeishuOverwriteBatch(t, request, pages[0].Rows)
	secondBatch := renderTestMallWeatherFeishuOverwriteBatch(t, request, pages[1].Rows)
	thirdBatch := renderTestMallWeatherFeishuOverwriteBatch(t, request, pages[2].Rows)
	checkpoints := &fakeMallWeatherFeishuAppendCheckpointStore{logs: map[int]*model.DeliveryLog{
		1: mallWeatherFeishuOverwriteCheckpoint(request, 1, 1, "success", 2, firstBatch),
		2: mallWeatherFeishuOverwriteCheckpoint(request, 2, 1, "unknown", 3, secondBatch),
		3: mallWeatherFeishuOverwriteCheckpoint(request, 3, 1, "unknown", 4, thirdBatch),
	}}
	sheets := &fakeMallWeatherFeishuAppendDatasetSheets{responses: []*feishu.SheetValues{
		{Rows: occupiedMallWeatherFeishuFirstColumnRows(6)},
		{Rows: secondBatch.Rows},
		{Rows: [][]feishu.SheetCell{{
			{Type: feishu.SheetCellString, Text: "different"},
			{Type: feishu.SheetCellNumber, Number: "22"},
		}}},
	}}
	pager := &fakeMallWeatherFeishuAppendDatasetPager{pages: pages}
	batches := &fakeMallWeatherFeishuOverwriteBatchRunner{}
	runner := newTestMallWeatherFeishuOverwriteDatasetRunner(t, pager, sheets, checkpoints, batches)

	result, err := runner.Run(t.Context(), request)
	if err != nil {
		t.Fatalf("Run() error=%v", err)
	}
	if result.BatchCount != 3 || result.RecordCount != 3 || result.LastCursor != 3 || result.DataLastRow != 4 ||
		result.StaleRowStart != 5 || result.StaleRowEnd != 7 || len(checkpoints.reconciled) != 1 ||
		checkpoints.reconciled[0].id != checkpoints.logs[2].ID || len(batches.requests) != 1 ||
		batches.requests[0].BatchNo != 3 || batches.requests[0].Attempt != 2 || batches.requests[0].RowStart != 4 {
		t.Fatalf("result=%+v checkpoints=%+v batches=%+v", result, checkpoints, batches.requests)
	}
	if len(sheets.ranges) != 3 || sheets.ranges[1].StartRow != 3 || sheets.ranges[2].StartRow != 4 {
		t.Fatalf("read ranges=%+v", sheets.ranges)
	}
}

func TestMallWeatherFeishuOverwriteDatasetRunnerRepairsChangedSuccessfulRange(t *testing.T) {
	t.Parallel()
	request := validMallWeatherFeishuOverwriteDatasetRequest()
	request.GridRows = 2
	page := &data_dao.MallWeatherExportDataPage{
		Rows:        []data_dao.MallWeatherExportDataRow{mallWeatherFeishuAppendDataRow(1, "mall-a", 20)},
		NextAfterID: 1,
	}
	batch := renderTestMallWeatherFeishuOverwriteBatch(t, request, page.Rows)
	checkpoints := &fakeMallWeatherFeishuAppendCheckpointStore{logs: map[int]*model.DeliveryLog{
		1: mallWeatherFeishuOverwriteCheckpoint(request, 1, 1, "success", 2, batch),
	}}
	sheets := &fakeMallWeatherFeishuAppendDatasetSheets{responses: []*feishu.SheetValues{
		{Rows: [][]feishu.SheetCell{}},
		{Rows: [][]feishu.SheetCell{{
			{Type: feishu.SheetCellString, Text: "changed"},
			{Type: feishu.SheetCellNumber, Number: "20"},
		}}},
	}}
	batches := &fakeMallWeatherFeishuOverwriteBatchRunner{}
	runner := newTestMallWeatherFeishuOverwriteDatasetRunner(
		t,
		&fakeMallWeatherFeishuAppendDatasetPager{pages: []*data_dao.MallWeatherExportDataPage{page}},
		sheets,
		checkpoints,
		batches,
	)
	result, err := runner.Run(t.Context(), request)
	if err != nil || result.BatchCount != 1 || len(batches.requests) != 1 || batches.requests[0].Attempt != 2 ||
		batches.requests[0].RowStart != 2 {
		t.Fatalf("Run() result=%+v error=%v batches=%+v", result, err, batches.requests)
	}
}

func TestMallWeatherFeishuOverwriteDatasetRunnerReportsEmptyDatasetStaleTail(t *testing.T) {
	t.Parallel()
	request := validMallWeatherFeishuOverwriteDatasetRequest()
	request.GridRows = 3
	runner := newTestMallWeatherFeishuOverwriteDatasetRunner(
		t,
		&fakeMallWeatherFeishuAppendDatasetPager{pages: []*data_dao.MallWeatherExportDataPage{{}}},
		&fakeMallWeatherFeishuAppendDatasetSheets{responses: []*feishu.SheetValues{{
			Rows: occupiedMallWeatherFeishuFirstColumnRows(2),
		}}},
		&fakeMallWeatherFeishuAppendCheckpointStore{},
		&fakeMallWeatherFeishuOverwriteBatchRunner{},
	)
	result, err := runner.Run(t.Context(), request)
	if err != nil || result.BatchCount != 0 || result.RecordCount != 0 || result.DataLastRow != 1 ||
		result.PreviousLastRow != 3 || result.StaleRowStart != 2 || result.StaleRowEnd != 3 {
		t.Fatalf("Run() result=%+v error=%v", result, err)
	}
}

func TestMallWeatherFeishuOverwriteDatasetRunnerFailsClosedOnCheckpointConflict(t *testing.T) {
	t.Parallel()
	request := validMallWeatherFeishuOverwriteDatasetRequest()
	request.GridRows = 2
	page := &data_dao.MallWeatherExportDataPage{
		Rows:        []data_dao.MallWeatherExportDataRow{mallWeatherFeishuAppendDataRow(1, "mall-a", 20)},
		NextAfterID: 1,
	}
	batch := renderTestMallWeatherFeishuOverwriteBatch(t, request, page.Rows)
	checkpoint := mallWeatherFeishuOverwriteCheckpoint(request, 1, 1, "success", 2, batch)
	checkpoint.RequestChecksum = strings.Repeat("f", 64)
	batches := &fakeMallWeatherFeishuOverwriteBatchRunner{}
	runner := newTestMallWeatherFeishuOverwriteDatasetRunner(
		t,
		&fakeMallWeatherFeishuAppendDatasetPager{pages: []*data_dao.MallWeatherExportDataPage{page}},
		&fakeMallWeatherFeishuAppendDatasetSheets{responses: []*feishu.SheetValues{{Rows: occupiedMallWeatherFeishuFirstColumnRows(1)}}},
		&fakeMallWeatherFeishuAppendCheckpointStore{logs: map[int]*model.DeliveryLog{1: checkpoint}},
		batches,
	)
	result, err := runner.Run(t.Context(), request)
	if !errors.Is(err, ErrMallWeatherFeishuOverwriteCheckpointConflict) || result.BatchCount != 0 ||
		len(batches.requests) != 0 {
		t.Fatalf("Run() result=%+v error=%v batches=%+v", result, err, batches.requests)
	}
}

func TestMallWeatherFeishuOverwriteDatasetRunnerDoesNotAdvanceFailedBatch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		batchErr  error
		rowOffset int64
	}{
		{name: "executor fails", batchErr: errors.New("overwrite unavailable")},
		{name: "executor returns shifted range", rowOffset: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := validMallWeatherFeishuOverwriteDatasetRequest()
			request.GridRows = 1
			page := &data_dao.MallWeatherExportDataPage{
				Rows:        []data_dao.MallWeatherExportDataRow{mallWeatherFeishuAppendDataRow(1, "mall-a", 20)},
				NextAfterID: 1,
			}
			batches := &fakeMallWeatherFeishuOverwriteBatchRunner{err: test.batchErr, rowOffset: test.rowOffset}
			runner := newTestMallWeatherFeishuOverwriteDatasetRunner(
				t,
				&fakeMallWeatherFeishuAppendDatasetPager{pages: []*data_dao.MallWeatherExportDataPage{page}},
				&fakeMallWeatherFeishuAppendDatasetSheets{},
				&fakeMallWeatherFeishuAppendCheckpointStore{},
				batches,
			)
			result, err := runner.Run(t.Context(), request)
			if err == nil || result.BatchCount != 0 || result.RecordCount != 0 || result.LastCursor != 0 ||
				len(batches.requests) != 1 {
				t.Fatalf("Run() result=%+v error=%v batches=%+v", result, err, batches.requests)
			}
		})
	}
}

func TestMallWeatherFeishuOverwriteDatasetRunnerPreservesUnknownWhenReconciliationFails(t *testing.T) {
	t.Parallel()
	request := validMallWeatherFeishuOverwriteDatasetRequest()
	request.GridRows = 2
	page := &data_dao.MallWeatherExportDataPage{
		Rows:        []data_dao.MallWeatherExportDataRow{mallWeatherFeishuAppendDataRow(1, "mall-a", 20)},
		NextAfterID: 1,
	}
	batch := renderTestMallWeatherFeishuOverwriteBatch(t, request, page.Rows)
	checkpoint := mallWeatherFeishuOverwriteCheckpoint(request, 1, 1, "unknown", 2, batch)
	checkpoints := &fakeMallWeatherFeishuAppendCheckpointStore{
		logs: map[int]*model.DeliveryLog{1: checkpoint}, reconcileErr: errors.New("database unavailable"),
	}
	batches := &fakeMallWeatherFeishuOverwriteBatchRunner{}
	runner := newTestMallWeatherFeishuOverwriteDatasetRunner(
		t,
		&fakeMallWeatherFeishuAppendDatasetPager{pages: []*data_dao.MallWeatherExportDataPage{page}},
		&fakeMallWeatherFeishuAppendDatasetSheets{responses: []*feishu.SheetValues{
			{Rows: occupiedMallWeatherFeishuFirstColumnRows(1)},
			{Rows: batch.Rows},
		}},
		checkpoints,
		batches,
	)
	result, err := runner.Run(t.Context(), request)
	if !errors.Is(err, ErrMallWeatherFeishuOverwriteStateUnknown) || result.BatchCount != 0 ||
		len(checkpoints.reconciled) != 1 || len(batches.requests) != 0 {
		t.Fatalf("Run() result=%+v error=%v checkpoints=%+v batches=%+v", result, err, checkpoints, batches)
	}
}

type fakeMallWeatherFeishuOverwriteBatchRunner struct {
	requests  []mallWeatherFeishuOverwriteBatchRequest
	err       error
	rowOffset int64
}

func (runner *fakeMallWeatherFeishuOverwriteBatchRunner) Execute(
	_ context.Context,
	request mallWeatherFeishuOverwriteBatchRequest,
) (mallWeatherFeishuOverwriteBatchResult, error) {
	runner.requests = append(runner.requests, request)
	if runner.err != nil {
		return mallWeatherFeishuOverwriteBatchResult{}, runner.err
	}
	rowEnd := request.RowStart + int64(len(request.Batch.Rows)) - 1
	return mallWeatherFeishuOverwriteBatchResult{
		BatchNo: request.BatchNo, RowStart: request.RowStart + runner.rowOffset, RowEnd: rowEnd + runner.rowOffset,
		RecordCount: len(request.Batch.Rows), CellCount: len(request.Batch.Rows) * len(request.Batch.Rows[0]),
	}, nil
}

func newTestMallWeatherFeishuOverwriteDatasetRunner(
	t *testing.T,
	pager mallWeatherFeishuDatasetPager,
	sheets mallWeatherFeishuRangeReader,
	checkpoints mallWeatherFeishuBatchCheckpointStore,
	batches mallWeatherFeishuOverwriteBatchRunner,
) *mallWeatherFeishuOverwriteDatasetRunner {
	t.Helper()
	runner, err := newMallWeatherFeishuOverwriteDatasetRunner(
		pager,
		sheets,
		checkpoints,
		batches,
		func() time.Time { return time.Date(2026, 7, 23, 4, 0, 0, 0, time.UTC) },
	)
	if err != nil {
		t.Fatalf("newMallWeatherFeishuOverwriteDatasetRunner() error=%v", err)
	}
	return runner
}

func validMallWeatherFeishuOverwriteDatasetRequest() mallWeatherFeishuOverwriteDatasetRequest {
	appendRequest := validMallWeatherFeishuAppendDatasetRequest()
	appendRequest.Destination.Config.WriteMode = "overwrite_range"
	return mallWeatherFeishuOverwriteDatasetRequest{
		RunID: appendRequest.RunID, TraceID: appendRequest.TraceID, Destination: appendRequest.Destination,
		Profile: appendRequest.Profile, Dataset: appendRequest.Dataset, Filter: appendRequest.Filter,
		SnapshotAt: appendRequest.SnapshotAt, GridRows: appendRequest.GridRows,
	}
}

func renderTestMallWeatherFeishuOverwriteBatch(
	t *testing.T,
	request mallWeatherFeishuOverwriteDatasetRequest,
	rows []data_dao.MallWeatherExportDataRow,
) mallWeatherFeishuRenderedBatch {
	t.Helper()
	batch, err := renderMallWeatherFeishuBatch(request.Profile, request.Dataset, rows)
	if err != nil {
		t.Fatalf("renderMallWeatherFeishuBatch() error=%v", err)
	}
	return batch
}

func mallWeatherFeishuOverwriteCheckpoint(
	request mallWeatherFeishuOverwriteDatasetRequest,
	batchNo int,
	attempt int,
	status string,
	rowStart int64,
	batch mallWeatherFeishuRenderedBatch,
) *model.DeliveryLog {
	return &model.DeliveryLog{
		BaseModel: model.BaseModel{ID: uint(200 + batchNo)},
		RunID:     request.RunID, DestinationID: request.Destination.DestinationID,
		DatasetKind: request.Dataset.Kind, BatchNo: batchNo, Attempt: attempt, Status: status,
		RowStart: rowStart, RowEnd: rowStart + int64(len(batch.Rows)) - 1,
		RecordCount: len(batch.Rows), CellCount: len(batch.Rows) * len(batch.Rows[0]),
		RequestChecksum: batch.Checksum,
	}
}

func occupiedMallWeatherFeishuFirstColumnRows(count int) [][]feishu.SheetCell {
	rows := make([][]feishu.SheetCell, count)
	for index := range rows {
		rows[index] = []feishu.SheetCell{{Type: feishu.SheetCellString, Text: "occupied"}}
	}
	return rows
}
