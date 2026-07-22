package data_dao

import (
	"strings"
	"testing"
	"time"
)

func TestBuildMinutelyQuerySupportsVersionModes(t *testing.T) {
	start := time.Date(2026, 7, 22, 4, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Hour)
	asOf := start.Add(time.Hour)
	cursor := start.Add(30 * time.Minute)
	issuedCursor := start.Add(-time.Minute)
	tests := []struct {
		name         string
		query        MinutelyQuery
		contains     []string
		wantArgCount int
	}{
		{
			name: "latest versions", query: MinutelyQuery{MallID: 7, StartUTC: start, EndUTC: end, Latest: true, Limit: 200},
			contains: []string{"ROW_NUMBER() OVER", "PARTITION BY w.forecast_minute_utc", "version_rank = 1"}, wantArgCount: 4,
		},
		{
			name: "as of keyset", query: MinutelyQuery{
				MallID: 7, StartUTC: start, EndUTC: end, AsOfUTC: &asOf, QualityStatus: "valid",
				AfterForecastMinute: &cursor, AfterID: 9, Limit: 200,
			},
			contains: []string{"w.issued_at_utc <= ?", "w.quality_status = ?", "ranked.forecast_minute_utc > ?"}, wantArgCount: 9,
		},
		{
			name: "version history keyset", query: MinutelyQuery{
				MallID: 7, StartUTC: start, EndUTC: end, AfterForecastMinute: &cursor, AfterIssuedAtUTC: &issuedCursor, AfterID: 9,
			},
			contains: []string{"w.issued_at_utc < ?", "ORDER BY w.forecast_minute_utc ASC"}, wantArgCount: 10,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			statement, args, err := buildMinutelyQuery(test.query)
			if err != nil {
				t.Fatalf("buildMinutelyQuery() error=%v", err)
			}
			for _, fragment := range test.contains {
				if !strings.Contains(statement, fragment) {
					t.Fatalf("query does not contain %q:\n%s", fragment, statement)
				}
			}
			if len(args) != test.wantArgCount || strings.Contains(statement, "valid") || strings.Contains(statement, "2026-") {
				t.Fatalf("statement=%s args=%v", statement, args)
			}
		})
	}
}

func TestBuildMinutelyQueryRejectsIncompleteHistoryCursor(t *testing.T) {
	start := time.Date(2026, 7, 22, 4, 0, 0, 0, time.UTC)
	if _, _, err := buildMinutelyQuery(MinutelyQuery{
		MallID: 7, StartUTC: start, EndUTC: start.Add(time.Hour), AfterForecastMinute: &start, AfterID: 1,
	}); err == nil {
		t.Fatal("buildMinutelyQuery() accepted history cursor without issued-at")
	}
}
