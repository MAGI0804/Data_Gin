package reportrepo

import (
	"strings"
	"testing"
	"time"

	"gin-biz-web-api/model"

	"gorm.io/gorm"
)

func TestClassifyRunStartProtectsOracleExecutionFromBlindTakeover(t *testing.T) {
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	activeLease := now.Add(time.Minute)
	oracleStarted := now.Add(-time.Minute)
	tests := []struct {
		name string
		run  model.ReportRun
		want RunDisposition
	}{
		{name: "queued", run: model.ReportRun{BaseModel: model.BaseModel{ID: 1}, Status: model.ReportRunStatusQueued}, want: RunDispositionAcquired},
		{name: "active lease", run: model.ReportRun{BaseModel: model.BaseModel{ID: 1}, Status: model.ReportRunStatusRunning, LeaseExpiresAt: &activeLease}, want: RunDispositionBusy},
		{name: "stale before oracle", run: model.ReportRun{BaseModel: model.BaseModel{ID: 1}, Status: model.ReportRunStatusRunning}, want: RunDispositionAcquired},
		{name: "stale after oracle", run: model.ReportRun{BaseModel: model.BaseModel{ID: 1}, Status: model.ReportRunStatusRunning, OracleStartedAt: &oracleStarted}, want: RunDispositionReconcile},
		{name: "unknown", run: model.ReportRun{BaseModel: model.BaseModel{ID: 1}, Status: model.ReportRunStatusUnknown}, want: RunDispositionReconcile},
		{name: "terminal", run: model.ReportRun{BaseModel: model.BaseModel{ID: 1}, Status: model.ReportRunStatusSucceeded}, want: RunDispositionTerminal},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := classifyRunStart(&test.run, now)
			if err != nil || got != test.want {
				t.Fatalf("classifyRunStart() = %v, %v, want %v", got, err, test.want)
			}
		})
	}
}

func TestOwnedExecutionUsesDedicatedLeaseTokenFence(t *testing.T) {
	repository := New(newDryRunDB(t))
	query := repository.ownedExecution(t.Context(), 31, "11111111-1111-4111-8111-111111111111").
		Session(&gorm.Session{DryRun: true}).Update("heartbeat_at", time.Now().UTC())
	statement := query.Statement.SQL.String()
	for _, fragment := range []string{"id = ?", "status = ?", "lease_token = ?"} {
		if !strings.Contains(statement, fragment) {
			t.Fatalf("owned query %q does not contain %q", statement, fragment)
		}
	}
	if strings.Contains(statement, "JSON_EXTRACT") {
		t.Fatalf("owned query unexpectedly uses JSON fencing: %q", statement)
	}
}

func TestReconciliationCandidatesIncludeExpiredCrashStates(t *testing.T) {
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	var runIDs []uint
	query := buildReconciliationCandidateQuery(newDryRunDB(t).Session(&gorm.Session{DryRun: true}), now, 20).Pluck("id", &runIDs)
	statement := query.Statement.SQL.String()
	for _, fragment := range []string{"next_reconcile_at", "oracle_started_at IS NOT NULL", "lease_expires_at IS NULL", "ORDER BY id ASC", "LIMIT 20"} {
		if !strings.Contains(statement, fragment) {
			t.Fatalf("candidate query %q does not contain %q", statement, fragment)
		}
	}
	for _, status := range []string{model.ReportRunStatusUnknown, model.ReportRunStatusReconciling, model.ReportRunStatusRunning} {
		found := false
		for _, value := range query.Statement.Vars {
			if value == status {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("candidate query does not include status %q: vars=%v", status, query.Statement.Vars)
		}
	}
}

func TestClassifyReconciliationStartRecoversExpiredCrashes(t *testing.T) {
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	past := now.Add(-time.Minute)
	future := now.Add(time.Minute)
	oracleStarted := now.Add(-time.Hour)
	tests := []struct {
		name string
		run  model.ReportRun
		want RunDisposition
	}{
		{name: "unknown", run: model.ReportRun{Status: model.ReportRunStatusUnknown}, want: RunDispositionAcquired},
		{name: "expired reconciliation", run: model.ReportRun{Status: model.ReportRunStatusReconciling, LeaseExpiresAt: &past}, want: RunDispositionAcquired},
		{name: "active reconciliation", run: model.ReportRun{Status: model.ReportRunStatusReconciling, LeaseExpiresAt: &future}, want: RunDispositionBusy},
		{name: "expired execution after Oracle start", run: model.ReportRun{Status: model.ReportRunStatusRunning, OracleStartedAt: &oracleStarted, LeaseExpiresAt: &past}, want: RunDispositionAcquired},
		{name: "active execution after Oracle start", run: model.ReportRun{Status: model.ReportRunStatusRunning, OracleStartedAt: &oracleStarted, LeaseExpiresAt: &future}, want: RunDispositionBusy},
		{name: "execution before Oracle start", run: model.ReportRun{Status: model.ReportRunStatusRunning, LeaseExpiresAt: &past}, want: RunDispositionTerminal},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := classifyReconciliationStart(test.run, now); got != test.want {
				t.Fatalf("classifyReconciliationStart() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestCancellableRunBeforeOracleDoesNotCancelLiveOrStartedLease(t *testing.T) {
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	future := now.Add(time.Minute)
	past := now.Add(-time.Minute)
	oracleStarted := now.Add(-time.Second)
	tests := []struct {
		name string
		run  model.ReportRun
		want bool
	}{
		{name: "queued flag", run: model.ReportRun{Status: model.ReportRunStatusQueued, CancelRequested: true}, want: true},
		{name: "requested status", run: model.ReportRun{Status: model.ReportRunStatusCancelRequested}, want: true},
		{name: "active worker owns cancellation", run: model.ReportRun{Status: model.ReportRunStatusRunning, CancelRequested: true, LeaseExpiresAt: &future}},
		{name: "stale before oracle", run: model.ReportRun{Status: model.ReportRunStatusRunning, CancelRequested: true, LeaseExpiresAt: &past}, want: true},
		{name: "oracle may have committed", run: model.ReportRun{Status: model.ReportRunStatusRunning, CancelRequested: true, LeaseExpiresAt: &past, OracleStartedAt: &oracleStarted}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := cancellableRunBeforeOracle(test.run, now); got != test.want {
				t.Fatalf("cancellableRunBeforeOracle() = %t, want %t", got, test.want)
			}
		})
	}
}
