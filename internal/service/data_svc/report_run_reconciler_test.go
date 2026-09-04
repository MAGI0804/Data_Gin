package data_svc

import (
	"context"
	"errors"
	"testing"
	"time"

	"gin-biz-web-api/internal/reportrepo"
	"gin-biz-web-api/job"
	"gin-biz-web-api/model"
)

func TestReportRunReconcilerFailsExpiredQueuedRunsBeforeDeliveryRecovery(t *testing.T) {
	now := time.Date(2026, 9, 4, 18, 0, 0, 0, time.UTC)
	store := &fakeReconciliationStore{expiredQueuedIDs: []uint{31}}
	reconciler := NewReportRunReconcilerWithDependencies(store, fakeReportCredentialDecryptor{}, &fakeResultEvidenceReader{})
	reconciler.now = func() time.Time { return now }

	if err := reconciler.Reconcile(t.Context()); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if !store.expiredCutoff.Equal(now.Add(-job.ReportRunSnapshotWaitLimit)) || store.queuedFailed != 1 || store.queuedFailedID != 31 {
		t.Fatalf("cutoff=%v failed=%d failedID=%d", store.expiredCutoff, store.queuedFailed, store.queuedFailedID)
	}
}

func TestReportRunReconcilerRecoversLostExportDelivery(t *testing.T) {
	now := time.Date(2026, 9, 4, 18, 0, 0, 0, time.UTC)
	store := &fakeReconciliationStore{missingExportIDs: []uint{41}}
	reconciler := NewReportRunReconcilerWithDependencies(store, fakeReportCredentialDecryptor{}, &fakeResultEvidenceReader{})
	reconciler.now = func() time.Time { return now }

	if err := reconciler.Reconcile(t.Context()); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if store.exportRecoveryCutoff != now || store.exportQueued != 1 || store.exportQueuedID != 41 {
		t.Fatalf("cutoff=%v queued=%d exportID=%d", store.exportRecoveryCutoff, store.exportQueued, store.exportQueuedID)
	}
}

func TestReportRunReconcilerUsesRunScopedResultAsCommitEvidence(t *testing.T) {
	store := &fakeReconciliationStore{runtime: reconciliationRuntime()}
	reader := &fakeResultEvidenceReader{rowCount: 12}
	reconciler := NewReportRunReconcilerWithDependencies(store, fakeReportCredentialDecryptor{}, reader)
	reconciler.now = func() time.Time { return time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC) }
	reconciler.stateTimeout = time.Second

	if err := reconciler.ReconcileOne(t.Context(), 31); err != nil {
		t.Fatalf("ReconcileOne() error = %v", err)
	}
	if reader.calls != 1 || store.succeeded != 1 || store.rowCount != 12 || store.pending != 0 {
		t.Fatalf("calls=%d succeeded=%d rowCount=%d pending=%d", reader.calls, store.succeeded, store.rowCount, store.pending)
	}
}

func TestReportRunReconcilerKeepsEmptyAmbiguousResultPending(t *testing.T) {
	store := &fakeReconciliationStore{runtime: reconciliationRuntime()}
	reconciler := NewReportRunReconcilerWithDependencies(store, fakeReportCredentialDecryptor{}, &fakeResultEvidenceReader{})
	reconciler.now = func() time.Time { return time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC) }
	reconciler.stateTimeout = time.Second

	if err := reconciler.ReconcileOne(t.Context(), 31); err != nil {
		t.Fatalf("ReconcileOne() error = %v", err)
	}
	if store.succeeded != 0 || store.pending != 1 || store.pendingCode != "ORACLE_RESULT_NOT_PROVEN" {
		t.Fatalf("succeeded=%d pending=%d code=%q", store.succeeded, store.pending, store.pendingCode)
	}
}

func TestReportRunReconcilerAcceptsEmptyCommittedJSONSnapshot(t *testing.T) {
	runtime := reconciliationRuntime()
	runtime.Version.ExecutionMode = model.ReportExecutionModeRefCursor
	store := &fakeReconciliationStore{runtime: runtime}
	reconciler := NewReportRunReconcilerWithDependencies(store, fakeReportCredentialDecryptor{}, &fakeResultEvidenceReader{})
	reconciler.now = func() time.Time { return time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC) }
	reconciler.stateTimeout = time.Second

	if err := reconciler.ReconcileOne(t.Context(), 31); err != nil {
		t.Fatalf("ReconcileOne() error = %v", err)
	}
	if store.succeeded != 1 || store.rowCount != 0 || store.pending != 0 {
		t.Fatalf("succeeded=%d rowCount=%d pending=%d", store.succeeded, store.rowCount, store.pending)
	}
}

func TestReportRunReconcilerKeepsUnavailableResultPending(t *testing.T) {
	store := &fakeReconciliationStore{runtime: reconciliationRuntime()}
	reconciler := NewReportRunReconcilerWithDependencies(store, fakeReportCredentialDecryptor{}, &fakeResultEvidenceReader{err: errors.New("oracle unavailable")})
	reconciler.now = func() time.Time { return time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC) }
	reconciler.stateTimeout = time.Second

	if err := reconciler.ReconcileOne(t.Context(), 31); err != nil {
		t.Fatalf("ReconcileOne() error = %v", err)
	}
	if store.succeeded != 0 || store.pending != 1 || store.pendingCode != "ORACLE_RESULT_UNAVAILABLE" {
		t.Fatalf("succeeded=%d pending=%d code=%q", store.succeeded, store.pending, store.pendingCode)
	}
}

func reconciliationRuntime() *reportrepo.RuntimeContract {
	return &reportrepo.RuntimeContract{
		Run:     model.ReportRun{BaseModel: model.BaseModel{ID: 31}, RunUUID: "11111111-1111-4111-8111-111111111111"},
		Version: model.ReportVersion{ResultTableOwner: "REPORT_OWNER", ResultTableName: "SALES_RESULT", ResultRunIDColumn: "RUN_ID", ResultRowIDColumn: "ID"},
	}
}

type fakeResultEvidenceReader struct {
	rowCount int64
	err      error
	calls    int
}

func (reader *fakeResultEvidenceReader) CountCommittedRows(context.Context, reportrepo.RuntimeContract, string) (int64, error) {
	reader.calls++
	return reader.rowCount, reader.err
}

type fakeReconciliationStore struct {
	runtime              *reportrepo.RuntimeContract
	succeeded            int
	rowCount             int64
	pending              int
	pendingCode          string
	expiredQueuedIDs     []uint
	expiredCutoff        time.Time
	queuedFailed         int
	queuedFailedID       uint
	missingExportIDs     []uint
	exportRecoveryCutoff time.Time
	exportQueued         int
	exportQueuedID       uint
}

func (store *fakeReconciliationStore) ListReconciliationCandidates(context.Context, time.Time, int) ([]uint, error) {
	return nil, nil
}
func (store *fakeReconciliationStore) BeginReconciliation(context.Context, uint, string, string, time.Time, time.Duration) (*reportrepo.RunLease, error) {
	return &reportrepo.RunLease{Disposition: reportrepo.RunDispositionAcquired}, nil
}
func (store *fakeReconciliationStore) LoadRuntimeContract(context.Context, uint, string) (*reportrepo.RuntimeContract, error) {
	return store.runtime, nil
}
func (store *fakeReconciliationStore) MarkReconciliationSucceeded(_ context.Context, _ uint, _ string, rowCount int64, _, _ time.Time) error {
	store.succeeded++
	store.rowCount = rowCount
	return nil
}
func (store *fakeReconciliationStore) MarkReconciliationPending(_ context.Context, _ uint, _, code, _ string, _ time.Time) error {
	store.pending++
	store.pendingCode = code
	return nil
}
func (store *fakeReconciliationStore) RecoverExpiredPreOracleRuns(context.Context, time.Time, int) (int64, error) {
	return 0, nil
}
func (store *fakeReconciliationStore) ListExpiredQueuedRuns(_ context.Context, cutoff time.Time, _ int) ([]uint, error) {
	store.expiredCutoff = cutoff
	return store.expiredQueuedIDs, nil
}
func (store *fakeReconciliationStore) MarkQueuedExecutionFailed(_ context.Context, runID uint, _, _ string, _ time.Time) error {
	store.queuedFailed++
	store.queuedFailedID = runID
	return nil
}
func (store *fakeReconciliationStore) ListQueuedRunsMissingDelivery(context.Context, int) ([]uint, error) {
	return nil, nil
}
func (store *fakeReconciliationStore) EnsureRunQueued(context.Context, uint, time.Time) error {
	return nil
}
func (store *fakeReconciliationStore) ListExportsMissingDelivery(_ context.Context, now time.Time, _ int) ([]uint, error) {
	store.exportRecoveryCutoff = now
	return store.missingExportIDs, nil
}
func (store *fakeReconciliationStore) EnsureExportQueued(_ context.Context, exportID uint, _ time.Time) error {
	store.exportQueued++
	store.exportQueuedID = exportID
	return nil
}
