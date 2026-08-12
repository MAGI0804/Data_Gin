package data_svc

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gin-biz-web-api/internal/reportoracle"
	"gin-biz-web-api/internal/reportrepo"
	"gin-biz-web-api/internal/reportsecret"

	"github.com/google/uuid"
)

const (
	defaultReportReconcilePollInterval = 30 * time.Second
	defaultReportReconcileBatchSize    = 20
)

type reportReconciliationStore interface {
	ListReconciliationCandidates(context.Context, time.Time, int) ([]uint, error)
	BeginReconciliation(context.Context, uint, string, string, time.Time, time.Duration) (*reportrepo.RunLease, error)
	LoadRuntimeContract(context.Context, uint, string) (*reportrepo.RuntimeContract, error)
	MarkReconciliationSucceeded(context.Context, uint, string, int64, time.Time, time.Time) error
	MarkReconciliationPending(context.Context, uint, string, string, string, time.Time) error
	RecoverExpiredPreOracleRuns(context.Context, time.Time, int) (int64, error)
	ListQueuedRunsMissingDelivery(context.Context, int) ([]uint, error)
	EnsureRunQueued(context.Context, uint, time.Time) error
}

type reportReceiptReader interface {
	Read(context.Context, reportrepo.RuntimeContract, string) (reportoracle.RunReceipt, error)
}

type ReportRunReconciler struct {
	store        reportReconciliationStore
	credential   reportDatasourceCredentialDecryptor
	receipts     reportReceiptReader
	workerID     string
	leaseTTL     time.Duration
	pollInterval time.Duration
	batchSize    int
	stateTimeout time.Duration
	retention    time.Duration
	now          func() time.Time
}

func NewReportRunReconciler() *ReportRunReconciler {
	return NewReportRunReconcilerWithDependencies(reportrepo.New(), reportsecretCredentialDecryptor{}, oracleReportReceiptReader{})
}

type reportsecretCredentialDecryptor struct{}

func (reportsecretCredentialDecryptor) Decrypt(version, ciphertext string) (string, error) {
	return (reportsecret.EnvironmentKeyring{}).Decrypt(version, ciphertext)
}

func NewReportRunReconcilerWithDependencies(store reportReconciliationStore, credential reportDatasourceCredentialDecryptor, receipts reportReceiptReader) *ReportRunReconciler {
	if store == nil || credential == nil || receipts == nil {
		panic("report run reconciliation: dependencies are required")
	}
	return &ReportRunReconciler{
		store: store, credential: credential, receipts: receipts, workerID: "report-reconcile-" + uuid.NewString(),
		leaseTTL: defaultReportRunLeaseTTL, pollInterval: defaultReportReconcilePollInterval,
		batchSize: defaultReportReconcileBatchSize, stateTimeout: defaultReportRunStateTimeout,
		retention: defaultReportResultRetention, now: func() time.Time { return time.Now().UTC() },
	}
}

func (reconciler *ReportRunReconciler) Run(ctx context.Context) error {
	if reconciler == nil || reconciler.store == nil || ctx == nil {
		return fmt.Errorf("report run reconciliation: invalid runner")
	}
	reconciler.reconcileCycle(ctx)
	ticker := time.NewTicker(reconciler.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			reconciler.reconcileCycle(ctx)
		}
	}
}

func (reconciler *ReportRunReconciler) reconcileCycle(ctx context.Context) {
	_ = reconciler.Reconcile(ctx)
}

func (reconciler *ReportRunReconciler) Reconcile(ctx context.Context) error {
	if _, err := reconciler.store.RecoverExpiredPreOracleRuns(ctx, reconciler.now().UTC(), reconciler.batchSize); err != nil {
		return err
	}
	runIDs, err := reconciler.store.ListReconciliationCandidates(ctx, reconciler.now().UTC(), reconciler.batchSize)
	if err != nil {
		return err
	}
	for _, runID := range runIDs {
		if err := reconciler.reconcileOne(ctx, runID); err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
	}
	queuedIDs, err := reconciler.store.ListQueuedRunsMissingDelivery(ctx, reconciler.batchSize)
	if err != nil {
		return err
	}
	for _, runID := range queuedIDs {
		if err := reconciler.store.EnsureRunQueued(ctx, runID, reconciler.now().UTC()); err != nil {
			return err
		}
	}
	return nil
}

func (reconciler *ReportRunReconciler) ReconcileOne(ctx context.Context, runID uint) error {
	if reconciler == nil || reconciler.store == nil || ctx == nil || runID == 0 {
		return fmt.Errorf("report run reconciliation: invalid run")
	}
	return reconciler.reconcileOne(ctx, runID)
}

func (reconciler *ReportRunReconciler) reconcileOne(ctx context.Context, runID uint) error {
	leaseToken := uuid.NewString()
	lease, err := reconciler.store.BeginReconciliation(ctx, runID, reconciler.workerID, leaseToken, reconciler.now().UTC(), reconciler.leaseTTL)
	if err != nil {
		return err
	}
	if lease.Disposition != reportrepo.RunDispositionAcquired {
		return nil
	}
	runtime, err := reconciler.store.LoadRuntimeContract(ctx, runID, leaseToken)
	if err != nil {
		return reconciler.pending(ctx, runID, leaseToken, "RECONCILE_CONTRACT_UNAVAILABLE", err)
	}
	password, err := reconciler.credential.Decrypt(runtime.Datasource.CredentialKeyVersion, runtime.Datasource.PasswordCiphertext)
	if err != nil {
		return reconciler.pending(ctx, runID, leaseToken, "RECONCILE_CREDENTIAL_UNAVAILABLE", err)
	}
	receipt, err := reconciler.receipts.Read(ctx, *runtime, password)
	if errors.Is(err, reportoracle.ErrRunReceiptNotFound) {
		return reconciler.pending(ctx, runID, leaseToken, "ORACLE_RECEIPT_NOT_FOUND", err)
	}
	if err != nil {
		return reconciler.pending(ctx, runID, leaseToken, "ORACLE_RECEIPT_UNAVAILABLE", err)
	}
	if receipt.RunID != runtime.Run.RunUUID || receipt.ReportCode != runtime.Definition.Code || receipt.ContractHash != runtime.Run.ContractHash {
		return reconciler.pending(ctx, runID, leaseToken, "ORACLE_RECEIPT_MISMATCH", errors.New("receipt contract mismatch"))
	}
	finishedAt := reconciler.now().UTC()
	stateCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), reconciler.stateTimeout)
	defer cancel()
	return reconciler.store.MarkReconciliationSucceeded(stateCtx, runID, leaseToken, receipt.RowCount, finishedAt, finishedAt.Add(reconciler.retention))
}

func (reconciler *ReportRunReconciler) pending(ctx context.Context, runID uint, leaseToken, code string, cause error) error {
	stateCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), reconciler.stateTimeout)
	defer cancel()
	if err := reconciler.store.MarkReconciliationPending(stateCtx, runID, leaseToken, code, "运行结果等待Oracle回执确认", reconciler.now().UTC()); err != nil {
		return fmt.Errorf("report run reconciliation: persist pending state")
	}
	return nil
}

type oracleReportReceiptReader struct{}

func (oracleReportReceiptReader) Read(ctx context.Context, runtime reportrepo.RuntimeContract, password string) (reportoracle.RunReceipt, error) {
	adapter, err := reportoracle.Open(ctx, oracleConfigFromDatasource(runtime.Datasource, password))
	if err != nil {
		return reportoracle.RunReceipt{}, err
	}
	defer func() { _ = adapter.Close() }()
	plan, err := reportoracle.BuildRunReceiptPlan(runtime.Version.ResultTableOwner)
	if err != nil {
		return reportoracle.RunReceipt{}, err
	}
	return adapter.ReadRunReceipt(ctx, plan, runtime.Run.RunUUID)
}
