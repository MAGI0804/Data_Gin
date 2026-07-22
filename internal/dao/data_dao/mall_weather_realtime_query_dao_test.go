package data_dao

import (
	"strings"
	"testing"
	"time"
)

func TestBuildRealtimeQueryUsesParameterizedKeyset(t *testing.T) {
	start := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	asOf := start.Add(12 * time.Hour)
	cursor := start.Add(6 * time.Hour)
	statement, args, err := buildRealtimeQuery(RealtimeQuery{
		MallID: 7, StartUTC: start, EndUTC: end, AsOfUTC: &asOf, QualityStatus: "valid",
		AfterSnapshot: &cursor, AfterID: 99, Limit: 201,
	})
	if err != nil {
		t.Fatalf("buildRealtimeQuery() error=%v", err)
	}
	for _, fragment := range []string{
		"w.mall_id = ?", "w.fetched_at_utc <= ?", "w.quality_status = ?",
		"w.snapshot_at_utc > ?", "ORDER BY w.snapshot_at_utc ASC", "LIMIT ?",
	} {
		if !strings.Contains(statement, fragment) {
			t.Fatalf("query does not contain %q:\n%s", fragment, statement)
		}
	}
	if len(args) != 9 || strings.Contains(statement, "valid") || strings.Contains(statement, "2026-") {
		t.Fatalf("statement=%s args=%v", statement, args)
	}
}

func TestBuildRealtimeQueryRejectsIncompleteCursor(t *testing.T) {
	start := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name  string
		query RealtimeQuery
	}{
		{name: "missing id", query: RealtimeQuery{MallID: 7, StartUTC: start, EndUTC: start.Add(time.Hour), AfterSnapshot: &start}},
		{name: "missing snapshot", query: RealtimeQuery{MallID: 7, StartUTC: start, EndUTC: start.Add(time.Hour), AfterID: 1}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, _, err := buildRealtimeQuery(test.query); err == nil {
				t.Fatal("buildRealtimeQuery() accepted incomplete cursor")
			}
		})
	}
}
