package data_svc

import (
	"context"
	"errors"
	"testing"
	"time"

	"gin-biz-web-api/model"
)

func TestMallWeatherQueryServiceCurrentRealtimeReturnsLatestSnapshot(t *testing.T) {
	now := time.Date(2026, 7, 29, 4, 0, 0, 0, time.UTC)
	longitude, latitude := 121.455, 31.228
	temperature := 31.2
	weather := &fakeMallWeatherQueryDAO{
		realtime: &model.MallWeatherRealtime{
			BaseModel: model.BaseModel{ID: 21}, MallID: 7,
			SnapshotAtUTC: now.Add(-2 * time.Minute), ProviderServerTimeUTC: now.Add(-2 * time.Minute),
			FetchedAtUTC: now.Add(-time.Minute), TemperatureC: &temperature,
			WeatherQualityFields: model.WeatherQualityFields{
				QualityStatus: "valid", QualityFlagsJSON: model.JSONText("[]"),
			},
		},
		latest: &model.MallWeatherLatest{
			FetchedAtUTC: now.Add(-time.Minute), FreshnessStatus: model.MallWeatherFreshnessStale,
		},
	}
	service, err := newMallWeatherQueryService(fakeMallWeatherQueryMallReader{mall: &model.Mall{
		BaseModel: model.BaseModel{ID: 7}, WeatherLongitude: &longitude, WeatherLatitude: &latitude,
		WeatherCoordinateSystem: "gcj02", GeocodeStatus: "confirmed", Timezone: "Asia/Shanghai",
		SamplingMode: "center", CoverageRadiusM: 1000,
	}}, weather, fakeMallPermissionChecker{allowed: true}, func() time.Time { return now })
	if err != nil {
		t.Fatalf("newMallWeatherQueryService() error=%v", err)
	}

	result, err := service.CurrentRealtime(context.Background(), 17, 7, "Asia/Shanghai")
	if err != nil {
		t.Fatalf("CurrentRealtime() error=%v", err)
	}
	if result.Realtime == nil || result.Realtime.TemperatureC == nil || *result.Realtime.TemperatureC != temperature ||
		result.Realtime.SnapshotAtLocal != "2026-07-29T11:58:00+08:00" || result.Meta.TimeZone != "Asia/Shanghai" ||
		result.Meta.FreshnessStatus != "STALE" || result.Meta.DataAgeSeconds == nil || *result.Meta.DataAgeSeconds != 60 {
		t.Fatalf("result=%+v", result)
	}
}

func TestMallWeatherQueryServiceCurrentRealtimeReturnsUnavailableWhenMissing(t *testing.T) {
	now := time.Date(2026, 7, 29, 4, 0, 0, 0, time.UTC)
	longitude, latitude := 121.455, 31.228
	service, err := newMallWeatherQueryService(fakeMallWeatherQueryMallReader{mall: &model.Mall{
		BaseModel: model.BaseModel{ID: 7}, WeatherLongitude: &longitude, WeatherLatitude: &latitude,
		GeocodeStatus: "confirmed", Timezone: "Asia/Shanghai", CoverageRadiusM: 1000,
	}}, &fakeMallWeatherQueryDAO{}, fakeMallPermissionChecker{allowed: true}, func() time.Time { return now })
	if err != nil {
		t.Fatalf("newMallWeatherQueryService() error=%v", err)
	}

	result, err := service.CurrentRealtime(context.Background(), 17, 7, "")
	if err != nil {
		t.Fatalf("CurrentRealtime() error=%v", err)
	}
	if result.Realtime != nil || result.Meta.FreshnessStatus != "UNAVAILABLE" || result.Meta.DataAgeSeconds != nil ||
		result.Meta.TimeZone != "Asia/Shanghai" {
		t.Fatalf("result=%+v", result)
	}
}

func TestMallWeatherQueryServiceCurrentRealtimeRejectsInvalidInput(t *testing.T) {
	now := time.Date(2026, 7, 29, 4, 0, 0, 0, time.UTC)
	longitude, latitude := 121.455, 31.228
	queryErr := errors.New("query failed")
	mall := &model.Mall{
		BaseModel: model.BaseModel{ID: 7}, WeatherLongitude: &longitude, WeatherLatitude: &latitude,
		GeocodeStatus: "confirmed", Timezone: "Asia/Shanghai",
	}
	tests := []struct {
		name       string
		actor      uint
		timeZone   string
		permission bool
		weather    *fakeMallWeatherQueryDAO
		want       error
	}{
		{name: "forbidden", actor: 17, want: ErrMallForbidden},
		{name: "invalid timezone", actor: 17, timeZone: "Etc/Not_A_Zone", permission: true, want: ErrMallWeatherInvalidQuery},
		{name: "query failure", actor: 17, permission: true, weather: &fakeMallWeatherQueryDAO{overviewError: queryErr}, want: queryErr},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			weather := test.weather
			if weather == nil {
				weather = &fakeMallWeatherQueryDAO{}
			}
			service, err := newMallWeatherQueryService(
				fakeMallWeatherQueryMallReader{mall: mall}, weather,
				fakeMallPermissionChecker{allowed: test.permission}, func() time.Time { return now },
			)
			if err != nil {
				t.Fatalf("newMallWeatherQueryService() error=%v", err)
			}
			_, err = service.CurrentRealtime(context.Background(), test.actor, 7, test.timeZone)
			if !errors.Is(err, test.want) {
				t.Fatalf("CurrentRealtime() error=%v want=%v", err, test.want)
			}
		})
	}
}
