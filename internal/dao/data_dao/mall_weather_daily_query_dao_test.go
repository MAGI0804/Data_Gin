package data_dao

import (
	"strings"
	"testing"
	"time"
)

func TestBuildDailyQueryUsesLocalWallClockAndVersions(t *testing.T) {
	location := time.FixedZone("CST", 8*60*60)
	start := time.Date(2026, 7, 22, 0, 0, 0, 0, location)
	end := start.Add(15 * 24 * time.Hour)
	asOf := start.UTC().Add(time.Hour)
	cursor := start.Add(24 * time.Hour)
	statement, args, err := buildDailyQuery(DailyQuery{
		MallID: 7, StartLocal: start, EndLocal: end, AsOfUTC: &asOf, QualityStatus: "valid",
		AfterForecastDateLocal: &cursor, AfterID: 9, Limit: 201,
	})
	if err != nil {
		t.Fatalf("buildDailyQuery() error=%v", err)
	}
	for _, fragment := range []string{
		"PARTITION BY w.forecast_date_local", "w.issued_at_utc <= ?", "w.quality_status = ?",
		"ranked.forecast_date_local > ?", "ORDER BY ranked.forecast_date_local ASC", "LIMIT ?",
	} {
		if !strings.Contains(statement, fragment) {
			t.Fatalf("query does not contain %q:\n%s", fragment, statement)
		}
	}
	if len(args) != 9 || args[1] != "2026-07-22 00:00:00.000" || args[2] != "2026-08-06 00:00:00.000" ||
		strings.Contains(statement, "valid") || strings.Contains(statement, "2026-") {
		t.Fatalf("statement=%s args=%v", statement, args)
	}
}

func TestBuildDailyQueryRejectsIncompleteHistoryCursor(t *testing.T) {
	start := time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)
	if _, _, err := buildDailyQuery(DailyQuery{
		MallID: 7, StartLocal: start, EndLocal: start.Add(24 * time.Hour), AfterForecastDateLocal: &start, AfterID: 1,
	}); err == nil {
		t.Fatal("buildDailyQuery() accepted history cursor without issued-at")
	}
}

func TestBuildDailyQuerySupportsVersionHistoryKeyset(t *testing.T) {
	start := time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)
	cursor := start.Add(24 * time.Hour)
	issuedCursor := start.Add(-time.Hour)
	statement, args, err := buildDailyQuery(DailyQuery{
		MallID: 7, StartLocal: start, EndLocal: start.Add(15 * 24 * time.Hour),
		AfterForecastDateLocal: &cursor, AfterIssuedAtUTC: &issuedCursor, AfterID: 9,
	})
	if err != nil {
		t.Fatalf("buildDailyQuery() error=%v", err)
	}
	if !strings.Contains(statement, "w.issued_at_utc < ?") || len(args) != 10 {
		t.Fatalf("statement=%s args=%v", statement, args)
	}
}
