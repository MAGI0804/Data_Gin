package data_svc

import (
	"context"
	"reflect"
	"testing"
	"time"

	"gin-biz-web-api/connector/caiyun"
	"gin-biz-web-api/internal/dao/data_dao"
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

func TestPersistWeatherAlertRelationsValidatesGraceWindow(t *testing.T) {
	seenAt := time.Date(2026, 7, 28, 2, 0, 0, 0, time.UTC)
	tests := []struct {
		name  string
		grace time.Duration
	}{
		{name: "zero", grace: 0},
		{name: "below minimum", grace: 59 * time.Second},
		{name: "above maximum", grace: 24*time.Hour + time.Second},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := persistWeatherAlertRelations(
				context.Background(),
				&fakeMallWeatherAlertRelationStore{},
				7,
				nil,
				seenAt,
				test.grace,
			); err == nil {
				t.Fatalf("persistWeatherAlertRelations() accepted grace=%s", test.grace)
			}
		})
	}
}

func TestPersistWeatherAlertRelationsReconcilesEmptyCurrentSet(t *testing.T) {
	seenAt := time.Date(2026, 7, 28, 2, 0, 0, 0, time.UTC)
	tests := []struct {
		name  string
		grace time.Duration
	}{
		{name: "minimum", grace: time.Minute},
		{name: "default", grace: 30 * time.Minute},
		{name: "maximum", grace: 24 * time.Hour},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &fakeMallWeatherAlertRelationStore{}
			if err := persistWeatherAlertRelations(
				context.Background(),
				store,
				7,
				nil,
				seenAt,
				test.grace,
			); err != nil {
				t.Fatalf("persistWeatherAlertRelations() error=%v", err)
			}
			if store.findCalls != 0 || store.upsertCalls != 0 || store.deactivateCalls != 1 ||
				store.mallID != 7 || store.provider != weatherdomain.ProviderCaiyun ||
				len(store.seenAlertPKs) != 0 ||
				!store.missingBefore.Equal(seenAt.Add(-test.grace)) {
				t.Fatalf("store=%+v", store)
			}
		})
	}
}

func TestPersistWeatherAlertRelationsKeepsCurrentAlertsActive(t *testing.T) {
	store := &fakeMallWeatherAlertRelationStore{
		stored: []model.MallWeatherAlert{{
			BaseModel: model.BaseModel{ID: 11},
			Provider:  weatherdomain.ProviderCaiyun,
			AlertID:   "alert-11",
		}},
	}
	seenAt := time.Date(2026, 7, 28, 2, 0, 0, 0, time.UTC)
	alerts := []model.MallWeatherAlert{{
		Provider: weatherdomain.ProviderCaiyun,
		AlertID:  "alert-11",
	}}
	if err := persistWeatherAlertRelations(
		context.Background(),
		store,
		7,
		alerts,
		seenAt,
		30*time.Minute,
	); err != nil {
		t.Fatalf("persistWeatherAlertRelations() error=%v", err)
	}
	if store.findCalls != 1 || store.upsertCalls != 1 || store.deactivateCalls != 1 ||
		!reflect.DeepEqual(store.alertIDs, []string{"alert-11"}) ||
		!reflect.DeepEqual(store.seenAlertPKs, []uint{11}) ||
		len(store.relations) != 1 || store.relations[0].MallID != 7 ||
		store.relations[0].AlertPK != 11 || !store.relations[0].IsActive ||
		!store.relations[0].LastSeenAt.Equal(seenAt) {
		t.Fatalf("store=%+v", store)
	}
}

type fakeMallWeatherAlertRelationStore struct {
	stored          []model.MallWeatherAlert
	findCalls       int
	upsertCalls     int
	deactivateCalls int
	alertIDs        []string
	relations       []model.MallWeatherAlertRelation
	mallID          uint
	provider        string
	seenAlertPKs    []uint
	missingBefore   time.Time
}

func (store *fakeMallWeatherAlertRelationStore) FindAlertsByProviderIDs(
	_ context.Context,
	_ string,
	alertIDs []string,
) ([]model.MallWeatherAlert, error) {
	store.findCalls++
	store.alertIDs = append([]string(nil), alertIDs...)
	return append([]model.MallWeatherAlert(nil), store.stored...), nil
}

func (store *fakeMallWeatherAlertRelationStore) UpsertAlertRelations(
	_ context.Context,
	relations []model.MallWeatherAlertRelation,
) (data_dao.UpsertResult, error) {
	store.upsertCalls++
	store.relations = append([]model.MallWeatherAlertRelation(nil), relations...)
	return data_dao.UpsertResult{AffectedRows: int64(len(relations))}, nil
}

func (store *fakeMallWeatherAlertRelationStore) DeactivateMissingAlertRelations(
	_ context.Context,
	mallID uint,
	provider string,
	seenAlertPKs []uint,
	missingBefore time.Time,
) (int64, error) {
	store.deactivateCalls++
	store.mallID = mallID
	store.provider = provider
	store.seenAlertPKs = append([]uint(nil), seenAlertPKs...)
	store.missingBefore = missingBefore
	return 0, nil
}
