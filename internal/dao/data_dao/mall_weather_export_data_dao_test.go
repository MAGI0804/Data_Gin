package data_dao

import (
	"slices"
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

func TestMallWeatherExportForecastDefaultsIncludeDisplayedDetail(t *testing.T) {
	tests := []struct {
		name     string
		kind     string
		expected []string
	}{
		{
			name:     "actual mall sampling point",
			kind:     "malls",
			expected: []string{"weather_longitude", "weather_latitude", "weather_coordinate_system", "weather_provider", "sampling_mode"},
		},
		{
			name:     "realtime comprehensive detail",
			kind:     "realtime",
			expected: []string{"wind_direction_deg", "cloudrate_ratio", "visibility_km", "dswrf_w_m2", "nearest_precip_distance_km", "nearest_precipitation_mm_h", "o3_ug_m3", "so2_ug_m3", "no2_ug_m3", "co_mg_m3", "comfort_index", "ultraviolet_index"},
		},
		{
			name:     "minutely detail",
			kind:     "minutely",
			expected: []string{"probability_window", "description", "forecast_keypoint", "datasource"},
		},
		{
			name:     "hourly detail",
			kind:     "hourly",
			expected: []string{"cloudrate_ratio", "dswrf_w_m2", "visibility_km", "hourly_description", "forecast_keypoint"},
		},
		{
			name:     "daily comprehensive detail",
			kind:     "daily",
			expected: []string{"temperature_avg_c", "day_temperature_avg_c", "night_temperature_avg_c", "day_precipitation_probability_pct", "night_precipitation_probability_pct", "wind_avg_direction_deg", "humidity_max_pct", "cloudrate_avg_ratio", "pressure_avg_pa", "visibility_min_km", "dswrf_max_w_m2", "pm25_avg_ug_m3", "aqi_avg_usa", "day_skycon", "night_skycon"},
		},
		{
			name:     "alert comprehensive detail",
			kind:     "alerts",
			expected: []string{"code", "alert_type_code", "alert_level_code", "location", "region_id", "adcode", "alert_latitude", "alert_longitude", "first_seen_at", "last_seen_at"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fields, fieldsOK := MallWeatherExportDatasetFields(test.kind)
			defaults, defaultsOK := MallWeatherExportDefaultFields(test.kind)
			if !fieldsOK || !defaultsOK {
				t.Fatalf("kind=%q is missing from export catalog", test.kind)
			}
			for _, expected := range test.expected {
				if _, exists := fields[expected]; !exists {
					t.Errorf("kind=%q field %q is missing from export catalog", test.kind, expected)
				}
				if !slices.Contains(defaults, expected) {
					t.Errorf("kind=%q field %q is missing from export defaults", test.kind, expected)
				}
			}
		})
	}
}

func TestMallWeatherExportLifeIndicesOnlyUseComprehensiveWeatherSource(t *testing.T) {
	dao := NewMallWeatherExportDataDAO(dryRunWeatherDAOTestDB(t))
	snapshot := time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC)
	query, _, err := dao.buildPageQuery(t.Context(), MallWeatherExportDataPageRequest{
		Kind: "life_indices", Limit: 100, SnapshotAt: snapshot,
	})
	if err != nil {
		t.Fatalf("buildPageQuery() error=%v", err)
	}
	var rows []struct{}
	query = query.Find(&rows)
	if query.Error != nil {
		t.Fatalf("build life-index query SQL error=%v", query.Error)
	}
	if statement := query.Statement.SQL.String(); !strings.Contains(statement, "w.source_api = ?") {
		t.Fatalf("life-index query does not restrict comprehensive source: %s", statement)
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
