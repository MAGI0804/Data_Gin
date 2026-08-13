package data_svc

import (
	"context"
	"errors"
	"testing"
	"time"

	"gin-biz-web-api/internal/reportrepo"

	"github.com/google/uuid"
)

func TestReportExportCleanerDeletesClaimedObjectAndExpiresMetadata(t *testing.T) {
	now := time.Date(2026, 8, 13, 8, 0, 0, 0, time.UTC)
	exportUUID := uuid.NewString()
	key := reportExportCleanupTestKey(exportUUID)
	store := &fakeReportExportCleanupStore{candidates: []reportrepo.ExportCleanupCandidate{{ID: 17, ExportUUID: exportUUID, ResultObjectKey: key}}}
	objects := &fakeReportExportCleanupObjectStore{}
	cleaner := testReportExportCleaner(now, store, objects)
	result, err := cleaner.Cleanup(t.Context())
	if err != nil {
		t.Fatalf("Cleanup() error=%v", err)
	}
	if result != (ReportExportCleanupResult{Scanned: 1, Claimed: 1, Expired: 1, Deleted: 1}) ||
		len(objects.deleted) != 1 || objects.deleted[0] != key || len(store.finished) != 1 || len(store.released) != 0 {
		t.Fatalf("result=%+v deleted=%v finished=%v released=%v", result, objects.deleted, store.finished, store.released)
	}
	if store.claimTTL != defaultReportExportCleanupLeaseTTL {
		t.Fatalf("claim TTL=%v", store.claimTTL)
	}
}

func TestReportExportCleanerReleasesFailedDeletionForRetry(t *testing.T) {
	now := time.Date(2026, 8, 13, 8, 0, 0, 0, time.UTC)
	exportUUID := uuid.NewString()
	store := &fakeReportExportCleanupStore{candidates: []reportrepo.ExportCleanupCandidate{{
		ID: 17, ExportUUID: exportUUID, ResultObjectKey: reportExportCleanupTestKey(exportUUID),
	}}}
	objects := &fakeReportExportCleanupObjectStore{err: errors.New("temporary OSS failure")}
	result, err := testReportExportCleaner(now, store, objects).Cleanup(t.Context())
	if err == nil || result.Claimed != 1 || result.Deleted != 0 || result.Expired != 0 ||
		len(store.released) != 1 || len(store.finished) != 0 {
		t.Fatalf("result=%+v error=%v released=%v finished=%v", result, err, store.released, store.finished)
	}
}

func TestReportExportCleanerRetriesIdempotentDeleteAfterFinishFailure(t *testing.T) {
	now := time.Date(2026, 8, 13, 8, 0, 0, 0, time.UTC)
	exportUUID := uuid.NewString()
	store := &fakeReportExportCleanupStore{
		candidates: []reportrepo.ExportCleanupCandidate{{
			ID: 17, ExportUUID: exportUUID, ResultObjectKey: reportExportCleanupTestKey(exportUUID),
		}},
		finishErr: errors.New("temporary MySQL failure"),
	}
	objects := &fakeReportExportCleanupObjectStore{}
	cleaner := testReportExportCleaner(now, store, objects)

	first, err := cleaner.Cleanup(t.Context())
	if err == nil || first.Deleted != 1 || first.Expired != 0 || len(store.released) != 0 {
		t.Fatalf("first cleanup result=%+v error=%v released=%v", first, err, store.released)
	}
	store.finishErr = nil
	second, err := cleaner.Cleanup(t.Context())
	if err != nil || second.Deleted != 1 || second.Expired != 1 || len(objects.deleted) != 2 {
		t.Fatalf("second cleanup result=%+v error=%v deleted=%v", second, err, objects.deleted)
	}
}

func TestReportExportCleanerBoundsObjectDeletionByLease(t *testing.T) {
	now := time.Date(2026, 8, 13, 8, 0, 0, 0, time.UTC)
	exportUUID := uuid.NewString()
	store := &fakeReportExportCleanupStore{candidates: []reportrepo.ExportCleanupCandidate{{
		ID: 17, ExportUUID: exportUUID, ResultObjectKey: reportExportCleanupTestKey(exportUUID),
	}}}
	objects := &fakeReportExportCleanupObjectStore{inspectContext: true}
	cleaner := testReportExportCleaner(now, store, objects)
	cleaner.deleteTimeout = 500 * time.Millisecond
	cleaner.leaseTTL = time.Second

	if _, err := cleaner.Cleanup(t.Context()); err != nil {
		t.Fatalf("Cleanup() error=%v", err)
	}
	if !objects.hadDeadline {
		t.Fatal("DeleteObject() context had no deadline")
	}
}

func TestReportExportCleanerSkipsLeaseLostToConcurrentWorker(t *testing.T) {
	now := time.Date(2026, 8, 13, 8, 0, 0, 0, time.UTC)
	exportUUID := uuid.NewString()
	store := &fakeReportExportCleanupStore{
		candidates:  []reportrepo.ExportCleanupCandidate{{ID: 17, ExportUUID: exportUUID, ResultObjectKey: reportExportCleanupTestKey(exportUUID)}},
		claimDenied: true,
	}
	objects := &fakeReportExportCleanupObjectStore{}
	result, err := testReportExportCleaner(now, store, objects).Cleanup(t.Context())
	if err != nil || result != (ReportExportCleanupResult{Scanned: 1}) || len(objects.deleted) != 0 {
		t.Fatalf("result=%+v error=%v deleted=%v", result, err, objects.deleted)
	}
}

func TestReportExportCleanerRejectsObjectOutsideReportNamespace(t *testing.T) {
	now := time.Date(2026, 8, 13, 8, 0, 0, 0, time.UTC)
	exportUUID := uuid.NewString()
	store := &fakeReportExportCleanupStore{candidates: []reportrepo.ExportCleanupCandidate{{
		ID: 17, ExportUUID: exportUUID, ResultObjectKey: "mall-weather-exports/2026/08/13/" + exportUUID + "/" + uuid.NewString() + "/result.xlsx",
	}}}
	objects := &fakeReportExportCleanupObjectStore{}
	result, err := testReportExportCleaner(now, store, objects).Cleanup(t.Context())
	if err == nil || result.Claimed != 1 || len(store.released) != 1 || len(objects.deleted) != 0 {
		t.Fatalf("result=%+v error=%v released=%v deleted=%v", result, err, store.released, objects.deleted)
	}
}

func TestValidReportExportResultObjectKeyRequiresExportIdentity(t *testing.T) {
	exportUUID := uuid.NewString()
	valid := reportExportCleanupTestKey(exportUUID)
	for _, test := range []struct {
		name  string
		value string
		want  bool
	}{
		{name: "valid", value: valid, want: true},
		{name: "wrong export", value: reportExportCleanupTestKey(uuid.NewString())},
		{name: "parent traversal", value: "report-exports/../private.xlsx"},
		{name: "wrong file", value: valid + ".bak"},
		{name: "absolute", value: "/" + valid},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := validReportExportResultObjectKey(test.value, exportUUID, reportExportWorkRootName); got != test.want {
				t.Fatalf("validReportExportResultObjectKey()=%v want=%v", got, test.want)
			}
		})
	}
}

func testReportExportCleaner(now time.Time, store *fakeReportExportCleanupStore, objects *fakeReportExportCleanupObjectStore) *ReportExportCleaner {
	return &ReportExportCleaner{
		store:          store,
		newObjectStore: func() (reportExportCleanupObjectStore, error) { return objects, nil },
		objectRoot:     reportExportWorkRootName,
		now:            func() time.Time { return now }, newToken: uuid.NewString,
		batchSize: 100, maxJobs: 1000, leaseTTL: defaultReportExportCleanupLeaseTTL,
		deleteTimeout: time.Second, stateTimeout: time.Second,
	}
}

func reportExportCleanupTestKey(exportUUID string) string {
	return "report-exports/2026/08/13/" + exportUUID + "/" + uuid.NewString() + "/result.xlsx"
}

type fakeReportExportCleanupStore struct {
	candidates  []reportrepo.ExportCleanupCandidate
	claimDenied bool
	claimTTL    time.Duration
	finishErr   error
	finished    []uint
	released    []uint
}

func (store *fakeReportExportCleanupStore) ListExportCleanupCandidates(_ context.Context, _ time.Time, afterID uint, limit int) ([]reportrepo.ExportCleanupCandidate, error) {
	rows := make([]reportrepo.ExportCleanupCandidate, 0, limit)
	for _, candidate := range store.candidates {
		if candidate.ID > afterID && len(rows) < limit {
			rows = append(rows, candidate)
		}
	}
	return rows, nil
}

func (store *fakeReportExportCleanupStore) ClaimExportCleanup(_ context.Context, _ reportrepo.ExportCleanupCandidate, _ string, _ time.Time, ttl time.Duration) (bool, error) {
	store.claimTTL = ttl
	return !store.claimDenied, nil
}

func (store *fakeReportExportCleanupStore) FinishExportCleanup(_ context.Context, candidate reportrepo.ExportCleanupCandidate, _ string, _ time.Time) error {
	store.finished = append(store.finished, candidate.ID)
	return store.finishErr
}

func (store *fakeReportExportCleanupStore) ReleaseExportCleanup(_ context.Context, candidate reportrepo.ExportCleanupCandidate, _ string, _ time.Time) error {
	store.released = append(store.released, candidate.ID)
	return nil
}

type fakeReportExportCleanupObjectStore struct {
	deleted        []string
	err            error
	inspectContext bool
	hadDeadline    bool
}

func (store *fakeReportExportCleanupObjectStore) DeleteObject(ctx context.Context, key string) error {
	store.deleted = append(store.deleted, key)
	if store.inspectContext {
		_, store.hadDeadline = ctx.Deadline()
	}
	return store.err
}
