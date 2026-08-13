package reportrepo

import (
	"strings"
	"testing"
	"time"

	"gin-biz-web-api/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestExportCleanupCandidateQueryIsExpiredPurgedAndLeaseAware(t *testing.T) {
	now := time.Date(2026, 8, 13, 8, 0, 0, 0, time.UTC)
	repository := New(newDryRunDB(t))
	query := repository.exportCleanupCandidateQuery(t.Context(), now).
		Select("id", "export_uuid", "result_object_key").
		Where("id > ?", 17).
		Order("id ASC").
		Limit(100).
		Find(&[]ExportCleanupCandidate{})
	if query.Error != nil {
		t.Fatalf("exportCleanupCandidateQuery() error=%v", query.Error)
	}
	statement := query.Statement.SQL.String()
	for _, fragment := range []string{
		"status = ?", "expires_at IS NOT NULL", "expires_at <= ?", "purged_at IS NOT NULL",
		"result_object_key <> ?", "lease_token IS NULL", "lease_expires_at <= ?", "id > ?",
		"ORDER BY id ASC", "LIMIT 100",
	} {
		if !strings.Contains(statement, fragment) {
			t.Fatalf("candidate SQL %q does not contain %q", statement, fragment)
		}
	}
	if strings.Contains(statement, "2026-08-13") {
		t.Fatalf("candidate SQL interpolates values: %s", statement)
	}
}

func TestExportCleanupLeaseQueriesFenceArtifactAndDoNotTouchOracleState(t *testing.T) {
	repository := New(newDryRunDB(t).Session(&gorm.Session{DryRun: true, SkipDefaultTransaction: true}))
	candidate := ExportCleanupCandidate{
		ID: 17, ExportUUID: uuid.NewString(),
		ResultObjectKey: "report-exports/2026/08/13/11111111-1111-4111-8111-111111111111/22222222-2222-4222-8222-222222222222/result.xlsx",
	}
	token := uuid.NewString()
	query := repository.exportCleanupLeaseQuery(t.Context(), candidate, token).
		Updates(map[string]interface{}{"status": model.ReportExportStatusExpired, "result_object_key": ""})
	if query.Error != nil {
		t.Fatalf("exportCleanupLeaseQuery() error=%v", query.Error)
	}
	statement := query.Statement.SQL.String()
	for _, fragment := range []string{"id = ?", "export_uuid = ?", "status = ?", "result_object_key = ?", "purged_at IS NOT NULL", "lease_token = ?"} {
		if !strings.Contains(statement, fragment) {
			t.Fatalf("lease SQL %q does not contain %q", statement, fragment)
		}
	}
	for _, forbidden := range []string{"report_runs", "result_purged_at", "purged_at ="} {
		if strings.Contains(statement, forbidden) {
			t.Fatalf("lease SQL unexpectedly mutates Oracle cleanup state: %s", statement)
		}
	}
}

func TestExportCleanupRejectsInvalidLeaseIdentity(t *testing.T) {
	repository := New(newDryRunDB(t))
	candidate := ExportCleanupCandidate{ID: 17, ExportUUID: uuid.NewString(), ResultObjectKey: "report-exports/result.xlsx"}
	if _, err := repository.ClaimExportCleanup(t.Context(), candidate, "bad-token", time.Now().UTC(), time.Minute); err == nil {
		t.Fatal("ClaimExportCleanup() accepted invalid token")
	}
	if err := repository.ReleaseExportCleanup(t.Context(), candidate, "bad-token", time.Now().UTC()); err == nil {
		t.Fatal("ReleaseExportCleanup() accepted invalid token")
	}
}
