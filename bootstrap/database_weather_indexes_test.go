package bootstrap

import "testing"

func TestMallWeatherVersionIndexSpecsMatchBusinessIdentities(t *testing.T) {
	specs := mallWeatherVersionIndexSpecs()
	if len(specs) != 2 {
		t.Fatalf("mallWeatherVersionIndexSpecs() count = %d, want 2", len(specs))
	}
	if specs[0].TableName != "mall_weather_daily" || specs[0].IndexName != "uk_daily_version" ||
		!mallWeatherVersionIndexColumnsMatch(specs[0].Columns, []string{"mall_id", "provider", "forecast_date_local", "issued_at_utc"}) {
		t.Fatalf("daily index spec = %+v", specs[0])
	}
	if specs[1].TableName != "mall_weather_life_indices" || specs[1].IndexName != "uk_life_version" ||
		!mallWeatherVersionIndexColumnsMatch(specs[1].Columns, []string{"mall_id", "provider", "source_api", "forecast_date_local", "index_type", "issued_at_utc"}) {
		t.Fatalf("life index spec = %+v", specs[1])
	}
}

func TestMallWeatherVersionIndexColumnsMatchRequiresExactOrder(t *testing.T) {
	tests := []struct {
		name     string
		actual   []string
		expected []string
		want     bool
	}{
		{name: "exact", actual: []string{"mall_id", "provider", "issued_at_utc"}, expected: []string{"mall_id", "provider", "issued_at_utc"}, want: true},
		{name: "case and whitespace", actual: []string{"MALL_ID", " provider ", "ISSUED_AT_UTC"}, expected: []string{"mall_id", "provider", "issued_at_utc"}, want: true},
		{name: "missing issued time", actual: []string{"mall_id", "provider"}, expected: []string{"mall_id", "provider", "issued_at_utc"}, want: false},
		{name: "wrong order", actual: []string{"provider", "mall_id", "issued_at_utc"}, expected: []string{"mall_id", "provider", "issued_at_utc"}, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := mallWeatherVersionIndexColumnsMatch(test.actual, test.expected); got != test.want {
				t.Fatalf("mallWeatherVersionIndexColumnsMatch() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestRestrictiveMallWeatherVersionIndexesFindsLegacyAndExtraUniqueKeys(t *testing.T) {
	spec := mallWeatherVersionIndexSpecs()[0]
	indexes := map[string][]string{
		"PRIMARY":                   {"id"},
		"uk_daily_version":          {"mall_id", "provider", "forecast_date_local"},
		"uk_daily_version_complete": {"mall_id", "provider", "forecast_date_local", "issued_at_utc"},
		"uk_daily_extra":            {"forecast_date_local", "provider", "mall_id"},
		"uk_unrelated":              {"provider", "raw_checksum"},
	}
	got := restrictiveMallWeatherVersionIndexes(indexes, spec)
	want := []string{"uk_daily_extra", "uk_daily_version"}
	if !mallWeatherVersionIndexColumnsMatch(got, want) {
		t.Fatalf("restrictiveMallWeatherVersionIndexes() = %v, want %v", got, want)
	}
}

func TestReplaceMallWeatherVersionIndexSQLKeepsReplacementInOneDDL(t *testing.T) {
	spec := mallWeatherVersionIndexSpecs()[0]
	got, err := replaceMallWeatherVersionIndexSQL(spec, true)
	if err != nil {
		t.Fatalf("replaceMallWeatherVersionIndexSQL() error = %v", err)
	}
	want := "ALTER TABLE `mall_weather_daily` DROP INDEX `uk_daily_version`, ADD UNIQUE INDEX `uk_daily_version` (`mall_id`, `provider`, `forecast_date_local`, `issued_at_utc`)"
	if got != want {
		t.Fatalf("replaceMallWeatherVersionIndexSQL() = %q, want %q", got, want)
	}

	got, err = replaceMallWeatherVersionIndexSQL(spec, false)
	if err != nil {
		t.Fatalf("replaceMallWeatherVersionIndexSQL() add error = %v", err)
	}
	want = "ALTER TABLE `mall_weather_daily` ADD UNIQUE INDEX `uk_daily_version` (`mall_id`, `provider`, `forecast_date_local`, `issued_at_utc`)"
	if got != want {
		t.Fatalf("replaceMallWeatherVersionIndexSQL() add = %q, want %q", got, want)
	}
}

func TestMallWeatherVersionIndexColumnsMatchesCanonicalNameCaseInsensitively(t *testing.T) {
	indexes := map[string][]string{"UK_DAILY_VERSION": {"mall_id", "provider", "forecast_date_local", "issued_at_utc"}}
	columns, exists := mallWeatherVersionIndexColumns(indexes, "uk_daily_version")
	if !exists || !mallWeatherVersionIndexColumnsMatch(columns, []string{"mall_id", "provider", "forecast_date_local", "issued_at_utc"}) {
		t.Fatalf("mallWeatherVersionIndexColumns() columns=%v exists=%v", columns, exists)
	}
}
