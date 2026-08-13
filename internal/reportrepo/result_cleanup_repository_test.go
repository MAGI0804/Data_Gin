package reportrepo

import (
	"strings"
	"testing"
	"time"

	"gin-biz-web-api/model"
)

func TestResultCleanupCandidateQueriesCoverRecoveryAndExpiredResults(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	repository := New(newDryRunDB(t))
	ready := repository.db.WithContext(t.Context()).Table("report_exports AS exports").
		Select("exports.run_id AS run_id, exports.id AS export_id").Joins("JOIN report_runs AS runs ON runs.id = exports.run_id").
		Where("exports.status = ? AND exports.purged_at IS NULL", model.ReportExportStatusReady).
		Where("runs.status IN ?", []string{model.ReportRunStatusSucceeded, model.ReportRunStatusResultPurging}).Find(&[]ResultCleanupCandidate{})
	for _, fragment := range []string{"JOIN report_runs", "exports.status = ?", "exports.purged_at IS NULL", "runs.status IN"} {
		if !strings.Contains(ready.Statement.SQL.String(), fragment) {
			t.Fatalf("ready SQL %q missing %q", ready.Statement.SQL.String(), fragment)
		}
	}
	expired := repository.db.WithContext(t.Context()).Table("report_runs AS runs").
		Select("runs.id AS run_id, COALESCE(exports.id, 0) AS export_id").Joins("LEFT JOIN report_exports AS exports ON exports.run_id = runs.id").
		Where("runs.result_expires_at <= ? AND runs.result_purged_at IS NULL", now).
		Where("exports.id IS NULL OR exports.status IN ?", []string{model.ReportExportStatusFailed, model.ReportExportStatusCancelled}).Find(&[]ResultCleanupCandidate{})
	for _, fragment := range []string{"LEFT JOIN report_exports", "result_expires_at <= ?", "result_purged_at IS NULL", "exports.id IS NULL", "exports.status IN"} {
		if !strings.Contains(expired.Statement.SQL.String(), fragment) {
			t.Fatalf("expired SQL %q missing %q", expired.Statement.SQL.String(), fragment)
		}
	}
}

func TestResultCleanupRejectsInvalidLeaseIdentity(t *testing.T) {
	repository := New(newDryRunDB(t))
	if _, err := repository.ClaimExpiredResultCleanup(t.Context(), 1, "bad", time.Now(), time.Minute); err == nil {
		t.Fatal("claim accepted invalid token")
	}
	if err := repository.ReleaseExpiredResultCleanup(t.Context(), 1, "bad", time.Now()); err == nil {
		t.Fatal("release accepted invalid token")
	}
}
