package data_svc

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/pkg/storage"

	"github.com/google/uuid"
)

const (
	defaultMallWeatherExportCleanupBatchSize  = 100
	defaultMallWeatherExportCleanupMaxJobs    = 1000
	defaultMallWeatherExportCleanupStaleAfter = 10 * time.Minute
)

type MallWeatherExportCleanupResult struct {
	Scanned int
	Claimed int
	Expired int
	Deleted int
}

type mallWeatherExportCleanupStore interface {
	ListCleanupCandidates(context.Context, time.Time, time.Time, uint, int) ([]data_dao.MallWeatherExportCleanupCandidate, error)
	ClaimCleanup(context.Context, data_dao.MallWeatherExportCleanupCandidate, string, time.Time, time.Time) (bool, error)
	FinishCleanup(context.Context, data_dao.MallWeatherExportCleanupCandidate, string, time.Time) error
	ReleaseCleanup(context.Context, data_dao.MallWeatherExportCleanupCandidate, string, time.Time) error
}

type mallWeatherExportCleanupObjectStore interface {
	DeleteObject(context.Context, string) error
}

type mallWeatherExportCleanupObjectStoreFactory func() (mallWeatherExportCleanupObjectStore, error)

type MallWeatherExportCleaner struct {
	jobs           mallWeatherExportCleanupStore
	newObjectStore mallWeatherExportCleanupObjectStoreFactory
	now            func() time.Time
	newToken       func() string
	batchSize      int
	maxJobs        int
	staleAfter     time.Duration
}

func NewMallWeatherExportCleaner() *MallWeatherExportCleaner {
	return &MallWeatherExportCleaner{
		jobs: data_dao.NewMallWeatherExportJobDAO(),
		newObjectStore: func() (mallWeatherExportCleanupObjectStore, error) {
			return storage.NewOSSClientFromConfig()
		},
		now:        time.Now,
		newToken:   uuid.NewString,
		batchSize:  defaultMallWeatherExportCleanupBatchSize,
		maxJobs:    defaultMallWeatherExportCleanupMaxJobs,
		staleAfter: defaultMallWeatherExportCleanupStaleAfter,
	}
}

func (cleaner *MallWeatherExportCleaner) Cleanup(
	ctx context.Context,
) (MallWeatherExportCleanupResult, error) {
	var result MallWeatherExportCleanupResult
	if cleaner == nil || cleaner.jobs == nil || cleaner.newObjectStore == nil || cleaner.now == nil ||
		cleaner.newToken == nil || ctx == nil || cleaner.batchSize < 1 || cleaner.batchSize > cleaner.maxJobs ||
		cleaner.maxJobs < 1 || cleaner.staleAfter <= 0 {
		return result, fmt.Errorf("mall weather export cleaner: invalid configuration")
	}
	now := cleaner.now().UTC()
	if now.IsZero() {
		return result, fmt.Errorf("mall weather export cleaner: invalid clock")
	}
	staleBefore := now.Add(-cleaner.staleAfter)
	var afterID uint
	var objectStore mallWeatherExportCleanupObjectStore
	for result.Scanned < cleaner.maxJobs {
		limit := min(cleaner.batchSize, cleaner.maxJobs-result.Scanned)
		candidates, err := cleaner.jobs.ListCleanupCandidates(ctx, now, staleBefore, afterID, limit)
		if err != nil {
			return result, fmt.Errorf("mall weather export cleaner: list candidates: %w", err)
		}
		if len(candidates) == 0 {
			return result, nil
		}
		if len(candidates) > limit {
			return result, fmt.Errorf("mall weather export cleaner: candidate batch exceeds limit")
		}
		for _, candidate := range candidates {
			if candidate.ID <= afterID {
				return result, fmt.Errorf("mall weather export cleaner: invalid candidate cursor")
			}
			afterID = candidate.ID
			result.Scanned++
			cleanupToken := cleaner.newToken()
			claimed, err := cleaner.jobs.ClaimCleanup(ctx, candidate, cleanupToken, now, staleBefore)
			if err != nil {
				return result, fmt.Errorf("mall weather export cleaner: claim job %d: %w", candidate.ID, err)
			}
			if !claimed {
				continue
			}
			result.Claimed++
			if candidate.ResultObjectKey != "" {
				if !validMallWeatherExportResultObjectKey(candidate.ResultObjectKey) {
					return result, cleaner.releaseAfterError(
						ctx, candidate, cleanupToken, fmt.Errorf("mall weather export cleaner: invalid stored object key"),
					)
				}
				if objectStore == nil {
					objectStore, err = cleaner.newObjectStore()
					if err == nil && objectStore == nil {
						err = fmt.Errorf("mall weather export cleaner: nil object store")
					}
					if err != nil {
						return result, cleaner.releaseAfterError(ctx, candidate, cleanupToken, err)
					}
				}
				if err := objectStore.DeleteObject(ctx, candidate.ResultObjectKey); err != nil {
					return result, cleaner.releaseAfterError(ctx, candidate, cleanupToken, fmt.Errorf(
						"mall weather export cleaner: delete object: %w", err,
					))
				}
				result.Deleted++
			}
			stateCtx, cancel := mallWeatherExportStateContext(ctx)
			err = cleaner.jobs.FinishCleanup(stateCtx, candidate, cleanupToken, cleaner.now().UTC())
			cancel()
			if err != nil {
				return result, fmt.Errorf("mall weather export cleaner: finish job %d: %w", candidate.ID, err)
			}
			result.Expired++
		}
		if len(candidates) < limit {
			return result, nil
		}
	}
	return result, nil
}

func (cleaner *MallWeatherExportCleaner) releaseAfterError(
	ctx context.Context,
	candidate data_dao.MallWeatherExportCleanupCandidate,
	cleanupToken string,
	cause error,
) error {
	stateCtx, cancel := mallWeatherExportStateContext(ctx)
	defer cancel()
	releaseErr := cleaner.jobs.ReleaseCleanup(stateCtx, candidate, cleanupToken, cleaner.now().UTC())
	if errors.Is(releaseErr, data_dao.ErrMallWeatherExportCleanupLeaseLost) {
		releaseErr = nil
	}
	return errors.Join(cause, releaseErr)
}
