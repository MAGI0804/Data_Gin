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

func TestMallWeatherQueryServiceFetchRunsMapsSafeAuditContractAndCursor(t *testing.T) {
	now := time.Date(2026, 7, 22, 4, 0, 0, 0, time.UTC)
	startedAt, finishedAt := now.Add(-time.Minute), now.Add(-30*time.Second)
	rows := []model.MallWeatherFetchRun{
		{
			BaseModel: model.BaseModel{ID: 11}, RunUUID: "5d5047eb-2673-4ea2-91df-09565b61d944", MallID: 7,
			TaskKind: "full", EndpointKind: "v26_weather", Provider: "caiyun", AttemptCount: 2,
			Status: "partial_success", StartedAt: &startedAt, FinishedAt: &finishedAt, DurationMS: 30000,
			RowCountsJSON:     model.JSONText(`{"hourly":360,"daily":15}`),
			ParseWarningsJSON: model.JSONText(`[{"code":"MISSING_FIELD","path":"result.daily"}]`),
			ErrorClass:        "partial", ErrorCode: "MISSING_FIELD", ErrorMessageSafe: "weather response is partial",
			WeatherTimestamps: model.WeatherTimestamps{CreatedAt: now.Add(-2 * time.Minute), UpdatedAt: now},
		},
		{
			BaseModel: model.BaseModel{ID: 10}, RunUUID: "ef618100-ad9d-4697-9003-192f2fdd050c", MallID: 7,
			TaskKind: "lifeindex", EndpointKind: "v3_life_index", Provider: "caiyun", Status: "success",
			WeatherTimestamps: model.WeatherTimestamps{CreatedAt: now.Add(-3 * time.Minute), UpdatedAt: now},
		},
	}
	weather := &fakeMallWeatherQueryDAO{fetchRunRows: rows}
	service, err := newMallWeatherQueryService(fakeMallWeatherQueryMallReader{mall: &model.Mall{
		BaseModel: model.BaseModel{ID: 7}, Timezone: "Asia/Shanghai",
	}}, weather, fakeMallPermissionChecker{allowed: true}, func() time.Time { return now })
	if err != nil {
		t.Fatalf("newMallWeatherQueryService() error=%v", err)
	}
	result, err := service.FetchRuns(context.Background(), 17, 7, requestbody.MallWeatherFetchRunQueryRequest{
		StartUTC: now.Add(-24 * time.Hour), EndUTC: now.Add(time.Hour), TaskKind: "full", EndpointKind: "v26_weather",
		Status: "partial_success", PageSize: 1,
	})
	if err != nil {
		t.Fatalf("FetchRuns() error=%v", err)
	}
	if len(result.Items) != 1 || result.Items[0].TaskKind != "FULL" || result.Items[0].Status != "PARTIAL_SUCCESS" ||
		result.Items[0].RowCounts["hourly"] != 360 || len(result.Items[0].ParseWarnings) != 1 || result.Items[0].StartedAtLocal == nil ||
		result.Pagination.NextCursor == "" || weather.fetchRunQuery.TaskKind != "full" || weather.fetchRunQuery.Limit != 2 ||
		result.Meta.TimeZone != "Asia/Shanghai" {
		t.Fatalf("result=%+v query=%+v", result, weather.fetchRunQuery)
	}
	cursor, err := decodeWeatherFetchRunCursor(result.Pagination.NextCursor)
	if err != nil || cursor.ID != 11 || cursor.CreatedAtUnixMS != rows[0].CreatedAt.UnixMilli() {
		t.Fatalf("cursor=%+v error=%v", cursor, err)
	}
}

func TestMallWeatherQueryServiceFetchRunsDoesNotRequireConfirmedCoordinate(t *testing.T) {
	now := time.Date(2026, 7, 22, 4, 0, 0, 0, time.UTC)
	service, err := newMallWeatherQueryService(fakeMallWeatherQueryMallReader{mall: &model.Mall{
		BaseModel: model.BaseModel{ID: 7}, Timezone: "Asia/Shanghai", GeocodeStatus: "failed",
	}}, &fakeMallWeatherQueryDAO{}, fakeMallPermissionChecker{allowed: true}, func() time.Time { return now })
	if err != nil {
		t.Fatalf("newMallWeatherQueryService() error=%v", err)
	}
	if _, err := service.FetchRuns(context.Background(), 17, 7, requestbody.MallWeatherFetchRunQueryRequest{
		StartUTC: now.Add(-time.Hour), EndUTC: now,
	}); err != nil {
		t.Fatalf("FetchRuns() error=%v", err)
	}
}

func TestWeatherFetchRunCursorRejectsUnknownFields(t *testing.T) {
	raw := base64.RawURLEncoding.EncodeToString([]byte(`{"v":1,"createdAtUnixMs":1784689200000,"id":1,"secret":"x"}`))
	if _, err := decodeWeatherFetchRunCursor(raw); !errors.Is(err, ErrMallWeatherInvalidQuery) {
		t.Fatalf("decodeWeatherFetchRunCursor() error=%v", err)
	}
}
