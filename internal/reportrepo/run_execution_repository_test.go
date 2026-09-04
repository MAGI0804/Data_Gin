package reportrepo

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"gin-biz-web-api/model"

	"gorm.io/gorm"
)

func TestAuditedExecutionUpdateRollsBackWhenAuditFails(t *testing.T) {
	db, transactionState := newTransactionDB(t)
	repository := New(db)
	auditCalled := false
	repository.writeSystemAudit = func(context.Context, *gorm.DB, string, string, uint, map[string]interface{}) error {
		auditCalled = true
		return errors.New("injected audit failure")
	}
	err := repository.updateOwnedExecutionWithAudit(t.Context(), 31, "11111111-1111-4111-8111-111111111111", map[string]interface{}{"status": model.ReportRunStatusSucceeded}, false, "REPORT_RUN_SUCCEEDED", nil)
	if err == nil || !auditCalled || transactionState.begins != 1 || transactionState.rollbacks != 1 || transactionState.commits != 0 {
		t.Fatalf("error=%v transaction=%#v", err, transactionState)
	}
}

func TestRuntimeContractDoesNotDependOnMutableDefinitionDatasource(t *testing.T) {
	runtime := RuntimeContract{
		Definition: model.ReportDefinition{DatasourceID: 99},
		Version:    model.ReportVersion{DatasourceID: 7, ContractHash: "contract", ProcedureSignatureHash: "procedure", ResultSchemaHash: "result"},
		Run:        model.ReportRun{ContractHash: "contract", ProcedureSignatureHash: "procedure", ResultSchemaHash: "result"},
	}
	if runtime.Version.DatasourceID == runtime.Definition.DatasourceID {
		t.Fatal("test fixture must model a definition rebound after run creation")
	}
	if !runtimeContractMatches(runtime.Run, runtime.Version) {
		t.Fatal("immutable run/version contract was rejected because the mutable definition changed")
	}
}

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
		{name: "superseded", run: model.ReportRun{BaseModel: model.BaseModel{ID: 1}, Status: model.ReportRunStatusSuperseded}, want: RunDispositionTerminal},
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

func TestTableSnapshotExecutionBlockerIsScopedToReportAndKeepsQueueOrder(t *testing.T) {
	run := model.ReportRun{BaseModel: model.BaseModel{ID: 31}, DefinitionID: 9, Status: model.ReportRunStatusQueued}
	query := tableSnapshotExecutionBlockerQuery(newDryRunDB(t), run).Count(new(int64))
	if query.Error != nil {
		t.Fatalf("build blocker query: %v", query.Error)
	}
	statement := query.Statement.SQL.String()
	for _, fragment := range []string{"report_runs.definition_id = ? AND report_runs.id <> ?", "report_runs.result_purged_at IS NULL", "report_runs.definition_id = ? AND report_runs.status = ? AND report_runs.id < ?"} {
		if !strings.Contains(statement, fragment) {
			t.Fatalf("blocker query %q does not contain %q", statement, fragment)
		}
	}
	if !containsSQLVariable(query.Statement.Vars, model.ReportRunStatusCancelRequested) {
		t.Fatalf("blocker vars do not include cancel-requested runs: %#v", query.Statement.Vars)
	}
	if !containsSQLVariable(query.Statement.Vars, model.ReportRunStatusSuperseded) {
		t.Fatalf("blocker vars do not include unpurged superseded runs: %#v", query.Statement.Vars)
	}
}

func TestExpiredQueuedRunQueryUsesCreationDeadline(t *testing.T) {
	cutoff := time.Date(2026, 9, 4, 18, 0, 0, 0, time.UTC)
	query := expiredQueuedRunQuery(newDryRunDB(t), cutoff, 20).Pluck("id", &[]uint{})
	if query.Error != nil {
		t.Fatalf("build expired queue query: %v", query.Error)
	}
	statement := query.Statement.SQL.String()
	for _, fragment := range []string{"status = ?", "created_at <= ?", "ORDER BY id ASC", "LIMIT"} {
		if !strings.Contains(statement, fragment) {
			t.Fatalf("expired queue query %q does not contain %q", statement, fragment)
		}
	}
}

func TestLegacySnapshotStateQueryIncludesUnpurgedLegacyStatuses(t *testing.T) {
	query := legacySnapshotStateQuery(newDryRunDB(t), 20).Find(&[]model.ReportRun{})
	if query.Error != nil {
		t.Fatalf("build legacy state query: %v", query.Error)
	}
	statement := query.Statement.SQL.String()
	for _, fragment := range []string{"status IN", "result_purged_at IS NULL", "ORDER BY id ASC", "LIMIT"} {
		if !strings.Contains(statement, fragment) {
			t.Fatalf("legacy state query %q does not contain %q", statement, fragment)
		}
	}
	for _, status := range []string{model.ReportRunStatusExporting, model.ReportRunStatusExported} {
		if !containsSQLVariable(query.Statement.Vars, status) {
			t.Fatalf("legacy state query does not include status %q: vars=%#v", status, query.Statement.Vars)
		}
	}
}

func TestLegacySupersededSnapshotRecoveryRequiresOverwriteEvidence(t *testing.T) {
	tests := []struct {
		name            string
		newerPurged     bool
		newerSnapshot   bool
		wantDisposition legacySupersededDisposition
	}{
		{name: "newer physically purged snapshot proves shared table cleanup", newerPurged: true, wantDisposition: legacySupersededOverwritten},
		{name: "newer snapshot owner must finish cleanup first", newerSnapshot: true, wantDisposition: legacySupersededWait},
		{name: "latest legacy snapshot requires physical purge", wantDisposition: legacySupersededNeedsPurge},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := classifyLegacySupersededSnapshot(test.newerPurged, test.newerSnapshot); got != test.wantDisposition {
				t.Fatalf("classifyLegacySupersededSnapshot() = %v, want %v", got, test.wantDisposition)
			}
		})
	}
	query := legacySupersededSnapshotQuery(newDryRunDB(t), 20).Pluck("id", &[]uint{})
	if query.Error != nil {
		t.Fatalf("build superseded query: %v", query.Error)
	}
	for _, fragment := range []string{"status = ?", "result_purged_at IS NULL", "ORDER BY id DESC", "LIMIT"} {
		if !strings.Contains(query.Statement.SQL.String(), fragment) {
			t.Fatalf("superseded query %q does not contain %q", query.Statement.SQL.String(), fragment)
		}
	}
}

func TestExpiredInterruptedRunQueryIncludesCancelledAndLeaseLessRuns(t *testing.T) {
	now := time.Date(2026, 9, 4, 18, 30, 0, 0, time.UTC)
	query := expiredInterruptedRunQuery(newDryRunDB(t), now, 20).Pluck("id", &[]uint{})
	if query.Error != nil {
		t.Fatalf("build interrupted run query: %v", query.Error)
	}
	statement := query.Statement.SQL.String()
	for _, fragment := range []string{"status IN", "lease_expires_at IS NULL", "lease_expires_at <= ?", "ORDER BY id ASC", "LIMIT"} {
		if !strings.Contains(statement, fragment) {
			t.Fatalf("interrupted run query %q does not contain %q", statement, fragment)
		}
	}
	for _, status := range []string{model.ReportRunStatusRunning, model.ReportRunStatusCancelRequested} {
		if !containsSQLVariable(query.Statement.Vars, status) {
			t.Fatalf("interrupted run query does not include status %q: vars=%v", status, query.Statement.Vars)
		}
	}
	if !containsSQLVariable(query.Statement.Vars, now) {
		t.Fatalf("interrupted run query does not include lease deadline %v: vars=%v", now, query.Statement.Vars)
	}
}

func TestAutomaticReportExportBelongsToRequestingUser(t *testing.T) {
	now := time.Date(2026, 9, 4, 17, 0, 0, 0, time.UTC)
	run := model.ReportRun{
		BaseModel: model.BaseModel{ID: 31}, RequestedBy: 17,
		PresentationSnapshotJSON: model.JSONText(`[{"fieldId":"order","exportVisible":true}]`),
	}
	export, outbox, err := buildAutomaticReportExport(run, "11111111-1111-4111-8111-111111111111", now)
	if err != nil {
		t.Fatalf("buildAutomaticReportExport() error = %v", err)
	}
	if export.RunID != run.ID || export.CreatedBy != run.RequestedBy || export.Status != model.ReportExportStatusPending ||
		string(export.FrozenFiltersJSON) != `[]` || string(export.FrozenSortJSON) != `[]` ||
		string(export.FrozenColumnsJSON) != string(run.PresentationSnapshotJSON) {
		t.Fatalf("automatic export = %#v", export)
	}
	if outbox.TaskKey != "report:export:"+export.ExportUUID || outbox.QueueName != "report_export" || outbox.AvailableAt != now {
		t.Fatalf("automatic export outbox = %#v", outbox)
	}
	if !json.Valid([]byte(export.FrozenColumnsJSON)) {
		t.Fatal("automatic export columns are invalid JSON")
	}
}

func TestRestoreJobOutboxCreatesOrResetsDeliveryAtomically(t *testing.T) {
	now := time.Date(2026, 9, 4, 21, 0, 0, 0, time.UTC)
	run := model.ReportRun{BaseModel: model.BaseModel{ID: 31}, RunUUID: "11111111-1111-4111-8111-111111111111"}
	query := restoreJobOutbox(newDryRunDB(t), recoveredRunOutbox(run, now), now)
	if query.Error != nil {
		t.Fatalf("restoreJobOutbox() error = %v", query.Error)
	}
	statement := query.Statement.SQL.String()
	for _, fragment := range []string{"INSERT INTO `async_job_outbox`", "ON DUPLICATE KEY UPDATE", "`payload_json`", "`published_at`", "`available_at`"} {
		if !strings.Contains(statement, fragment) {
			t.Fatalf("restore outbox SQL %q does not contain %q", statement, fragment)
		}
	}
	if !containsSQLVariable(query.Statement.Vars, model.JSONText(`{"run_id":31}`)) {
		t.Fatalf("restore outbox does not include rebuilt payload: vars=%#v", query.Statement.Vars)
	}
}

func TestQueuedSnapshotTimeoutBecomesAuditedFailure(t *testing.T) {
	db, transactionState := newTransactionDB(t)
	repository := New(db)
	auditCalled := false
	repository.writeSystemAudit = func(_ context.Context, _ *gorm.DB, action, targetType string, targetID uint, detail map[string]interface{}) error {
		auditCalled = action == "REPORT_RUN_FAILED" && targetType == "REPORT_RUN" && targetID == 31 && detail["reasonCode"] == "SNAPSHOT_WAIT_TIMEOUT"
		return nil
	}
	err := repository.MarkQueuedExecutionFailed(t.Context(), 31, "SNAPSHOT_WAIT_TIMEOUT", "等待超时", time.Now().UTC())
	if err != nil || !auditCalled || transactionState.commits != 1 || transactionState.rollbacks != 0 {
		t.Fatalf("error=%v audit=%t transaction=%#v", err, auditCalled, transactionState)
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
