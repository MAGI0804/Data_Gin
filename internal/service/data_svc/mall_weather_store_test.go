package data_svc

import (
	"context"
	"testing"
	"time"

	"gin-biz-web-api/connector/caiyun"
	weatherdomain "gin-biz-web-api/internal/weather"
	"gin-biz-web-api/model"

	"gorm.io/gorm"
)

func TestValidateWeatherStoreResponseChecksExecutionIdentity(t *testing.T) {
	mallID := uint(7)
	store := &gormMallWeatherTaskStore{db: &gorm.DB{}}
	execution := &mallWeatherExecution{
		Mall: model.Mall{BaseModel: model.BaseModel{ID: mallID}},
		Run: model.MallWeatherFetchRun{
			BaseModel: model.BaseModel{ID: 17}, EndpointKind: caiyun.EndpointWeatherV26,
		},
		Attempt: model.MallWeatherFetchAttempt{BaseModel: model.BaseModel{ID: 23}},
	}
	response := &caiyun.ProviderResponse{EndpointKind: caiyun.EndpointWeatherV26}
	snapshot := &model.ProviderRawSnapshot{
		Provider: weatherdomain.ProviderCaiyun, EndpointKind: caiyun.EndpointWeatherV26,
		MallID: &mallID, ResponseChecksum: "checksum",
	}
	if err := validateWeatherStoreResponse(store, context.Background(), execution, response, snapshot); err != nil {
		t.Fatalf("validateWeatherStoreResponse() error=%v", err)
	}
	snapshot.EndpointKind = caiyun.EndpointLifeIndexV3
	if err := validateWeatherStoreResponse(store, context.Background(), execution, response, snapshot); err == nil {
		t.Fatal("validateWeatherStoreResponse() accepted mismatched endpoint")
	}
}

func TestNonNegativeMilliseconds(t *testing.T) {
	if got := nonNegativeMilliseconds(-time.Second); got != 0 {
		t.Fatalf("nonNegativeMilliseconds(-1s)=%d", got)
	}
	if got := nonNegativeMilliseconds(1500 * time.Millisecond); got != 1500 {
		t.Fatalf("nonNegativeMilliseconds(1.5s)=%d", got)
	}
}

func TestAddChecksumConflictWarningIsDeterministic(t *testing.T) {
	batch := &mallWeatherModelBatch{
		ParseWarningsJSON: model.JSONText(`[{"code":"UNKNOWN_TYPE","path":"data[0]"}]`),
		RowCountsJSON:     model.JSONText(`{"life_index":3}`),
	}
	if err := addChecksumConflictWarning(batch, 2); err != nil {
		t.Fatalf("addChecksumConflictWarning() error=%v", err)
	}
	if string(batch.RowCountsJSON) != `{"checksum_conflicts":2,"life_index":3}` {
		t.Fatalf("row counts=%s", batch.RowCountsJSON)
	}
	if err := addChecksumConflictWarning(batch, 1); err != nil {
		t.Fatalf("addChecksumConflictWarning(second) error=%v", err)
	}
	if string(batch.RowCountsJSON) != `{"checksum_conflicts":3,"life_index":3}` {
		t.Fatalf("row counts=%s", batch.RowCountsJSON)
	}
}

func TestWeatherLatestSourcesIncludesSuccessfulModules(t *testing.T) {
	realtime := model.MallWeatherRealtime{MallID: 7}
	batch := &mallWeatherModelBatch{
		Forecasts: &weatherdomain.ForecastModelBatch{
			Realtime: &realtime,
			Minutely: []model.MallWeatherMinutely{{MallID: 7}},
			Hourly:   []model.MallWeatherHourly{{MallID: 7}},
		},
		Daily: &weatherdomain.DailyModelBatch{
			Daily:       []model.MallWeatherDaily{{MallID: 7}},
			LifeIndices: []model.MallWeatherLifeIndex{{MallID: 7, SourceAPI: "v26_daily"}},
		},
		LifeIndices: &weatherdomain.LifeIndexModelBatch{
			LifeIndices: []model.MallWeatherLifeIndex{{MallID: 7, SourceAPI: "v3_lifeindex"}},
		},
	}
	sources := weatherLatestSources(batch)
	if len(sources.Realtime) != 1 || len(sources.Minutely) != 1 || len(sources.Hourly) != 1 || len(sources.Daily) != 1 || len(sources.LifeIndices) != 2 {
		t.Fatalf("latest sources=%+v", sources)
	}
}
