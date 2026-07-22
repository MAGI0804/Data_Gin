package job

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gin-biz-web-api/model"

	"github.com/hibiken/asynq"
)

const (
	defaultOutboxPollInterval = time.Second
	defaultOutboxLockTimeout  = time.Minute
	defaultOutboxBatchSize    = 100
	defaultOutboxRetryBase    = 5 * time.Second
	defaultOutboxRetryMax     = 5 * time.Minute
	defaultWeatherTaskTimeout = 5 * time.Minute

	safeInvalidOutboxTaskError = "invalid outbox task"
	safeQueuePublishError      = "queue publish failed"
)

var (
	ErrInvalidOutboxTask = errors.New("invalid outbox task")
	ErrOutboxPublish     = errors.New("outbox publish failed")
	ErrOutboxState       = errors.New("outbox state update failed")
)

type OutboxStore interface {
	ClaimBatch(ctx context.Context, workerID string, now time.Time, lockTimeout time.Duration, limit int) ([]model.AsyncJobOutbox, error)
	MarkPublished(ctx context.Context, id uint, publishedAt time.Time) error
	MarkFailed(ctx context.Context, id uint, availableAt time.Time, safeError string) error
}

type TaskPublishOptions struct {
	TaskID   string
	Queue    string
	MaxRetry int
	Timeout  time.Duration
}

type MallWeatherTaskPublisher interface {
	Publish(ctx context.Context, task *asynq.Task, options TaskPublishOptions) error
}

type AsynqMallWeatherTaskPublisher struct {
	client *asynq.Client
}

func NewAsynqMallWeatherTaskPublisher(client *asynq.Client) *AsynqMallWeatherTaskPublisher {
	return &AsynqMallWeatherTaskPublisher{client: client}
}

func (publisher *AsynqMallWeatherTaskPublisher) Publish(ctx context.Context, task *asynq.Task, options TaskPublishOptions) error {
	if publisher == nil || publisher.client == nil {
		return fmt.Errorf("mall weather publisher: client is required")
	}
	optionsList := []asynq.Option{
		asynq.TaskID(options.TaskID),
		asynq.Queue(options.Queue),
		asynq.MaxRetry(options.MaxRetry),
	}
	if options.Timeout > 0 {
		optionsList = append(optionsList, asynq.Timeout(options.Timeout))
	}
	_, err := publisher.client.EnqueueContext(ctx, task, optionsList...)
	return err
}

type OutboxDispatcherConfig struct {
	WorkerID     string
	PollInterval time.Duration
	LockTimeout  time.Duration
	BatchSize    int
	RetryBase    time.Duration
	RetryMax     time.Duration
	MaxRetry     int
	TaskTimeout  time.Duration
	Now          func() time.Time
	OnCycleError func(error)
}

type OutboxDispatcher struct {
	store        OutboxStore
	publisher    MallWeatherTaskPublisher
	workerID     string
	pollInterval time.Duration
	lockTimeout  time.Duration
	batchSize    int
	retryBase    time.Duration
	retryMax     time.Duration
	maxRetry     int
	taskTimeout  time.Duration
	now          func() time.Time
	onCycleError func(error)
}

func NewOutboxDispatcher(store OutboxStore, publisher MallWeatherTaskPublisher, cfg OutboxDispatcherConfig) (*OutboxDispatcher, error) {
	if store == nil {
		return nil, fmt.Errorf("outbox dispatcher: store is required")
	}
	if publisher == nil {
		return nil, fmt.Errorf("outbox dispatcher: publisher is required")
	}
	if cfg.WorkerID == "" {
		return nil, fmt.Errorf("outbox dispatcher: worker id is required")
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = defaultOutboxPollInterval
	}
	if cfg.LockTimeout <= 0 {
		cfg.LockTimeout = defaultOutboxLockTimeout
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = defaultOutboxBatchSize
	}
	if cfg.RetryBase <= 0 {
		cfg.RetryBase = defaultOutboxRetryBase
	}
	if cfg.RetryMax <= 0 {
		cfg.RetryMax = defaultOutboxRetryMax
	}
	if cfg.RetryMax < cfg.RetryBase {
		return nil, fmt.Errorf("outbox dispatcher: retry max must not be less than retry base")
	}
	if cfg.MaxRetry < 0 {
		return nil, fmt.Errorf("outbox dispatcher: max retry must not be negative")
	}
	if cfg.TaskTimeout <= 0 {
		cfg.TaskTimeout = defaultWeatherTaskTimeout
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}

	return &OutboxDispatcher{
		store:        store,
		publisher:    publisher,
		workerID:     cfg.WorkerID,
		pollInterval: cfg.PollInterval,
		lockTimeout:  cfg.LockTimeout,
		batchSize:    cfg.BatchSize,
		retryBase:    cfg.RetryBase,
		retryMax:     cfg.RetryMax,
		maxRetry:     cfg.MaxRetry,
		taskTimeout:  cfg.TaskTimeout,
		now:          cfg.Now,
		onCycleError: cfg.OnCycleError,
	}, nil
}

// Run dispatches immediately, then on each ticker event, until ctx is
// cancelled. Transient cycle errors are reported and retried on the next tick.
func (dispatcher *OutboxDispatcher) Run(ctx context.Context) error {
	ticker := time.NewTicker(dispatcher.pollInterval)
	defer ticker.Stop()

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := dispatcher.DispatchOnce(ctx); err != nil && !errors.Is(err, context.Canceled) && dispatcher.onCycleError != nil {
			dispatcher.onCycleError(err)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (dispatcher *OutboxDispatcher) DispatchOnce(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	now := dispatcher.now().UTC()
	rows, err := dispatcher.store.ClaimBatch(ctx, dispatcher.workerID, now, dispatcher.lockTimeout, dispatcher.batchSize)
	if err != nil {
		return fmt.Errorf("outbox dispatcher: claim batch: %w", err)
	}

	var dispatchErrors []error
	for i := range rows {
		if err := ctx.Err(); err != nil {
			dispatchErrors = append(dispatchErrors, err)
			break
		}
		if err := dispatcher.dispatchRow(ctx, rows[i]); err != nil {
			dispatchErrors = append(dispatchErrors, err)
		}
	}
	return errors.Join(dispatchErrors...)
}

func (dispatcher *OutboxDispatcher) dispatchRow(ctx context.Context, row model.AsyncJobOutbox) error {
	expectedQueue, ok := ExpectedMallWeatherQueue(row.TaskType)
	if !ok || row.TaskKey == "" || row.QueueName != expectedQueue {
		return dispatcher.failRow(ctx, row, safeInvalidOutboxTaskError, ErrInvalidOutboxTask)
	}

	task, err := NewMallWeatherTask(row.TaskType, []byte(row.PayloadJSON))
	if err != nil {
		return dispatcher.failRow(ctx, row, safeInvalidOutboxTaskError, ErrInvalidOutboxTask)
	}

	publishOptions := TaskPublishOptions{
		TaskID:   row.TaskKey,
		Queue:    row.QueueName,
		MaxRetry: dispatcher.maxRetry,
	}
	if IsMallWeatherFetchTaskType(row.TaskType) {
		publishOptions.Timeout = dispatcher.taskTimeout
	}
	err = dispatcher.publisher.Publish(ctx, task, publishOptions)
	if err != nil && !errors.Is(err, asynq.ErrTaskIDConflict) {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return dispatcher.failRow(ctx, row, safeQueuePublishError, ErrOutboxPublish)
	}

	if err := dispatcher.store.MarkPublished(ctx, row.ID, dispatcher.now().UTC()); err != nil {
		return fmt.Errorf("%w: mark row %d published", ErrOutboxState, row.ID)
	}
	return nil
}

func (dispatcher *OutboxDispatcher) failRow(ctx context.Context, row model.AsyncJobOutbox, safeError string, cause error) error {
	delay := outboxBackoff(dispatcher.retryBase, dispatcher.retryMax, row.Attempts)
	if err := dispatcher.store.MarkFailed(ctx, row.ID, dispatcher.now().UTC().Add(delay), safeError); err != nil {
		return errors.Join(
			fmt.Errorf("%w: row %d", cause, row.ID),
			fmt.Errorf("%w: mark row %d failed", ErrOutboxState, row.ID),
		)
	}
	return fmt.Errorf("%w: row %d", cause, row.ID)
}

func outboxBackoff(base, maximum time.Duration, attempts int) time.Duration {
	if attempts < 0 {
		attempts = 0
	}
	delay := base
	for i := 0; i < attempts && delay < maximum; i++ {
		if delay > maximum/2 {
			return maximum
		}
		delay *= 2
	}
	if delay > maximum {
		return maximum
	}
	return delay
}
