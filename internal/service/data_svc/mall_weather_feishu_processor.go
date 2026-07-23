package data_svc

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gin-biz-web-api/internal/dao/data_dao"

	"github.com/google/uuid"
)

const (
	defaultMallWeatherFeishuRunStaleAfter      = 10 * time.Minute
	defaultMallWeatherFeishuHeartbeatInterval  = 30 * time.Second
	defaultMallWeatherFeishuStateUpdateTimeout = 10 * time.Second
)

var ErrMallWeatherFeishuProcessNonRetryable = errors.New(
	"mall weather feishu processor: non-retryable",
)

type MallWeatherFeishuExecutionError struct {
	Retryable   bool
	SafeMessage string
	cause       error
}

func (processError *MallWeatherFeishuExecutionError) Error() string {
	return "mall weather feishu processor: execution failed"
}

func (processError *MallWeatherFeishuExecutionError) Unwrap() error {
	if processError == nil {
		return nil
	}
	return processError.cause
}

type MallWeatherFeishuExecutionResult struct {
	SuccessCount int
	FailedCount  int
	SafeError    string
}

type mallWeatherFeishuRunStore interface {
	BeginRun(context.Context, uint, string, time.Time, time.Duration) (*data_dao.MallWeatherFeishuRunLease, error)
	HeartbeatRun(context.Context, uint, string, time.Time) error
	UpdateRunProgress(context.Context, uint, string, data_dao.MallWeatherFeishuRunProgress) error
	FinishRun(context.Context, uint, string, data_dao.MallWeatherFeishuRunFinish) error
	ReleaseRunForRetry(context.Context, uint, string, time.Time) error
}

type mallWeatherFeishuRunExecutor interface {
	Execute(
		context.Context,
		data_dao.MallWeatherFeishuRunRecord,
		func(successCount, failedCount int) error,
	) (MallWeatherFeishuExecutionResult, error)
}

type MallWeatherFeishuProcessor struct {
	runs              mallWeatherFeishuRunStore
	executor          mallWeatherFeishuRunExecutor
	now               func() time.Time
	newRunToken       func() string
	staleAfter        time.Duration
	heartbeatInterval time.Duration
	stateTimeout      time.Duration
}

func newMallWeatherFeishuProcessor(
	runs mallWeatherFeishuRunStore,
	executor mallWeatherFeishuRunExecutor,
	now func() time.Time,
	newRunToken func() string,
	staleAfter time.Duration,
	heartbeatInterval time.Duration,
	stateTimeout time.Duration,
) (*MallWeatherFeishuProcessor, error) {
	if runs == nil || executor == nil || now == nil || newRunToken == nil ||
		staleAfter <= 0 || heartbeatInterval <= 0 || heartbeatInterval >= staleAfter || stateTimeout <= 0 {
		return nil, errors.New("mall weather feishu processor: invalid configuration")
	}
	return &MallWeatherFeishuProcessor{
		runs: runs, executor: executor, now: now, newRunToken: newRunToken,
		staleAfter: staleAfter, heartbeatInterval: heartbeatInterval, stateTimeout: stateTimeout,
	}, nil
}

func (processor *MallWeatherFeishuProcessor) Process(
	ctx context.Context,
	pipelineRunID uint,
	retryAllowed bool,
) error {
	if processor == nil || processor.runs == nil || processor.executor == nil || processor.now == nil ||
		processor.newRunToken == nil || ctx == nil || pipelineRunID == 0 || processor.staleAfter <= 0 ||
		processor.heartbeatInterval <= 0 || processor.heartbeatInterval >= processor.staleAfter ||
		processor.stateTimeout <= 0 {
		return fmt.Errorf("%w: invalid processor", ErrMallWeatherFeishuProcessNonRetryable)
	}
	runToken := processor.newRunToken()
	if uuid.Validate(runToken) != nil {
		return fmt.Errorf("%w: invalid run token", ErrMallWeatherFeishuProcessNonRetryable)
	}
	startedAt := processor.now().UTC()
	lease, err := processor.runs.BeginRun(ctx, pipelineRunID, runToken, startedAt, processor.staleAfter)
	if err != nil {
		return fmt.Errorf("mall weather feishu processor: begin run: %w", err)
	}
	if lease == nil {
		return errors.New("mall weather feishu processor: nil run lease")
	}
	switch lease.Disposition {
	case data_dao.MallWeatherFeishuRunDispositionBusy, data_dao.MallWeatherFeishuRunDispositionTerminal:
		return nil
	case data_dao.MallWeatherFeishuRunDispositionAcquired:
	default:
		return errors.New("mall weather feishu processor: invalid run disposition")
	}
	if lease.RunToken != runToken || lease.Record.Pipeline.ID != pipelineRunID ||
		lease.Record.Detail.PipelineRunID != pipelineRunID {
		return processor.finishError(
			ctx,
			pipelineRunID,
			runToken,
			MallWeatherFeishuExecutionResult{},
			errors.New("mall weather feishu processor: acquired lease identity mismatch"),
			false,
			"飞书推送任务状态无效",
		)
	}
	monitor, runCtx := startMallWeatherFeishuRunMonitor(
		ctx,
		processor.runs,
		pipelineRunID,
		runToken,
		processor.heartbeatInterval,
		processor.now,
	)
	result, executeErr := processor.executor.Execute(
		runCtx,
		lease.Record,
		func(successCount, failedCount int) error {
			return processor.runs.UpdateRunProgress(runCtx, pipelineRunID, runToken, data_dao.MallWeatherFeishuRunProgress{
				SuccessCount: successCount,
				FailedCount:  failedCount,
				UpdatedAt:    processor.now().UTC(),
			})
		},
	)
	monitorErr := monitor.Stop()
	if executeErr != nil || monitorErr != nil {
		cause := errors.Join(executeErr, monitorErr)
		retryable, safeMessage := classifyMallWeatherFeishuExecutionError(cause)
		return processor.finishError(ctx, pipelineRunID, runToken, result, cause, retryable && retryAllowed, safeMessage)
	}
	finish, err := mallWeatherFeishuSuccessfulFinish(result, processor.now().UTC())
	if err != nil {
		return processor.finishError(
			ctx,
			pipelineRunID,
			runToken,
			result,
			err,
			false,
			"飞书推送结果无效",
		)
	}
	stateCtx, cancel := processor.stateContext(ctx)
	defer cancel()
	if err := processor.runs.FinishRun(stateCtx, pipelineRunID, runToken, finish); err != nil {
		if errors.Is(err, data_dao.ErrMallWeatherFeishuRunLeaseLost) {
			return nil
		}
		return processor.finishError(
			ctx,
			pipelineRunID,
			runToken,
			result,
			fmt.Errorf("mall weather feishu processor: finish run: %w", err),
			retryAllowed,
			"飞书推送状态保存失败",
		)
	}
	return nil
}

func (processor *MallWeatherFeishuProcessor) finishError(
	ctx context.Context,
	pipelineRunID uint,
	runToken string,
	result MallWeatherFeishuExecutionResult,
	cause error,
	retry bool,
	safeMessage string,
) error {
	if cause == nil {
		cause = errors.New("mall weather feishu processor: unknown failure")
	}
	if errors.Is(cause, data_dao.ErrMallWeatherFeishuRunLeaseLost) {
		return nil
	}
	stateCtx, cancel := processor.stateContext(ctx)
	defer cancel()
	now := processor.now().UTC()
	if retry {
		err := processor.runs.ReleaseRunForRetry(stateCtx, pipelineRunID, runToken, now)
		if errors.Is(err, data_dao.ErrMallWeatherFeishuRunLeaseLost) {
			return nil
		}
		return errors.Join(cause, err)
	}
	finish := mallWeatherFeishuFailedFinish(result, safeMessage, now)
	err := processor.runs.FinishRun(stateCtx, pipelineRunID, runToken, finish)
	if errors.Is(err, data_dao.ErrMallWeatherFeishuRunLeaseLost) {
		return nil
	}
	if err != nil {
		return errors.Join(cause, err)
	}
	return errors.Join(ErrMallWeatherFeishuProcessNonRetryable, cause)
}

func (processor *MallWeatherFeishuProcessor) stateContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), processor.stateTimeout)
}

func classifyMallWeatherFeishuExecutionError(err error) (bool, string) {
	safeMessage := "飞书推送执行失败"
	var executionError *MallWeatherFeishuExecutionError
	if errors.As(err, &executionError) && executionError != nil {
		if executionError.SafeMessage != "" {
			safeMessage = executionError.SafeMessage
		}
		return executionError.Retryable, safeMessage
	}
	return true, safeMessage
}

func mallWeatherFeishuSuccessfulFinish(
	result MallWeatherFeishuExecutionResult,
	finishedAt time.Time,
) (data_dao.MallWeatherFeishuRunFinish, error) {
	finish := data_dao.MallWeatherFeishuRunFinish{
		SuccessCount: result.SuccessCount,
		FailedCount:  result.FailedCount,
		SafeError:    result.SafeError,
		FinishedAt:   finishedAt,
	}
	switch {
	case result.SuccessCount < 0 || result.FailedCount < 0 || finishedAt.IsZero():
		return finish, errors.New("mall weather feishu processor: invalid execution result")
	case result.FailedCount == 0 && result.SafeError == "":
		finish.Status = "success"
	case result.SuccessCount > 0 && result.FailedCount > 0 && result.SafeError != "":
		finish.Status = "partial_success"
	case result.SuccessCount == 0 && result.FailedCount > 0 && result.SafeError != "":
		finish.Status = "failed"
	default:
		return finish, errors.New("mall weather feishu processor: inconsistent execution result")
	}
	return finish, nil
}

func mallWeatherFeishuFailedFinish(
	result MallWeatherFeishuExecutionResult,
	safeMessage string,
	finishedAt time.Time,
) data_dao.MallWeatherFeishuRunFinish {
	if result.SuccessCount < 0 {
		result.SuccessCount = 0
	}
	if result.FailedCount < 1 {
		result.FailedCount = 1
	}
	status := "failed"
	if result.SuccessCount > 0 {
		status = "partial_success"
	}
	return data_dao.MallWeatherFeishuRunFinish{
		Status: status, SuccessCount: result.SuccessCount, FailedCount: result.FailedCount,
		SafeError: safeMessage, FinishedAt: finishedAt,
	}
}

type mallWeatherFeishuRunMonitor struct {
	stop      context.CancelFunc
	cancelRun context.CancelFunc
	done      chan struct{}
	result    chan error
}

func startMallWeatherFeishuRunMonitor(
	ctx context.Context,
	runs mallWeatherFeishuRunStore,
	pipelineRunID uint,
	runToken string,
	interval time.Duration,
	now func() time.Time,
) (*mallWeatherFeishuRunMonitor, context.Context) {
	runCtx, cancelRun := context.WithCancel(ctx)
	monitorCtx, stop := context.WithCancel(ctx)
	monitor := &mallWeatherFeishuRunMonitor{
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
				if err := runs.HeartbeatRun(monitorCtx, pipelineRunID, runToken, now().UTC()); err != nil {
					monitor.result <- err
					cancelRun()
					return
				}
			}
		}
	}()
	return monitor, runCtx
}

func (monitor *mallWeatherFeishuRunMonitor) Stop() error {
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
