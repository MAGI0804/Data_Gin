package job

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"gin-biz-web-api/model"

	"github.com/hibiken/asynq"
)

type fakeOutboxStore struct {
	mu              sync.Mutex
	rows            []model.AsyncJobOutbox
	claimErr        error
	claimCalls      int
	published       []publishedOutboxRow
	failed          []failedOutboxRow
	firstClaimReady chan struct{}
	claimReadyOnce  sync.Once
}

type publishedOutboxRow struct {
	id          uint
	publishedAt time.Time
}

type failedOutboxRow struct {
	id          uint
	availableAt time.Time
	safeError   string
}

func (store *fakeOutboxStore) ClaimBatch(_ context.Context, _ string, _ time.Time, _ time.Duration, _ int) ([]model.AsyncJobOutbox, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.claimCalls++
	if store.firstClaimReady != nil {
		store.claimReadyOnce.Do(func() { close(store.firstClaimReady) })
	}
	return append([]model.AsyncJobOutbox(nil), store.rows...), store.claimErr
}

func (store *fakeOutboxStore) MarkPublished(_ context.Context, id uint, publishedAt time.Time) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.published = append(store.published, publishedOutboxRow{id: id, publishedAt: publishedAt})
	return nil
}

func (store *fakeOutboxStore) MarkFailed(_ context.Context, id uint, availableAt time.Time, safeError string) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.failed = append(store.failed, failedOutboxRow{id: id, availableAt: availableAt, safeError: safeError})
	return nil
}

type fakeMallWeatherPublisher struct {
	calls []publishCall
	err   error
}

type publishCall struct {
	taskType string
	payload  string
	options  TaskPublishOptions
}

func (publisher *fakeMallWeatherPublisher) Publish(_ context.Context, task *asynq.Task, options TaskPublishOptions) error {
	publisher.calls = append(publisher.calls, publishCall{
		taskType: task.Type(),
		payload:  string(task.Payload()),
		options:  options,
	})
	return publisher.err
}

func TestOutboxDispatcherDispatchOncePublishesTask(t *testing.T) {
	now := time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC)
	store := &fakeOutboxStore{rows: []model.AsyncJobOutbox{weatherOutboxRow(1, 0)}}
	publisher := &fakeMallWeatherPublisher{}
	dispatcher := newTestOutboxDispatcher(t, store, publisher, now)

	if err := dispatcher.DispatchOnce(context.Background()); err != nil {
		t.Fatalf("DispatchOnce() error = %v", err)
	}
	if len(publisher.calls) != 1 {
		t.Fatalf("publish calls = %d, want 1", len(publisher.calls))
	}
	call := publisher.calls[0]
	if call.taskType != TypeMallWeatherFull || call.payload != `{"mall_id":123,"task_window":"full:123:2026071710"}` {
		t.Fatalf("published task = %q %s", call.taskType, call.payload)
	}
	if call.options.TaskID != "weather:full:123:2026071710" || call.options.Queue != MallWeatherQueueName || call.options.MaxRetry != 2 {
		t.Fatalf("publish options = %+v", call.options)
	}
	if len(store.published) != 1 || store.published[0].id != 1 || !store.published[0].publishedAt.Equal(now) {
		t.Fatalf("published rows = %+v", store.published)
	}
	if len(store.failed) != 0 {
		t.Fatalf("failed rows = %+v", store.failed)
	}
}

func TestOutboxDispatcherTreatsTaskIDConflictAsPublished(t *testing.T) {
	now := time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC)
	store := &fakeOutboxStore{rows: []model.AsyncJobOutbox{weatherOutboxRow(2, 0)}}
	publisher := &fakeMallWeatherPublisher{err: asynq.ErrTaskIDConflict}
	dispatcher := newTestOutboxDispatcher(t, store, publisher, now)

	if err := dispatcher.DispatchOnce(context.Background()); err != nil {
		t.Fatalf("DispatchOnce() error = %v", err)
	}
	if len(store.published) != 1 || store.published[0].id != 2 {
		t.Fatalf("published rows = %+v", store.published)
	}
	if len(store.failed) != 0 {
		t.Fatalf("failed rows = %+v", store.failed)
	}
}

func TestOutboxDispatcherStoresSafeFailureAndBackoff(t *testing.T) {
	now := time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC)
	store := &fakeOutboxStore{rows: []model.AsyncJobOutbox{weatherOutboxRow(3, 2)}}
	publisher := &fakeMallWeatherPublisher{err: errors.New("redis://user:top-secret@queue.invalid")}
	dispatcher := newTestOutboxDispatcher(t, store, publisher, now)

	err := dispatcher.DispatchOnce(context.Background())
	if !errors.Is(err, ErrOutboxPublish) {
		t.Fatalf("DispatchOnce() error = %v, want ErrOutboxPublish", err)
	}
	if strings.Contains(err.Error(), "top-secret") {
		t.Fatalf("DispatchOnce() exposed publisher error: %v", err)
	}
	if len(store.failed) != 1 {
		t.Fatalf("failed rows = %+v", store.failed)
	}
	failure := store.failed[0]
	if failure.id != 3 || failure.safeError != safeQueuePublishError {
		t.Fatalf("failure = %+v", failure)
	}
	if want := now.Add(4 * time.Second); !failure.availableAt.Equal(want) {
		t.Fatalf("available at = %v, want %v", failure.availableAt, want)
	}
	if len(store.published) != 0 {
		t.Fatalf("published rows = %+v", store.published)
	}
}

func TestOutboxDispatcherRejectsUnsafePayloadBeforePublish(t *testing.T) {
	now := time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC)
	row := weatherOutboxRow(4, 0)
	row.PayloadJSON = `{"mall_id":123,"task_window":"full:123:2026071710","app_secret":"do-not-queue"}`
	store := &fakeOutboxStore{rows: []model.AsyncJobOutbox{row}}
	publisher := &fakeMallWeatherPublisher{}
	dispatcher := newTestOutboxDispatcher(t, store, publisher, now)

	err := dispatcher.DispatchOnce(context.Background())
	if !errors.Is(err, ErrInvalidOutboxTask) {
		t.Fatalf("DispatchOnce() error = %v, want ErrInvalidOutboxTask", err)
	}
	if len(publisher.calls) != 0 {
		t.Fatalf("publish calls = %d, want 0", len(publisher.calls))
	}
	if len(store.failed) != 1 || store.failed[0].safeError != safeInvalidOutboxTaskError {
		t.Fatalf("failed rows = %+v", store.failed)
	}
}

func TestOutboxDispatcherRunStopsOnCancellation(t *testing.T) {
	now := time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC)
	firstClaimReady := make(chan struct{})
	store := &fakeOutboxStore{firstClaimReady: firstClaimReady}
	publisher := &fakeMallWeatherPublisher{}
	dispatcher := newTestOutboxDispatcher(t, store, publisher, now)
	dispatcher.pollInterval = time.Hour

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- dispatcher.Run(ctx) }()
	<-firstClaimReady
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run() error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run() did not stop after cancellation")
	}
}

func TestOutboxBackoffIsBounded(t *testing.T) {
	tests := []struct {
		name     string
		attempts int
		want     time.Duration
	}{
		{"negative", -1, time.Second},
		{"first", 0, time.Second},
		{"third", 2, 4 * time.Second},
		{"capped", 20, 10 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := outboxBackoff(time.Second, 10*time.Second, tt.attempts); got != tt.want {
				t.Fatalf("outboxBackoff() = %v, want %v", got, tt.want)
			}
		})
	}
}

func newTestOutboxDispatcher(t *testing.T, store OutboxStore, publisher MallWeatherTaskPublisher, now time.Time) *OutboxDispatcher {
	t.Helper()
	dispatcher, err := NewOutboxDispatcher(store, publisher, OutboxDispatcherConfig{
		WorkerID:     "test-dispatcher",
		PollInterval: time.Second,
		LockTimeout:  time.Minute,
		BatchSize:    10,
		RetryBase:    time.Second,
		RetryMax:     10 * time.Second,
		MaxRetry:     2,
		Now:          func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewOutboxDispatcher() error = %v", err)
	}
	return dispatcher
}

func weatherOutboxRow(id uint, attempts int) model.AsyncJobOutbox {
	return model.AsyncJobOutbox{
		BaseModel:   model.BaseModel{ID: id},
		TaskKey:     "weather:full:123:2026071710",
		TaskType:    TypeMallWeatherFull,
		PayloadJSON: model.JSONText(`{"mall_id":123,"task_window":"full:123:2026071710"}`),
		QueueName:   MallWeatherQueueName,
		Attempts:    attempts,
	}
}
