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

func TestMallWeatherQueryServiceDailyMapsCompleteContractAndCursor(t *testing.T) {
	now := time.Date(2026, 7, 22, 4, 0, 0, 0, time.UTC)
	longitude, latitude := 121.455, 31.228
	maxTemperature, nightPrecipitation, humidity := 35.2, 1.25, 0.82
	rows := []model.MallWeatherDaily{
		{
			BaseModel: model.BaseModel{ID: 11}, MallID: 7, ForecastDateLocal: time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC),
			IssuedAtUTC: now.Add(-time.Hour), FetchedAtUTC: now.Add(-30 * time.Minute),
			TemperatureMaxC: &maxTemperature, NightPrecipitationAvgMMH: &nightPrecipitation, HumidityMaxRatio: &humidity,
			DaySkycon: "CLEAR_DAY", SunriseLocalTime: "05:03",
			WeatherQualityFields: model.WeatherQualityFields{QualityStatus: "valid", QualityFlagsJSON: model.JSONText("[]")},
		},
		{
			BaseModel: model.BaseModel{ID: 12}, MallID: 7, ForecastDateLocal: time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC),
			IssuedAtUTC: now.Add(-time.Hour), FetchedAtUTC: now.Add(-30 * time.Minute),
			WeatherQualityFields: model.WeatherQualityFields{QualityStatus: "valid", QualityFlagsJSON: model.JSONText("[]")},
		},
	}
	weather := &fakeMallWeatherQueryDAO{
		dailyRows: rows,
		latest:    &model.MallWeatherLatest{FetchedAtUTC: now.Add(-13 * time.Hour), FreshnessStatus: model.MallWeatherFreshnessFresh},
	}
	service, err := newMallWeatherQueryService(fakeMallWeatherQueryMallReader{mall: &model.Mall{
		BaseModel: model.BaseModel{ID: 7}, WeatherLongitude: &longitude, WeatherLatitude: &latitude,
		GeocodeStatus: "confirmed", Timezone: "Asia/Shanghai", CoverageRadiusM: 1000,
	}}, weather, fakeMallPermissionChecker{allowed: true}, func() time.Time { return now })
	if err != nil {
		t.Fatalf("newMallWeatherQueryService() error=%v", err)
	}
	result, err := service.Daily(context.Background(), 17, 7, requestbody.MallWeatherDailyQueryRequest{
		StartUTC: now, EndUTC: now.Add(15 * 24 * time.Hour), Latest: true, PageSize: 1,
	})
	if err != nil {
		t.Fatalf("Daily() error=%v", err)
	}
	if len(result.Items) != 1 || result.Items[0].ForecastDateLocal != "2026-07-23" ||
		result.Items[0].TemperatureMaxC == nil || *result.Items[0].TemperatureMaxC != maxTemperature ||
		result.Items[0].NightPrecipitationAvgMMH == nil || *result.Items[0].NightPrecipitationAvgMMH != nightPrecipitation ||
		result.Items[0].HumidityMaxPct == nil || *result.Items[0].HumidityMaxPct != 82 ||
		result.Items[0].DaySkycon != "CLEAR_DAY" || result.Meta.FreshnessStatus != "WARNING" ||
		result.Pagination.NextCursor == "" || !weather.dailyQuery.Latest || weather.dailyQuery.Limit != 2 ||
		weather.dailyQuery.StartLocal.Location().String() != "Asia/Shanghai" || weather.dailyQuery.StartLocal.Hour() != 0 || weather.dailyQuery.EndLocal.Hour() != 0 {
		t.Fatalf("result=%+v query=%+v", result, weather.dailyQuery)
	}
	cursor, err := decodeWeatherDailyCursor(result.Pagination.NextCursor)
	if err != nil || cursor.ID != 11 || cursor.ForecastDateLocal != "2026-07-23" {
		t.Fatalf("cursor=%+v error=%v", cursor, err)
	}
}

func TestWeatherDailyCursorRejectsUnknownFields(t *testing.T) {
	raw := base64.RawURLEncoding.EncodeToString([]byte(`{"v":1,"forecastDateLocal":"2026-07-23","issuedAtUnixMs":1784689200000,"id":1,"secret":"x"}`))
	if _, err := decodeWeatherDailyCursor(raw); !errors.Is(err, ErrMallWeatherInvalidQuery) {
		t.Fatalf("decodeWeatherDailyCursor() error=%v", err)
	}
}

func TestMallWeatherQueryServiceDailyRejectsSameLocalDateRange(t *testing.T) {
	now := time.Date(2026, 7, 22, 4, 0, 0, 0, time.UTC)
	longitude, latitude := 121.455, 31.228
	service, err := newMallWeatherQueryService(fakeMallWeatherQueryMallReader{mall: &model.Mall{
		BaseModel: model.BaseModel{ID: 7}, WeatherLongitude: &longitude, WeatherLatitude: &latitude,
		GeocodeStatus: "confirmed", Timezone: "Asia/Shanghai",
	}}, &fakeMallWeatherQueryDAO{}, fakeMallPermissionChecker{allowed: true}, func() time.Time { return now })
	if err != nil {
		t.Fatalf("newMallWeatherQueryService() error=%v", err)
	}
	_, err = service.Daily(context.Background(), 17, 7, requestbody.MallWeatherDailyQueryRequest{
		StartUTC: now, EndUTC: now.Add(time.Hour), Latest: true,
	})
	if !errors.Is(err, ErrMallWeatherInvalidQuery) {
		t.Fatalf("Daily() error=%v", err)
	}
}
