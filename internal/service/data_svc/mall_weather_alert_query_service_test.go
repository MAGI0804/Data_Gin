package data_svc

import (
	"context"
	"testing"
	"time"

	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/model"
)

func TestMallWeatherQueryServiceAlertsMapsHistoryAndCursor(t *testing.T) {
	now := time.Date(2026, 7, 22, 4, 0, 0, 0, time.UTC)
	longitude, latitude := 121.455, 31.228
	publishedAt := now.Add(-2 * time.Hour)
	endedAt := now.Add(-time.Hour)
	rows := []model.MallWeatherAlert{
		{
			BaseModel: model.BaseModel{ID: 11}, AlertID: "alert-1", Status: "inactive", Title: "Heavy rain",
			PublishedAtUTC: &publishedAt, FirstSeenAt: publishedAt, LastSeenAt: endedAt, EndedAt: &endedAt,
			Province: "Shanghai", Adcode: "310000", QualityStatus: "valid", QualityFlagsJSON: model.JSONText("[]"),
		},
		{
			BaseModel: model.BaseModel{ID: 10}, AlertID: "alert-2", Status: "active", Title: "Wind",
			FirstSeenAt: now.Add(-3 * time.Hour), LastSeenAt: now, QualityStatus: "valid", QualityFlagsJSON: model.JSONText("[]"),
		},
	}
	weather := &fakeMallWeatherQueryDAO{
		alertRows: rows,
		latest:    &model.MallWeatherLatest{FetchedAtUTC: now.Add(-20 * time.Minute), FreshnessStatus: model.MallWeatherFreshnessFresh},
	}
	service, err := newMallWeatherQueryService(fakeMallWeatherQueryMallReader{mall: &model.Mall{
		BaseModel: model.BaseModel{ID: 7}, WeatherLongitude: &longitude, WeatherLatitude: &latitude,
		GeocodeStatus: "confirmed", Timezone: "Asia/Shanghai", CoverageRadiusM: 1000,
	}}, weather, fakeMallPermissionChecker{allowed: true}, func() time.Time { return now })
	if err != nil {
		t.Fatalf("newMallWeatherQueryService() error=%v", err)
	}
	result, err := service.Alerts(context.Background(), 17, 7, requestbody.MallWeatherAlertQueryRequest{
		StartUTC: now.Add(-24 * time.Hour), EndUTC: now.Add(time.Hour), Latest: false, PageSize: 1,
	})
	if err != nil {
		t.Fatalf("Alerts() error=%v", err)
	}
	if len(result.Items) != 1 || result.Items[0].AlertID != "alert-1" || result.Items[0].Province != "Shanghai" ||
		result.Items[0].EndedAtLocal == nil || *result.Items[0].EndedAtLocal != "2026-07-22T11:00:00+08:00" ||
		result.Meta.FreshnessStatus != "WARNING" || result.Pagination.NextCursor == "" || weather.alertQuery.Limit != 2 {
		t.Fatalf("result=%+v query=%+v", result, weather.alertQuery)
	}
	cursor, err := decodeWeatherAlertCursor(result.Pagination.NextCursor)
	if err != nil || cursor.ID != 11 || cursor.SortTimeUnixMS != publishedAt.UnixMilli() {
		t.Fatalf("cursor=%+v error=%v", cursor, err)
	}
}
