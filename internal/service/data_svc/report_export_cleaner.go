package data_svc

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"
	"unicode"

	"gin-biz-web-api/internal/reportrepo"
	"gin-biz-web-api/pkg/storage"

	"github.com/google/uuid"
)

const (
	defaultReportExportCleanupBatchSize = 100
	defaultReportExportCleanupMaxJobs   = 1000
	defaultReportExportCleanupLeaseTTL  = 10 * time.Minute
	defaultReportExportCleanupDeleteTTL = time.Minute
	defaultReportExportCleanupTimeout   = 10 * time.Second
)

type ReportExportCleanupResult struct {
	Scanned int
	Claimed int
	Expired int
	Deleted int
}

type reportExportCleanupStore interface {
	ListExportCleanupCandidates(context.Context, time.Time, uint, int) ([]reportrepo.ExportCleanupCandidate, error)
	ClaimExportCleanup(context.Context, reportrepo.ExportCleanupCandidate, string, time.Time, time.Duration) (bool, error)
	FinishExportCleanup(context.Context, reportrepo.ExportCleanupCandidate, string, time.Time) error
	ReleaseExportCleanup(context.Context, reportrepo.ExportCleanupCandidate, string, time.Time) error
}

type reportExportCleanupObjectStore interface {
	DeleteObject(context.Context, string) error
}

type ReportExportCleaner struct {
	store          reportExportCleanupStore
	newObjectStore func() (reportExportCleanupObjectStore, error)
	objectRoot     string
	now            func() time.Time
	newToken       func() string
	batchSize      int
	maxJobs        int
	leaseTTL       time.Duration
	deleteTimeout  time.Duration
	stateTimeout   time.Duration
}

func NewReportExportCleaner() *ReportExportCleaner {
	return &ReportExportCleaner{
		store: reportrepo.New(),
		newObjectStore: func() (reportExportCleanupObjectStore, error) {
			return storage.NewOSSClientFromConfig()
		},
		objectRoot:    storage.BuildObjectKey(reportExportWorkRootName),
		now:           func() time.Time { return time.Now().UTC() },
		newToken:      uuid.NewString,
		batchSize:     defaultReportExportCleanupBatchSize,
		maxJobs:       defaultReportExportCleanupMaxJobs,
		leaseTTL:      defaultReportExportCleanupLeaseTTL,
		deleteTimeout: defaultReportExportCleanupDeleteTTL,
		stateTimeout:  defaultReportExportCleanupTimeout,
	}
}

func (cleaner *ReportExportCleaner) Cleanup(ctx context.Context) (ReportExportCleanupResult, error) {
	var result ReportExportCleanupResult
	if cleaner == nil || cleaner.store == nil || cleaner.newObjectStore == nil || cleaner.now == nil || cleaner.newToken == nil ||
		ctx == nil || cleaner.batchSize < 1 || cleaner.batchSize > cleaner.maxJobs || cleaner.maxJobs < 1 ||
		cleaner.leaseTTL <= 0 || cleaner.deleteTimeout <= 0 || cleaner.deleteTimeout >= cleaner.leaseTTL ||
		cleaner.stateTimeout <= 0 || strings.TrimSpace(cleaner.objectRoot) == "" {
		return result, fmt.Errorf("report export cleaner: invalid configuration")
	}
	now := cleaner.now().UTC()
	if now.IsZero() {
		return result, fmt.Errorf("report export cleaner: invalid clock")
	}
	var afterID uint
	var objectStore reportExportCleanupObjectStore
	for result.Scanned < cleaner.maxJobs {
		limit := min(cleaner.batchSize, cleaner.maxJobs-result.Scanned)
		candidates, err := cleaner.store.ListExportCleanupCandidates(ctx, now, afterID, limit)
		if err != nil {
			return result, fmt.Errorf("report export cleaner: list candidates: %w", err)
		}
		if len(candidates) == 0 {
			return result, nil
		}
		if len(candidates) > limit {
			return result, fmt.Errorf("report export cleaner: candidate batch exceeds limit")
		}
		for _, candidate := range candidates {
			if candidate.ID <= afterID {
				return result, fmt.Errorf("report export cleaner: invalid candidate cursor")
			}
			afterID = candidate.ID
			result.Scanned++
			leaseToken := cleaner.newToken()
			claimed, err := cleaner.store.ClaimExportCleanup(ctx, candidate, leaseToken, now, cleaner.leaseTTL)
			if err != nil {
				return result, fmt.Errorf("report export cleaner: claim export %d: %w", candidate.ID, err)
			}
			if !claimed {
				continue
			}
			result.Claimed++
			if !validReportExportResultObjectKey(candidate.ResultObjectKey, candidate.ExportUUID, cleaner.objectRoot) {
				return result, cleaner.releaseAfterError(ctx, candidate, leaseToken, fmt.Errorf("report export cleaner: invalid stored object key"))
			}
			if objectStore == nil {
				objectStore, err = cleaner.newObjectStore()
				if err == nil && objectStore == nil {
					err = fmt.Errorf("report export cleaner: nil object store")
				}
				if err != nil {
					return result, cleaner.releaseAfterError(ctx, candidate, leaseToken, err)
				}
			}
			deleteCtx, cancel := context.WithTimeout(ctx, cleaner.deleteTimeout)
			err = objectStore.DeleteObject(deleteCtx, candidate.ResultObjectKey)
			cancel()
			if err != nil {
				return result, cleaner.releaseAfterError(ctx, candidate, leaseToken, fmt.Errorf("report export cleaner: delete object: %w", err))
			}
			result.Deleted++
			stateCtx, cancel := cleaner.stateContext(ctx)
			err = cleaner.store.FinishExportCleanup(stateCtx, candidate, leaseToken, cleaner.now().UTC())
			cancel()
			if err != nil {
				return result, fmt.Errorf("report export cleaner: finish export %d: %w", candidate.ID, err)
			}
			result.Expired++
		}
		if len(candidates) < limit {
			return result, nil
		}
	}
	return result, nil
}

func (cleaner *ReportExportCleaner) releaseAfterError(
	ctx context.Context,
	candidate reportrepo.ExportCleanupCandidate,
	leaseToken string,
	cause error,
) error {
	stateCtx, cancel := cleaner.stateContext(ctx)
	defer cancel()
	releaseErr := cleaner.store.ReleaseExportCleanup(stateCtx, candidate, leaseToken, cleaner.now().UTC())
	if errors.Is(releaseErr, reportrepo.ErrReportExportCleanupLeaseLost) {
		releaseErr = nil
	}
	return errors.Join(cause, releaseErr)
}

func (cleaner *ReportExportCleaner) stateContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(parent), cleaner.stateTimeout)
}

func validReportExportResultObjectKey(value, exportUUID, expectedRoot string) bool {
	if uuid.Validate(exportUUID) != nil || value == "" || len(value) > 1024 || value != strings.TrimSpace(value) ||
		strings.HasPrefix(value, "/") || strings.Contains(value, "\\") {
		return false
	}
	for _, char := range value {
		if unicode.IsControl(char) {
			return false
		}
	}
	root := strings.Trim(strings.TrimSpace(expectedRoot), "/")
	if root == "" || path.Clean(value) != value || !strings.HasPrefix(value, root+"/") {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(value, root+"/"), "/")
	if len(parts) != 6 || len(parts[0]) != 4 || len(parts[1]) != 2 || len(parts[2]) != 2 ||
		parts[3] != exportUUID || uuid.Validate(parts[4]) != nil || parts[5] != "result.xlsx" {
		return false
	}
	for _, datePart := range parts[:3] {
		for _, char := range datePart {
			if char < '0' || char > '9' {
				return false
			}
		}
	}
	return true
}
