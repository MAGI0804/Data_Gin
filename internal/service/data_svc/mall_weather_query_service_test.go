package data_svc

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/model"
)

func TestMallWeatherQueryServiceHourlyMapsMetadataAndCursor(t *testing.T) {
	now := time.Date(2026, 7, 22, 4, 0, 0, 0, time.UTC)
	longitude, latitude := 121.455, 31.228
	mall := &model.Mall{
		BaseModel: model.BaseModel{ID: 7}, WeatherLongitude: &longitude, WeatherLatitude: &latitude,
		WeatherCoordinateSystem: "gcj02", GeocodeStatus: "confirmed", Timezone: "Asia/Shanghai",
		SamplingMode: "center", CoverageRadiusM: 1000,
	}
	temperature, humidity := 31.2, 0.72
	rows := []model.MallWeatherHourly{
		{
			BaseModel: model.BaseModel{ID: 11}, MallID: 7,
			ForecastTimeUTC: now.Add(time.Hour), IssuedAtUTC: now.Add(-time.Hour), FetchedAtUTC: now.Add(-30 * time.Minute),
			TemperatureC: &temperature, HumidityRatio: &humidity,
			WeatherQualityFields: model.WeatherQualityFields{QualityStatus: "valid", QualityFlagsJSON: model.JSONText("[]")},
		},
		{
			BaseModel: model.BaseModel{ID: 12}, MallID: 7,
			ForecastTimeUTC: now.Add(2 * time.Hour), IssuedAtUTC: now.Add(-time.Hour), FetchedAtUTC: now.Add(-30 * time.Minute),
			WeatherQualityFields: model.WeatherQualityFields{QualityStatus: "warning", QualityFlagsJSON: model.JSONText(`[{"code":"TEST","path":"hourly"}]`)},
		},
	}
	weather := &fakeMallWeatherQueryDAO{
		rows: rows,
		latest: &model.MallWeatherLatest{
			FetchedAtUTC: now.Add(-3 * time.Hour), FreshnessStatus: model.MallWeatherFreshnessFresh,
		},
	}
	service, err := newMallWeatherQueryService(fakeMallWeatherQueryMallReader{mall: mall}, weather, fakeMallPermissionChecker{allowed: true}, func() time.Time { return now })
	if err != nil {
		t.Fatalf("newMallWeatherQueryService() error=%v", err)
	}
	result, err := service.Hourly(context.Background(), 17, 7, requestbody.MallWeatherHourlyQueryRequest{
		StartUTC: now, EndUTC: now.Add(24 * time.Hour), TimeZone: "Asia/Shanghai", Latest: true, PageSize: 1,
	})
	if err != nil {
		t.Fatalf("Hourly() error=%v", err)
	}
	if len(result.Items) != 1 || result.Items[0].HumidityPct == nil || *result.Items[0].HumidityPct != 72 ||
		result.Items[0].ForecastTimeLocal != "2026-07-22T13:00:00+08:00" || result.Meta.FreshnessStatus != "WARNING" ||
		result.Meta.DataAgeSeconds == nil || *result.Meta.DataAgeSeconds != 3*60*60 || result.Pagination.NextCursor == "" {
		t.Fatalf("result=%+v", result)
	}
	decoded, err := decodeWeatherHourlyCursor(result.Pagination.NextCursor)
	if err != nil || decoded.ID != 11 || weather.query.Limit != 2 {
		t.Fatalf("cursor=%+v query=%+v error=%v", decoded, weather.query, err)
	}
}

func TestMallWeatherQueryServiceHourlyPreservesExplicitStale(t *testing.T) {
	now := time.Date(2026, 7, 22, 4, 0, 0, 0, time.UTC)
	longitude, latitude := 121.455, 31.228
	weather := &fakeMallWeatherQueryDAO{latest: &model.MallWeatherLatest{
		FetchedAtUTC: now.Add(-time.Minute), FreshnessStatus: model.MallWeatherFreshnessStale,
	}}
	service, err := newMallWeatherQueryService(fakeMallWeatherQueryMallReader{mall: &model.Mall{
		BaseModel: model.BaseModel{ID: 7}, WeatherLongitude: &longitude, WeatherLatitude: &latitude,
		GeocodeStatus: "confirmed", Timezone: "Asia/Shanghai", CoverageRadiusM: 1000,
	}}, weather, fakeMallPermissionChecker{allowed: true}, func() time.Time { return now })
	if err != nil {
		t.Fatalf("newMallWeatherQueryService() error=%v", err)
	}
	result, err := service.Hourly(context.Background(), 17, 7, requestbody.MallWeatherHourlyQueryRequest{
		StartUTC: now, EndUTC: now.Add(time.Hour), Latest: true,
	})
	if err != nil || result.Meta.FreshnessStatus != "STALE" {
		t.Fatalf("result=%+v error=%v", result, err)
	}
}

func TestMallWeatherQueryServiceHourlyRejectsInvalidBoundaryInput(t *testing.T) {
	now := time.Now().UTC()
	longitude, latitude := 121.455, 31.228
	validMall := &model.Mall{
		BaseModel: model.BaseModel{ID: 7}, WeatherLongitude: &longitude, WeatherLatitude: &latitude,
		GeocodeStatus: "confirmed", Timezone: "Asia/Shanghai",
	}
	tests := []struct {
		name       string
		permission bool
		mall       *model.Mall
		request    requestbody.MallWeatherHourlyQueryRequest
		want       error
	}{
		{name: "forbidden", mall: validMall, request: validHourlyRequest(now), want: ErrMallForbidden},
		{name: "coordinate missing", permission: true, mall: &model.Mall{BaseModel: model.BaseModel{ID: 7}}, request: validHourlyRequest(now), want: ErrMallWeatherCoordinateUnconfirmed},
		{name: "range too large", permission: true, mall: validMall, request: requestbody.MallWeatherHourlyQueryRequest{StartUTC: now, EndUTC: now.Add(maxWeatherQueryRange + time.Second)}, want: ErrMallWeatherInvalidQuery},
		{name: "invalid zone", permission: true, mall: validMall, request: requestbody.MallWeatherHourlyQueryRequest{StartUTC: now, EndUTC: now.Add(time.Hour), TimeZone: "Etc/Not_A_Zone"}, want: ErrMallWeatherInvalidQuery},
		{name: "invalid cursor", permission: true, mall: validMall, request: requestbody.MallWeatherHourlyQueryRequest{StartUTC: now, EndUTC: now.Add(time.Hour), Cursor: "%%%"}, want: ErrMallWeatherInvalidQuery},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service, err := newMallWeatherQueryService(fakeMallWeatherQueryMallReader{mall: test.mall}, &fakeMallWeatherQueryDAO{}, fakeMallPermissionChecker{allowed: test.permission}, func() time.Time { return now })
			if err != nil {
				t.Fatalf("newMallWeatherQueryService() error=%v", err)
			}
			_, err = service.Hourly(context.Background(), 17, 7, test.request)
			if !errors.Is(err, test.want) {
				t.Fatalf("Hourly() error=%v want=%v", err, test.want)
			}
		})
	}
}

func TestWeatherHourlyCursorRejectsUnknownFields(t *testing.T) {
	raw := base64.RawURLEncoding.EncodeToString([]byte(`{"v":1,"forecastTimeUnixMs":123,"id":1,"secret":"x"}`))
	if _, err := decodeWeatherHourlyCursor(raw); !errors.Is(err, ErrMallWeatherInvalidQuery) {
		t.Fatalf("decodeWeatherHourlyCursor() error=%v", err)
	}
	if _, err := decodeWeatherHourlyCursor(strings.Repeat("a", maxWeatherCursorLength+1)); !errors.Is(err, ErrMallWeatherInvalidQuery) {
		t.Fatalf("oversized cursor error=%v", err)
	}
}

func validHourlyRequest(now time.Time) requestbody.MallWeatherHourlyQueryRequest {
	return requestbody.MallWeatherHourlyQueryRequest{StartUTC: now, EndUTC: now.Add(time.Hour), Latest: true}
}

type fakeMallWeatherQueryMallReader struct {
	mall *model.Mall
	err  error
}

func (reader fakeMallWeatherQueryMallReader) FindByID(context.Context, uint) (*model.Mall, error) {
	return reader.mall, reader.err
}

type fakeMallWeatherQueryDAO struct {
	rows   []model.MallWeatherHourly
	latest *model.MallWeatherLatest
	err    error
	query  data_dao.HourlyQuery
}

func (dao *fakeMallWeatherQueryDAO) QueryHourly(_ context.Context, query data_dao.HourlyQuery) ([]model.MallWeatherHourly, error) {
	dao.query = query
	return append([]model.MallWeatherHourly(nil), dao.rows...), dao.err
}

func (dao *fakeMallWeatherQueryDAO) FindCurrentLatest(context.Context, uint, string) (*model.MallWeatherLatest, error) {
	if dao.latest == nil && dao.err == nil {
		return nil, data_dao.ErrMallWeatherLatestNotFound
	}
	return dao.latest, dao.err
}
