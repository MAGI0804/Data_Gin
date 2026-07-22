package data_dao

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestMallWeatherExportCleanupCandidateQueryIsBoundedAndLeaseAware(t *testing.T) {
	now := time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC)
	dao := NewMallWeatherExportJobDAO(dryRunWeatherDAOTestDB(t))
	query := dao.cleanupCandidateQuery(t.Context(), now, now.Add(-10*time.Minute)).
		Select("id", "result_object_key").
		Where("id > ?", 17).
		Order("id ASC").
		Limit(100).
		Find(&[]MallWeatherExportCleanupCandidate{})
	if query.Error != nil {
		t.Fatalf("cleanupCandidateQuery() error=%v", query.Error)
	}
	statement := query.Statement.SQL.String()
	for _, fragment := range []string{
		"expires_at IS NOT NULL AND expires_at <= ?",
		"status IN (?,?,?)",
		"$.cleanupToken",
		"updated_at <= ?",
		"id > ?",
		"ORDER BY id ASC",
		"LIMIT 100",
	} {
		if !strings.Contains(statement, fragment) {
			t.Fatalf("cleanup candidate statement missing %q: %s", fragment, statement)
		}
	}
	if strings.Contains(statement, "2026-07-22") {
		t.Fatalf("cleanup candidate statement interpolates values: %s", statement)
	}
}

func TestMallWeatherExportCleanupLeaseQueriesFenceTokenAndObject(t *testing.T) {
	now := time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC)
	token := uuid.NewString()
	candidate := MallWeatherExportCleanupCandidate{ID: 17, ResultObjectKey: "weather-exports/job/result.xlsx"}
	dao := NewMallWeatherExportJobDAO(dryRunWeatherDAOTestDB(t))
	dao.db = dao.db.Session(&gorm.Session{SkipDefaultTransaction: true})
	claim := dao.cleanupClaimQuery(t.Context(), candidate, now, now.Add(-10*time.Minute)).
		Update("last_cursor_json", `{"cleanupToken":"`+token+`"}`)
	if claim.Error != nil {
		t.Fatalf("cleanupClaimQuery() error=%v", claim.Error)
	}
	claimSQL := claim.Statement.SQL.String()
	if !strings.Contains(claimSQL, "result_object_key = ?") || !strings.Contains(claimSQL, "expires_at <= ?") ||
		strings.Contains(claimSQL, token) || strings.Contains(claimSQL, candidate.ResultObjectKey) {
		t.Fatalf("claim statement is not safely fenced: %s", claimSQL)
	}

	finish := dao.cleanupLeaseQuery(t.Context(), candidate, token).Update("result_object_key", "")
	if finish.Error != nil {
		t.Fatalf("cleanupLeaseQuery() error=%v", finish.Error)
	}
	finishSQL := finish.Statement.SQL.String()
	if !strings.Contains(finishSQL, "status = ?") || !strings.Contains(finishSQL, "$.cleanupToken") ||
		!strings.Contains(finishSQL, "result_object_key = ?") || strings.Contains(finishSQL, token) ||
		strings.Contains(finishSQL, candidate.ResultObjectKey) {
		t.Fatalf("finish statement is not safely fenced: %s", finishSQL)
	}
	if !validMallWeatherExportCleanupToken(token) || validMallWeatherExportCleanupToken("not-a-token") {
		t.Fatal("validMallWeatherExportCleanupToken() returned an invalid result")
	}
}

func TestMallWeatherExportCleanupRejectsInvalidInputs(t *testing.T) {
	dao := &MallWeatherExportJobDAO{}
	now := time.Now().UTC()
	if _, err := dao.ListCleanupCandidates(t.Context(), now, now.Add(-time.Minute), 0, 100); err == nil {
		t.Fatal("ListCleanupCandidates() accepted an unconfigured DAO")
	}
	if err := dao.FinishCleanup(
		t.Context(),
		MallWeatherExportCleanupCandidate{ID: 1},
		uuid.NewString(),
		now,
	); err == nil {
		t.Fatal("FinishCleanup() accepted an unconfigured DAO")
	}
}
