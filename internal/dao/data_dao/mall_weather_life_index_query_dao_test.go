package data_dao

import (
	"strings"
	"testing"
	"time"
)

func TestBuildLifeIndexQueryUsesBothSourcesAndLatestVersions(t *testing.T) {
	location := time.FixedZone("CST", 8*60*60)
	start := time.Date(2026, 7, 22, 0, 0, 0, 0, location)
	end := start.Add(15 * 24 * time.Hour)
	asOf := start.UTC().Add(time.Hour)
	cursorDate := start.Add(24 * time.Hour)
	statement, args, err := buildLifeIndexQuery(LifeIndexQuery{
		MallID: 7, StartLocal: start, EndLocal: end, AsOfUTC: &asOf, Latest: true, QualityStatus: "valid",
		AfterForecastDateLocal: &cursorDate, AfterSourceAPI: "v26_daily", AfterIndexType: 26, AfterID: 9, Limit: 201,
	})
	if err != nil {
		t.Fatalf("buildLifeIndexQuery() error=%v", err)
	}
	for _, fragment := range []string{
		"FROM mall_weather_life_indices AS w",
		"PARTITION BY w.forecast_date_local, w.source_api, w.index_type",
		"w.issued_at_utc <= ?",
		"w.quality_status = ?",
		"ranked.source_api > ?",
		"ranked.index_type > ?",
		"ORDER BY ranked.forecast_date_local ASC, ranked.source_api ASC, ranked.index_type ASC",
		"LIMIT ?",
	} {
		if !strings.Contains(statement, fragment) {
			t.Fatalf("query does not contain %q:\n%s", fragment, statement)
		}
	}
	if len(args) != 16 || args[1] != "2026-07-22 00:00:00.000" || args[2] != "2026-08-06 00:00:00.000" ||
		strings.Contains(statement, "v26_daily") || strings.Contains(statement, "valid") || strings.Contains(statement, "2026-") {
		t.Fatalf("statement=%s args=%v", statement, args)
	}
}

func TestBuildLifeIndexQuerySupportsVersionHistoryKeyset(t *testing.T) {
	start := time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)
	cursorDate := start.Add(24 * time.Hour)
	issuedAt := start.Add(-time.Hour)
	statement, args, err := buildLifeIndexQuery(LifeIndexQuery{
		MallID: 7, StartLocal: start, EndLocal: start.Add(15 * 24 * time.Hour),
		AfterForecastDateLocal: &cursorDate, AfterSourceAPI: "v3_lifeindex", AfterIndexType: 99,
		AfterIssuedAtUTC: &issuedAt, AfterID: 9,
	})
	if err != nil {
		t.Fatalf("buildLifeIndexQuery() error=%v", err)
	}
	if !strings.Contains(statement, "w.issued_at_utc < ?") || !strings.Contains(statement, "w.id > ?") || len(args) != 19 {
		t.Fatalf("statement=%s args=%v", statement, args)
	}
}

func TestBuildLifeIndexQueryRejectsIncompleteHistoryCursor(t *testing.T) {
	start := time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)
	if _, _, err := buildLifeIndexQuery(LifeIndexQuery{
		MallID: 7, StartLocal: start, EndLocal: start.Add(24 * time.Hour),
		AfterForecastDateLocal: &start, AfterSourceAPI: "v3_lifeindex", AfterIndexType: 1, AfterID: 1,
	}); err == nil {
		t.Fatal("buildLifeIndexQuery() accepted history cursor without issued-at")
	}
}
