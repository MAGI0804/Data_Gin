package data_dao

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	weatherdomain "gin-biz-web-api/internal/weather"
	"gin-biz-web-api/model"

	"gorm.io/gorm/clause"
)

func TestFindCurrentLatestLifeSourceRejectsUnknownSource(t *testing.T) {
	if _, err := (&MallWeatherDAO{}).FindCurrentLatestLifeSource(context.Background(), 7, "future_source"); err == nil {
		t.Fatal("FindCurrentLatestLifeSource() accepted an unknown source")
	}
}

func TestBuildCurrentLatestByKindsQueryUsesBoundedIndexSeeks(t *testing.T) {
	statement, args, err := buildCurrentLatestByKindsQuery(7, []string{
		model.MallWeatherDataKindMinutely,
		model.MallWeatherDataKindHourly,
		model.MallWeatherDataKindMinutely,
	})
	if err != nil {
		t.Fatalf("buildCurrentLatestByKindsQuery() error=%v", err)
	}
	if strings.Count(statement, "SELECT * FROM mall_weather_latest") != 2 ||
		strings.Count(statement, "ORDER BY fetched_at_utc DESC, issued_at_utc DESC, id DESC") != 2 ||
		strings.Count(statement, "LIMIT 1") != 2 || strings.Count(statement, "UNION ALL") != 1 {
		t.Fatalf("statement=%s", statement)
	}
	wantArgs := []interface{}{
		uint(7), model.MallWeatherDataKindHourly,
		uint(7), model.MallWeatherDataKindMinutely,
	}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args=%#v want=%#v", args, wantArgs)
	}
}

func TestBuildCurrentLatestByKindsQueryRejectsAmbiguousKinds(t *testing.T) {
	for _, kinds := range [][]string{
		nil,
		{model.MallWeatherDataKindLife},
		{"alerts"},
	} {
		if _, _, err := buildCurrentLatestByKindsQuery(7, kinds); err == nil {
			t.Fatalf("buildCurrentLatestByKindsQuery() accepted kinds=%v", kinds)
		}
	}
}

func TestLatestConvertersBuildStableBusinessKeys(t *testing.T) {
	issuedAt := time.Date(2026, 7, 22, 3, 4, 5, 678000000, time.UTC)
	fetchedAt := issuedAt.Add(time.Minute)
	hourly, err := latestFromHourly(model.MallWeatherHourly{
		BaseModel: model.BaseModel{ID: 11}, MallID: 7,
		ForecastTimeUTC: issuedAt.Add(time.Hour), IssuedAtUTC: issuedAt, FetchedAtUTC: fetchedAt,
	})
	if err != nil {
		t.Fatalf("latestFromHourly() error=%v", err)
	}
	if hourly.DataKind != model.MallWeatherDataKindHourly || hourly.BusinessKey != "20260722T040405.678Z" ||
		hourly.BusinessTime == nil || !hourly.BusinessTime.Equal(issuedAt.Add(time.Hour)) || hourly.SourceRowID != 11 {
		t.Fatalf("hourly latest=%+v", hourly)
	}

	localDate := time.Date(2026, 7, 23, 0, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	life, err := latestFromLifeIndex(model.MallWeatherLifeIndex{
		BaseModel: model.BaseModel{ID: 12}, MallID: 7, SourceAPI: "v3_lifeindex", IndexType: 18,
		ForecastDateLocal: localDate, IssuedAtUTC: issuedAt, FetchedAtUTC: fetchedAt,
	})
	if err != nil {
		t.Fatalf("latestFromLifeIndex() error=%v", err)
	}
	if life.BusinessKey != "2026-07-23|v3_lifeindex:18" || life.Subtype != "v3_lifeindex:18" ||
		life.BusinessDate == nil || life.BusinessDate.Format("2006-01-02") != "2026-07-23" {
		t.Fatalf("life latest=%+v", life)
	}
}

func TestNewWeatherLatestRejectsIncompleteIdentity(t *testing.T) {
	_, err := newWeatherLatest(0, 7, model.MallWeatherDataKindHourly, "key", nil, nil, "", time.Now(), time.Now())
	if err == nil {
		t.Fatal("newWeatherLatest() accepted zero source row id")
	}
	_, err = newWeatherLatest(1, 7, model.MallWeatherDataKindDaily, "2026-07-23", nil, nil, "", time.Now(), time.Now())
	if err == nil {
		t.Fatal("newWeatherLatest() accepted a daily pointer without business date")
	}
}

func TestLatestMonotonicUpdateSetKeepsVersionComparisonStable(t *testing.T) {
	updates := latestMonotonicUpdateSet()
	if len(updates) == 0 || updates[len(updates)-1].Column.Name != "issued_at_utc" {
		t.Fatalf("updates=%+v", updates)
	}
	byName := make(map[string]clause.Assignment, len(updates))
	for _, update := range updates {
		byName[update.Column.Name] = update
	}
	for _, name := range []string{"mall_id", "data_kind", "business_key", "created_at"} {
		if _, exists := byName[name]; exists {
			t.Fatalf("immutable latest column %q is updated", name)
		}
	}
	source, ok := byName["source_row_id"].Value.(clause.Expr)
	if !ok || !strings.Contains(source.SQL, "VALUES(`issued_at_utc`) > `issued_at_utc`") {
		t.Fatalf("source row assignment=%+v", byName["source_row_id"])
	}
	fetched, ok := byName["fetched_at_utc"].Value.(clause.Expr)
	if !ok || !strings.Contains(fetched.SQL, "GREATEST(`fetched_at_utc`, VALUES(`fetched_at_utc`))") {
		t.Fatalf("fetched-at assignment=%+v", byName["fetched_at_utc"])
	}
}

func TestBuildLatestFreshnessPlansUsesSharedThresholds(t *testing.T) {
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	plans, err := buildLatestFreshnessPlans(now)
	if err != nil {
		t.Fatalf("buildLatestFreshnessPlans() error=%v", err)
	}
	if len(plans) != 5 {
		t.Fatalf("plans=%+v", plans)
	}
	byKind := make(map[string]latestFreshnessPlan, len(plans))
	for _, plan := range plans {
		byKind[plan.DataKind] = plan
	}
	hourly := byKind[model.MallWeatherDataKindHourly]
	if !hourly.WarningBefore.Equal(now.UTC().Add(-2*time.Hour)) || !hourly.CriticalBefore.Equal(now.UTC().Add(-4*time.Hour)) {
		t.Fatalf("hourly plan=%+v", hourly)
	}
	life := byKind[model.MallWeatherDataKindLife]
	if !life.WarningBefore.Equal(now.UTC().Add(-3*time.Hour)) || !life.CriticalBefore.Equal(now.UTC().Add(-8*time.Hour)) {
		t.Fatalf("life plan=%+v", life)
	}
}

func TestLatestStaleScopePredicateScopesProviderModules(t *testing.T) {
	tests := []struct {
		name        string
		scope       MallWeatherLatestStaleScope
		wantSQL     string
		wantArgs    []interface{}
		wantFailure bool
	}{
		{
			name: "selected weather and basic life indices",
			scope: MallWeatherLatestStaleScope{
				DataKinds:      []string{model.MallWeatherDataKindRealtime, model.MallWeatherDataKindMinutely},
				LifeSourceAPIs: []string{weatherdomain.SourceAPIV26Daily},
			},
			wantSQL: "(data_kind IN ? OR (data_kind = ? AND subtype LIKE ?))",
			wantArgs: []interface{}{
				[]string{model.MallWeatherDataKindMinutely, model.MallWeatherDataKindRealtime},
				model.MallWeatherDataKindLife, weatherdomain.SourceAPIV26Daily + ":%",
			},
		},
		{
			name:     "v3 rich life indices",
			scope:    MallWeatherLatestStaleScope{LifeSourceAPIs: []string{weatherdomain.SourceAPIV3LifeIndex}},
			wantSQL:  "((data_kind = ? AND subtype LIKE ?))",
			wantArgs: []interface{}{model.MallWeatherDataKindLife, weatherdomain.SourceAPIV3LifeIndex + ":%"},
		},
		{name: "empty scope", wantFailure: true},
		{name: "unknown data kind", scope: MallWeatherLatestStaleScope{DataKinds: []string{"alerts"}}, wantFailure: true},
		{name: "unknown life source", scope: MallWeatherLatestStaleScope{LifeSourceAPIs: []string{"v4"}}, wantFailure: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotSQL, gotArgs, err := latestStaleScopePredicate(test.scope)
			if test.wantFailure {
				if err == nil {
					t.Fatal("latestStaleScopePredicate() accepted invalid scope")
				}
				return
			}
			if err != nil {
				t.Fatalf("latestStaleScopePredicate() error=%v", err)
			}
			if gotSQL != test.wantSQL || !reflect.DeepEqual(gotArgs, test.wantArgs) {
				t.Fatalf("predicate=(%q, %#v) want=(%q, %#v)", gotSQL, gotArgs, test.wantSQL, test.wantArgs)
			}
		})
	}
}
