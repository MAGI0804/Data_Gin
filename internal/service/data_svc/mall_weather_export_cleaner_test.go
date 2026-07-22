package data_svc

import (
	"context"
	"errors"
	"testing"
	"time"

	"gin-biz-web-api/internal/dao/data_dao"

	"github.com/google/uuid"
)

func TestMallWeatherExportCleanerDeletesClaimedObjectsAndExpiresMetadata(t *testing.T) {
	now := time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC)
	jobs := &fakeMallWeatherExportCleanupStore{candidates: []data_dao.MallWeatherExportCleanupCandidate{
		{ID: 17, ResultObjectKey: "mall-weather-exports/17/result.xlsx"},
		{ID: 18},
	}}
	objects := &fakeMallWeatherExportCleanupObjectStore{}
	cleaner := testMallWeatherExportCleaner(now, jobs, objects)
	result, err := cleaner.Cleanup(t.Context())
	if err != nil {
		t.Fatalf("Cleanup() error=%v", err)
	}
	if result != (MallWeatherExportCleanupResult{Scanned: 2, Claimed: 2, Expired: 2, Deleted: 1}) {
		t.Fatalf("Cleanup() result=%+v", result)
	}
	if len(objects.deleted) != 1 || objects.deleted[0] != jobs.candidates[0].ResultObjectKey ||
		len(jobs.finished) != 2 || len(jobs.released) != 0 {
		t.Fatalf("deleted=%v finished=%v released=%v", objects.deleted, jobs.finished, jobs.released)
	}
	if jobs.listNow != now || jobs.staleBefore != now.Add(-10*time.Minute) {
		t.Fatalf("list now=%v staleBefore=%v", jobs.listNow, jobs.staleBefore)
	}
}

func TestMallWeatherExportCleanerReleasesFailedDeletionForRetry(t *testing.T) {
	now := time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC)
	jobs := &fakeMallWeatherExportCleanupStore{candidates: []data_dao.MallWeatherExportCleanupCandidate{{
		ID: 17, ResultObjectKey: "mall-weather-exports/17/result.xlsx",
	}}}
	objects := &fakeMallWeatherExportCleanupObjectStore{err: errors.New("temporary OSS failure")}
	cleaner := testMallWeatherExportCleaner(now, jobs, objects)
	result, err := cleaner.Cleanup(t.Context())
	if err == nil || result.Claimed != 1 || result.Expired != 0 || result.Deleted != 0 ||
		len(jobs.released) != 1 || len(jobs.finished) != 0 {
		t.Fatalf("Cleanup() result=%+v error=%v released=%v", result, err, jobs.released)
	}
}

func TestMallWeatherExportCleanerSkipsLeaseLostToAnotherInstance(t *testing.T) {
	now := time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC)
	jobs := &fakeMallWeatherExportCleanupStore{
		candidates:  []data_dao.MallWeatherExportCleanupCandidate{{ID: 17, ResultObjectKey: "object.xlsx"}},
		claimDenied: true,
	}
	objects := &fakeMallWeatherExportCleanupObjectStore{}
	cleaner := testMallWeatherExportCleaner(now, jobs, objects)
	result, err := cleaner.Cleanup(t.Context())
	if err != nil || result != (MallWeatherExportCleanupResult{Scanned: 1}) || len(objects.deleted) != 0 {
		t.Fatalf("Cleanup() result=%+v error=%v deleted=%v", result, err, objects.deleted)
	}
}

func TestMallWeatherExportCleanerRejectsUnsafeStoredObjectKey(t *testing.T) {
	now := time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC)
	jobs := &fakeMallWeatherExportCleanupStore{candidates: []data_dao.MallWeatherExportCleanupCandidate{{
		ID: 17, ResultObjectKey: "../another-job/result.xlsx",
	}}}
	objects := &fakeMallWeatherExportCleanupObjectStore{}
	cleaner := testMallWeatherExportCleaner(now, jobs, objects)
	result, err := cleaner.Cleanup(t.Context())
	if err == nil || result.Claimed != 1 || len(jobs.released) != 1 || len(objects.deleted) != 0 {
		t.Fatalf("Cleanup() result=%+v error=%v released=%v deleted=%v", result, err, jobs.released, objects.deleted)
	}
}

func testMallWeatherExportCleaner(
	now time.Time,
	jobs *fakeMallWeatherExportCleanupStore,
	objects *fakeMallWeatherExportCleanupObjectStore,
) *MallWeatherExportCleaner {
	return &MallWeatherExportCleaner{
		jobs: jobs,
		newObjectStore: func() (mallWeatherExportCleanupObjectStore, error) {
			return objects, nil
		},
		now:        func() time.Time { return now },
		newToken:   uuid.NewString,
		batchSize:  100,
		maxJobs:    1000,
		staleAfter: 10 * time.Minute,
	}
}

type fakeMallWeatherExportCleanupStore struct {
	candidates  []data_dao.MallWeatherExportCleanupCandidate
	claimDenied bool
	listNow     time.Time
	staleBefore time.Time
	finished    []uint
	released    []uint
}

func (store *fakeMallWeatherExportCleanupStore) ListCleanupCandidates(
	_ context.Context,
	now time.Time,
	staleBefore time.Time,
	afterID uint,
	limit int,
) ([]data_dao.MallWeatherExportCleanupCandidate, error) {
	store.listNow = now
	store.staleBefore = staleBefore
	rows := make([]data_dao.MallWeatherExportCleanupCandidate, 0, limit)
	for _, candidate := range store.candidates {
		if candidate.ID > afterID && len(rows) < limit {
			rows = append(rows, candidate)
		}
	}
	return rows, nil
}

func (store *fakeMallWeatherExportCleanupStore) ClaimCleanup(
	_ context.Context,
	_ data_dao.MallWeatherExportCleanupCandidate,
	_ string,
	_ time.Time,
	_ time.Time,
) (bool, error) {
	if store.claimDenied {
		return false, nil
	}
	return true, nil
}

func (store *fakeMallWeatherExportCleanupStore) FinishCleanup(
	_ context.Context,
	candidate data_dao.MallWeatherExportCleanupCandidate,
	_ string,
	_ time.Time,
) error {
	store.finished = append(store.finished, candidate.ID)
	return nil
}

func (store *fakeMallWeatherExportCleanupStore) ReleaseCleanup(
	_ context.Context,
	candidate data_dao.MallWeatherExportCleanupCandidate,
	_ string,
	_ time.Time,
) error {
	store.released = append(store.released, candidate.ID)
	return nil
}

type fakeMallWeatherExportCleanupObjectStore struct {
	deleted []string
	err     error
}

func (store *fakeMallWeatherExportCleanupObjectStore) DeleteObject(_ context.Context, objectKey string) error {
	store.deleted = append(store.deleted, objectKey)
	return store.err
}
