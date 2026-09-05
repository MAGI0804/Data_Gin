package data_svc

import (
	"context"
	"errors"
	"testing"
	"time"

	"gin-biz-web-api/internal/reportoracle"
	"gin-biz-web-api/internal/reportrepo"
	"gin-biz-web-api/model"
)

func TestReportResultCleanerRecoversReadyAndPurgesExpiredRuns(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	store := &fakeReportResultCleanupStore{
		ready:   []reportrepo.ResultCleanupCandidate{{RunID: 31, ExportID: 41}},
		expired: []reportrepo.ResultCleanupCandidate{{RunID: 32}},
		runtime: testResultCleanupRuntime(now),
	}
	ready := &fakeReadyResultPurger{}
	session := &fakeReportExportOracleSession{purgeCounts: []int64{5000, 2}}
	cleaner := testReportResultCleaner(now, store, ready, session)
	result, err := cleaner.Cleanup(t.Context())
	if err != nil {
		t.Fatalf("Cleanup() error=%v", err)
	}
	if result != (ReportResultCleanupResult{Scanned: 2, Claimed: 2, Purged: 2}) || ready.exportID != 41 ||
		store.claimed != 32 || !store.finished || store.finishedRows != 9 || session.purgeCalls != 2 {
		t.Fatalf("result=%+v ready=%d store=%#v purgeCalls=%d", result, ready.exportID, store, session.purgeCalls)
	}
}

func TestReportResultCleanerReleasesExpiredRunAfterOracleFailure(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	store := &fakeReportResultCleanupStore{expired: []reportrepo.ResultCleanupCandidate{{RunID: 32}}, runtime: testResultCleanupRuntime(now)}
	session := &fakeResultCleanupOracleSession{err: errors.New("oracle unavailable")}
	cleaner := testReportResultCleaner(now, store, &fakeReadyResultPurger{}, session)
	result, err := cleaner.Cleanup(t.Context())
	if err == nil || result.Claimed != 1 || result.Purged != 0 || !store.released || store.finished {
		t.Fatalf("result=%+v error=%v store=%#v", result, err, store)
	}
}

func TestReportResultCleanerTableSnapshotExpiresWithoutOracleDelete(t *testing.T) {
	now := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	runtime := testResultCleanupRuntime(now)
	runtime.Version.ExecutionMode = model.ReportExecutionModeTableSnapshot
	store := &fakeReportResultCleanupStore{expired: []reportrepo.ResultCleanupCandidate{{RunID: 32}}, runtime: runtime}
	session := &fakeReportExportOracleSession{purgeCounts: []int64{1}}
	cleaner := NewReportResultCleanerWithDependencies(store, &fakeReadyResultPurger{}, failingReportCredentialDecryptor{}, fakeResultCleanupOracleFactory{session: session})
	cleaner.now = func() time.Time { return now }
	cleaner.newToken = func() string { return "11111111-1111-4111-8111-111111111111" }
	cleaner.batchSize, cleaner.maxRuns = 10, 100

	result, err := cleaner.Cleanup(t.Context())
	if err != nil {
		t.Fatalf("Cleanup() error=%v", err)
	}
	if result != (ReportResultCleanupResult{Scanned: 1, Claimed: 1, Purged: 1}) || !store.finished ||
		store.finishedRows != runtime.Run.RowCount || session.purgeCalls != 0 {
		t.Fatalf("result=%+v store=%#v purgeCalls=%d", result, store, session.purgeCalls)
	}
}

func testReportResultCleaner(now time.Time, store *fakeReportResultCleanupStore, ready *fakeReadyResultPurger, session reportExportOracleSession) *ReportResultCleaner {
	cleaner := NewReportResultCleanerWithDependencies(store, ready, staticReportCredentialDecryptor{}, fakeResultCleanupOracleFactory{session: session})
	cleaner.now = func() time.Time { return now }
	cleaner.newToken = func() string { return "11111111-1111-4111-8111-111111111111" }
	cleaner.batchSize, cleaner.maxRuns = 10, 100
	return cleaner
}

func testResultCleanupRuntime(now time.Time) *reportrepo.ResultCleanupRuntime {
	expires := now.Add(-time.Hour)
	return &reportrepo.ResultCleanupRuntime{
		Run:        model.ReportRun{BaseModel: model.BaseModel{ID: 32}, RunUUID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", Status: model.ReportRunStatusResultPurging, RowCount: 9, ResultExpiresAt: &expires},
		Version:    model.ReportVersion{BaseModel: model.BaseModel{ID: 23}, ExecutionMode: model.ReportExecutionModeRefCursor, ResultTableOwner: "REPORT_OWNER", ResultTableName: "REPORT_RESULT", ResultRunIDColumn: "RUN_ID", ResultRowIDColumn: "ID"},
		Datasource: model.ReportDatasource{CredentialKeyVersion: "v1", PasswordCiphertext: "cipher"},
	}
}

type fakeReadyResultPurger struct{ exportID uint }

func (purger *fakeReadyResultPurger) CleanupReadyResult(_ context.Context, exportID uint) error {
	purger.exportID = exportID
	return nil
}

type fakeResultCleanupOracleFactory struct{ session reportExportOracleSession }

func (factory fakeResultCleanupOracleFactory) Open(context.Context, reportrepo.ResultCleanupRuntime, string) (reportExportOracleSession, error) {
	return factory.session, nil
}

type fakeResultCleanupOracleSession struct{ err error }

func (*fakeResultCleanupOracleSession) Read(context.Context, []string, *reportoracle.ResultCursor, int) (reportoracle.ResultPage, error) {
	return reportoracle.ResultPage{}, errors.New("not supported")
}
func (session *fakeResultCleanupOracleSession) Purge(context.Context, int) (int64, error) {
	return 0, session.err
}
func (*fakeResultCleanupOracleSession) Close() error { return nil }

type fakeReportResultCleanupStore struct {
	ready        []reportrepo.ResultCleanupCandidate
	expired      []reportrepo.ResultCleanupCandidate
	runtime      *reportrepo.ResultCleanupRuntime
	claimed      uint
	finished     bool
	finishedRows int64
	released     bool
}

func (store *fakeReportResultCleanupStore) ListReadyResultCleanupCandidates(_ context.Context, _ time.Time, after uint, limit int) ([]reportrepo.ResultCleanupCandidate, error) {
	return resultCleanupCandidatesAfter(store.ready, after, limit, true), nil
}
func (store *fakeReportResultCleanupStore) ListExpiredResultCleanupCandidates(_ context.Context, _ time.Time, after uint, limit int) ([]reportrepo.ResultCleanupCandidate, error) {
	return resultCleanupCandidatesAfter(store.expired, after, limit, false), nil
}
func resultCleanupCandidatesAfter(candidates []reportrepo.ResultCleanupCandidate, after uint, limit int, exportCursor bool) []reportrepo.ResultCleanupCandidate {
	result := make([]reportrepo.ResultCleanupCandidate, 0, limit)
	for _, candidate := range candidates {
		cursor := candidate.RunID
		if exportCursor {
			cursor = candidate.ExportID
		}
		if cursor > after && len(result) < limit {
			result = append(result, candidate)
		}
	}
	return result
}
func (store *fakeReportResultCleanupStore) ClaimExpiredResultCleanup(_ context.Context, runID uint, _ string, _ time.Time, _ time.Duration) (*reportrepo.ResultCleanupRuntime, error) {
	store.claimed = runID
	copy := *store.runtime
	return &copy, nil
}
func (*fakeReportResultCleanupStore) UpdateExpiredResultCleanupProgress(context.Context, uint, string, int64, time.Time, time.Duration) error {
	return nil
}
func (store *fakeReportResultCleanupStore) MarkExpiredResultPurged(_ context.Context, _ uint, _ string, rows int64, _ time.Time) error {
	store.finished, store.finishedRows = true, rows
	return nil
}
func (store *fakeReportResultCleanupStore) ReleaseExpiredResultCleanup(context.Context, uint, string, time.Time) error {
	store.released = true
	return nil
}
