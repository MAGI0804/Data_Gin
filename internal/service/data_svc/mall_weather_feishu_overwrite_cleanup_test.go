package data_svc

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"gin-biz-web-api/connector/feishu"
	"gin-biz-web-api/model"
)

func TestMallWeatherFeishuOverwriteCleanupRunnerClearsStableBatches(t *testing.T) {
	t.Parallel()
	request := validMallWeatherFeishuOverwriteCleanupRequest()
	request.Destination.Config.BatchRows = 2
	request.StartBatchNo = 3
	request.RowStart = 5
	request.RowEnd = 9
	batches := &fakeMallWeatherFeishuOverwriteClearBatchRunner{}
	runner := newTestMallWeatherFeishuOverwriteCleanupRunner(
		t,
		&fakeMallWeatherFeishuAppendDatasetSheets{},
		&fakeMallWeatherFeishuAppendCheckpointStore{},
		batches,
	)

	result, err := runner.Run(t.Context(), request)
	if err != nil {
		t.Fatalf("Run() error=%v", err)
	}
	if result.DatasetKind != "hourly" || result.BatchCount != 3 || result.ClearedRows != 5 ||
		result.CellCount != 10 || result.LastRow != 9 || len(batches.requests) != 3 {
		t.Fatalf("result=%+v batches=%+v", result, batches.requests)
	}
	if batches.requests[0].BatchNo != 3 || batches.requests[0].Attempt != 1 ||
		batches.requests[0].RowStart != 5 || batches.requests[0].RowEnd != 6 ||
		batches.requests[1].BatchNo != 4 || batches.requests[1].RowStart != 7 || batches.requests[1].RowEnd != 8 ||
		batches.requests[2].BatchNo != 5 || batches.requests[2].RowStart != 9 || batches.requests[2].RowEnd != 9 {
		t.Fatalf("batch requests=%+v", batches.requests)
	}
}

func TestMallWeatherFeishuOverwriteCleanupRunnerRecoversCheckpoints(t *testing.T) {
	t.Parallel()
	request := validMallWeatherFeishuOverwriteCleanupRequest()
	request.Destination.Config.BatchRows = 2
	request.StartBatchNo = 3
	request.RowStart = 5
	request.RowEnd = 10
	checksum, err := checksumMallWeatherFeishuRows(nil, 2, request.Columns)
	if err != nil {
		t.Fatalf("checksumMallWeatherFeishuRows() error=%v", err)
	}
	checkpoints := &fakeMallWeatherFeishuAppendCheckpointStore{logs: map[int]*model.DeliveryLog{
		3: mallWeatherFeishuOverwriteCleanupCheckpoint(request, 3, 1, "success", 5, 6, checksum),
		4: mallWeatherFeishuOverwriteCleanupCheckpoint(request, 4, 1, "unknown", 7, 8, checksum),
		5: mallWeatherFeishuOverwriteCleanupCheckpoint(request, 5, 1, "unknown", 9, 10, checksum),
	}}
	sheets := &fakeMallWeatherFeishuAppendDatasetSheets{responses: []*feishu.SheetValues{
		{},
		{},
		{Rows: [][]feishu.SheetCell{{{Type: feishu.SheetCellString, Text: "stale"}}}},
	}}
	batches := &fakeMallWeatherFeishuOverwriteClearBatchRunner{}
	runner := newTestMallWeatherFeishuOverwriteCleanupRunner(t, sheets, checkpoints, batches)

	result, err := runner.Run(t.Context(), request)
	if err != nil {
		t.Fatalf("Run() error=%v", err)
	}
	if result.BatchCount != 3 || result.ClearedRows != 6 || result.LastRow != 10 ||
		len(checkpoints.reconciled) != 1 || checkpoints.reconciled[0].id != checkpoints.logs[4].ID ||
		len(batches.requests) != 1 || batches.requests[0].BatchNo != 5 || batches.requests[0].Attempt != 2 ||
		batches.requests[0].RowStart != 9 || batches.requests[0].RowEnd != 10 || len(sheets.ranges) != 3 {
		t.Fatalf("result=%+v checkpoints=%+v batches=%+v sheets=%+v", result, checkpoints, batches.requests, sheets)
	}
}

func TestMallWeatherFeishuOverwriteCleanupRunnerFailsClosedOnCheckpointConflict(t *testing.T) {
	t.Parallel()
	request := validMallWeatherFeishuOverwriteCleanupRequest()
	request.StartBatchNo = 3
	request.RowStart = 5
	request.RowEnd = 6
	checksum, err := checksumMallWeatherFeishuRows(nil, 2, request.Columns)
	if err != nil {
		t.Fatalf("checksumMallWeatherFeishuRows() error=%v", err)
	}
	checkpoint := mallWeatherFeishuOverwriteCleanupCheckpoint(request, 3, 1, "success", 5, 6, checksum)
	checkpoint.RequestChecksum = strings.Repeat("f", 64)
	batches := &fakeMallWeatherFeishuOverwriteClearBatchRunner{}
	runner := newTestMallWeatherFeishuOverwriteCleanupRunner(
		t,
		&fakeMallWeatherFeishuAppendDatasetSheets{},
		&fakeMallWeatherFeishuAppendCheckpointStore{logs: map[int]*model.DeliveryLog{3: checkpoint}},
		batches,
	)
	result, err := runner.Run(t.Context(), request)
	if !errors.Is(err, ErrMallWeatherFeishuOverwriteCleanupCheckpointConflict) || result.BatchCount != 0 ||
		len(batches.requests) != 0 {
		t.Fatalf("Run() result=%+v error=%v batches=%+v", result, err, batches.requests)
	}
}

func TestMallWeatherFeishuOverwriteCleanupRunnerRepairsChangedSuccessfulRange(t *testing.T) {
	t.Parallel()
	request := validMallWeatherFeishuOverwriteCleanupRequest()
	request.RowStart = 5
	request.RowEnd = 6
	checksum, err := checksumMallWeatherFeishuRows(nil, 2, request.Columns)
	if err != nil {
		t.Fatalf("checksumMallWeatherFeishuRows() error=%v", err)
	}
	checkpoint := mallWeatherFeishuOverwriteCleanupCheckpoint(request, 1, 1, "success", 5, 6, checksum)
	batches := &fakeMallWeatherFeishuOverwriteClearBatchRunner{}
	runner := newTestMallWeatherFeishuOverwriteCleanupRunner(
		t,
		&fakeMallWeatherFeishuAppendDatasetSheets{responses: []*feishu.SheetValues{{Rows: [][]feishu.SheetCell{{
			{Type: feishu.SheetCellString, Text: "restored-stale-data"},
		}}}}},
		&fakeMallWeatherFeishuAppendCheckpointStore{logs: map[int]*model.DeliveryLog{1: checkpoint}},
		batches,
	)
	result, err := runner.Run(t.Context(), request)
	if err != nil || result.BatchCount != 1 || len(batches.requests) != 1 || batches.requests[0].Attempt != 2 ||
		batches.requests[0].RowStart != 5 || batches.requests[0].RowEnd != 6 {
		t.Fatalf("Run() result=%+v error=%v batches=%+v", result, err, batches.requests)
	}
}

func TestMallWeatherFeishuOverwriteCleanupRunnerDoesNotAdvanceFailedBatch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		batchErr  error
		rowOffset int64
	}{
		{name: "clear fails", batchErr: errors.New("clear unavailable")},
		{name: "clear returns shifted range", rowOffset: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := validMallWeatherFeishuOverwriteCleanupRequest()
			request.RowStart = 5
			request.RowEnd = 6
			batches := &fakeMallWeatherFeishuOverwriteClearBatchRunner{err: test.batchErr, rowOffset: test.rowOffset}
			runner := newTestMallWeatherFeishuOverwriteCleanupRunner(
				t,
				&fakeMallWeatherFeishuAppendDatasetSheets{},
				&fakeMallWeatherFeishuAppendCheckpointStore{},
				batches,
			)
			result, err := runner.Run(t.Context(), request)
			if err == nil || result.BatchCount != 0 || result.ClearedRows != 0 || result.LastRow != 0 ||
				len(batches.requests) != 1 {
				t.Fatalf("Run() result=%+v error=%v batches=%+v", result, err, batches.requests)
			}
		})
	}
}

func TestMallWeatherFeishuOverwriteCleanupRunnerRejectsInvalidRequest(t *testing.T) {
	t.Parallel()
	request := validMallWeatherFeishuOverwriteCleanupRequest()
	request.RowStart = 1
	batches := &fakeMallWeatherFeishuOverwriteClearBatchRunner{}
	runner := newTestMallWeatherFeishuOverwriteCleanupRunner(
		t,
		&fakeMallWeatherFeishuAppendDatasetSheets{},
		&fakeMallWeatherFeishuAppendCheckpointStore{},
		batches,
	)
	if _, err := runner.Run(t.Context(), request); err == nil || len(batches.requests) != 0 {
		t.Fatalf("Run() error=%v batches=%+v", err, batches.requests)
	}
}

type fakeMallWeatherFeishuOverwriteClearBatchRunner struct {
	requests  []mallWeatherFeishuOverwriteClearBatchRequest
	err       error
	rowOffset int64
}

func (runner *fakeMallWeatherFeishuOverwriteClearBatchRunner) Clear(
	_ context.Context,
	request mallWeatherFeishuOverwriteClearBatchRequest,
) (mallWeatherFeishuOverwriteBatchResult, error) {
	runner.requests = append(runner.requests, request)
	if runner.err != nil {
		return mallWeatherFeishuOverwriteBatchResult{}, runner.err
	}
	rowCount := int(request.RowEnd - request.RowStart + 1)
	return mallWeatherFeishuOverwriteBatchResult{
		BatchNo: request.BatchNo, RowStart: request.RowStart + runner.rowOffset,
		RowEnd: request.RowEnd + runner.rowOffset, RecordCount: rowCount, CellCount: rowCount * request.Columns,
	}, nil
}

func newTestMallWeatherFeishuOverwriteCleanupRunner(
	t *testing.T,
	sheets mallWeatherFeishuRangeReader,
	checkpoints mallWeatherFeishuBatchCheckpointStore,
	batches mallWeatherFeishuOverwriteClearBatchRunner,
) *mallWeatherFeishuOverwriteCleanupRunner {
	t.Helper()
	runner, err := newMallWeatherFeishuOverwriteCleanupRunner(
		sheets,
		checkpoints,
		batches,
		func() time.Time { return time.Date(2026, 7, 23, 4, 0, 0, 0, time.UTC) },
	)
	if err != nil {
		t.Fatalf("newMallWeatherFeishuOverwriteCleanupRunner() error=%v", err)
	}
	return runner
}

func validMallWeatherFeishuOverwriteCleanupRequest() mallWeatherFeishuOverwriteCleanupRequest {
	datasetRequest := validMallWeatherFeishuOverwriteDatasetRequest()
	return mallWeatherFeishuOverwriteCleanupRequest{
		RunID: datasetRequest.RunID, TraceID: datasetRequest.TraceID, Destination: datasetRequest.Destination,
		DatasetKind: datasetRequest.Dataset.Kind, StartBatchNo: 1,
		RowStart: 5, RowEnd: 8, Columns: len(datasetRequest.Dataset.Columns),
	}
}

func mallWeatherFeishuOverwriteCleanupCheckpoint(
	request mallWeatherFeishuOverwriteCleanupRequest,
	batchNo int,
	attempt int,
	status string,
	rowStart int64,
	rowEnd int64,
	checksum string,
) *model.DeliveryLog {
	rowCount := int(rowEnd - rowStart + 1)
	return &model.DeliveryLog{
		BaseModel: model.BaseModel{ID: uint(300 + batchNo)},
		RunID:     request.RunID, DestinationID: request.Destination.DestinationID,
		DatasetKind: request.DatasetKind, BatchNo: batchNo, Attempt: attempt, Status: status,
		RowStart: rowStart, RowEnd: rowEnd, RecordCount: rowCount, CellCount: rowCount * request.Columns,
		RequestChecksum: checksum,
	}
}
