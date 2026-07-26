package data_dao

import (
	"strings"
	"testing"
	"time"

	"gin-biz-web-api/model"

	"gorm.io/gorm"
)

func TestExcelMatchExpiredCleanupQueryIncludesRecoverableExpiredJobs(t *testing.T) {
	dao := &ExcelMatchJobDAO{db: dryRunWeatherDAOTestDB(t)}
	now := time.Date(2026, 7, 26, 8, 0, 0, 0, time.UTC)
	query := dao.expiredCleanupCandidateQuery(t.Context(), now, now.Add(-20*time.Minute)).
		Where("id > ?", 17).
		Order("id ASC").
		Limit(100).
		Find(&[]model.ExcelMatchJob{})
	if query.Error != nil {
		t.Fatalf("expiredCleanupQuery() error=%v", query.Error)
	}
	statement := query.Statement.SQL.String()
	for _, fragment := range []string{
		"expires_at IS NOT NULL AND expires_at <= ?",
		"status IN (?,?,?) OR",
		"status = ? AND updated_at <= ?",
		"result_object_key <> ? OR result_url <> ? OR work_dir <> ? OR",
		"source_file_path <> ? OR result_file_path <> ?",
		"id > ?",
		"ORDER BY id ASC",
		"LIMIT 100",
	} {
		if !strings.Contains(statement, fragment) {
			t.Fatalf("cleanup candidate statement missing %q: %s", fragment, statement)
		}
	}
	if strings.Contains(statement, "2026-07-26") {
		t.Fatalf("cleanup candidate statement interpolates values: %s", statement)
	}
	for _, value := range query.Statement.Vars {
		if value == "running" {
			t.Fatalf("cleanup candidate query includes running jobs: vars=%v", query.Statement.Vars)
		}
	}
}

func TestExcelMatchFinishExpiredCleanupFencesObjectKeyAndClearsStorage(t *testing.T) {
	dao := &ExcelMatchJobDAO{db: dryRunWeatherDAOTestDB(t)}
	dao.db = dao.db.Session(&gorm.Session{SkipDefaultTransaction: true})
	query := dao.expiredCleanupLeaseQuery(
		t.Context(),
		17,
		"excel-match-results/2026/07/26/17/result.xlsx",
		nowUnixForExcelCleanupTest,
	).
		Updates(map[string]interface{}{
			"status":            "expired",
			"result_object_key": "",
			"result_url":        "",
		})
	if query.Error != nil {
		t.Fatalf("finish cleanup query error=%v", query.Error)
	}
	statement := query.Statement.SQL.String()
	for _, fragment := range []string{
		"id = ? AND status = ? AND result_object_key = ? AND updated_at = ?",
		"result_object_key",
		"result_url",
	} {
		if !strings.Contains(statement, fragment) {
			t.Fatalf("finish cleanup statement missing %q: %s", fragment, statement)
		}
	}
	if strings.Contains(statement, "excel-match-results/2026") {
		t.Fatalf("finish cleanup statement interpolates object key: %s", statement)
	}
}

const nowUnixForExcelCleanupTest = int64(1785052800)

func TestExcelMatchMarkRunningClaimAllowsOnlyFreshQueuedOrStaleRunningJobs(t *testing.T) {
	dao := &ExcelMatchJobDAO{db: dryRunWeatherDAOTestDB(t)}
	dao.db = dao.db.Session(&gorm.Session{SkipDefaultTransaction: true})
	now := time.Date(2026, 7, 26, 8, 0, 0, 0, time.UTC)
	query := dao.markRunningClaimQuery(t.Context(), 17, now, now.Add(-35*time.Minute)).
		Updates(map[string]interface{}{"status": "running", "updated_at": now.Unix()})
	if query.Error != nil {
		t.Fatalf("mark running claim query error=%v", query.Error)
	}
	statement := query.Statement.SQL.String()
	normalizedStatement := strings.Join(strings.Fields(statement), " ")
	for _, fragment := range []string{
		"id = ?",
		"status IN (?,?) AND (expires_at IS NULL OR expires_at > ?)",
		"OR (status = ? AND updated_at <= ?)",
	} {
		if !strings.Contains(normalizedStatement, fragment) {
			t.Fatalf("mark running statement missing %q: %s", fragment, statement)
		}
	}
}
