package data_svc

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"gin-biz-web-api/internal/reportrepo"
	"gin-biz-web-api/model"
)

func TestReportRunProcessorExecutesProcedureOnceAndPersistsSuccess(t *testing.T) {
	store := newFakeReportExecutionStore()
	executor := &fakeReportProcedureExecutor{rowCount: 18}
	processor := newTestReportRunProcessor(store, executor)
	if err := processor.Process(t.Context(), 31, true); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if executor.calls != 1 || store.succeeded != 1 || store.unknown != 0 || store.failed != 0 {
		t.Fatalf("calls=%d succeeded=%d unknown=%d failed=%d", executor.calls, store.succeeded, store.unknown, store.failed)
	}
}

func TestReportRunProcessorMarksUnknownWithoutRetryingAfterCommitAmbiguity(t *testing.T) {
	store := newFakeReportExecutionStore()
	executor := &fakeReportProcedureExecutor{err: errOracleCommitOutcomeUnknown}
	processor := newTestReportRunProcessor(store, executor)
	err := processor.Process(t.Context(), 31, true)
	if !errors.Is(err, ErrReportRunProcessNonRetryable) {
		t.Fatalf("Process() error = %v, want non-retryable", err)
	}
	if executor.calls != 1 || store.unknown != 1 || store.failed != 0 || store.unknownCode != "ORACLE_COMMIT_OUTCOME_UNKNOWN" {
		t.Fatalf("calls=%d unknown=%d failed=%d code=%q", executor.calls, store.unknown, store.failed, store.unknownCode)
	}
}

func TestReportRunProcessorCommitAmbiguityOverridesConcurrentCancellation(t *testing.T) {
	store := newFakeReportExecutionStore()
	store.cancelOnInspectAfterOracle = true
	executor := &fakeReportProcedureExecutor{err: errOracleCommitOutcomeUnknown}
	processor := newTestReportRunProcessor(store, executor)
	err := processor.Process(t.Context(), 31, true)
	if !errors.Is(err, ErrReportRunProcessNonRetryable) || store.unknown != 1 || store.cancelled != 0 {
		t.Fatalf("error=%v unknown=%d cancelled=%d", err, store.unknown, store.cancelled)
	}
}

func TestReportRunProcessorRoutesUnknownRunsToReconciliation(t *testing.T) {
	store := newFakeReportExecutionStore()
	store.beginDisposition = reportrepo.RunDispositionReconcile
	processor := newTestReportRunProcessor(store, &fakeReportProcedureExecutor{})
	called := 0
	processor.reconcileOne = func(context.Context, uint) error { called++; return nil }
	if err := processor.Process(t.Context(), 31, true); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if called != 1 {
		t.Fatalf("reconcile calls = %d", called)
	}
}

func TestReportRunProcessorHeartbeatCancelsOracleExecution(t *testing.T) {
	store := newFakeReportExecutionStore()
	store.cancelOnHeartbeat = true
	executor := &fakeReportProcedureExecutor{waitForCancel: true}
	processor := newTestReportRunProcessor(store, executor)
	processor.heartbeatInterval = time.Millisecond
	processor.stateTimeout = time.Second
	if err := processor.Process(t.Context(), 31, true); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if executor.calls != 1 || store.cancelled != 1 {
		t.Fatalf("calls=%d cancelled=%d", executor.calls, store.cancelled)
	}
}

func TestReportRunProcessorRejectsRuntimeContractBeforeOracleStart(t *testing.T) {
	store := newFakeReportExecutionStore()
	store.loadErr = errors.New("hash mismatch")
	executor := &fakeReportProcedureExecutor{}
	processor := newTestReportRunProcessor(store, executor)
	err := processor.Process(t.Context(), 31, false)
	if !errors.Is(err, ErrReportRunProcessNonRetryable) || executor.calls != 0 || store.failed != 1 || store.oracleStarted != 0 {
		t.Fatalf("error=%v calls=%d failed=%d oracleStarted=%d", err, executor.calls, store.failed, store.oracleStarted)
	}
}

func newTestReportRunProcessor(store *fakeReportExecutionStore, executor *fakeReportProcedureExecutor) *ReportRunProcessor {
	processor := NewReportRunProcessorWithDependencies(store, fakeReportCredentialDecryptor{}, fakeRunParameterDecryptor{}, executor)
	processor.heartbeatInterval = time.Hour
	processor.stateTimeout = time.Second
	processor.now = func() time.Time { return time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC) }
	return processor
}

type fakeReportExecutionStore struct {
	mu                         sync.Mutex
	runtime                    *reportrepo.RuntimeContract
	loadErr                    error
	heartbeatControl           reportrepo.RunControl
	cancelOnHeartbeat          bool
	cancelOnInspectAfterOracle bool
	oracleStarted              int
	succeeded                  int
	failed                     int
	cancelled                  int
	unknown                    int
	unknownCode                string
	beginDisposition           reportrepo.RunDisposition
}

func newFakeReportExecutionStore() *fakeReportExecutionStore {
	return &fakeReportExecutionStore{
		heartbeatControl: reportrepo.RunControlContinue,
		runtime: &reportrepo.RuntimeContract{
			Run: model.ReportRun{BaseModel: model.BaseModel{ID: 31}, RunUUID: "11111111-1111-4111-8111-111111111111", NormalizedParametersJSON: model.JSONText(`{"storeCode":"S001"}`)},
			Parameters: []model.ReportParameter{
				{ParameterCode: "runId", ProcedureArgName: "P_RUN_ID", Position: 1, Direction: "IN", LogicalType: "string", OracleType: "VARCHAR2", Cardinality: "SINGLE", Required: true, SystemInjected: true, NullPolicy: "TYPED_NULL"},
				{ParameterCode: "storeCode", ProcedureArgName: "P_STORE_CODE", Position: 2, Direction: "IN", LogicalType: "string", OracleType: "VARCHAR2", Cardinality: "SINGLE", Required: true, NullPolicy: "TYPED_NULL"},
			},
		},
	}
}

func (store *fakeReportExecutionStore) BeginExecution(context.Context, uint, string, string, time.Time, time.Duration) (*reportrepo.RunLease, error) {
	disposition := store.beginDisposition
	if disposition == reportrepo.RunDispositionUnknown {
		disposition = reportrepo.RunDispositionAcquired
	}
	return &reportrepo.RunLease{Disposition: disposition}, nil
}
func (store *fakeReportExecutionStore) HeartbeatExecution(context.Context, uint, string, time.Time, time.Duration) (reportrepo.RunControl, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.cancelOnHeartbeat {
		store.heartbeatControl = reportrepo.RunControlCancelRequested
	}
	return store.heartbeatControl, nil
}
func (store *fakeReportExecutionStore) InspectExecution(context.Context, uint, string) (reportrepo.RunControl, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.cancelOnInspectAfterOracle && store.oracleStarted > 0 {
		return reportrepo.RunControlCancelRequested, nil
	}
	return store.heartbeatControl, nil
}
func (store *fakeReportExecutionStore) MarkOracleExecutionStarted(context.Context, uint, string, time.Time) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.oracleStarted++
	return nil
}
func (store *fakeReportExecutionStore) MarkExecutionSucceeded(context.Context, uint, string, int64, time.Time, time.Time) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.succeeded++
	return nil
}
func (store *fakeReportExecutionStore) ConfirmExecutionSucceeded(context.Context, uint, int64) (bool, error) {
	return false, nil
}
func (store *fakeReportExecutionStore) MarkExecutionFailed(context.Context, uint, string, string, string, time.Time) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.failed++
	return nil
}
func (store *fakeReportExecutionStore) MarkExecutionCancelled(context.Context, uint, string, time.Time) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.cancelled++
	return nil
}
func (store *fakeReportExecutionStore) MarkExecutionUnknown(_ context.Context, _ uint, _, code, _ string, _ time.Time) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.unknown++
	store.unknownCode = code
	return nil
}
func (store *fakeReportExecutionStore) ReleaseExecutionForRetry(context.Context, uint, string, time.Time) error {
	return nil
}
func (store *fakeReportExecutionStore) ListReconciliationCandidates(context.Context, time.Time, int) ([]uint, error) {
	return nil, nil
}
func (store *fakeReportExecutionStore) BeginReconciliation(context.Context, uint, string, string, time.Time, time.Duration) (*reportrepo.RunLease, error) {
	return &reportrepo.RunLease{Disposition: reportrepo.RunDispositionTerminal}, nil
}
func (store *fakeReportExecutionStore) MarkReconciliationSucceeded(context.Context, uint, string, int64, time.Time, time.Time) error {
	return nil
}
func (store *fakeReportExecutionStore) MarkReconciliationPending(context.Context, uint, string, string, string, time.Time) error {
	return nil
}
func (store *fakeReportExecutionStore) RecoverExpiredPreOracleRuns(context.Context, time.Time, int) (int64, error) {
	return 0, nil
}
func (store *fakeReportExecutionStore) ListQueuedRunsMissingDelivery(context.Context, int) ([]uint, error) {
	return nil, nil
}
func (store *fakeReportExecutionStore) EnsureRunQueued(context.Context, uint, time.Time) error {
	return nil
}
func (store *fakeReportExecutionStore) LoadRuntimeContract(context.Context, uint, string) (*reportrepo.RuntimeContract, error) {
	return store.runtime, store.loadErr
}

type fakeReportCredentialDecryptor struct{}

func (fakeReportCredentialDecryptor) Decrypt(string, string) (string, error) { return "password", nil }

type fakeRunParameterDecryptor struct{}

func (fakeRunParameterDecryptor) Decrypt(string, string) ([]byte, error) { return []byte(`{}`), nil }

type fakeReportProcedureExecutor struct {
	mu            sync.Mutex
	calls         int
	rowCount      int64
	err           error
	waitForCancel bool
}

func (executor *fakeReportProcedureExecutor) Execute(ctx context.Context, request reportProcedureExecutionRequest, password string) (int64, error) {
	executor.mu.Lock()
	executor.calls++
	executor.mu.Unlock()
	if password != "password" || request.Values["runId"] == nil || request.Values["storeCode"] != "S001" {
		return 0, errors.New("unexpected execution request")
	}
	if executor.waitForCancel {
		<-ctx.Done()
		return 0, ctx.Err()
	}
	return executor.rowCount, executor.err
}
