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

func TestMallWeatherQueryServiceOverviewReturnsBoundedSummary(t *testing.T) {
	now := time.Date(2026, 7, 22, 4, 0, 37, 0, time.UTC)
	longitude, latitude := 121.455, 31.228
	humidity, probability := 0.72, 0.35
	publishedAt := now.Add(-time.Hour)
	weather := &fakeMallWeatherQueryDAO{
		realtime: &model.MallWeatherRealtime{
			BaseModel: model.BaseModel{ID: 21}, MallID: 7, SnapshotAtUTC: now.Add(-5 * time.Minute),
			ProviderServerTimeUTC: now.Add(-5 * time.Minute), FetchedAtUTC: now.Add(-5 * time.Minute),
			HumidityRatio: &humidity, WeatherQualityFields: model.WeatherQualityFields{QualityStatus: "valid", QualityFlagsJSON: model.JSONText("[]")},
		},
		minutely: []model.MallWeatherMinutely{{
			BaseModel: model.BaseModel{ID: 31}, MallID: 7, ForecastMinuteUTC: now.Add(time.Minute), IssuedAtUTC: now.Add(-10 * time.Minute),
			FetchedAtUTC: now.Add(-10 * time.Minute), MinuteOffset: 1, ProbabilityRatio: &probability,
			WeatherQualityFields: model.WeatherQualityFields{QualityStatus: "warning", QualityFlagsJSON: model.JSONText(`[{"code":"MINUTE_GAP","path":"minutely"}]`)},
		}},
		rows: []model.MallWeatherHourly{{
			BaseModel: model.BaseModel{ID: 41}, MallID: 7, ForecastTimeUTC: now.Add(time.Hour), IssuedAtUTC: now.Add(-time.Hour), FetchedAtUTC: now.Add(-time.Hour),
			WeatherQualityFields: model.WeatherQualityFields{QualityStatus: "valid", QualityFlagsJSON: model.JSONText("[]")},
		}},
		alerts: []model.MallWeatherAlert{{
			BaseModel: model.BaseModel{ID: 51}, AlertID: "alert-1", Status: "active", Title: "Heavy rain", PublishedAtUTC: &publishedAt,
			FirstSeenAt: publishedAt, LastSeenAt: now,
			QualityStatus: "valid", QualityFlagsJSON: model.JSONText("[]"),
		}},
		latestByKind: map[string]*model.MallWeatherLatest{
			model.MallWeatherDataKindRealtime: {FetchedAtUTC: now.Add(-5 * time.Minute), FreshnessStatus: model.MallWeatherFreshnessFresh},
			model.MallWeatherDataKindMinutely: {FetchedAtUTC: now.Add(-10 * time.Minute), FreshnessStatus: model.MallWeatherFreshnessFresh},
			model.MallWeatherDataKindHourly:   {FetchedAtUTC: now.Add(-3 * time.Hour), FreshnessStatus: model.MallWeatherFreshnessStale},
		},
	}
	mall := &model.Mall{
		BaseModel: model.BaseModel{ID: 7}, WeatherLongitude: &longitude, WeatherLatitude: &latitude,
		WeatherCoordinateSystem: "gcj02", GeocodeStatus: "confirmed", Timezone: "Asia/Shanghai",
		SamplingMode: "center", CoverageRadiusM: 1000,
	}
	service, err := newMallWeatherQueryService(fakeMallWeatherQueryMallReader{mall: mall}, weather, fakeMallPermissionChecker{allowed: true}, func() time.Time { return now })
	if err != nil {
		t.Fatalf("newMallWeatherQueryService() error=%v", err)
	}
	result, err := service.Overview(context.Background(), 17, 7, "Asia/Shanghai")
	if err != nil {
		t.Fatalf("Overview() error=%v", err)
	}
	if result.Realtime == nil || result.Realtime.HumidityPct == nil || *result.Realtime.HumidityPct != 72 ||
		result.Realtime.SnapshotAtLocal != "2026-07-22T11:55:37+08:00" || len(result.Minutely) != 1 ||
		result.Minutely[0].ProbabilityPct == nil || *result.Minutely[0].ProbabilityPct != 35 ||
		len(result.Minutely[0].QualityWarnings) != 1 || len(result.Hourly) != 1 || len(result.Alerts) != 1 ||
		result.Alerts[0].PublishedAtLocal == nil || *result.Alerts[0].PublishedAtLocal != "2026-07-22T11:00:37+08:00" {
		t.Fatalf("result=%+v", result)
	}
	if result.Meta.FreshnessStatus != "STALE" || result.Meta.DataAgeSeconds == nil || *result.Meta.DataAgeSeconds != 3*60*60 ||
		weather.minutelyStartUTC != now.Truncate(time.Minute) || weather.minutelyEndUTC != now.Truncate(time.Minute).Add(2*time.Hour) || weather.minutelyLimit != maxWeatherOverviewMinutely ||
		weather.query.StartUTC != now.Truncate(time.Hour) || weather.query.EndUTC != now.Truncate(time.Hour).Add(24*time.Hour) ||
		!weather.query.Latest || weather.query.Limit != 24 || weather.alertLimit != maxWeatherOverviewAlerts {
		t.Fatalf("meta=%+v query=%+v minutely=[%s,%s,%d] alertLimit=%d", result.Meta, weather.query, weather.minutelyStartUTC, weather.minutelyEndUTC, weather.minutelyLimit, weather.alertLimit)
	}
}

func TestMallWeatherQueryServiceOverviewMarksMissingModuleUnavailable(t *testing.T) {
	now := time.Date(2026, 7, 22, 4, 0, 0, 0, time.UTC)
	longitude, latitude := 121.455, 31.228
	weather := &fakeMallWeatherQueryDAO{latestByKind: map[string]*model.MallWeatherLatest{
		model.MallWeatherDataKindRealtime: {FetchedAtUTC: now.Add(-5 * time.Minute), FreshnessStatus: model.MallWeatherFreshnessFresh},
		model.MallWeatherDataKindHourly:   {FetchedAtUTC: now.Add(-time.Hour), FreshnessStatus: model.MallWeatherFreshnessFresh},
	}}
	service, err := newMallWeatherQueryService(fakeMallWeatherQueryMallReader{mall: &model.Mall{
		BaseModel: model.BaseModel{ID: 7}, WeatherLongitude: &longitude, WeatherLatitude: &latitude,
		GeocodeStatus: "confirmed", Timezone: "Asia/Shanghai", CoverageRadiusM: 1000,
	}}, weather, fakeMallPermissionChecker{allowed: true}, func() time.Time { return now })
	if err != nil {
		t.Fatalf("newMallWeatherQueryService() error=%v", err)
	}
	result, err := service.Overview(context.Background(), 17, 7, "")
	if err != nil {
		t.Fatalf("Overview() error=%v", err)
	}
	if result.Meta.FreshnessStatus != "UNAVAILABLE" || result.Meta.DataAgeSeconds == nil || *result.Meta.DataAgeSeconds != 60*60 ||
		result.Realtime != nil || result.Minutely == nil || result.Hourly == nil || result.Alerts == nil {
		t.Fatalf("result=%+v", result)
	}
}

func TestWeatherQualityWarningsRejectsMalformedJSON(t *testing.T) {
	if _, err := weatherQualityWarnings(model.JSONText(`{"code":`)); err == nil {
		t.Fatal("weatherQualityWarnings() accepted malformed JSON")
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
	rows             []model.MallWeatherHourly
	realtimeRows     []model.MallWeatherRealtime
	minutelyRows     []model.MallWeatherMinutely
	dailyRows        []model.MallWeatherDaily
	alertRows        []model.MallWeatherAlert
	latest           *model.MallWeatherLatest
	latestByKind     map[string]*model.MallWeatherLatest
	realtime         *model.MallWeatherRealtime
	minutely         []model.MallWeatherMinutely
	alerts           []model.MallWeatherAlert
	err              error
	overviewError    error
	query            data_dao.HourlyQuery
	realtimeQuery    data_dao.RealtimeQuery
	minutelyQuery    data_dao.MinutelyQuery
	dailyQuery       data_dao.DailyQuery
	alertQuery       data_dao.AlertQuery
	minutelyStartUTC time.Time
	minutelyEndUTC   time.Time
	minutelyLimit    int
	alertLimit       int
}

func (dao *fakeMallWeatherQueryDAO) QueryRealtime(_ context.Context, query data_dao.RealtimeQuery) ([]model.MallWeatherRealtime, error) {
	dao.realtimeQuery = query
	return append([]model.MallWeatherRealtime(nil), dao.realtimeRows...), dao.err
}

func (dao *fakeMallWeatherQueryDAO) QueryMinutely(_ context.Context, query data_dao.MinutelyQuery) ([]model.MallWeatherMinutely, error) {
	dao.minutelyQuery = query
	return append([]model.MallWeatherMinutely(nil), dao.minutelyRows...), dao.err
}

func (dao *fakeMallWeatherQueryDAO) QueryDaily(_ context.Context, query data_dao.DailyQuery) ([]model.MallWeatherDaily, error) {
	dao.dailyQuery = query
	return append([]model.MallWeatherDaily(nil), dao.dailyRows...), dao.err
}

func (dao *fakeMallWeatherQueryDAO) QueryAlerts(_ context.Context, query data_dao.AlertQuery) ([]model.MallWeatherAlert, error) {
	dao.alertQuery = query
	return append([]model.MallWeatherAlert(nil), dao.alertRows...), dao.err
}

func (dao *fakeMallWeatherQueryDAO) QueryHourly(_ context.Context, query data_dao.HourlyQuery) ([]model.MallWeatherHourly, error) {
	dao.query = query
	return append([]model.MallWeatherHourly(nil), dao.rows...), dao.err
}

func (dao *fakeMallWeatherQueryDAO) FindCurrentLatest(_ context.Context, _ uint, dataKind string) (*model.MallWeatherLatest, error) {
	if dao.latestByKind != nil {
		latest, exists := dao.latestByKind[dataKind]
		if !exists {
			return nil, data_dao.ErrMallWeatherLatestNotFound
		}
		return latest, dao.err
	}
	if dao.latest == nil && dao.err == nil {
		return nil, data_dao.ErrMallWeatherLatestNotFound
	}
	return dao.latest, dao.err
}

func (dao *fakeMallWeatherQueryDAO) FindOverviewRealtime(context.Context, uint) (*model.MallWeatherRealtime, error) {
	if dao.realtime == nil && dao.overviewError == nil {
		return nil, data_dao.ErrMallWeatherLatestNotFound
	}
	return dao.realtime, dao.overviewError
}

func (dao *fakeMallWeatherQueryDAO) ListOverviewMinutely(_ context.Context, _ uint, startUTC, endUTC time.Time, limit int) ([]model.MallWeatherMinutely, error) {
	dao.minutelyStartUTC = startUTC
	dao.minutelyEndUTC = endUTC
	dao.minutelyLimit = limit
	return append([]model.MallWeatherMinutely(nil), dao.minutely...), dao.overviewError
}

func (dao *fakeMallWeatherQueryDAO) ListOverviewAlerts(_ context.Context, _ uint, limit int) ([]model.MallWeatherAlert, error) {
	dao.alertLimit = limit
	return append([]model.MallWeatherAlert(nil), dao.alerts...), dao.overviewError
}
