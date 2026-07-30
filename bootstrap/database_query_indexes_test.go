package bootstrap

import (
	"strings"
	"testing"
)

func TestQueryIndexSpecsCoverOpenQueryPaths(t *testing.T) {
	specs := queryIndexSpecs()
	if len(specs) != 12 {
		t.Fatalf("queryIndexSpecs() count=%d want=12", len(specs))
	}
	want := map[string]bool{
		"mall_weather_realtime.idx_weather_realtime_query":               false,
		"mall_weather_minutely.idx_weather_minutely_query":               false,
		"mall_weather_hourly.idx_weather_hourly_query":                   false,
		"mall_weather_daily.idx_weather_daily_query":                     false,
		"mall_weather_life_indices.idx_weather_life_query":               false,
		"mall_weather_latest.idx_weather_latest_current":                 false,
		"mall_weather_latest.idx_weather_latest_business_time":           false,
		"mall_weather_alert_relations.idx_weather_alert_relation_active": false,
		"bojun_retail_orders.idx_bojun_open_query":                       false,
		"bojun_retail_orders.idx_bojun_open_completed_query":             false,
		"bojun_retail_orders.idx_bojun_open_completed_mall_query":        false,
		"malls.idx_malls_open_weather_query":                             false,
	}
	for _, spec := range specs {
		key := spec.TableName + "." + spec.IndexName
		if _, exists := want[key]; !exists {
			t.Fatalf("unexpected index spec %s", key)
		}
		want[key] = true
	}
	for key, found := range want {
		if !found {
			t.Fatalf("missing index spec %s", key)
		}
	}
}

func TestBuildAddQueryIndexSQLUsesOnlineDDLAndDirections(t *testing.T) {
	statement, err := buildAddQueryIndexSQL(queryIndexSpec{
		TableName: "weather", IndexName: "idx_weather_query",
		Columns: []queryIndexColumn{{Name: "mall_id"}, {Name: "issued_at_utc", Desc: true}},
	})
	if err != nil {
		t.Fatalf("buildAddQueryIndexSQL() error=%v", err)
	}
	want := "ALTER TABLE `weather` ADD INDEX `idx_weather_query` (`mall_id` ASC, `issued_at_utc` DESC), ALGORITHM=INPLACE, LOCK=NONE"
	if statement != want {
		t.Fatalf("buildAddQueryIndexSQL()=%q want=%q", statement, want)
	}
	if _, err := buildAddQueryIndexSQL(queryIndexSpec{
		TableName: "weather; DROP TABLE users", IndexName: "idx", Columns: []queryIndexColumn{{Name: "id"}},
	}); err == nil {
		t.Fatal("buildAddQueryIndexSQL() accepted unsafe table name")
	}
}

func TestQueryIndexStateMatchesRequiresExactNonUniqueDefinition(t *testing.T) {
	expected := []queryIndexColumn{{Name: "mall_id"}, {Name: "issued_at_utc", Desc: true}}
	if !queryIndexStateMatches([]queryIndexStateColumn{
		{Name: "MALL_ID", NonUnique: 1, IndexType: "BTREE", Visible: true, FullWidth: true},
		{Name: "issued_at_utc", Desc: true, NonUnique: 1, IndexType: "BTREE", Visible: true, FullWidth: true},
	}, expected) {
		t.Fatal("queryIndexStateMatches() rejected the canonical definition")
	}
	for name, actual := range map[string][]queryIndexStateColumn{
		"unique":       {{Name: "mall_id", IndexType: "BTREE", Visible: true, FullWidth: true}, {Name: "issued_at_utc", Desc: true, IndexType: "BTREE", Visible: true, FullWidth: true}},
		"wrong order":  {{Name: "issued_at_utc", Desc: true, NonUnique: 1, IndexType: "BTREE", Visible: true, FullWidth: true}, {Name: "mall_id", NonUnique: 1, IndexType: "BTREE", Visible: true, FullWidth: true}},
		"wrong sort":   {{Name: "mall_id", NonUnique: 1, IndexType: "BTREE", Visible: true, FullWidth: true}, {Name: "issued_at_utc", NonUnique: 1, IndexType: "BTREE", Visible: true, FullWidth: true}},
		"missing col":  {{Name: "mall_id", NonUnique: 1, IndexType: "BTREE", Visible: true, FullWidth: true}},
		"extra column": {{Name: "mall_id", NonUnique: 1, IndexType: "BTREE", Visible: true, FullWidth: true}, {Name: "issued_at_utc", Desc: true, NonUnique: 1, IndexType: "BTREE", Visible: true, FullWidth: true}, {Name: "id", NonUnique: 1, IndexType: "BTREE", Visible: true, FullWidth: true}},
		"hash":         {{Name: "mall_id", NonUnique: 1, IndexType: "HASH", Visible: true, FullWidth: true}, {Name: "issued_at_utc", Desc: true, NonUnique: 1, IndexType: "HASH", Visible: true, FullWidth: true}},
		"invisible":    {{Name: "mall_id", NonUnique: 1, IndexType: "BTREE", FullWidth: true}, {Name: "issued_at_utc", Desc: true, NonUnique: 1, IndexType: "BTREE", FullWidth: true}},
		"prefix":       {{Name: "mall_id", NonUnique: 1, IndexType: "BTREE", Visible: true}, {Name: "issued_at_utc", Desc: true, NonUnique: 1, IndexType: "BTREE", Visible: true, FullWidth: true}},
	} {
		t.Run(name, func(t *testing.T) {
			if queryIndexStateMatches(actual, expected) {
				t.Fatalf("queryIndexStateMatches() accepted %s", name)
			}
		})
	}
}

func TestQueryIndexSpecsUseSafeIdentifiers(t *testing.T) {
	for _, spec := range queryIndexSpecs() {
		statement, err := buildAddQueryIndexSQL(spec)
		if err != nil {
			t.Fatalf("buildAddQueryIndexSQL(%s) error=%v", spec.IndexName, err)
		}
		if !strings.Contains(statement, "ALGORITHM=INPLACE, LOCK=NONE") {
			t.Fatalf("index %s is not online DDL: %s", spec.IndexName, statement)
		}
	}
}
