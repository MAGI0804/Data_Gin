package data_svc

import (
	"context"
	"errors"
	"testing"
	"time"

	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/model"
)

func TestOpenWeatherHistoryDayUsesMallLocalDay(t *testing.T) {
	now := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	longitude, latitude := -74.006, 40.7128
	weather := &fakeMallWeatherQueryDAO{totalItems: 0}
	service, err := newMallWeatherQueryService(fakeMallWeatherQueryMallReader{mall: &model.Mall{
		BaseModel: model.BaseModel{ID: 7}, WeatherLongitude: &longitude, WeatherLatitude: &latitude,
		GeocodeStatus: "confirmed", Timezone: "America/New_York",
	}}, weather, fakeMallPermissionChecker{allowed: true}, func() time.Time { return now })
	if err != nil {
		t.Fatalf("newMallWeatherQueryService() error=%v", err)
	}

	result, err := service.HistoryDay(context.Background(), 17, 7, requestbody.OpenWeatherHistoryDayQueryRequest{
		Date: "2026-03-08", PageSize: 50,
	})
	if err != nil {
		t.Fatalf("HistoryDay() error=%v", err)
	}
	if got := weather.realtimeQuery.EndUTC.Sub(weather.realtimeQuery.StartUTC); got != 23*time.Hour {
		t.Fatalf("day duration=%s start=%s end=%s", got, weather.realtimeQuery.StartUTC, weather.realtimeQuery.EndUTC)
	}
	if result.Meta.TimeZone != "America/New_York" || result.Pagination.Page != 1 || result.Pagination.TotalItems == nil {
		t.Fatalf("result=%+v", result)
	}
}

func TestOpenWeatherHistoryRangeParsesStrictLocalDateTimes(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	longitude, latitude := 121.455, 31.228
	weather := &fakeMallWeatherQueryDAO{}
	service, err := newMallWeatherQueryService(fakeMallWeatherQueryMallReader{mall: &model.Mall{
		BaseModel: model.BaseModel{ID: 7}, WeatherLongitude: &longitude, WeatherLatitude: &latitude,
		GeocodeStatus: "confirmed", Timezone: "Asia/Shanghai",
	}}, weather, fakeMallPermissionChecker{allowed: true}, func() time.Time { return now })
	if err != nil {
		t.Fatalf("newMallWeatherQueryService() error=%v", err)
	}

	_, err = service.HistoryRange(context.Background(), 17, 7, requestbody.OpenWeatherHistoryRangeQueryRequest{
		StartTime: "2026-07-28 01:02:03", EndTime: "2026-07-28 04:05:06", PageSize: 25,
	})
	if err != nil {
		t.Fatalf("HistoryRange() error=%v", err)
	}
	wantStart := time.Date(2026, 7, 27, 17, 2, 3, 0, time.UTC)
	wantEnd := time.Date(2026, 7, 27, 20, 5, 6, 0, time.UTC)
	if !weather.realtimeQuery.StartUTC.Equal(wantStart) || !weather.realtimeQuery.EndUTC.Equal(wantEnd) || weather.realtimeQuery.Limit != 26 {
		t.Fatalf("query=%+v", weather.realtimeQuery)
	}
}

func TestOpenWeatherHistoryRejectsNonHistoricalOrWrongPrecision(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	longitude, latitude := 121.455, 31.228
	service, err := newMallWeatherQueryService(fakeMallWeatherQueryMallReader{mall: &model.Mall{
		BaseModel: model.BaseModel{ID: 7}, WeatherLongitude: &longitude, WeatherLatitude: &latitude,
		GeocodeStatus: "confirmed", Timezone: "Asia/Shanghai",
	}}, &fakeMallWeatherQueryDAO{}, fakeMallPermissionChecker{allowed: true}, func() time.Time { return now })
	if err != nil {
		t.Fatalf("newMallWeatherQueryService() error=%v", err)
	}

	dayRequests := []requestbody.OpenWeatherHistoryDayQueryRequest{
		{Date: "2026-07-30"},
		{Date: "2026-07-29 00:00:00"},
		{Date: "2026-02-30"},
	}
	for _, request := range dayRequests {
		if _, err := service.HistoryDay(context.Background(), 17, 7, request); !errors.Is(err, ErrMallWeatherInvalidQuery) {
			t.Fatalf("HistoryDay(%+v) error=%v", request, err)
		}
	}

	rangeRequests := []requestbody.OpenWeatherHistoryRangeQueryRequest{
		{StartTime: "2026-07-28T00:00:00+08:00", EndTime: "2026-07-29 00:00:00"},
		{StartTime: "2026-07-29 00:00:00", EndTime: "2026-07-29 00:00:00"},
		{StartTime: "2026-07-29 00:00:00", EndTime: "2026-07-31 00:00:00"},
	}
	for _, request := range rangeRequests {
		if _, err := service.HistoryRange(context.Background(), 17, 7, request); !errors.Is(err, ErrMallWeatherInvalidQuery) {
			t.Fatalf("HistoryRange(%+v) error=%v", request, err)
		}
	}
}

func TestOpenWeatherHistoryDaySummaryReturnsOneAggregateForLocalDay(t *testing.T) {
	now := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	longitude, latitude := -74.006, 40.7128
	start := time.Date(2026, 3, 8, 5, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 9, 3, 50, 0, 0, time.UTC)
	temperatureMin, temperatureMax, temperatureAvg := 1.2, 9.8, 5.4
	humidityRatio, precipitationMax := 0.725, 2.4
	skycon := "CLOUDY"
	weather := &fakeMallWeatherQueryDAO{latest: &model.MallWeatherLatest{
		DataKind: model.MallWeatherDataKindRealtime, FetchedAtUTC: now.Add(-time.Minute),
	}, daySummary: &data_dao.RealtimeDaySummary{
		SampleCount: 138, ObservedStartUTC: &start, ObservedEndUTC: &end,
		DominantSkycon: &skycon, TemperatureMinC: &temperatureMin,
		TemperatureMaxC: &temperatureMax, TemperatureAvgC: &temperatureAvg,
		HumidityAvgRatio: &humidityRatio, PrecipitationMaxMMH: &precipitationMax,
	}}
	service, err := newMallWeatherQueryService(fakeMallWeatherQueryMallReader{mall: &model.Mall{
		BaseModel: model.BaseModel{ID: 7}, WeatherLongitude: &longitude, WeatherLatitude: &latitude,
		GeocodeStatus: "confirmed", Timezone: "America/New_York", WeatherProvider: "caiyun",
	}}, weather, fakeMallPermissionChecker{allowed: true}, func() time.Time { return now })
	if err != nil {
		t.Fatalf("newMallWeatherQueryService() error=%v", err)
	}

	result, err := service.HistoryDaySummary(context.Background(), 17, 7, requestbody.OpenWeatherHistoryDaySummaryRequest{
		Date: "2026-03-08", QualityStatus: "VALID",
	})
	if err != nil {
		t.Fatalf("HistoryDaySummary() error=%v", err)
	}
	if result.Summary == nil || result.Summary.SampleCount != 138 || result.Summary.HumidityAvgPct == nil ||
		*result.Summary.HumidityAvgPct != 72.5 || result.Summary.ObservedStartLocal != "2026-03-08T00:00:00-05:00" ||
		result.ObservationStatus != "AVAILABLE" || result.Meta.FreshnessStatus != "FRESH" {
		t.Fatalf("result=%+v", result)
	}
	if got := weather.daySummaryQuery.EndUTC.Sub(weather.daySummaryQuery.StartUTC); got != 23*time.Hour {
		t.Fatalf("summary day duration=%s query=%+v", got, weather.daySummaryQuery)
	}
	if weather.daySummaryQuery.QualityStatus != "valid" {
		t.Fatalf("query=%+v", weather.daySummaryQuery)
	}
}

func TestOpenWeatherHistoryDaySummaryReturnsNullWhenNoObservations(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	longitude, latitude := 121.455, 31.228
	service, err := newMallWeatherQueryService(fakeMallWeatherQueryMallReader{mall: &model.Mall{
		BaseModel: model.BaseModel{ID: 7}, WeatherLongitude: &longitude, WeatherLatitude: &latitude,
		GeocodeStatus: "confirmed", Timezone: "Asia/Shanghai", WeatherProvider: "caiyun",
	}}, &fakeMallWeatherQueryDAO{}, fakeMallPermissionChecker{allowed: true}, func() time.Time { return now })
	if err != nil {
		t.Fatalf("newMallWeatherQueryService() error=%v", err)
	}
	result, err := service.HistoryDaySummary(context.Background(), 17, 7, requestbody.OpenWeatherHistoryDaySummaryRequest{Date: "2026-07-28"})
	if err != nil || result.Summary != nil || result.ObservationStatus != "UNAVAILABLE" ||
		result.Meta.FreshnessStatus != "UNAVAILABLE" {
		t.Fatalf("result=%+v error=%v", result, err)
	}
}

func TestOpenWeatherHistoryDaySummaryRejectsInvalidInputBeforeDAO(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	longitude, latitude := 121.455, 31.228
	weather := &fakeMallWeatherQueryDAO{}
	service, err := newMallWeatherQueryService(fakeMallWeatherQueryMallReader{mall: &model.Mall{
		BaseModel: model.BaseModel{ID: 7}, WeatherLongitude: &longitude, WeatherLatitude: &latitude,
		GeocodeStatus: "confirmed", Timezone: "Asia/Shanghai", WeatherProvider: "caiyun",
	}}, weather, fakeMallPermissionChecker{allowed: true}, func() time.Time { return now })
	if err != nil {
		t.Fatalf("newMallWeatherQueryService() error=%v", err)
	}
	for _, request := range []requestbody.OpenWeatherHistoryDaySummaryRequest{
		{Date: "2026-07-30"},
		{Date: "2026-07-28 00:00:00"},
		{Date: "2026-07-28", QualityStatus: "unknown"},
	} {
		if _, err := service.HistoryDaySummary(context.Background(), 17, 7, request); !errors.Is(err, ErrMallWeatherInvalidQuery) {
			t.Fatalf("HistoryDaySummary(%+v) error=%v", request, err)
		}
	}
	if weather.daySummaryCalls != 0 {
		t.Fatalf("summary DAO calls=%d", weather.daySummaryCalls)
	}
}
