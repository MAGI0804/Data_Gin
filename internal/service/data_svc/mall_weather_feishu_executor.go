package data_svc

import (
	"context"
	"errors"
	"math"
	"sort"
	"strconv"
	"time"

	"gin-biz-web-api/connector/feishu"
	"gin-biz-web-api/internal/dao/data_dao"
	weatherdomain "gin-biz-web-api/internal/weather"
)

type mallWeatherFeishuExecutionSheets interface {
	Inspect(context.Context, string, []string) (*feishu.SpreadsheetMetadata, error)
	ReadRange(context.Context, string, feishu.SheetRange) (*feishu.SheetValues, error)
	BatchUpdateValues(context.Context, string, []feishu.SheetWriteRange) (*feishu.SheetWriteResult, error)
}

type mallWeatherFeishuAppendDatasetExecutor interface {
	Run(context.Context, mallWeatherFeishuAppendDatasetRequest) (mallWeatherFeishuAppendDatasetResult, error)
}

type mallWeatherFeishuOverwriteDatasetExecutor interface {
	Run(context.Context, mallWeatherFeishuOverwriteDatasetRequest) (mallWeatherFeishuOverwriteDatasetResult, error)
}

type mallWeatherFeishuOverwriteCleanupExecutor interface {
	Run(context.Context, mallWeatherFeishuOverwriteCleanupRequest) (mallWeatherFeishuOverwriteCleanupResult, error)
}

type mallWeatherFeishuExecutor struct {
	resources          mallWeatherFeishuResourceResolver
	sheets             mallWeatherFeishuExecutionSheets
	locker             weatherdomain.TaskLocker
	appendDatasets     mallWeatherFeishuAppendDatasetExecutor
	overwriteDatasets  mallWeatherFeishuOverwriteDatasetExecutor
	overwriteCleanup   mallWeatherFeishuOverwriteCleanupExecutor
	lockReleaseTimeout time.Duration
}

func newMallWeatherFeishuExecutor(
	resources mallWeatherFeishuResourceResolver,
	sheets mallWeatherFeishuExecutionSheets,
	locker weatherdomain.TaskLocker,
	appendDatasets mallWeatherFeishuAppendDatasetExecutor,
	overwriteDatasets mallWeatherFeishuOverwriteDatasetExecutor,
	overwriteCleanup mallWeatherFeishuOverwriteCleanupExecutor,
	lockReleaseTimeout time.Duration,
) (*mallWeatherFeishuExecutor, error) {
	if resources == nil || sheets == nil || locker == nil || appendDatasets == nil ||
		overwriteDatasets == nil || overwriteCleanup == nil || lockReleaseTimeout <= 0 {
		return nil, errors.New("mall weather feishu executor: invalid configuration")
	}
	return &mallWeatherFeishuExecutor{
		resources: resources, sheets: sheets, locker: locker, appendDatasets: appendDatasets,
		overwriteDatasets: overwriteDatasets, overwriteCleanup: overwriteCleanup,
		lockReleaseTimeout: lockReleaseTimeout,
	}, nil
}

func (executor *mallWeatherFeishuExecutor) Execute(
	ctx context.Context,
	record data_dao.MallWeatherFeishuRunRecord,
	progress func(successCount, failedCount int) error,
) (result MallWeatherFeishuExecutionResult, resultErr error) {
	if executor == nil || executor.resources == nil || executor.sheets == nil || executor.locker == nil ||
		executor.appendDatasets == nil || executor.overwriteDatasets == nil || executor.overwriteCleanup == nil ||
		executor.lockReleaseTimeout <= 0 || ctx == nil || progress == nil {
		return result, permanentMallWeatherFeishuExecutionError(
			"飞书推送执行器配置无效",
			errors.New("mall weather feishu executor: invalid execution"),
		)
	}
	prepared, err := prepareMallWeatherFeishuExecution(record, executor.resources)
	if err != nil {
		return result, permanentMallWeatherFeishuExecutionError("飞书推送快照无效", err)
	}
	lock, acquired, err := executor.locker.Acquire(ctx, mallWeatherFeishuDestinationLockKey(prepared.Destination))
	if err != nil {
		return result, retryableMallWeatherFeishuExecutionError("飞书推送锁获取失败", err)
	}
	if !acquired {
		return result, retryableMallWeatherFeishuExecutionError(
			"飞书目标正在执行其他推送",
			errors.New("mall weather feishu executor: destination lock is busy"),
		)
	}
	if lock == nil {
		return result, retryableMallWeatherFeishuExecutionError(
			"飞书推送锁状态无效",
			errors.New("mall weather feishu executor: acquired lock is nil"),
		)
	}
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), executor.lockReleaseTimeout)
		defer cancel()
		if releaseErr := lock.Release(releaseCtx); releaseErr != nil {
			wrapped := retryableMallWeatherFeishuExecutionError("飞书推送锁释放失败", releaseErr)
			if resultErr == nil {
				resultErr = wrapped
			} else {
				resultErr = errors.Join(resultErr, wrapped)
			}
		}
	}()
	metadata, err := executor.inspect(ctx, prepared)
	if err != nil {
		return result, classifyMallWeatherFeishuExecutorError("飞书目标预检失败", err)
	}
	if _, err := ensureMallWeatherFeishuHeaders(ctx, prepared.Destination, prepared.Profile, executor.sheets); err != nil {
		return result, classifyMallWeatherFeishuExecutorError("飞书表头校验失败", err)
	}
	for _, dataset := range prepared.Profile.Datasets {
		if err := ctx.Err(); err != nil {
			return result, retryableMallWeatherFeishuExecutionError("飞书推送已中断", err)
		}
		sheet := metadata[dataset.Kind]
		var successCount int64
		switch prepared.Destination.Config.WriteMode {
		case "append":
			datasetResult, runErr := executor.appendDatasets.Run(ctx, mallWeatherFeishuAppendDatasetRequest{
				RunID: record.Pipeline.ID, TraceID: record.Pipeline.TraceID,
				Destination: prepared.Destination, Profile: prepared.Profile, Dataset: dataset,
				Filter: prepared.Filter, SnapshotAt: prepared.SnapshotAt, GridRows: sheet.GridProperties.RowCount,
			})
			if runErr != nil {
				result.FailedCount++
				return result, classifyMallWeatherFeishuExecutorError("飞书追加数据集失败", runErr)
			}
			successCount = datasetResult.RecordCount
		case "overwrite_range":
			datasetResult, runErr := executor.overwriteDatasets.Run(ctx, mallWeatherFeishuOverwriteDatasetRequest{
				RunID: record.Pipeline.ID, TraceID: record.Pipeline.TraceID,
				Destination: prepared.Destination, Profile: prepared.Profile, Dataset: dataset,
				Filter: prepared.Filter, SnapshotAt: prepared.SnapshotAt, GridRows: sheet.GridProperties.RowCount,
			})
			if runErr != nil {
				result.FailedCount++
				return result, classifyMallWeatherFeishuExecutorError("飞书覆盖数据集失败", runErr)
			}
			if datasetResult.StaleRowStart > 0 {
				columns, columnErr := mallWeatherExportRenderColumns(dataset)
				if columnErr != nil || len(columns) == 0 {
					result.FailedCount++
					return result, permanentMallWeatherFeishuExecutionError(
						"飞书覆盖清理配置无效",
						columnErr,
					)
				}
				_, cleanupErr := executor.overwriteCleanup.Run(ctx, mallWeatherFeishuOverwriteCleanupRequest{
					RunID: record.Pipeline.ID, TraceID: record.Pipeline.TraceID,
					Destination: prepared.Destination, DatasetKind: dataset.Kind,
					StartBatchNo: datasetResult.BatchCount + 1,
					RowStart:     datasetResult.StaleRowStart, RowEnd: datasetResult.StaleRowEnd,
					Columns: len(columns),
				})
				if cleanupErr != nil {
					result.FailedCount++
					return result, classifyMallWeatherFeishuExecutorError("飞书覆盖尾部清理失败", cleanupErr)
				}
			}
			successCount = datasetResult.RecordCount
		case "upsert":
			result.FailedCount++
			return result, permanentMallWeatherFeishuExecutionError(
				"飞书 upsert 尚未启用",
				errors.New("mall weather feishu executor: upsert runner is unavailable"),
			)
		default:
			result.FailedCount++
			return result, permanentMallWeatherFeishuExecutionError(
				"飞书写入模式无效",
				errors.New("mall weather feishu executor: unsupported write mode"),
			)
		}
		if successCount < 0 || successCount > int64(math.MaxInt-result.SuccessCount) {
			result.FailedCount++
			return result, permanentMallWeatherFeishuExecutionError(
				"飞书推送计数无效",
				errors.New("mall weather feishu executor: record count overflow"),
			)
		}
		result.SuccessCount += int(successCount)
		if err := progress(result.SuccessCount, result.FailedCount); err != nil {
			return result, retryableMallWeatherFeishuExecutionError("飞书推送进度保存失败", err)
		}
	}
	return result, nil
}

func (executor *mallWeatherFeishuExecutor) inspect(
	ctx context.Context,
	prepared *mallWeatherFeishuPreparedExecution,
) (map[string]feishu.SheetMetadata, error) {
	requiredSheetIDs := make([]string, 0, len(prepared.Profile.Datasets))
	for _, dataset := range prepared.Profile.Datasets {
		requiredSheetIDs = append(requiredSheetIDs, prepared.Destination.SheetIDs[dataset.Kind])
	}
	sort.Strings(requiredSheetIDs)
	metadata, err := executor.sheets.Inspect(ctx, prepared.Destination.SpreadsheetToken, requiredSheetIDs)
	if err != nil {
		return nil, err
	}
	if metadata == nil || metadata.SpreadsheetToken != prepared.Destination.SpreadsheetToken {
		return nil, errors.New("mall weather feishu executor: invalid spreadsheet metadata")
	}
	bySheetID := make(map[string]feishu.SheetMetadata, len(metadata.Sheets))
	for _, sheet := range metadata.Sheets {
		if sheet.SheetID == "" {
			return nil, errors.New("mall weather feishu executor: invalid sheet metadata")
		}
		if _, duplicate := bySheetID[sheet.SheetID]; duplicate {
			return nil, errors.New("mall weather feishu executor: duplicate sheet metadata")
		}
		bySheetID[sheet.SheetID] = sheet
	}
	result := make(map[string]feishu.SheetMetadata, len(prepared.Profile.Datasets))
	for _, dataset := range prepared.Profile.Datasets {
		columns, err := mallWeatherExportRenderColumns(dataset)
		sheet, exists := bySheetID[prepared.Destination.SheetIDs[dataset.Kind]]
		if err != nil || !exists || sheet.ResourceType != "sheet" || sheet.GridProperties.RowCount < 1 ||
			sheet.GridProperties.RowCount > maxMallWeatherFeishuSheetRow ||
			sheet.GridProperties.ColumnCount < int64(len(columns)) {
			return nil, errors.New("mall weather feishu executor: target sheet capacity is invalid")
		}
		result[dataset.Kind] = sheet
	}
	return result, nil
}

func mallWeatherFeishuDestinationLockKey(destination *MallWeatherFeishuResolvedDestination) string {
	if destination == nil {
		return "invalid"
	}
	return "destination:" + strconv.FormatUint(uint64(destination.DestinationID), 10)
}

func classifyMallWeatherFeishuExecutorError(safeMessage string, err error) error {
	if err == nil {
		err = errors.New("mall weather feishu executor: unknown failure")
	}
	if errors.Is(err, ErrMallWeatherFeishuHeaderConflict) ||
		errors.Is(err, ErrMallWeatherFeishuAppendCheckpointConflict) ||
		errors.Is(err, ErrMallWeatherFeishuOverwriteCheckpointConflict) ||
		errors.Is(err, ErrMallWeatherFeishuOverwriteCleanupCheckpointConflict) {
		return permanentMallWeatherFeishuExecutionError(safeMessage, err)
	}
	var sheetsError *feishu.SheetsError
	if errors.As(err, &sheetsError) && sheetsError != nil && !sheetsError.Retryable {
		return permanentMallWeatherFeishuExecutionError(safeMessage, err)
	}
	return retryableMallWeatherFeishuExecutionError(safeMessage, err)
}

func permanentMallWeatherFeishuExecutionError(safeMessage string, cause error) error {
	return &MallWeatherFeishuExecutionError{Retryable: false, SafeMessage: safeMessage, cause: nonNilMallWeatherFeishuError(cause)}
}

func retryableMallWeatherFeishuExecutionError(safeMessage string, cause error) error {
	return &MallWeatherFeishuExecutionError{Retryable: true, SafeMessage: safeMessage, cause: nonNilMallWeatherFeishuError(cause)}
}
