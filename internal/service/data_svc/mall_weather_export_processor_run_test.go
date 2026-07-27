package data_svc

import (
	"context"
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/storage"

	"github.com/google/uuid"
)

func TestMallWeatherExportProcessorCompletesOwnedRun(t *testing.T) {
	runToken := uuid.NewString()
	store := newFakeMallWeatherExportRunStore(t)
	var renderedDir string
	renderer := mallWeatherExportRendererFunc(func(
		_ context.Context,
		request MallWeatherExportRenderRequest,
		onProgress func(MallWeatherExportRenderProgress) error,
	) (MallWeatherExportRenderResult, error) {
		renderedDir = filepath.Dir(request.OutputPath)
		if !strings.HasPrefix(os.TempDir(), renderedDir) {
			t.Fatalf("Excelize temp dir=%q, want under %q", os.TempDir(), renderedDir)
		}
		if err := onProgress(MallWeatherExportRenderProgress{
			ProcessedRows: 1, CurrentSheet: "商场", Cursor: []byte(`{"afterId":1}`),
		}); err != nil {
			return MallWeatherExportRenderResult{}, err
		}
		if err := os.WriteFile(request.OutputPath, []byte("xlsx artifact"), 0o600); err != nil {
			return MallWeatherExportRenderResult{}, err
		}
		return MallWeatherExportRenderResult{ProcessedRows: 1, SheetCount: 1}, nil
	})
	objectStore := &fakeMallWeatherExportObjectStore{}
	processor := newTestMallWeatherExportProcessor(t, store, renderer, objectStore, runToken)
	if err := processor.Process(t.Context(), 17, true); err != nil {
		t.Fatalf("Process() error=%v", err)
	}
	state := store.snapshot()
	if state.succeededKey == "" || !strings.Contains(state.succeededKey, runToken) ||
		state.succeededChecksum == "" || state.progressUpdates != 1 {
		t.Fatalf("store state=%+v", state)
	}
	if objectStore.uploadedKey != state.succeededKey || objectStore.downloadName == "" {
		t.Fatalf("object store=%+v", objectStore)
	}
	if _, err := os.Stat(renderedDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("work directory was not removed: %v", err)
	}
}

func TestMallWeatherExportProcessorKeepsObjectWhenSuccessCommitIsAmbiguous(t *testing.T) {
	runToken := uuid.NewString()
	store := newFakeMallWeatherExportRunStore(t)
	store.succeedErr = errors.New("database response lost after commit")
	store.commitSuccessOnError = true
	objectStore := &fakeMallWeatherExportObjectStore{}
	metrics := &fakeMallWeatherMetricRecorder{}
	processor := newTestMallWeatherExportProcessorWithMetrics(
		t,
		store,
		mallWeatherExportRendererFunc(writeFakeMallWeatherExportArtifact),
		objectStore,
		runToken,
		metrics,
	)

	if err := processor.Process(t.Context(), 17, true); err != nil {
		t.Fatalf("Process() error=%v", err)
	}
	state := store.snapshot()
	if state.successConfirmations != 1 || objectStore.deletedKey != "" ||
		state.released != 0 || state.failed != 0 {
		t.Fatalf("store state=%+v object store=%+v", state, objectStore)
	}
	assertFakeMallWeatherMetricCounters(t, metrics.counters, []fakeMallWeatherMetricCounter{{
		name: MallWeatherMetricExportRowsTotal, value: 1,
	}, {
		name: MallWeatherMetricExportRunsTotal, labels: map[string]string{"status": mallWeatherMetricStatusSucceeded}, value: 1,
	}})
}

func TestMallWeatherExportProcessorPreservesObjectAfterAmbiguousDatabaseFailure(t *testing.T) {
	runToken := uuid.NewString()
	store := newFakeMallWeatherExportRunStore(t)
	store.succeedErr = errors.New("database update result unknown")
	objectStore := &fakeMallWeatherExportObjectStore{}
	processor := newTestMallWeatherExportProcessor(
		t,
		store,
		mallWeatherExportRendererFunc(writeFakeMallWeatherExportArtifact),
		objectStore,
		runToken,
	)

	err := processor.Process(t.Context(), 17, true)
	if err == nil {
		t.Fatal("Process() returned nil error")
	}
	state := store.snapshot()
	if state.successConfirmations != 1 || objectStore.deletedKey != "" || state.released != 1 {
		t.Fatalf("store state=%+v object store=%+v", state, objectStore)
	}
}

func TestMallWeatherExportProcessorDeletesObjectAfterConfirmedLeaseLoss(t *testing.T) {
	runToken := uuid.NewString()
	store := newFakeMallWeatherExportRunStore(t)
	store.succeedErr = data_dao.ErrMallWeatherExportRunLeaseLost
	store.heartbeatControl = data_dao.MallWeatherExportRunControlLeaseLost
	objectStore := &fakeMallWeatherExportObjectStore{}
	processor := newTestMallWeatherExportProcessor(
		t,
		store,
		mallWeatherExportRendererFunc(writeFakeMallWeatherExportArtifact),
		objectStore,
		runToken,
	)

	if err := processor.Process(t.Context(), 17, true); err != nil {
		t.Fatalf("Process() error=%v", err)
	}
	state := store.snapshot()
	if state.successConfirmations != 1 || objectStore.deletedKey == "" ||
		objectStore.deletedKey != objectStore.uploadedKey || state.released != 0 || state.failed != 0 {
		t.Fatalf("store state=%+v object store=%+v", state, objectStore)
	}
}

func TestMallWeatherExportProcessorCancelsAfterConfirmedLeaseLoss(t *testing.T) {
	runToken := uuid.NewString()
	store := newFakeMallWeatherExportRunStore(t)
	store.succeedErr = data_dao.ErrMallWeatherExportRunLeaseLost
	store.heartbeatControl = data_dao.MallWeatherExportRunControlCancelRequested
	objectStore := &fakeMallWeatherExportObjectStore{}
	processor := newTestMallWeatherExportProcessor(
		t,
		store,
		mallWeatherExportRendererFunc(writeFakeMallWeatherExportArtifact),
		objectStore,
		runToken,
	)

	if err := processor.Process(t.Context(), 17, true); err != nil {
		t.Fatalf("Process() error=%v", err)
	}
	state := store.snapshot()
	if state.successConfirmations != 1 || objectStore.deletedKey == "" || state.cancelled != 1 {
		t.Fatalf("store state=%+v object store=%+v", state, objectStore)
	}
}

func TestMallWeatherExportProcessorPreservesObjectOnFinalSuccessUpdateFailure(t *testing.T) {
	runToken := uuid.NewString()
	store := newFakeMallWeatherExportRunStore(t)
	store.succeedErr = errors.New("database update result unknown")
	objectStore := &fakeMallWeatherExportObjectStore{}
	processor := newTestMallWeatherExportProcessor(
		t,
		store,
		mallWeatherExportRendererFunc(writeFakeMallWeatherExportArtifact),
		objectStore,
		runToken,
	)

	err := processor.Process(t.Context(), 17, false)
	if !errors.Is(err, ErrMallWeatherExportProcessNonRetryable) {
		t.Fatalf("Process() error=%v", err)
	}
	state := store.snapshot()
	if state.successConfirmations != 1 || objectStore.deletedKey != "" || state.failed != 1 {
		t.Fatalf("store state=%+v object store=%+v", state, objectStore)
	}
}

func TestMallWeatherExportProcessorPreservesObjectWhenSuccessCannotBeConfirmed(t *testing.T) {
	runToken := uuid.NewString()
	store := newFakeMallWeatherExportRunStore(t)
	store.succeedErr = errors.New("database update result unknown")
	store.confirmSuccessErr = errors.New("database confirmation unavailable")
	objectStore := &fakeMallWeatherExportObjectStore{}
	processor := newTestMallWeatherExportProcessor(
		t,
		store,
		mallWeatherExportRendererFunc(writeFakeMallWeatherExportArtifact),
		objectStore,
		runToken,
	)

	err := processor.Process(t.Context(), 17, true)
	if err == nil || !strings.Contains(err.Error(), "confirmation unavailable") {
		t.Fatalf("Process() error=%v", err)
	}
	state := store.snapshot()
	if state.successConfirmations != 1 || objectStore.deletedKey != "" || state.released != 1 {
		t.Fatalf("store state=%+v object store=%+v", state, objectStore)
	}
}

func TestMallWeatherExportProcessorRecordsSuccessfulExportRows(t *testing.T) {
	runToken := uuid.NewString()
	store := newFakeMallWeatherExportRunStore(t)
	renderer := mallWeatherExportRendererFunc(func(
		_ context.Context,
		request MallWeatherExportRenderRequest,
		_ func(MallWeatherExportRenderProgress) error,
	) (MallWeatherExportRenderResult, error) {
		if err := os.WriteFile(request.OutputPath, []byte("xlsx artifact"), 0o600); err != nil {
			return MallWeatherExportRenderResult{}, err
		}
		return MallWeatherExportRenderResult{ProcessedRows: 3, SheetCount: 1}, nil
	})
	metrics := &fakeMallWeatherMetricRecorder{}
	processor := newTestMallWeatherExportProcessorWithMetrics(
		t,
		store,
		renderer,
		&fakeMallWeatherExportObjectStore{},
		runToken,
		metrics,
	)

	if err := processor.Process(t.Context(), 17, true); err != nil {
		t.Fatalf("Process() error=%v", err)
	}
	assertFakeMallWeatherMetricCounters(t, metrics.counters, []fakeMallWeatherMetricCounter{{
		name: MallWeatherMetricExportRowsTotal, value: 3,
	}, {
		name: MallWeatherMetricExportRunsTotal, labels: map[string]string{"status": mallWeatherMetricStatusSucceeded}, value: 1,
	}})
}

func TestMallWeatherExportProcessorDoesNotRecordRowsForUnfinishedExport(t *testing.T) {
	store := newFakeMallWeatherExportRunStore(t)
	metrics := &fakeMallWeatherMetricRecorder{}
	processor := newTestMallWeatherExportProcessorWithMetrics(
		t,
		store,
		mallWeatherExportRendererFunc(writeFakeMallWeatherExportArtifact),
		&fakeMallWeatherExportObjectStore{uploadErr: errors.New("temporary OSS failure")},
		uuid.NewString(),
		metrics,
	)

	if err := processor.Process(t.Context(), 17, true); err == nil {
		t.Fatal("Process() returned nil error")
	}
	if len(metrics.counters) != 0 {
		t.Fatalf("metrics=%+v, want none", metrics.counters)
	}
}

func TestMallWeatherExportProcessorRecordsTerminalRunMetrics(t *testing.T) {
	t.Run("failed final attempt", func(t *testing.T) {
		store := newFakeMallWeatherExportRunStore(t)
		metrics := &fakeMallWeatherMetricRecorder{}
		processor := newTestMallWeatherExportProcessorWithMetrics(
			t,
			store,
			mallWeatherExportRendererFunc(writeFakeMallWeatherExportArtifact),
			&fakeMallWeatherExportObjectStore{uploadErr: errors.New("permanent OSS failure")},
			uuid.NewString(),
			metrics,
		)

		err := processor.Process(t.Context(), 17, false)
		if !errors.Is(err, ErrMallWeatherExportProcessNonRetryable) {
			t.Fatalf("Process() error=%v", err)
		}
		assertFakeMallWeatherMetricCounters(t, metrics.counters, []fakeMallWeatherMetricCounter{{
			name: MallWeatherMetricExportRunsTotal, labels: map[string]string{"status": mallWeatherMetricStatusFailed}, value: 1,
		}})
	})

	t.Run("cancelled", func(t *testing.T) {
		store := newFakeMallWeatherExportRunStore(t)
		store.progressControl = data_dao.MallWeatherExportRunControlCancelRequested
		metrics := &fakeMallWeatherMetricRecorder{}
		renderer := mallWeatherExportRendererFunc(func(
			_ context.Context,
			_ MallWeatherExportRenderRequest,
			onProgress func(MallWeatherExportRenderProgress) error,
		) (MallWeatherExportRenderResult, error) {
			return MallWeatherExportRenderResult{}, onProgress(MallWeatherExportRenderProgress{
				ProcessedRows: 1, CurrentSheet: "商场", Cursor: []byte(`{"afterId":1}`),
			})
		})
		processor := newTestMallWeatherExportProcessorWithMetrics(
			t,
			store,
			renderer,
			&fakeMallWeatherExportObjectStore{},
			uuid.NewString(),
			metrics,
		)

		if err := processor.Process(t.Context(), 17, true); err != nil {
			t.Fatalf("Process() error=%v", err)
		}
		assertFakeMallWeatherMetricCounters(t, metrics.counters, []fakeMallWeatherMetricCounter{{
			name: MallWeatherMetricExportRunsTotal, labels: map[string]string{"status": "cancelled"}, value: 1,
		}})
	})
}

func TestMallWeatherExportProcessorRejectsMissingMetricsRecorder(t *testing.T) {
	store := newFakeMallWeatherExportRunStore(t)
	processor := newTestMallWeatherExportProcessorWithMetrics(
		t,
		store,
		mallWeatherExportRendererFunc(writeFakeMallWeatherExportArtifact),
		&fakeMallWeatherExportObjectStore{},
		uuid.NewString(),
		nil,
	)

	if err := processor.Process(t.Context(), 17, true); err == nil {
		t.Fatal("Process() accepted missing metrics recorder")
	}
}

func TestMallWeatherExportProcessorHonorsProgressCancellation(t *testing.T) {
	store := newFakeMallWeatherExportRunStore(t)
	store.progressControl = data_dao.MallWeatherExportRunControlCancelRequested
	renderer := mallWeatherExportRendererFunc(func(
		_ context.Context,
		_ MallWeatherExportRenderRequest,
		onProgress func(MallWeatherExportRenderProgress) error,
	) (MallWeatherExportRenderResult, error) {
		return MallWeatherExportRenderResult{}, onProgress(MallWeatherExportRenderProgress{
			ProcessedRows: 1, CurrentSheet: "商场", Cursor: []byte(`{"afterId":1}`),
		})
	})
	objectStore := &fakeMallWeatherExportObjectStore{}
	processor := newTestMallWeatherExportProcessor(t, store, renderer, objectStore, uuid.NewString())
	if err := processor.Process(t.Context(), 17, true); err != nil {
		t.Fatalf("Process() error=%v", err)
	}
	state := store.snapshot()
	if state.cancelled != 1 || state.released != 0 || objectStore.uploadedKey != "" {
		t.Fatalf("store state=%+v object store=%+v", state, objectStore)
	}
}

func TestMallWeatherExportProcessorClassifiesUploadFailure(t *testing.T) {
	tests := []struct {
		name          string
		retryAllowed  bool
		wantReleased  int
		wantFailed    int
		wantPermanent bool
	}{
		{name: "retry available", retryAllowed: true, wantReleased: 1},
		{name: "final attempt", wantFailed: 1, wantPermanent: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newFakeMallWeatherExportRunStore(t)
			renderer := mallWeatherExportRendererFunc(writeFakeMallWeatherExportArtifact)
			objectStore := &fakeMallWeatherExportObjectStore{uploadErr: errors.New("temporary OSS failure")}
			processor := newTestMallWeatherExportProcessor(t, store, renderer, objectStore, uuid.NewString())
			err := processor.Process(t.Context(), 17, tt.retryAllowed)
			if err == nil || errors.Is(err, ErrMallWeatherExportProcessNonRetryable) != tt.wantPermanent {
				t.Fatalf("Process() error=%v", err)
			}
			state := store.snapshot()
			if state.released != tt.wantReleased || state.failed != tt.wantFailed || state.progressUpdates != 1 {
				t.Fatalf("store state=%+v", state)
			}
		})
	}
}

func TestMallWeatherExportProcessorHeartbeatCancelsBlockedRenderer(t *testing.T) {
	store := newFakeMallWeatherExportRunStore(t)
	store.heartbeatControl = data_dao.MallWeatherExportRunControlCancelRequested
	renderer := mallWeatherExportRendererFunc(func(
		ctx context.Context,
		_ MallWeatherExportRenderRequest,
		_ func(MallWeatherExportRenderProgress) error,
	) (MallWeatherExportRenderResult, error) {
		<-ctx.Done()
		return MallWeatherExportRenderResult{}, ctx.Err()
	})
	processor := newTestMallWeatherExportProcessor(
		t,
		store,
		renderer,
		&fakeMallWeatherExportObjectStore{},
		uuid.NewString(),
	)
	processor.heartbeatInterval = time.Millisecond
	if err := processor.Process(t.Context(), 17, true); err != nil {
		t.Fatalf("Process() error=%v", err)
	}
	if state := store.snapshot(); state.cancelled != 1 || state.released != 0 {
		t.Fatalf("store state=%+v", state)
	}
}

func TestMallWeatherExportProcessorPermanentlyFailsInvalidSnapshot(t *testing.T) {
	store := newFakeMallWeatherExportRunStore(t)
	store.lease.Job.ProfileSnapshotJSON = model.JSONText(`{"invalid":true}`)
	renderer := mallWeatherExportRendererFunc(func(
		context.Context,
		MallWeatherExportRenderRequest,
		func(MallWeatherExportRenderProgress) error,
	) (MallWeatherExportRenderResult, error) {
		t.Fatal("renderer called for an invalid snapshot")
		return MallWeatherExportRenderResult{}, nil
	})
	processor := newTestMallWeatherExportProcessor(
		t,
		store,
		renderer,
		&fakeMallWeatherExportObjectStore{},
		uuid.NewString(),
	)
	err := processor.Process(t.Context(), 17, true)
	if !errors.Is(err, ErrMallWeatherExportProcessNonRetryable) {
		t.Fatalf("Process() error=%v", err)
	}
	if state := store.snapshot(); state.failed != 1 || state.released != 0 {
		t.Fatalf("store state=%+v", state)
	}
}

func writeFakeMallWeatherExportArtifact(
	_ context.Context,
	request MallWeatherExportRenderRequest,
	_ func(MallWeatherExportRenderProgress) error,
) (MallWeatherExportRenderResult, error) {
	if err := os.WriteFile(request.OutputPath, []byte("xlsx artifact"), 0o600); err != nil {
		return MallWeatherExportRenderResult{}, err
	}
	return MallWeatherExportRenderResult{ProcessedRows: 1, SheetCount: 1}, nil
}

type mallWeatherExportRendererFunc func(
	context.Context,
	MallWeatherExportRenderRequest,
	func(MallWeatherExportRenderProgress) error,
) (MallWeatherExportRenderResult, error)

func (renderer mallWeatherExportRendererFunc) Render(
	ctx context.Context,
	request MallWeatherExportRenderRequest,
	onProgress func(MallWeatherExportRenderProgress) error,
) (MallWeatherExportRenderResult, error) {
	return renderer(ctx, request, onProgress)
}

type fakeMallWeatherExportRunStore struct {
	mu                   sync.Mutex
	lease                data_dao.MallWeatherExportRunLease
	progressControl      data_dao.MallWeatherExportRunControl
	heartbeatControl     data_dao.MallWeatherExportRunControl
	progressUpdates      int
	released             int
	failed               int
	cancelled            int
	succeededKey         string
	succeededChecksum    string
	succeededFileSize    int64
	succeedErr           error
	commitSuccessOnError bool
	confirmSuccessErr    error
	successConfirmations int
}

func newFakeMallWeatherExportRunStore(t *testing.T) *fakeMallWeatherExportRunStore {
	t.Helper()
	job := validMallWeatherExportProcessorJob(t)
	return &fakeMallWeatherExportRunStore{
		lease: data_dao.MallWeatherExportRunLease{
			Disposition: data_dao.MallWeatherExportRunDispositionAcquired,
			Job:         job,
		},
		progressControl:  data_dao.MallWeatherExportRunControlContinue,
		heartbeatControl: data_dao.MallWeatherExportRunControlContinue,
	}
}

func (store *fakeMallWeatherExportRunStore) BeginRun(
	_ context.Context,
	_ uint,
	runToken string,
	_ time.Time,
	_ time.Duration,
) (*data_dao.MallWeatherExportRunLease, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	lease := store.lease
	lease.RunToken = runToken
	return &lease, nil
}

func (store *fakeMallWeatherExportRunStore) UpdateRunProgress(
	context.Context,
	uint,
	string,
	data_dao.MallWeatherExportRunProgress,
) (data_dao.MallWeatherExportRunControl, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.progressUpdates++
	return store.progressControl, nil
}

func (store *fakeMallWeatherExportRunStore) HeartbeatRun(
	context.Context,
	uint,
	string,
	time.Time,
) (data_dao.MallWeatherExportRunControl, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.heartbeatControl, nil
}

func (store *fakeMallWeatherExportRunStore) InspectRun(
	context.Context,
	uint,
	string,
) (data_dao.MallWeatherExportRunControl, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.heartbeatControl, nil
}

func (store *fakeMallWeatherExportRunStore) MarkRunSucceeded(
	_ context.Context,
	_ uint,
	_ string,
	objectKey string,
	checksum string,
	fileSize int64,
	_ time.Time,
	_ time.Time,
) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.succeedErr == nil || store.commitSuccessOnError {
		store.succeededKey = objectKey
		store.succeededChecksum = checksum
		store.succeededFileSize = fileSize
	}
	return store.succeedErr
}

func (store *fakeMallWeatherExportRunStore) ConfirmRunSucceeded(
	_ context.Context,
	_ uint,
	objectKey string,
	checksum string,
	fileSize int64,
) (bool, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.successConfirmations++
	if store.confirmSuccessErr != nil {
		return false, store.confirmSuccessErr
	}
	return store.succeededKey == objectKey && store.succeededChecksum == checksum &&
		store.succeededFileSize == fileSize, nil
}

func (store *fakeMallWeatherExportRunStore) MarkRunFailed(
	context.Context,
	uint,
	string,
	string,
	time.Time,
	time.Time,
) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.failed++
	return nil
}

func (store *fakeMallWeatherExportRunStore) MarkRunCancelled(
	context.Context,
	uint,
	string,
	time.Time,
	time.Time,
) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.cancelled++
	return nil
}

func (store *fakeMallWeatherExportRunStore) ReleaseRunForRetry(
	context.Context,
	uint,
	string,
	time.Time,
) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.released++
	return nil
}

type fakeMallWeatherExportRunStoreState struct {
	progressUpdates      int
	released             int
	failed               int
	cancelled            int
	succeededKey         string
	succeededChecksum    string
	successConfirmations int
}

func (store *fakeMallWeatherExportRunStore) snapshot() fakeMallWeatherExportRunStoreState {
	store.mu.Lock()
	defer store.mu.Unlock()
	return fakeMallWeatherExportRunStoreState{
		progressUpdates:      store.progressUpdates,
		released:             store.released,
		failed:               store.failed,
		cancelled:            store.cancelled,
		succeededKey:         store.succeededKey,
		succeededChecksum:    store.succeededChecksum,
		successConfirmations: store.successConfirmations,
	}
}

type fakeMallWeatherExportObjectStore struct {
	uploadedKey  string
	downloadName string
	uploadErr    error
	deletedKey   string
}

func (store *fakeMallWeatherExportObjectStore) UploadFile(
	_ context.Context,
	objectKey string,
	_ string,
	downloadName string,
) (storage.UploadResult, error) {
	store.uploadedKey = objectKey
	store.downloadName = downloadName
	return storage.UploadResult{ObjectKey: objectKey}, store.uploadErr
}

func (store *fakeMallWeatherExportObjectStore) DeleteObject(_ context.Context, objectKey string) error {
	store.deletedKey = objectKey
	return nil
}

func newTestMallWeatherExportProcessor(
	t *testing.T,
	runs mallWeatherExportRunStore,
	renderer mallWeatherExportWorkbookRenderer,
	objectStore mallWeatherExportObjectStore,
	runToken string,
) *MallWeatherExportProcessor {
	t.Helper()
	return newTestMallWeatherExportProcessorWithMetrics(
		t,
		runs,
		renderer,
		objectStore,
		runToken,
		noopMallWeatherMetricRecorder{},
	)
}

func newTestMallWeatherExportProcessorWithMetrics(
	t *testing.T,
	runs mallWeatherExportRunStore,
	renderer mallWeatherExportWorkbookRenderer,
	objectStore mallWeatherExportObjectStore,
	runToken string,
	metrics mallWeatherMetricRecorder,
) *MallWeatherExportProcessor {
	t.Helper()
	return &MallWeatherExportProcessor{
		runs:              runs,
		renderer:          renderer,
		newObjectStore:    func() (mallWeatherExportObjectStore, error) { return objectStore, nil },
		buildObjectKey:    func(parts ...string) string { return path.Join(parts...) },
		now:               func() time.Time { return time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC) },
		newRunToken:       func() string { return runToken },
		metrics:           metrics,
		workRoot:          t.TempDir(),
		staleAfter:        time.Minute,
		heartbeatInterval: time.Hour,
		retention:         7 * 24 * time.Hour,
	}
}

var _ mallWeatherExportRunStore = (*fakeMallWeatherExportRunStore)(nil)
var _ mallWeatherExportWorkbookRenderer = mallWeatherExportRendererFunc(nil)
var _ mallWeatherExportObjectStore = (*fakeMallWeatherExportObjectStore)(nil)
