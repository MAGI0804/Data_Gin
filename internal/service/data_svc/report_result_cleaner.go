package data_svc

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gin-biz-web-api/internal/reportrepo"
	"gin-biz-web-api/internal/reportsecret"

	"github.com/google/uuid"
)

const (
	defaultReportResultCleanupBatchSize = 100
	defaultReportResultCleanupMaxRuns   = 1000
	defaultReportResultCleanupLeaseTTL  = 10 * time.Minute
	defaultReportResultCleanupTimeout   = 10 * time.Second
)

type ReportResultCleanupResult struct {
	Scanned int
	Claimed int
	Purged  int
}

type reportResultCleanupStore interface {
	ListReadyResultCleanupCandidates(context.Context, time.Time, uint, int) ([]reportrepo.ResultCleanupCandidate, error)
	ListExpiredResultCleanupCandidates(context.Context, time.Time, uint, int) ([]reportrepo.ResultCleanupCandidate, error)
	ClaimExpiredResultCleanup(context.Context, uint, string, time.Time, time.Duration) (*reportrepo.ResultCleanupRuntime, error)
	UpdateExpiredResultCleanupProgress(context.Context, uint, string, int64, time.Time, time.Duration) error
	MarkExpiredResultPurged(context.Context, uint, string, int64, time.Time) error
	ReleaseExpiredResultCleanup(context.Context, uint, string, time.Time) error
}

type reportReadyResultPurger interface {
	CleanupReadyResult(context.Context, uint) error
}

type ReportResultCleaner struct {
	store          reportResultCleanupStore
	ready          reportReadyResultPurger
	credential     reportDatasourceCredentialDecryptor
	oracle         reportResultCleanupOracleFactory
	now            func() time.Time
	newToken       func() string
	batchSize      int
	maxRuns        int
	leaseTTL       time.Duration
	stateTimeout   time.Duration
	purgeBatchSize int
}

func NewReportResultCleaner() *ReportResultCleaner {
	return NewReportResultCleanerWithDependencies(
		reportrepo.New(), NewReportExportProcessor(), reportsecret.EnvironmentKeyring{}, oracleReportResultCleanupFactory{},
	)
}

func NewReportResultCleanerWithDependencies(store reportResultCleanupStore, ready reportReadyResultPurger, credential reportDatasourceCredentialDecryptor, oracle reportResultCleanupOracleFactory) *ReportResultCleaner {
	if store == nil || ready == nil || credential == nil || oracle == nil {
		panic("report result cleaner: dependencies are required")
	}
	return &ReportResultCleaner{
		store: store, ready: ready, credential: credential, oracle: oracle,
		now: func() time.Time { return time.Now().UTC() }, newToken: uuid.NewString,
		batchSize: defaultReportResultCleanupBatchSize, maxRuns: defaultReportResultCleanupMaxRuns,
		leaseTTL: defaultReportResultCleanupLeaseTTL, stateTimeout: defaultReportResultCleanupTimeout,
		purgeBatchSize: defaultReportExportPurgeBatchSize,
	}
}

func (cleaner *ReportResultCleaner) Cleanup(ctx context.Context) (ReportResultCleanupResult, error) {
	var result ReportResultCleanupResult
	if cleaner == nil || cleaner.store == nil || cleaner.ready == nil || cleaner.credential == nil || cleaner.oracle == nil || cleaner.now == nil || cleaner.newToken == nil ||
		ctx == nil || cleaner.batchSize < 1 || cleaner.maxRuns < 1 || cleaner.batchSize > cleaner.maxRuns || cleaner.leaseTTL <= 0 || cleaner.stateTimeout <= 0 || cleaner.purgeBatchSize < 1 {
		return result, fmt.Errorf("report result cleaner: invalid configuration")
	}
	now := cleaner.now().UTC()
	var cleanupErrors []error
	readyIDs := make(map[uint]struct{})
	var afterExportID uint
	for result.Scanned < cleaner.maxRuns {
		limit := min(cleaner.batchSize, cleaner.maxRuns-result.Scanned)
		candidates, err := cleaner.store.ListReadyResultCleanupCandidates(ctx, now, afterExportID, limit)
		if err != nil {
			return result, errors.Join(append(cleanupErrors, err)...)
		}
		if len(candidates) == 0 {
			break
		}
		for _, candidate := range candidates {
			if candidate.ExportID <= afterExportID || candidate.RunID == 0 {
				return result, errors.Join(append(cleanupErrors, errors.New("report result cleaner: invalid ready cursor"))...)
			}
			afterExportID = candidate.ExportID
			readyIDs[candidate.RunID] = struct{}{}
			result.Scanned++
			if err := cleaner.ready.CleanupReadyResult(ctx, candidate.ExportID); err != nil {
				cleanupErrors = append(cleanupErrors, fmt.Errorf("report result cleaner: ready export %d: %w", candidate.ExportID, err))
				continue
			}
			result.Claimed++
			result.Purged++
		}
		if len(candidates) < limit {
			break
		}
	}
	var afterRunID uint
	for result.Scanned < cleaner.maxRuns {
		limit := min(cleaner.batchSize, cleaner.maxRuns-result.Scanned)
		candidates, err := cleaner.store.ListExpiredResultCleanupCandidates(ctx, now, afterRunID, limit)
		if err != nil {
			return result, errors.Join(append(cleanupErrors, err)...)
		}
		if len(candidates) == 0 {
			break
		}
		for _, candidate := range candidates {
			if candidate.RunID <= afterRunID {
				return result, errors.Join(append(cleanupErrors, errors.New("report result cleaner: invalid expired cursor"))...)
			}
			afterRunID = candidate.RunID
			if _, recoveredAsReady := readyIDs[candidate.RunID]; recoveredAsReady {
				continue
			}
			result.Scanned++
			claimed, err := cleaner.cleanupExpired(ctx, candidate.RunID)
			if claimed {
				result.Claimed++
			}
			if err != nil {
				if !errors.Is(err, reportrepo.ErrReportResultCleanupConflict) {
					cleanupErrors = append(cleanupErrors, fmt.Errorf("report result cleaner: run %d: %w", candidate.RunID, err))
				}
				continue
			}
			if claimed {
				result.Purged++
			}
		}
		if len(candidates) < limit {
			break
		}
	}
	return result, errors.Join(cleanupErrors...)
}

func (cleaner *ReportResultCleaner) CleanupRun(ctx context.Context, runID uint) (bool, error) {
	if cleaner == nil || cleaner.store == nil || cleaner.credential == nil || cleaner.oracle == nil || cleaner.now == nil || cleaner.newToken == nil ||
		ctx == nil || runID == 0 || cleaner.leaseTTL <= 0 || cleaner.stateTimeout <= 0 || cleaner.purgeBatchSize < 1 {
		return false, fmt.Errorf("report result cleaner: invalid targeted cleanup")
	}
	claimed, err := cleaner.cleanupExpired(ctx, runID)
	if errors.Is(err, reportrepo.ErrReportResultCleanupConflict) {
		return false, nil
	}
	return claimed, err
}

func (cleaner *ReportResultCleaner) cleanupExpired(ctx context.Context, runID uint) (bool, error) {
	token := cleaner.newToken()
	stateCtx, cancel := cleaner.stateContext(ctx)
	runtime, err := cleaner.store.ClaimExpiredResultCleanup(stateCtx, runID, token, cleaner.now(), cleaner.leaseTTL)
	cancel()
	if err != nil {
		return false, err
	}
	password, err := cleaner.credential.Decrypt(runtime.Datasource.CredentialKeyVersion, runtime.Datasource.PasswordCiphertext)
	if err != nil {
		return true, cleaner.release(ctx, runID, token, err)
	}
	session, err := cleaner.oracle.Open(ctx, *runtime, password)
	if err != nil {
		return true, cleaner.release(ctx, runID, token, err)
	}
	defer func() { _ = session.Close() }()
	var cumulative int64
	for {
		deleted, purgeErr := session.Purge(ctx, cleaner.purgeBatchSize)
		if purgeErr != nil {
			return true, cleaner.release(ctx, runID, token, purgeErr)
		}
		cumulative += deleted
		stateCtx, cancel = cleaner.stateContext(ctx)
		progressErr := cleaner.store.UpdateExpiredResultCleanupProgress(stateCtx, runID, token, cumulative, cleaner.now(), cleaner.leaseTTL)
		cancel()
		if progressErr != nil {
			return true, cleaner.release(ctx, runID, token, progressErr)
		}
		if deleted < int64(cleaner.purgeBatchSize) {
			break
		}
	}
	stateCtx, cancel = cleaner.stateContext(ctx)
	err = cleaner.store.MarkExpiredResultPurged(stateCtx, runID, token, runtime.Run.RowCount, cleaner.now())
	cancel()
	return true, err
}

func (cleaner *ReportResultCleaner) release(ctx context.Context, runID uint, token string, cause error) error {
	stateCtx, cancel := cleaner.stateContext(ctx)
	defer cancel()
	err := cleaner.store.ReleaseExpiredResultCleanup(stateCtx, runID, token, cleaner.now())
	if errors.Is(err, reportrepo.ErrReportResultCleanupLeaseLost) {
		err = nil
	}
	return errors.Join(cause, err)
}

func (cleaner *ReportResultCleaner) stateContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(parent), cleaner.stateTimeout)
}
