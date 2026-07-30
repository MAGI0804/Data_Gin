package data_dao

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestBuildFilteredWeatherCount(t *testing.T) {
	cutoff := time.Date(2026, 7, 30, 8, 9, 10, 0, time.FixedZone("CST", 8*60*60))
	tests := []struct {
		name          string
		filter        weatherCountFilter
		wantCount     string
		wantFragments []string
		wantArgs      []interface{}
	}{
		{
			name: "version history counts physical rows",
			filter: weatherCountFilter{
				Table: "mall_weather_hourly", RangeColumn: "forecast_time_utc",
				Start: "start", End: "end",
			},
			wantCount: "COUNT(*)",
			wantArgs:  []interface{}{uint(9), "start", "end"},
		},
		{
			name: "latest life indices count logical rows with all filters",
			filter: weatherCountFilter{
				Table: "mall_weather_life_indices", RangeColumn: "forecast_date_local",
				Start: "2026-07-01", End: "2026-08-01",
				CutoffColumn: "issued_at_utc", Cutoff: cutoff.UTC(), QualityStatus: "valid",
				ExtraWhere: []string{"w.source_api = ?"}, ExtraArgs: []interface{}{"weatherapi"},
				DistinctFields: "w.forecast_date_local, w.source_api, w.index_type",
			},
			wantCount: "COUNT(DISTINCT w.forecast_date_local, w.source_api, w.index_type)",
			wantFragments: []string{
				"w.issued_at_utc <= ?",
				"w.quality_status = ?",
				"w.source_api = ?",
			},
			wantArgs: []interface{}{uint(9), "2026-07-01", "2026-08-01", cutoff.UTC(), "valid", "weatherapi"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statement, args := buildFilteredWeatherCount(9, tt.filter)
			if !strings.Contains(statement, tt.wantCount) {
				t.Fatalf("statement does not contain %q: %s", tt.wantCount, statement)
			}
			for _, fragment := range tt.wantFragments {
				if !strings.Contains(statement, fragment) {
					t.Fatalf("statement does not contain %q: %s", fragment, statement)
				}
			}
			for _, forbidden := range []string{"w.*", "ROW_NUMBER", "ORDER BY", "LIMIT"} {
				if strings.Contains(statement, forbidden) {
					t.Fatalf("statement contains expensive fragment %q: %s", forbidden, statement)
				}
			}
			if !reflect.DeepEqual(args, tt.wantArgs) {
				t.Fatalf("args = %#v, want %#v", args, tt.wantArgs)
			}
		})
	}
}

func TestBuildAlertCountQueryPreservesSnapshotFiltersAndIgnoresCursor(t *testing.T) {
	asOf := time.Date(2026, 7, 30, 1, 2, 3, 0, time.UTC)
	cursor := asOf.Add(-time.Hour)
	statement, args := buildAlertCountQuery(AlertQuery{
		MallID: 7, StartUTC: asOf.Add(-24 * time.Hour), EndUTC: asOf,
		AsOfUTC: &asOf, QualityStatus: "valid", AfterSortTime: &cursor, AfterID: 99, Limit: 50,
	})
	for _, fragment := range []string{
		"relation.first_seen_at <= ?",
		"COALESCE(alert.published_at_utc, alert.first_seen_at) <= ?",
		"(alert.ended_at IS NULL OR alert.ended_at > ?)",
		"alert.quality_status = ?",
	} {
		if !strings.Contains(statement, fragment) {
			t.Fatalf("statement does not contain %q: %s", fragment, statement)
		}
	}
	for _, forbidden := range []string{"ORDER BY", "LIMIT", "alert.id < ?"} {
		if strings.Contains(statement, forbidden) {
			t.Fatalf("statement contains cursor/paging fragment %q: %s", forbidden, statement)
		}
	}
	if len(args) != 7 {
		t.Fatalf("args = %#v, want 7 filtering arguments", args)
	}
}
