package data_dao

import (
	"strings"
	"testing"
	"time"

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
