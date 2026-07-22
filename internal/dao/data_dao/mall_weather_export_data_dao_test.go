package data_dao

import (
	"strings"
	"testing"
	"time"
)

func TestMallWeatherExportDataQueryUsesCatalogAndStableKeyset(t *testing.T) {
	dao := NewMallWeatherExportDataDAO(dryRunWeatherDAOTestDB(t))
	snapshot := time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC)
	start := snapshot.Add(-24 * time.Hour)
	end := snapshot.Add(24 * time.Hour)
	query, fields, err := dao.buildPageQuery(t.Context(), MallWeatherExportDataPageRequest{
		Kind: "hourly", Fields: []string{"mall_code", "forecast_time", "temperature_c"},
		Filter: MallWeatherExportEstimateFilter{
			MallIDs: []uint{7}, Cities: []string{"shanghai"}, MallStatuses: []string{"active"},
			QualityStatuses: []string{"valid"}, StartUTC: &start, EndUTC: &end,
		},
		Latest: true, AfterID: 31, Limit: 250, SnapshotAt: snapshot,
	})
	if err != nil {
		t.Fatalf("buildPageQuery() error=%v", err)
	}
	var result []struct{}
	query = query.Find(&result)
	if query.Error != nil {
		t.Fatalf("build query SQL error=%v", query.Error)
	}
	statement := query.Statement.SQL.String()
	for _, fragment := range []string{
		"w.id > ?", "w.created_at <= ?", "m.id IN", "m.city IN", "m.status IN", "w.quality_status IN",
		"w.forecast_time_utc >= ?", "w.forecast_time_utc < ?", "w.issued_at_utc <= ?", "NOT EXISTS",
		"ORDER BY w.id ASC", "LIMIT 251",
		"m.mall_code AS mall_code", "w.forecast_time_utc AS forecast_time", "w.temperature_c AS temperature_c",
	} {
		if !strings.Contains(statement, fragment) {
			t.Fatalf("statement does not contain %q: %s", fragment, statement)
		}
	}
	if strings.Contains(statement, "shanghai") || strings.Contains(statement, "active") || len(fields) != 3 {
		t.Fatalf("statement interpolated input or fields changed: fields=%v SQL=%s", fields, statement)
	}
}

func TestMallWeatherExportDataCatalogIsCanonicalAndDefensive(t *testing.T) {
	for _, kind := range []string{"malls", "realtime", "minutely", "hourly", "daily", "alerts", "life_indices", "fetch_runs"} {
		fields, ok := MallWeatherExportDatasetFields(kind)
		defaults, defaultsOK := MallWeatherExportDefaultFields(kind)
		if !ok || !defaultsOK || len(fields) == 0 || len(defaults) == 0 {
			t.Fatalf("kind=%q fields=%v defaults=%v", kind, fields, defaults)
		}
		for _, field := range defaults {
			if _, exists := fields[field]; !exists {
				t.Fatalf("kind=%q default field %q is not in catalog", kind, field)
			}
		}
		delete(fields, defaults[0])
		defaults[0] = "mutated"
		freshFields, _ := MallWeatherExportDatasetFields(kind)
		freshDefaults, _ := MallWeatherExportDefaultFields(kind)
		if _, exists := freshFields[freshDefaults[0]]; !exists || freshDefaults[0] == "mutated" {
			t.Fatalf("kind=%q catalog leaked mutable state", kind)
		}
	}
}

func TestMallWeatherExportDataQueryRejectsDynamicSQLInput(t *testing.T) {
	dao := NewMallWeatherExportDataDAO(dryRunWeatherDAOTestDB(t))
	request := MallWeatherExportDataPageRequest{
		Kind: "hourly", Fields: []string{"temperature_c; DROP TABLE malls"},
		Limit: 100, SnapshotAt: time.Now().UTC(),
	}
	if _, _, err := dao.buildPageQuery(t.Context(), request); err == nil {
		t.Fatal("buildPageQuery() accepted a non-catalog field")
	}
	request.Kind = "hourly; DROP TABLE malls"
	request.Fields = nil
	if _, _, err := dao.buildPageQuery(t.Context(), request); err == nil {
		t.Fatal("buildPageQuery() accepted a non-catalog dataset")
	}
}

func TestMallWeatherExportDataValueNormalization(t *testing.T) {
	if got := normalizeMallWeatherExportSQLValue([]byte("=SUM(A1:A2)")); got != "=SUM(A1:A2)" {
		t.Fatalf("normalizeMallWeatherExportSQLValue()=%v", got)
	}
	for _, test := range []struct {
		value interface{}
		want  uint
		ok    bool
	}{
		{value: int64(17), want: 17, ok: true},
		{value: uint64(18), want: 18, ok: true},
		{value: "19", want: 19, ok: true},
		{value: int64(0), ok: false},
		{value: "not-a-number", ok: false},
	} {
		got, ok := mallWeatherExportCursorUint(test.value)
		if got != test.want || ok != test.ok {
			t.Fatalf("mallWeatherExportCursorUint(%v)=(%d,%v), want=(%d,%v)", test.value, got, ok, test.want, test.ok)
		}
	}
}
