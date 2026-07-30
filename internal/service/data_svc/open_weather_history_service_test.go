package data_svc

import (
	"context"
	"errors"
	"testing"
	"time"

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
