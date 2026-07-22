package data_dao

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"gin-biz-web-api/connector/caiyun"
	weatherdomain "gin-biz-web-api/internal/weather"
	"gin-biz-web-api/model"

	"gorm.io/gorm/clause"
)

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

func TestLatestEndpointPredicateScopesProviderModules(t *testing.T) {
	tests := []struct {
		name        string
		endpoint    string
		wantSQL     string
		wantArgs    []interface{}
		wantFailure bool
	}{
		{
			name: "v26 weather and basic life indices", endpoint: caiyun.EndpointWeatherV26,
			wantSQL: "(data_kind IN ? OR (data_kind = ? AND subtype LIKE ?))",
			wantArgs: []interface{}{
				[]string{model.MallWeatherDataKindRealtime, model.MallWeatherDataKindMinutely, model.MallWeatherDataKindHourly, model.MallWeatherDataKindDaily},
				model.MallWeatherDataKindLife, weatherdomain.SourceAPIV26Daily + ":%",
			},
		},
		{
			name: "v3 rich life indices", endpoint: caiyun.EndpointLifeIndexV3,
			wantSQL:  "data_kind = ? AND subtype LIKE ?",
			wantArgs: []interface{}{model.MallWeatherDataKindLife, weatherdomain.SourceAPIV3LifeIndex + ":%"},
		},
		{name: "unknown endpoint", endpoint: "v4", wantFailure: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotSQL, gotArgs, err := latestEndpointPredicate(test.endpoint)
			if test.wantFailure {
				if err == nil {
					t.Fatal("latestEndpointPredicate() accepted unsupported endpoint")
				}
				return
			}
			if err != nil {
				t.Fatalf("latestEndpointPredicate() error=%v", err)
			}
			if gotSQL != test.wantSQL || !reflect.DeepEqual(gotArgs, test.wantArgs) {
				t.Fatalf("predicate=(%q, %#v) want=(%q, %#v)", gotSQL, gotArgs, test.wantSQL, test.wantArgs)
			}
		})
	}
}
