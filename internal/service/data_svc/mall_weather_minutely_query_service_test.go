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

func TestMallWeatherQueryServiceMinutelyMapsVersionsAndCursor(t *testing.T) {
	now := time.Date(2026, 7, 22, 4, 0, 0, 0, time.UTC)
	longitude, latitude := 121.455, 31.228
	probability, window := 0.35, 30
	rows := []model.MallWeatherMinutely{
		{
			BaseModel: model.BaseModel{ID: 11}, MallID: 7, ForecastMinuteUTC: now.Add(time.Minute),
			IssuedAtUTC: now.Add(-time.Minute), FetchedAtUTC: now, ProbabilityRatio: &probability,
			ProbabilityWindow: &window, Datasource: "radar",
			WeatherQualityFields: model.WeatherQualityFields{QualityStatus: "valid", QualityFlagsJSON: model.JSONText("[]")},
		},
		{
			BaseModel: model.BaseModel{ID: 12}, MallID: 7, ForecastMinuteUTC: now.Add(2 * time.Minute),
			IssuedAtUTC: now.Add(-time.Minute), FetchedAtUTC: now,
			WeatherQualityFields: model.WeatherQualityFields{QualityStatus: "valid", QualityFlagsJSON: model.JSONText("[]")},
		},
	}
	weather := &fakeMallWeatherQueryDAO{
		minutelyRows: rows,
		latest:       &model.MallWeatherLatest{FetchedAtUTC: now.Add(-20 * time.Minute), FreshnessStatus: model.MallWeatherFreshnessFresh},
	}
	service, err := newMallWeatherQueryService(fakeMallWeatherQueryMallReader{mall: &model.Mall{
		BaseModel: model.BaseModel{ID: 7}, WeatherLongitude: &longitude, WeatherLatitude: &latitude,
		GeocodeStatus: "confirmed", Timezone: "Asia/Shanghai", CoverageRadiusM: 1000,
	}}, weather, fakeMallPermissionChecker{allowed: true}, func() time.Time { return now })
	if err != nil {
		t.Fatalf("newMallWeatherQueryService() error=%v", err)
	}
	result, err := service.Minutely(context.Background(), 17, 7, requestbody.MallWeatherMinutelyQueryRequest{
		StartUTC: now, EndUTC: now.Add(2 * time.Hour), Latest: true, PageSize: 1,
	})
	if err != nil {
		t.Fatalf("Minutely() error=%v", err)
	}
	if len(result.Items) != 1 || result.Items[0].ProbabilityPct == nil || *result.Items[0].ProbabilityPct != 35 ||
		result.Items[0].ProbabilityWindow == nil || *result.Items[0].ProbabilityWindow != 30 || result.Items[0].Datasource != "radar" ||
		result.Items[0].ForecastMinuteLocal != "2026-07-22T12:01:00+08:00" ||
		result.Meta.FreshnessStatus != "WARNING" || result.Pagination.NextCursor == "" ||
		!weather.minutelyQuery.Latest || weather.minutelyQuery.Limit != 2 {
		t.Fatalf("result=%+v query=%+v", result, weather.minutelyQuery)
	}
	cursor, err := decodeWeatherMinutelyCursor(result.Pagination.NextCursor)
	if err != nil || cursor.ID != 11 || cursor.IssuedAtUnixMS != rows[0].IssuedAtUTC.UnixMilli() {
		t.Fatalf("cursor=%+v error=%v", cursor, err)
	}
}

func TestWeatherMinutelyCursorRejectsUnknownFields(t *testing.T) {
	raw := base64.RawURLEncoding.EncodeToString([]byte(`{"v":1,"forecastMinuteUnixMs":1784692860000,"issuedAtUnixMs":1784692740000,"id":1,"secret":"x"}`))
	if _, err := decodeWeatherMinutelyCursor(raw); !errors.Is(err, ErrMallWeatherInvalidQuery) {
		t.Fatalf("decodeWeatherMinutelyCursor() error=%v", err)
	}
}
