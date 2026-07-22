package data_svc

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/model"
)

func TestMallWeatherQueryServiceLifeIndicesMapsBothSourcesUnknownTypeAndCursor(t *testing.T) {
	now := time.Date(2026, 7, 22, 4, 0, 0, 0, time.UTC)
	longitude, latitude := 121.455, 31.228
	level := 4
	rows := []model.MallWeatherLifeIndex{
		{
			BaseModel: model.BaseModel{ID: 11}, MallID: 7, SourceAPI: "v26_daily",
			ForecastDateLocal: time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC), IndexType: 26,
			IssuedAtUTC: now.Add(-time.Hour), FetchedAtUTC: now.Add(-30 * time.Minute),
			IndexCode: "ULTRAVIOLET", IndexName: "紫外线/防晒", Level: &level, ShortDesc: "较强",
			WeatherQualityFields: model.WeatherQualityFields{QualityStatus: "valid", QualityFlagsJSON: model.JSONText("[]")},
		},
		{
			BaseModel: model.BaseModel{ID: 12}, MallID: 7, SourceAPI: "v3_lifeindex",
			ForecastDateLocal: time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC), IndexType: 99,
			IssuedAtUTC: now.Add(-time.Hour), FetchedAtUTC: now.Add(-30 * time.Minute),
			IndexCode: "UNKNOWN_99", IndexName: "未知指数", IsUnknownType: true,
			WeatherQualityFields: model.WeatherQualityFields{QualityStatus: "warning", QualityFlagsJSON: model.JSONText(`[{"code":"UNKNOWN_TYPE","path":"data[0].lifeindex[0].type"}]`)},
		},
	}
	weather := &fakeMallWeatherQueryDAO{
		lifeIndexRows: rows,
		latest:        &model.MallWeatherLatest{FetchedAtUTC: now.Add(-4 * time.Hour), FreshnessStatus: model.MallWeatherFreshnessFresh},
	}
	service, err := newMallWeatherQueryService(fakeMallWeatherQueryMallReader{mall: &model.Mall{
		BaseModel: model.BaseModel{ID: 7}, WeatherLongitude: &longitude, WeatherLatitude: &latitude,
		GeocodeStatus: "confirmed", Timezone: "Asia/Shanghai", CoverageRadiusM: 1000,
	}}, weather, fakeMallPermissionChecker{allowed: true}, func() time.Time { return now })
	if err != nil {
		t.Fatalf("newMallWeatherQueryService() error=%v", err)
	}
	result, err := service.LifeIndices(context.Background(), 17, 7, requestbody.MallWeatherLifeIndexQueryRequest{
		StartUTC: now, EndUTC: now.Add(15 * 24 * time.Hour), Latest: true, PageSize: 1,
	})
	if err != nil {
		t.Fatalf("LifeIndices() error=%v", err)
	}
	if len(result.Items) != 1 || result.Items[0].SourceAPI != "v26_daily" || result.Items[0].Level == nil || *result.Items[0].Level != level ||
		result.Meta.APIVersion != "v2.6+v3" || result.Meta.FreshnessStatus != "WARNING" || result.Pagination.NextCursor == "" ||
		!weather.lifeIndexQuery.Latest || weather.lifeIndexQuery.Limit != 2 || weather.lifeIndexQuery.StartLocal.Location().String() != "Asia/Shanghai" {
		t.Fatalf("result=%+v query=%+v", result, weather.lifeIndexQuery)
	}
	cursor, err := decodeWeatherLifeIndexCursor(result.Pagination.NextCursor)
	if err != nil || cursor.ID != 11 || cursor.SourceAPI != "v26_daily" || cursor.IndexType != 26 {
		t.Fatalf("cursor=%+v error=%v", cursor, err)
	}
	unknown, err := lifeIndexWeatherDTO(&rows[1], time.FixedZone("CST", 8*60*60))
	if err != nil || !unknown.IsUnknownType || unknown.IndexCode != "UNKNOWN_99" || len(unknown.QualityWarnings) != 1 {
		t.Fatalf("unknown=%+v error=%v", unknown, err)
	}
}

func TestWeatherLifeIndexCursorRejectsUnknownFieldsAndSource(t *testing.T) {
	values := []string{
		`{"v":1,"forecastDateLocal":"2026-07-23","sourceApi":"v3_lifeindex","indexType":99,"issuedAtUnixMs":1784689200000,"id":1,"secret":"x"}`,
		`{"v":1,"forecastDateLocal":"2026-07-23","sourceApi":"future_api","indexType":99,"issuedAtUnixMs":1784689200000,"id":1}`,
	}
	for _, value := range values {
		raw := base64.RawURLEncoding.EncodeToString([]byte(value))
		if _, err := decodeWeatherLifeIndexCursor(raw); !errors.Is(err, ErrMallWeatherInvalidQuery) {
			t.Fatalf("decodeWeatherLifeIndexCursor() error=%v", err)
		}
	}
}

func TestMallWeatherQueryServiceLifeIndicesRejectsSameLocalDateRange(t *testing.T) {
	now := time.Date(2026, 7, 22, 4, 0, 0, 0, time.UTC)
	longitude, latitude := 121.455, 31.228
	service, err := newMallWeatherQueryService(fakeMallWeatherQueryMallReader{mall: &model.Mall{
		BaseModel: model.BaseModel{ID: 7}, WeatherLongitude: &longitude, WeatherLatitude: &latitude,
		GeocodeStatus: "confirmed", Timezone: "Asia/Shanghai",
	}}, &fakeMallWeatherQueryDAO{}, fakeMallPermissionChecker{allowed: true}, func() time.Time { return now })
	if err != nil {
		t.Fatalf("newMallWeatherQueryService() error=%v", err)
	}
	_, err = service.LifeIndices(context.Background(), 17, 7, requestbody.MallWeatherLifeIndexQueryRequest{
		StartUTC: now, EndUTC: now.Add(time.Hour), Latest: true,
	})
	if !errors.Is(err, ErrMallWeatherInvalidQuery) {
		t.Fatalf("LifeIndices() error=%v", err)
	}
}
