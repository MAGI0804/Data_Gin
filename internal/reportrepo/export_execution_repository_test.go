package reportrepo

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"gin-biz-web-api/model"

	"gorm.io/gorm"
)

func TestAuditedExportUpdateRollsBackWhenAuditFails(t *testing.T) {
	db, transactionState := newTransactionDB(t)
	repository := New(db)
	auditCalled := false
	repository.writeSystemAudit = func(context.Context, *gorm.DB, string, string, uint, map[string]interface{}) error {
		auditCalled = true
		return errors.New("injected audit failure")
	}
	err := repository.updateOwnedExportWithAudit(t.Context(), 41, "11111111-1111-4111-8111-111111111111", map[string]interface{}{"status": model.ReportExportStatusReady}, false, "REPORT_EXPORT_READY", nil)
	if err == nil || !auditCalled || transactionState.begins != 1 || transactionState.rollbacks != 1 || transactionState.commits != 0 {
		t.Fatalf("error=%v transaction=%#v", err, transactionState)
	}
}

func TestMarkExportFailedSchedulesRunResultCleanup(t *testing.T) {
	db, transactionState := newTransactionDB(t)
	repository := New(db)
	repository.writeSystemAudit = func(context.Context, *gorm.DB, string, string, uint, map[string]interface{}) error { return nil }
	err := repository.MarkExportFailed(t.Context(), 41, "11111111-1111-4111-8111-111111111111", "EXPORT_FAILED", "导出失败", time.Now().UTC())
	if err != nil || transactionState.commits != 1 || transactionState.rollbacks != 0 {
		t.Fatalf("error=%v transaction=%#v", err, transactionState)
	}
	foundCleanupUpdate := false
	for _, statement := range transactionState.execs {
		if strings.Contains(statement, "UPDATE `report_runs`") && strings.Contains(statement, "`result_expires_at`") {
			foundCleanupUpdate = true
			break
		}
	}
	if !foundCleanupUpdate {
		t.Fatalf("run cleanup update missing from SQL: %#v", transactionState.execs)
	}
}

func TestClassifyExportStartUsesLeaseExpiry(t *testing.T) {
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	past, future := now.Add(-time.Minute), now.Add(time.Minute)
	tests := []struct {
		name   string
		export model.ReportExport
		want   ExportDisposition
	}{
		{name: "pending", export: model.ReportExport{BaseModel: model.BaseModel{ID: 1}, Status: model.ReportExportStatusPending}, want: ExportDispositionAcquired},
		{name: "expired running", export: model.ReportExport{BaseModel: model.BaseModel{ID: 1}, Status: model.ReportExportStatusRunning, LeaseExpiresAt: &past}, want: ExportDispositionAcquired},
		{name: "active running", export: model.ReportExport{BaseModel: model.BaseModel{ID: 1}, Status: model.ReportExportStatusRunning, LeaseExpiresAt: &future}, want: ExportDispositionBusy},
		{name: "ready", export: model.ReportExport{BaseModel: model.BaseModel{ID: 1}, Status: model.ReportExportStatusReady}, want: ExportDispositionTerminal},
		{name: "failed", export: model.ReportExport{BaseModel: model.BaseModel{ID: 1}, Status: model.ReportExportStatusFailed}, want: ExportDispositionTerminal},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := classifyExportStart(test.export, now)
			if err != nil || got != test.want {
				t.Fatalf("classifyExportStart() = %v, %v, want %v", got, err, test.want)
			}
		})
	}
}

func TestExportDeliveryRecoveryQueryIncludesPendingAndExpiredRunning(t *testing.T) {
	now := time.Date(2026, 9, 4, 18, 0, 0, 0, time.UTC)
	query := exportDeliveryRecoveryQuery(newDryRunDB(t), now, 20).Pluck("id", &[]uint{})
	if query.Error != nil {
		t.Fatalf("build export recovery query: %v", query.Error)
	}
	statement := query.Statement.SQL.String()
	for _, fragment := range []string{"updated_at <= ?", "status = ? OR", "lease_expires_at IS NULL", "lease_expires_at <= ?", "ORDER BY id ASC", "LIMIT"} {
		if !strings.Contains(statement, fragment) {
			t.Fatalf("export recovery query %q does not contain %q", statement, fragment)
		}
	}
	if !containsSQLVariable(query.Statement.Vars, model.ReportExportStatusPending) ||
		!containsSQLVariable(query.Statement.Vars, model.ReportExportStatusRunning) {
		t.Fatalf("export recovery vars = %#v", query.Statement.Vars)
	}
}

func TestSuccessfulRunsMissingExportQueryTargetsLegacySnapshots(t *testing.T) {
	now := time.Date(2026, 9, 4, 20, 0, 0, 0, time.UTC)
	query := successfulRunsMissingExportQuery(newDryRunDB(t), now, 20).Pluck("runs.id", &[]uint{})
	if query.Error != nil {
		t.Fatalf("build missing export query: %v", query.Error)
	}
	statement := query.Statement.SQL.String()
	for _, fragment := range []string{"JOIN report_versions", "LEFT JOIN report_exports", "runs.status = ?", "runs.result_purged_at IS NULL", "runs.result_expires_at IS NULL OR runs.result_expires_at > ?", "versions.execution_mode = ?", "exports.id IS NULL", "ORDER BY runs.id ASC", "LIMIT"} {
		if !strings.Contains(statement, fragment) {
			t.Fatalf("missing export query %q does not contain %q", statement, fragment)
		}
	}
	if !containsSQLVariable(query.Statement.Vars, model.ReportRunStatusSucceeded) ||
		!containsSQLVariable(query.Statement.Vars, model.ReportExecutionModeTableSnapshot) {
		t.Fatalf("missing export vars = %#v", query.Statement.Vars)
	}
}

func TestOwnedExportUsesDedicatedLeaseFence(t *testing.T) {
	repository := New(newDryRunDB(t))
	query := repository.ownedExport(t.Context(), 41, "11111111-1111-4111-8111-111111111111").
		Session(&gorm.Session{DryRun: true}).Update("heartbeat_at", time.Now().UTC())
	statement := query.Statement.SQL.String()
	for _, fragment := range []string{"id = ?", "status = ?", "lease_token = ?"} {
		if !strings.Contains(statement, fragment) {
			t.Fatalf("owned export query %q does not contain %q", statement, fragment)
		}
	}
	if strings.Contains(statement, "JSON_EXTRACT") {
		t.Fatalf("owned export unexpectedly uses JSON fencing: %q", statement)
	}
}

func TestExportProgressAndPurgeProgressAreFenced(t *testing.T) {
	repository := New(newDryRunDB(t).Session(&gorm.Session{DryRun: true}))
	token := "11111111-1111-4111-8111-111111111111"
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	query := repository.ownedExport(t.Context(), 41, token).Where("cancel_requested = ?", false).Updates(map[string]interface{}{
		"processed_rows": int64(10), "checkpoint_json": model.JSONText(`{"after":10}`), "updated_at": now,
	})
	for _, fragment := range []string{"id = ?", "status = ?", "lease_token = ?", "cancel_requested = ?"} {
		if !strings.Contains(query.Statement.SQL.String(), fragment) {
			t.Fatalf("export progress SQL %q does not contain %q", query.Statement.SQL.String(), fragment)
		}
	}
	var export model.ReportExport
	purge := repository.db.Model(&export).Where("id = ? AND status = ? AND purged_at IS NULL AND lease_token = ? AND purged_rows <= ?", 41, model.ReportExportStatusReady, token, int64(10)).Updates(map[string]interface{}{"purged_rows": int64(10)})
	for _, fragment := range []string{"id = ?", "status = ?", "purged_at IS NULL", "lease_token = ?", "purged_rows <= ?"} {
		if !strings.Contains(purge.Statement.SQL.String(), fragment) {
			t.Fatalf("purge progress SQL %q does not contain %q", purge.Statement.SQL.String(), fragment)
		}
	}
}

func TestExportStoredValueValidation(t *testing.T) {
	if !validExportCheckpoint(model.JSONText(`{"after":10}`)) || validExportCheckpoint(model.JSONText(`{"after":`)) {
		t.Fatal("checkpoint validation mismatch")
	}
	if !validExportArtifact("reports/run.xlsx", strings.Repeat("a", 64), 1) ||
		validExportArtifact("reports/run.xlsx\nheader", strings.Repeat("a", 64), 1) ||
		validExportArtifact("reports/run.xlsx", "bad", 1) {
		t.Fatal("artifact validation mismatch")
	}
}
