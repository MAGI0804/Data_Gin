package data_svc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"gin-biz-web-api/connector/feishu"
	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/model"

	"github.com/google/uuid"
)

const maxMallWeatherFeishuUpsertUpdateRanges = 100

var ErrMallWeatherFeishuUpsertStateUnknown = errors.New(
	"mall weather feishu upsert: remote state is unknown",
)

type mallWeatherFeishuUpsertSheets interface {
	AppendValues(context.Context, string, feishu.SheetWriteRange) (*feishu.SheetWriteResult, error)
	BatchUpdateValues(context.Context, string, []feishu.SheetWriteRange) (*feishu.SheetWriteResult, error)
	ReadRange(context.Context, string, feishu.SheetRange) (*feishu.SheetValues, error)
}

type mallWeatherFeishuUpsertWriteRow struct {
	BusinessKey string
	RowNumber   int64
	Checksum    string
	Cells       []feishu.SheetCell
}

type mallWeatherFeishuUpsertBatchRequest struct {
	RunID       uint
	TraceID     string
	Destination *MallWeatherFeishuResolvedDestination
	DatasetKind string
	BatchNo     int
	Attempt     int
	Mode        string
	Rows        []mallWeatherFeishuUpsertWriteRow
}

type mallWeatherFeishuUpsertBatchResult struct {
	DeliveryLogID uint
	Mode          string
	BatchNo       int
	RecordCount   int
	CellCount     int
	Revision      int64
	Rows          []mallWeatherFeishuUpsertWriteRow
}

type mallWeatherFeishuUpsertBatchExecutor struct {
	sheets mallWeatherFeishuUpsertSheets
	logs   mallWeatherFeishuBatchLogStore
	now    func() time.Time
}

func newMallWeatherFeishuUpsertBatchExecutor(
	sheets mallWeatherFeishuUpsertSheets,
	logs mallWeatherFeishuBatchLogStore,
	now func() time.Time,
) (*mallWeatherFeishuUpsertBatchExecutor, error) {
	if sheets == nil || logs == nil || now == nil {
		return nil, errors.New("mall weather feishu upsert batch: invalid configuration")
	}
	return &mallWeatherFeishuUpsertBatchExecutor{sheets: sheets, logs: logs, now: now}, nil
}

func (executor *mallWeatherFeishuUpsertBatchExecutor) Execute(
	ctx context.Context,
	request mallWeatherFeishuUpsertBatchRequest,
) (mallWeatherFeishuUpsertBatchResult, error) {
	var result mallWeatherFeishuUpsertBatchResult
	rows, columns, rowStart, rowEnd, requestChecksum, err := validateMallWeatherFeishuUpsertBatchRequest(ctx, request)
	if executor == nil || executor.sheets == nil || executor.logs == nil || executor.now == nil || err != nil {
		return result, errors.New("mall weather feishu upsert batch: invalid request")
	}
	startedAt := executor.now().UTC()
	if startedAt.IsZero() {
		return result, errors.New("mall weather feishu upsert batch: invalid start time")
	}
	log := &model.DeliveryLog{
		TraceID: request.TraceID, RunID: request.RunID, DestinationID: request.Destination.DestinationID,
		SourceCode: "mall_weather", DestinationCode: request.Destination.Code,
		DestinationName: request.Destination.Code,
		BusinessKey:     request.DatasetKind + ":upsert:" + request.Mode + ":" + strconv.Itoa(request.BatchNo),
		DatasetKind:     request.DatasetKind, BatchNo: request.BatchNo, RowStart: rowStart, RowEnd: rowEnd,
		RecordCount: len(rows), CellCount: len(rows) * columns, RequestChecksum: requestChecksum,
		Status: "running", Attempt: request.Attempt, RetryCount: request.Attempt - 1,
		StartedAt: &model.TimeNormal{Time: startedAt},
	}
	logID, err := executor.logs.Create(ctx, log)
	if err != nil || logID == 0 {
		return result, fmt.Errorf("mall weather feishu upsert batch: create checkpoint: %w", nonNilMallWeatherFeishuError(err))
	}
	result = mallWeatherFeishuUpsertBatchResult{
		DeliveryLogID: logID, Mode: request.Mode, BatchNo: request.BatchNo,
		RecordCount: len(rows), CellCount: len(rows) * columns, Rows: rows,
	}
	var finish data_dao.DeliveryLogBatchFinish
	var remoteErr error
	switch request.Mode {
	case "append":
		finish, result.Revision, remoteErr = executor.executeAppend(ctx, request, rows, columns, rowStart, rowEnd)
	case "update":
		finish, result.Revision, remoteErr = executor.executeUpdates(ctx, request, rows, columns)
	default:
		return result, errors.New("mall weather feishu upsert batch: unsupported mode")
	}
	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), mallWeatherFeishuCheckpointTimeout)
	finishErr := executor.logs.FinishWeatherBatch(finishCtx, logID, finish)
	cancel()
	if finishErr != nil {
		return result, fmt.Errorf("%w: persist upsert batch outcome", ErrMallWeatherFeishuUpsertStateUnknown)
	}
	switch finish.Status {
	case "success":
		return result, nil
	case "failed":
		return result, classifyMallWeatherFeishuExecutorError(
			"飞书 upsert 批次被拒绝",
			remoteErr,
		)
	default:
		return result, ErrMallWeatherFeishuUpsertStateUnknown
	}
}

func (executor *mallWeatherFeishuUpsertBatchExecutor) executeAppend(
	ctx context.Context,
	request mallWeatherFeishuUpsertBatchRequest,
	rows []mallWeatherFeishuUpsertWriteRow,
	columns int,
	rowStart int64,
	rowEnd int64,
) (data_dao.DeliveryLogBatchFinish, int64, error) {
	writeRows := make([][]feishu.SheetCell, len(rows))
	for index, row := range rows {
		writeRows[index] = row.Cells
	}
	acknowledgement, writeErr := executor.sheets.AppendValues(ctx, request.Destination.SpreadsheetToken, feishu.SheetWriteRange{
		Range: feishu.SheetRange{
			SheetID: request.Destination.SheetIDs[request.DatasetKind], StartRow: rowStart, EndRow: rowEnd,
			StartColumn: 1, EndColumn: int64(columns),
		},
		Rows: writeRows,
	})
	if writeErr == nil && !validMallWeatherFeishuAppendAcknowledgement(
		acknowledgement,
		rowStart,
		rowEnd,
		len(rows),
		columns,
		len(rows)*columns,
	) {
		acknowledgement = nil
	}
	finish := mallWeatherFeishuAppendFinish(acknowledgement, writeErr, executor.now().UTC())
	if acknowledgement == nil {
		return finish, 0, writeErr
	}
	return finish, acknowledgement.Revision, writeErr
}

func (executor *mallWeatherFeishuUpsertBatchExecutor) executeUpdates(
	ctx context.Context,
	request mallWeatherFeishuUpsertBatchRequest,
	rows []mallWeatherFeishuUpsertWriteRow,
	columns int,
) (data_dao.DeliveryLogBatchFinish, int64, error) {
	writes := mallWeatherFeishuUpsertUpdateRanges(request.Destination.SheetIDs[request.DatasetKind], rows, columns)
	acknowledgement, writeErr := executor.sheets.BatchUpdateValues(
		ctx,
		request.Destination.SpreadsheetToken,
		writes,
	)
	shouldVerify := writeErr == nil || mallWeatherFeishuOverwriteOutcomeUnknown(writeErr)
	matched := false
	var verifyErr error
	if shouldVerify {
		matched, verifyErr = verifyMallWeatherFeishuUpsertWrites(
			ctx,
			executor.sheets,
			request.Destination.SpreadsheetToken,
			writes,
		)
	}
	finish := mallWeatherFeishuOverwriteFinish(writeErr, shouldVerify, matched, verifyErr, executor.now().UTC())
	if acknowledgement == nil {
		return finish, 0, writeErr
	}
	return finish, acknowledgement.Revision, writeErr
}

func validateMallWeatherFeishuUpsertBatchRequest(
	ctx context.Context,
	request mallWeatherFeishuUpsertBatchRequest,
) ([]mallWeatherFeishuUpsertWriteRow, int, int64, int64, string, error) {
	_, datasetAllowed := mallWeatherFeishuDatasetKinds[request.DatasetKind]
	if ctx == nil || request.RunID == 0 || uuid.Validate(request.TraceID) != nil || request.Destination == nil ||
		request.Destination.DestinationID == 0 || request.Destination.Code == "" ||
		request.Destination.Config.WriteMode != "upsert" || request.Destination.SpreadsheetToken == "" ||
		!datasetAllowed || request.Destination.SheetIDs[request.DatasetKind] == "" ||
		request.Destination.Config.SheetIDEnvMapping[request.DatasetKind] == "" || request.BatchNo < 1 ||
		request.Attempt < 1 || (request.Mode != "append" && request.Mode != "update") || len(request.Rows) == 0 ||
		len(request.Rows) > request.Destination.Config.BatchRows || len(request.Rows) > maxMallWeatherFeishuBatchRows {
		return nil, 0, 0, 0, "", errors.New("invalid request")
	}
	rows := append([]mallWeatherFeishuUpsertWriteRow(nil), request.Rows...)
	sort.Slice(rows, func(left, right int) bool { return rows[left].RowNumber < rows[right].RowNumber })
	columns := len(rows[0].Cells)
	if columns == 0 || columns > maxMallWeatherFeishuColumns || len(rows) > math.MaxInt/columns {
		return nil, 0, 0, 0, "", errors.New("invalid dimensions")
	}
	seenKeys := make(map[string]struct{}, len(rows))
	seenRows := make(map[int64]struct{}, len(rows))
	for index, row := range rows {
		if !validMallWeatherFeishuUpsertBusinessKey(row.BusinessKey) || row.RowNumber < 2 ||
			row.RowNumber > maxMallWeatherFeishuSheetRow || len(row.Checksum) != 64 || len(row.Cells) != columns ||
			mallWeatherFeishuFirstCellIsBlank(row.Cells) {
			return nil, 0, 0, 0, "", errors.New("invalid row")
		}
		if _, duplicate := seenKeys[row.BusinessKey]; duplicate {
			return nil, 0, 0, 0, "", errors.New("duplicate key")
		}
		if _, duplicate := seenRows[row.RowNumber]; duplicate {
			return nil, 0, 0, 0, "", errors.New("duplicate row")
		}
		seenKeys[row.BusinessKey] = struct{}{}
		seenRows[row.RowNumber] = struct{}{}
		checksum, err := checksumMallWeatherFeishuRows([][]feishu.SheetCell{row.Cells}, 1, columns)
		if err != nil || checksum != row.Checksum {
			return nil, 0, 0, 0, "", errors.New("row checksum mismatch")
		}
		if request.Mode == "append" && index > 0 && row.RowNumber != rows[index-1].RowNumber+1 {
			return nil, 0, 0, 0, "", errors.New("append rows are not contiguous")
		}
	}
	writes := mallWeatherFeishuUpsertUpdateRanges(request.Destination.SheetIDs[request.DatasetKind], rows, columns)
	if request.Mode == "update" && len(writes) > maxMallWeatherFeishuUpsertUpdateRanges {
		return nil, 0, 0, 0, "", errors.New("too many update ranges")
	}
	requestChecksum, err := checksumMallWeatherFeishuUpsertWriteRequest(request.Mode, rows)
	if err != nil {
		return nil, 0, 0, 0, "", err
	}
	return rows, columns, rows[0].RowNumber, rows[len(rows)-1].RowNumber, requestChecksum, nil
}

func validMallWeatherFeishuUpsertBusinessKey(value string) bool {
	if len(value) != 71 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	decoded, err := hex.DecodeString(strings.TrimPrefix(value, "sha256:"))
	return err == nil && len(decoded) == 32 && value == strings.ToLower(value)
}

func mallWeatherFeishuUpsertUpdateRanges(
	sheetID string,
	rows []mallWeatherFeishuUpsertWriteRow,
	columns int,
) []feishu.SheetWriteRange {
	if len(rows) == 0 {
		return nil
	}
	writes := make([]feishu.SheetWriteRange, 0, len(rows))
	start := 0
	for start < len(rows) {
		end := start + 1
		for end < len(rows) && rows[end].RowNumber == rows[end-1].RowNumber+1 {
			end++
		}
		values := make([][]feishu.SheetCell, end-start)
		for index := start; index < end; index++ {
			values[index-start] = rows[index].Cells
		}
		writes = append(writes, feishu.SheetWriteRange{
			Range: feishu.SheetRange{
				SheetID: sheetID, StartRow: rows[start].RowNumber, EndRow: rows[end-1].RowNumber,
				StartColumn: 1, EndColumn: int64(columns),
			},
			Rows: values,
		})
		start = end
	}
	return writes
}

func verifyMallWeatherFeishuUpsertWrites(
	ctx context.Context,
	sheets mallWeatherFeishuRangeReader,
	spreadsheetToken string,
	writes []feishu.SheetWriteRange,
) (bool, error) {
	if ctx == nil || sheets == nil || spreadsheetToken == "" || len(writes) == 0 ||
		len(writes) > maxMallWeatherFeishuUpsertUpdateRanges {
		return false, errors.New("mall weather feishu upsert batch: invalid verification")
	}
	for _, write := range writes {
		rows := int(write.Range.EndRow - write.Range.StartRow + 1)
		columns := int(write.Range.EndColumn - write.Range.StartColumn + 1)
		expected, err := checksumMallWeatherFeishuRows(write.Rows, rows, columns)
		if err != nil {
			return false, err
		}
		matched, err := verifyMallWeatherFeishuRangeChecksum(
			ctx,
			sheets,
			spreadsheetToken,
			write.Range.SheetID,
			write.Range.StartRow,
			write.Range.EndRow,
			columns,
			expected,
		)
		if err != nil || !matched {
			return false, err
		}
	}
	return true, nil
}

func checksumMallWeatherFeishuUpsertWriteRequest(
	mode string,
	rows []mallWeatherFeishuUpsertWriteRow,
) (string, error) {
	canonical := make([]struct {
		BusinessKey string `json:"businessKey"`
		RowNumber   int64  `json:"rowNumber"`
		Checksum    string `json:"checksum"`
	}, len(rows))
	for index, row := range rows {
		canonical[index].BusinessKey = row.BusinessKey
		canonical[index].RowNumber = row.RowNumber
		canonical[index].Checksum = row.Checksum
	}
	encoded, err := json.Marshal(struct {
		Mode string      `json:"mode"`
		Rows interface{} `json:"rows"`
	}{Mode: mode, Rows: canonical})
	if err != nil {
		return "", errors.New("mall weather feishu upsert batch: encode request checksum")
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}
