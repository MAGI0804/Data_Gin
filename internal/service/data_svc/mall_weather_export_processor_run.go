package data_svc

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/storage"

	"github.com/google/uuid"
)

const (
	defaultMallWeatherExportRunStaleAfter      = 10 * time.Minute
	defaultMallWeatherExportHeartbeatInterval  = 30 * time.Second
	defaultMallWeatherExportRetention          = 7 * 24 * time.Hour
	defaultMallWeatherExportStateUpdateTimeout = 10 * time.Second
)

var (
	ErrMallWeatherExportProcessNonRetryable = errors.New("mall weather export processor: non-retryable")
	errMallWeatherExportProcessCancelled    = errors.New("mall weather export processor: cancelled")
)

type mallWeatherExportRunStore interface {
	BeginRun(context.Context, uint, string, time.Time, time.Duration) (*data_dao.MallWeatherExportRunLease, error)
	UpdateRunProgress(
		context.Context,
		uint,
		string,
		data_dao.MallWeatherExportRunProgress,
	) (data_dao.MallWeatherExportRunControl, error)
	HeartbeatRun(context.Context, uint, string, time.Time) (data_dao.MallWeatherExportRunControl, error)
	InspectRun(context.Context, uint, string) (data_dao.MallWeatherExportRunControl, error)
	MarkRunSucceeded(context.Context, uint, string, string, string, int64, time.Time, time.Time) error
	ConfirmRunSucceeded(context.Context, uint, string, string, int64) (bool, error)
	MarkRunFailed(context.Context, uint, string, string, time.Time, time.Time) error
	MarkRunCancelled(context.Context, uint, string, time.Time, time.Time) error
	ReleaseRunForRetry(context.Context, uint, string, time.Time) error
}

type mallWeatherExportWorkbookRenderer interface {
	Render(
		context.Context,
		MallWeatherExportRenderRequest,
		func(MallWeatherExportRenderProgress) error,
	) (MallWeatherExportRenderResult, error)
}

type mallWeatherExportObjectStore interface {
	UploadFile(context.Context, string, string, string) (storage.UploadResult, error)
	DeleteObject(context.Context, string) error
}

type mallWeatherExportObjectStoreFactory func() (mallWeatherExportObjectStore, error)

type MallWeatherExportProcessor struct {
	runs              mallWeatherExportRunStore
	renderer          mallWeatherExportWorkbookRenderer
	newObjectStore    mallWeatherExportObjectStoreFactory
	buildObjectKey    func(...string) string
	now               func() time.Time
	newRunToken       func() string
	metrics           mallWeatherMetricRecorder
	workRoot          string
	staleAfter        time.Duration
	heartbeatInterval time.Duration
	retention         time.Duration
}

func NewMallWeatherExportProcessor() *MallWeatherExportProcessor {
	return &MallWeatherExportProcessor{
		runs:     data_dao.NewMallWeatherExportJobDAO(),
		renderer: NewMallWeatherExportRenderer(),
		newObjectStore: func() (mallWeatherExportObjectStore, error) {
			return storage.NewOSSClientFromConfig()
		},
		buildObjectKey:    storage.BuildObjectKey,
		now:               time.Now,
		newRunToken:       uuid.NewString,
		metrics:           mallWeatherRuntimeMetrics,
		workRoot:          excelTempRootDir(),
		staleAfter:        defaultMallWeatherExportRunStaleAfter,
		heartbeatInterval: defaultMallWeatherExportHeartbeatInterval,
		retention:         defaultMallWeatherExportRetention,
	}
}

func (processor *MallWeatherExportProcessor) Process(
	ctx context.Context,
	jobID uint,
	retryAllowed bool,
) (returnErr error) {
	if err := processor.validate(ctx, jobID); err != nil {
		return err
	}
	runToken := processor.newRunToken()
	startedAt := processor.now().UTC()
	lease, err := processor.runs.BeginRun(ctx, jobID, runToken, startedAt, processor.staleAfter)
	if err != nil {
		return fmt.Errorf("mall weather export processor: begin run: %w", err)
	}
	if lease == nil {
		return fmt.Errorf("mall weather export processor: nil run lease")
	}
	switch lease.Disposition {
	case data_dao.MallWeatherExportRunDispositionBusy, data_dao.MallWeatherExportRunDispositionTerminal:
		return nil
	case data_dao.MallWeatherExportRunDispositionAcquired:
	default:
		return fmt.Errorf("mall weather export processor: invalid run disposition")
	}
	if lease.RunToken != runToken || lease.Job.ID != jobID {
		return fmt.Errorf("mall weather export processor: acquired lease identity mismatch")
	}
	prepared, err := prepareMallWeatherExportJob(lease.Job, startedAt)
	if err != nil {
		return processor.finishError(ctx, jobID, runToken, err, true, false, "导出任务配置无效")
	}
	workDir, err := createMallWeatherExportWorkDir(processor.workRoot, jobID, runToken)
	if err != nil {
		return processor.finishError(ctx, jobID, runToken, err, false, retryAllowed, "导出工作目录创建失败")
	}
	defer func() {
		if err := os.RemoveAll(workDir); err != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("mall weather export processor: remove work directory: %w", err))
		}
	}()
	returnErr = processor.processOwnedRun(
		ctx,
		lease.Job,
		runToken,
		prepared,
		workDir,
		retryAllowed,
	)
	return returnErr
}

func (processor *MallWeatherExportProcessor) processOwnedRun(
	ctx context.Context,
	job model.MallWeatherExportJob,
	runToken string,
	prepared mallWeatherExportPreparedJob,
	workDir string,
	retryAllowed bool,
) error {
	monitor, runCtx := startMallWeatherExportRunMonitor(
		ctx,
		processor.runs,
		job.ID,
		runToken,
		processor.heartbeatInterval,
		processor.now,
	)
	outputPath := filepath.Join(workDir, prepared.FileName)
	var renderResult MallWeatherExportRenderResult
	var lastProgress MallWeatherExportRenderProgress
	renderErr := withExcelizeTempDir(filepath.Join(workDir, excelizeTempDirName), func() error {
		var err error
		renderResult, err = processor.renderer.Render(runCtx, MallWeatherExportRenderRequest{
			Config:        prepared.Config,
			Filter:        prepared.Filter,
			SnapshotAt:    job.CreatedAt.UTC(),
			EstimatedRows: job.TotalRows,
			OutputPath:    outputPath,
		}, func(progress MallWeatherExportRenderProgress) error {
			if err := processor.updateProgress(runCtx, job.ID, runToken, progress); err != nil {
				return err
			}
			lastProgress = progress
			return nil
		})
		return err
	})
	if renderErr != nil {
		monitorErr := monitor.Stop()
		return processor.finishError(
			ctx,
			job.ID,
			runToken,
			errors.Join(renderErr, monitorErr),
			false,
			retryAllowed,
			"导出文件生成失败",
		)
	}
	if renderResult.ProcessedRows < lastProgress.ProcessedRows {
		monitorErr := monitor.Stop()
		return processor.finishError(
			ctx,
			job.ID,
			runToken,
			errors.Join(fmt.Errorf("mall weather export processor: invalid rendered row count"), monitorErr),
			true,
			false,
			"导出文件结果无效",
		)
	}
	if renderResult.ProcessedRows != lastProgress.ProcessedRows {
		lastProgress.ProcessedRows = renderResult.ProcessedRows
		if err := processor.updateProgress(runCtx, job.ID, runToken, lastProgress); err != nil {
			monitorErr := monitor.Stop()
			return processor.finishError(ctx, job.ID, runToken, errors.Join(err, monitorErr), false, retryAllowed, "导出进度更新失败")
		}
	}
	checksum, fileSize, err := inspectMallWeatherExportArtifact(outputPath)
	if err != nil {
		monitorErr := monitor.Stop()
		return processor.finishError(ctx, job.ID, runToken, errors.Join(err, monitorErr), false, retryAllowed, "导出文件校验失败")
	}
	objectKeySuffix, err := mallWeatherExportObjectKey(job.JobUUID, runToken, processor.now().UTC())
	if err != nil {
		monitorErr := monitor.Stop()
		return processor.finishError(ctx, job.ID, runToken, errors.Join(err, monitorErr), true, false, "导出对象标识无效")
	}
	objectKey := processor.buildObjectKey(objectKeySuffix)
	objectStore, err := processor.newObjectStore()
	if objectKey == "" && err == nil {
		err = fmt.Errorf("mall weather export processor: empty object key")
	}
	if objectStore == nil && err == nil {
		err = fmt.Errorf("mall weather export processor: nil object store")
	}
	if err == nil {
		_, err = objectStore.UploadFile(runCtx, objectKey, outputPath, prepared.FileName)
	}
	monitorErr := monitor.Stop()
	if err != nil || monitorErr != nil {
		if err == nil {
			err = processor.deleteObject(ctx, objectStore, objectKey)
		}
		return processor.finishError(
			ctx,
			job.ID,
			runToken,
			errors.Join(err, monitorErr),
			false,
			retryAllowed,
			"导出文件上传失败",
		)
	}
	finishedAt := processor.now().UTC()
	err = processor.runs.MarkRunSucceeded(
		ctx,
		job.ID,
		runToken,
		objectKey,
		checksum,
		fileSize,
		finishedAt,
		finishedAt.Add(processor.retention),
	)
	if err == nil {
		recordMallWeatherExportRows(processor.metrics, renderResult)
		recordMallWeatherExportRun(processor.metrics, mallWeatherMetricStatusSucceeded)
		return nil
	}
	confirmed, confirmErr := processor.confirmRunSucceeded(ctx, job.ID, objectKey, checksum, fileSize)
	if confirmed {
		recordMallWeatherExportRows(processor.metrics, renderResult)
		recordMallWeatherExportRun(processor.metrics, mallWeatherMetricStatusSucceeded)
		return nil
	}
	if errors.Is(err, data_dao.ErrMallWeatherExportRunLeaseLost) {
		var deleteErr error
		if confirmErr == nil {
			deleteErr = processor.deleteObject(ctx, objectStore, objectKey)
		}
		control, inspectErr := processor.inspectRun(ctx, job.ID, runToken)
		if inspectErr == nil && control == data_dao.MallWeatherExportRunControlCancelRequested {
			return processor.finishError(ctx, job.ID, runToken, errMallWeatherExportProcessCancelled, false, false, "导出任务已取消")
		}
		return errors.Join(confirmErr, deleteErr, inspectErr)
	}
	return processor.finishError(
		ctx,
		job.ID,
		runToken,
		errors.Join(err, confirmErr),
		false,
		retryAllowed,
		"导出任务完成状态更新失败",
	)
}

func (processor *MallWeatherExportProcessor) updateProgress(
	ctx context.Context,
	jobID uint,
	runToken string,
	progress MallWeatherExportRenderProgress,
) error {
	control, err := processor.runs.UpdateRunProgress(ctx, jobID, runToken, data_dao.MallWeatherExportRunProgress{
		ProcessedRows: progress.ProcessedRows,
		CurrentSheet:  progress.CurrentSheet,
		Checkpoint: data_dao.MallWeatherExportRunCheckpoint{
			RunToken:     runToken,
			DatasetIndex: progress.DatasetIndex,
			SheetIndex:   progress.SheetIndex,
			RowsInSheet:  progress.RowsInSheet,
			Cursor:       progress.Cursor,
		},
		UpdatedAt: processor.now().UTC(),
	})
	if err != nil {
		return fmt.Errorf("mall weather export processor: update progress: %w", err)
	}
	return mallWeatherExportRunControlError(control)
}

func (processor *MallWeatherExportProcessor) validate(ctx context.Context, jobID uint) error {
	if processor == nil || processor.runs == nil || processor.renderer == nil || processor.newObjectStore == nil ||
		processor.buildObjectKey == nil ||
		processor.now == nil || processor.newRunToken == nil || processor.metrics == nil ||
		ctx == nil || jobID == 0 || processor.workRoot == "" ||
		processor.staleAfter <= 0 || processor.heartbeatInterval <= 0 || processor.retention <= 0 {
		return fmt.Errorf("mall weather export processor: invalid configuration")
	}
	return nil
}

func (processor *MallWeatherExportProcessor) finishError(
	ctx context.Context,
	jobID uint,
	runToken string,
	cause error,
	permanent bool,
	retryAllowed bool,
	safeMessage string,
) error {
	if cause == nil {
		cause = fmt.Errorf("mall weather export processor: unknown failure")
	}
	if errors.Is(cause, data_dao.ErrMallWeatherExportRunLeaseLost) {
		return nil
	}
	stateCtx, cancel := mallWeatherExportStateContext(ctx)
	defer cancel()
	now := processor.now().UTC()
	if errors.Is(cause, errMallWeatherExportProcessCancelled) {
		err := processor.runs.MarkRunCancelled(stateCtx, jobID, runToken, now, now.Add(processor.retention))
		if errors.Is(err, data_dao.ErrMallWeatherExportRunLeaseLost) {
			return nil
		}
		if err == nil {
			recordMallWeatherExportRun(processor.metrics, "cancelled")
		}
		return err
	}
	if !permanent && retryAllowed {
		err := processor.runs.ReleaseRunForRetry(stateCtx, jobID, runToken, now)
		if errors.Is(err, data_dao.ErrMallWeatherExportRunLeaseLost) {
			return nil
		}
		return errors.Join(cause, err)
	}
	err := processor.runs.MarkRunFailed(stateCtx, jobID, runToken, safeMessage, now, now.Add(processor.retention))
	if errors.Is(err, data_dao.ErrMallWeatherExportRunLeaseLost) {
		return nil
	}
	if err != nil {
		return errors.Join(cause, err)
	}
	recordMallWeatherExportRun(processor.metrics, mallWeatherMetricStatusFailed)
	return errors.Join(ErrMallWeatherExportProcessNonRetryable, cause)
}

func (processor *MallWeatherExportProcessor) deleteObject(
	ctx context.Context,
	objectStore mallWeatherExportObjectStore,
	objectKey string,
) error {
	if objectStore == nil || objectKey == "" {
		return nil
	}
	stateCtx, cancel := mallWeatherExportStateContext(ctx)
	defer cancel()
	if err := objectStore.DeleteObject(stateCtx, objectKey); err != nil {
		return fmt.Errorf("mall weather export processor: delete uploaded object: %w", err)
	}
	return nil
}

func (processor *MallWeatherExportProcessor) inspectRun(
	ctx context.Context,
	jobID uint,
	runToken string,
) (data_dao.MallWeatherExportRunControl, error) {
	stateCtx, cancel := mallWeatherExportStateContext(ctx)
	defer cancel()
	return processor.runs.InspectRun(stateCtx, jobID, runToken)
}

func (processor *MallWeatherExportProcessor) confirmRunSucceeded(
	ctx context.Context,
	jobID uint,
	objectKey string,
	checksum string,
	fileSize int64,
) (bool, error) {
	stateCtx, cancel := mallWeatherExportStateContext(ctx)
	defer cancel()
	confirmed, err := processor.runs.ConfirmRunSucceeded(stateCtx, jobID, objectKey, checksum, fileSize)
	if err != nil {
		return false, fmt.Errorf("mall weather export processor: confirm completed run: %w", err)
	}
	return confirmed, nil
}

func mallWeatherExportStateContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), defaultMallWeatherExportStateUpdateTimeout)
}

func mallWeatherExportRunControlError(control data_dao.MallWeatherExportRunControl) error {
	switch control {
	case data_dao.MallWeatherExportRunControlContinue:
		return nil
	case data_dao.MallWeatherExportRunControlCancelRequested:
		return errMallWeatherExportProcessCancelled
	case data_dao.MallWeatherExportRunControlLeaseLost:
		return data_dao.ErrMallWeatherExportRunLeaseLost
	default:
		return fmt.Errorf("mall weather export processor: unknown run control")
	}
}

type mallWeatherExportRunMonitor struct {
	stop      context.CancelFunc
	cancelRun context.CancelFunc
	done      chan struct{}
	result    chan error
}

func startMallWeatherExportRunMonitor(
	ctx context.Context,
	runs mallWeatherExportRunStore,
	jobID uint,
	runToken string,
	interval time.Duration,
	now func() time.Time,
) (*mallWeatherExportRunMonitor, context.Context) {
	runCtx, cancelRun := context.WithCancel(ctx)
	monitorCtx, stop := context.WithCancel(ctx)
	monitor := &mallWeatherExportRunMonitor{
		stop: stop, cancelRun: cancelRun, done: make(chan struct{}), result: make(chan error, 1),
	}
	go func() {
		defer close(monitor.done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-monitorCtx.Done():
				return
			case <-ticker.C:
				control, err := runs.HeartbeatRun(monitorCtx, jobID, runToken, now().UTC())
				if err == nil {
					err = mallWeatherExportRunControlError(control)
				}
				if err != nil {
					monitor.result <- err
					cancelRun()
					return
				}
			}
		}
	}()
	return monitor, runCtx
}

func (monitor *mallWeatherExportRunMonitor) Stop() error {
	if monitor == nil {
		return nil
	}
	monitor.stop()
	<-monitor.done
	monitor.cancelRun()
	select {
	case err := <-monitor.result:
		return err
	default:
		return nil
	}
}
