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

func TestMallWeatherQueryServiceRealtimeMapsMetadataAndCursor(t *testing.T) {
	now := time.Date(2026, 7, 22, 4, 0, 0, 0, time.UTC)
	longitude, latitude := 121.455, 31.228
	humidity, ozone := 0.72, 88.5
	rows := []model.MallWeatherRealtime{
		{
			BaseModel: model.BaseModel{ID: 11}, MallID: 7, SnapshotAtUTC: now.Add(-2 * time.Minute),
			ProviderServerTimeUTC: now.Add(-2 * time.Minute), FetchedAtUTC: now.Add(-time.Minute),
			HumidityRatio: &humidity, O3UGM3: &ozone,
			WeatherQualityFields: model.WeatherQualityFields{QualityStatus: "valid", QualityFlagsJSON: model.JSONText("[]")},
		},
		{
			BaseModel: model.BaseModel{ID: 12}, MallID: 7, SnapshotAtUTC: now.Add(-time.Minute),
			ProviderServerTimeUTC: now.Add(-time.Minute), FetchedAtUTC: now,
			WeatherQualityFields: model.WeatherQualityFields{QualityStatus: "valid", QualityFlagsJSON: model.JSONText("[]")},
		},
	}
	weather := &fakeMallWeatherQueryDAO{
		realtimeRows: rows,
		latest:       &model.MallWeatherLatest{FetchedAtUTC: now.Add(-20 * time.Minute), FreshnessStatus: model.MallWeatherFreshnessFresh},
	}
	service, err := newMallWeatherQueryService(fakeMallWeatherQueryMallReader{mall: &model.Mall{
		BaseModel: model.BaseModel{ID: 7}, WeatherLongitude: &longitude, WeatherLatitude: &latitude,
		WeatherCoordinateSystem: "gcj02", GeocodeStatus: "confirmed", Timezone: "Asia/Shanghai",
		SamplingMode: "center", CoverageRadiusM: 1000,
	}}, weather, fakeMallPermissionChecker{allowed: true}, func() time.Time { return now })
	if err != nil {
		t.Fatalf("newMallWeatherQueryService() error=%v", err)
	}
	asOf := now.Add(-30 * time.Second)
	result, err := service.Realtime(context.Background(), 17, 7, requestbody.MallWeatherRealtimeQueryRequest{
		StartUTC: now.Add(-time.Hour), EndUTC: now.Add(time.Hour), TimeZone: "Asia/Shanghai",
		Latest: true, AsOfUTC: &asOf, QualityStatus: "VALID", PageSize: 1,
	})
	if err != nil {
		t.Fatalf("Realtime() error=%v", err)
	}
	if len(result.Items) != 1 || result.Items[0].HumidityPct == nil || *result.Items[0].HumidityPct != 72 ||
		result.Items[0].O3UGM3 == nil || *result.Items[0].O3UGM3 != ozone ||
		result.Items[0].SnapshotAtLocal != "2026-07-22T11:58:00+08:00" ||
		result.Meta.FreshnessStatus != "WARNING" || result.Meta.DataAgeSeconds == nil || *result.Meta.DataAgeSeconds != 20*60 ||
		result.Pagination.NextCursor == "" || weather.realtimeQuery.Limit != 2 || weather.realtimeQuery.QualityStatus != "valid" ||
		weather.realtimeQuery.AsOfUTC == nil || !weather.realtimeQuery.AsOfUTC.Equal(asOf) {
		t.Fatalf("result=%+v query=%+v", result, weather.realtimeQuery)
	}
	cursor, err := decodeWeatherRealtimeCursor(result.Pagination.NextCursor)
	if err != nil || cursor.ID != 11 || cursor.SnapshotUnixMS != rows[0].SnapshotAtUTC.UnixMilli() {
		t.Fatalf("cursor=%+v error=%v", cursor, err)
	}
}

func TestMallWeatherQueryServiceRealtimeRejectsInvalidInput(t *testing.T) {
	now := time.Date(2026, 7, 22, 4, 0, 0, 0, time.UTC)
	longitude, latitude := 121.455, 31.228
	mall := &model.Mall{
		BaseModel: model.BaseModel{ID: 7}, WeatherLongitude: &longitude, WeatherLatitude: &latitude,
		GeocodeStatus: "confirmed", Timezone: "Asia/Shanghai",
	}
	tests := []struct {
		name    string
		request requestbody.MallWeatherRealtimeQueryRequest
	}{
		{name: "invalid range", request: requestbody.MallWeatherRealtimeQueryRequest{StartUTC: now, EndUTC: now}},
		{name: "range too large", request: requestbody.MallWeatherRealtimeQueryRequest{StartUTC: now, EndUTC: now.Add(maxWeatherQueryRange + time.Second)}},
		{name: "invalid quality", request: requestbody.MallWeatherRealtimeQueryRequest{StartUTC: now, EndUTC: now.Add(time.Hour), QualityStatus: "secret"}},
		{name: "invalid cursor", request: requestbody.MallWeatherRealtimeQueryRequest{StartUTC: now, EndUTC: now.Add(time.Hour), Cursor: "%%%"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service, err := newMallWeatherQueryService(fakeMallWeatherQueryMallReader{mall: mall}, &fakeMallWeatherQueryDAO{}, fakeMallPermissionChecker{allowed: true}, func() time.Time { return now })
			if err != nil {
				t.Fatalf("newMallWeatherQueryService() error=%v", err)
			}
			if _, err := service.Realtime(context.Background(), 17, 7, test.request); !errors.Is(err, ErrMallWeatherInvalidQuery) {
				t.Fatalf("Realtime() error=%v", err)
			}
		})
	}
}

func TestWeatherRealtimeCursorRejectsUnknownFields(t *testing.T) {
	raw := base64.RawURLEncoding.EncodeToString([]byte(`{"v":1,"snapshotUnixMs":1784692800000,"id":1,"secret":"x"}`))
	if _, err := decodeWeatherRealtimeCursor(raw); !errors.Is(err, ErrMallWeatherInvalidQuery) {
		t.Fatalf("decodeWeatherRealtimeCursor() error=%v", err)
	}
}
