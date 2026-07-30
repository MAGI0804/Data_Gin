package data_dao

import (
	"strings"
	"testing"
	"time"
)

func TestBuildRealtimeDaySummaryQueryUsesBoundedParameterizedAggregation(t *testing.T) {
	start := time.Date(2026, 7, 27, 16, 0, 0, 0, time.UTC)
	statement, args, err := buildRealtimeDaySummaryQuery(RealtimeDaySummaryQuery{
		MallID: 7, StartUTC: start, EndUTC: start.Add(24 * time.Hour),
		QualityStatus: "valid",
	})
	if err != nil {
		t.Fatalf("buildRealtimeDaySummaryQuery() error=%v", err)
	}
	for _, fragment := range []string{
		"COUNT(*) AS sample_count",
		"w.mall_id = ?",
		"w.snapshot_at_utc >= ?",
		"w.snapshot_at_utc < ?",
		"w.quality_status = ?",
		"GROUP BY s.skycon",
		"AVG(w.temperature_c)",
		"MAX(w.local_precip_mm_h)",
		"COALESCE(SUM(CASE WHEN w.local_precip_mm_h > 0 THEN 1 ELSE 0 END), 0)",
	} {
		if !strings.Contains(statement, fragment) {
			t.Fatalf("query does not contain %q:\n%s", fragment, statement)
		}
	}
	if len(args) != 8 || strings.Contains(statement, "2026-") ||
		strings.Contains(statement, "SUM(w.local_precip_mm_h)") {
		t.Fatalf("statement=%s args=%v", statement, args)
	}
}

func TestBuildRealtimeDaySummaryQueryRejectsInvalidRange(t *testing.T) {
	start := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	for _, query := range []RealtimeDaySummaryQuery{
		{StartUTC: start, EndUTC: start.Add(time.Hour)},
		{MallID: 7, EndUTC: start.Add(time.Hour)},
		{MallID: 7, StartUTC: start, EndUTC: start},
	} {
		if _, _, err := buildRealtimeDaySummaryQuery(query); err == nil {
			t.Fatalf("buildRealtimeDaySummaryQuery(%+v) accepted invalid query", query)
		}
	}
}
