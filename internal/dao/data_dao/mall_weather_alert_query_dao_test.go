package data_dao

import (
	"strings"
	"testing"
	"time"
)

func TestBuildAlertQueryUsesAsOfInsteadOfCurrentState(t *testing.T) {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(31 * 24 * time.Hour)
	asOf := start.Add(15 * 24 * time.Hour)
	cursor := asOf.Add(-time.Hour)
	statement, args, err := buildAlertQuery(AlertQuery{
		MallID: 7, StartUTC: start, EndUTC: end, AsOfUTC: &asOf, Latest: true,
		QualityStatus: "valid", AfterSortTime: &cursor, AfterID: 9, Limit: 201,
	})
	if err != nil {
		t.Fatalf("buildAlertQuery() error=%v", err)
	}
	for _, fragment := range []string{
		"relation.first_seen_at <= ?", "COALESCE(alert.published_at_utc, alert.first_seen_at) <= ?", "alert.ended_at > ?", "alert.quality_status = ?",
		"COALESCE(alert.published_at_utc, alert.first_seen_at) < ?", "alert.id < ?",
	} {
		if !strings.Contains(statement, fragment) {
			t.Fatalf("query does not contain %q:\n%s", fragment, statement)
		}
	}
	if strings.Contains(statement, "relation.is_active = ?") || len(args) != 11 || strings.Contains(statement, "valid") {
		t.Fatalf("statement=%s args=%v", statement, args)
	}
}

func TestBuildAlertQueryLatestUsesActiveRelation(t *testing.T) {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	statement, args, err := buildAlertQuery(AlertQuery{MallID: 7, StartUTC: start, EndUTC: start.Add(31 * 24 * time.Hour), Latest: true})
	if err != nil || !strings.Contains(statement, "relation.is_active = ?") || !strings.Contains(statement, "alert.ended_at IS NULL") || len(args) != 5 {
		t.Fatalf("statement=%s args=%v error=%v", statement, args, err)
	}
}

func TestBuildAlertQueryRejectsIncompleteCursor(t *testing.T) {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	if _, _, err := buildAlertQuery(AlertQuery{
		MallID: 7, StartUTC: start, EndUTC: start.Add(24 * time.Hour), AfterID: 1,
	}); err == nil {
		t.Fatal("buildAlertQuery() accepted cursor without sort time")
	}
}
