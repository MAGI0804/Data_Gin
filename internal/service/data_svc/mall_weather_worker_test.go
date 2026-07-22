package data_svc

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"
	"time"

	"gin-biz-web-api/connector/caiyun"
	"gin-biz-web-api/internal/dao/data_dao"
	weatherdomain "gin-biz-web-api/internal/weather"
	"gin-biz-web-api/job"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/providerhttp"
)

func TestMallWeatherProcessorPersistsLifeIndexAfterRawSnapshot(t *testing.T) {
	raw := readMallWeatherFixture(t, "../../../connector/caiyun/testdata/life_index_v3.json")
	provider := &fakeMallWeatherProvider{lifeResponse: &caiyun.ProviderResponse{
		EndpointKind: caiyun.EndpointLifeIndexV3, HTTPStatus: 200, ProviderStatus: "ok", RawBody: raw,
	}}
	store := newFakeMallWeatherTaskStore(data_dao.FetchAttemptDispositionAcquired)
	processor := newTestMallWeatherProcessor(t, provider, store)
	payload := job.MallTaskPayload{MallID: 7, TaskWindow: "life:7:2026072203"}

	if err := processor.Process(context.Background(), job.TypeMallWeatherLifeIndex, payload); err != nil {
		t.Fatalf("Process() error=%v", err)
	}
	if !reflect.DeepEqual(store.events, []string{"start", "response", "persist"}) {
		t.Fatalf("events=%v", store.events)
	}
	if store.snapshot == nil || store.snapshot.ResponseChecksum == "" || store.batch == nil ||
		store.batch.Status != weatherFetchStatusSuccess || store.batch.LifeIndices == nil || len(store.batch.LifeIndices.LifeIndices) == 0 {
		t.Fatalf("snapshot=%+v batch=%+v", store.snapshot, store.batch)
	}
	if provider.lifeRequest.Days != 15 || provider.lifeRequest.Fields != "all" || provider.weatherCalls != 0 || provider.lifeCalls != 1 {
		t.Fatalf("provider=%+v", provider)
	}
}

func TestMallWeatherProcessorPersistsAvailableV26ModulesAsPartialSuccess(t *testing.T) {
	raw := readMallWeatherFixture(t, "../../../connector/caiyun/testdata/weather_v26_realtime.json")
	provider := &fakeMallWeatherProvider{weatherResponse: &caiyun.ProviderResponse{
		EndpointKind: caiyun.EndpointWeatherV26, HTTPStatus: 200, ProviderStatus: "ok", RawBody: raw,
	}}
	store := newFakeMallWeatherTaskStore(data_dao.FetchAttemptDispositionAcquired)
	processor := newTestMallWeatherProcessor(t, provider, store)
	payload := job.MallTaskPayload{MallID: 7, TaskWindow: "fast:7:202607220310"}

	if err := processor.Process(context.Background(), job.TypeMallWeatherFast, payload); err != nil {
		t.Fatalf("Process() error=%v", err)
	}
	if store.batch == nil || store.batch.Status != weatherFetchStatusPartialSuccess ||
		store.batch.Forecasts == nil || store.batch.Forecasts.Realtime == nil {
		t.Fatalf("batch=%+v", store.batch)
	}
	if provider.weatherRequest.HourlySteps != 24 || provider.weatherRequest.DailySteps != 1 ||
		provider.weatherRequest.Unit != "metric:v2" || !provider.weatherRequest.Alert {
		t.Fatalf("request=%+v", provider.weatherRequest)
	}
}

func TestMallWeatherProcessorRecordsProviderBodyBeforeRetryableFailure(t *testing.T) {
	providerFailure := &caiyun.ProviderError{
		Class: providerhttp.ErrorClassProvider, HTTPStatus: 503, Retryable: true,
	}
	provider := &fakeMallWeatherProvider{
		weatherResponse: &caiyun.ProviderResponse{
			EndpointKind: caiyun.EndpointWeatherV26, HTTPStatus: 503,
			ProviderStatus: "unavailable", RawBody: []byte(`{"status":"unavailable"}`),
		},
		weatherErr: providerFailure,
	}
	store := newFakeMallWeatherTaskStore(data_dao.FetchAttemptDispositionAcquired)
	processor := newTestMallWeatherProcessor(t, provider, store)
	err := processor.Process(context.Background(), job.TypeMallWeatherFull, job.MallTaskPayload{
		MallID: 7, TaskWindow: "full:7:2026072203",
	})
	var processError *MallWeatherProcessError
	if !errors.As(err, &processError) || !processError.Retryable {
		t.Fatalf("Process() error=%v", err)
	}
	if !reflect.DeepEqual(store.events, []string{"start", "response", "fail"}) || store.snapshot == nil ||
		store.failure.AttemptStatus != "provider_failed" || store.failure.ErrorClass != string(providerhttp.ErrorClassProvider) {
		t.Fatalf("events=%v snapshot=%+v failure=%+v", store.events, store.snapshot, store.failure)
	}
}

func TestMallWeatherProcessorClassifiesTransportAndParseFailures(t *testing.T) {
	tests := []struct {
		name          string
		provider      *fakeMallWeatherProvider
		wantEvents    []string
		wantStatus    string
		wantRetryable bool
	}{
		{
			name: "transport without body",
			provider: &fakeMallWeatherProvider{weatherErr: &caiyun.ProviderError{
				Class: providerhttp.ErrorClassTransport, Retryable: true,
			}},
			wantEvents: []string{"start", "fail"}, wantStatus: "transport_failed", wantRetryable: true,
		},
		{
			name: "invalid response after snapshot",
			provider: &fakeMallWeatherProvider{weatherResponse: &caiyun.ProviderResponse{
				EndpointKind: caiyun.EndpointWeatherV26, HTTPStatus: 200, ProviderStatus: "ok", RawBody: []byte(`{}`),
			}},
			wantEvents: []string{"start", "response", "fail"}, wantStatus: "parse_failed", wantRetryable: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := newFakeMallWeatherTaskStore(data_dao.FetchAttemptDispositionAcquired)
			processor := newTestMallWeatherProcessor(t, test.provider, store)
			err := processor.Process(context.Background(), job.TypeMallWeatherFull, job.MallTaskPayload{
				MallID: 7, TaskWindow: "full:7:2026072203",
			})
			var processError *MallWeatherProcessError
			if !errors.As(err, &processError) || processError.Retryable != test.wantRetryable {
				t.Fatalf("Process() error=%v", err)
			}
			if !reflect.DeepEqual(store.events, test.wantEvents) || store.failure.AttemptStatus != test.wantStatus {
				t.Fatalf("events=%v failure=%+v", store.events, store.failure)
			}
		})
	}
}

func TestMallWeatherProcessorSkipsTerminalOrBusyRun(t *testing.T) {
	for _, disposition := range []data_dao.FetchAttemptDisposition{
		data_dao.FetchAttemptDispositionTerminal,
		data_dao.FetchAttemptDispositionBusy,
	} {
		store := newFakeMallWeatherTaskStore(disposition)
		provider := &fakeMallWeatherProvider{}
		processor := newTestMallWeatherProcessor(t, provider, store)
		if err := processor.Process(context.Background(), job.TypeMallWeatherFull, job.MallTaskPayload{
			MallID: 7, TaskWindow: "full:7:2026072203",
		}); err != nil {
			t.Fatalf("Process(%v) error=%v", disposition, err)
		}
		if provider.weatherCalls != 0 || !reflect.DeepEqual(store.events, []string{"start"}) {
			t.Fatalf("disposition=%v provider calls=%d events=%v", disposition, provider.weatherCalls, store.events)
		}
	}
}

func TestMallWeatherProcessorFinalizesCanceledAttempt(t *testing.T) {
	provider := &fakeMallWeatherProvider{weatherErr: context.Canceled}
	store := newFakeMallWeatherTaskStore(data_dao.FetchAttemptDispositionAcquired)
	processor := newTestMallWeatherProcessor(t, provider, store)
	err := processor.Process(context.Background(), job.TypeMallWeatherFull, job.MallTaskPayload{
		MallID: 7, TaskWindow: "full:7:2026072203",
	})
	if !errors.Is(err, context.Canceled) || store.failure.ErrorCode != "CANCELED" || store.failure.AttemptStatus != "transport_failed" {
		t.Fatalf("Process() error=%v failure=%+v", err, store.failure)
	}
}

func TestMallWeatherProcessorDiscardsSupersededAttemptResults(t *testing.T) {
	raw := readMallWeatherFixture(t, "../../../connector/caiyun/testdata/life_index_v3.json")
	provider := &fakeMallWeatherProvider{lifeResponse: &caiyun.ProviderResponse{
		EndpointKind: caiyun.EndpointLifeIndexV3, HTTPStatus: 200, ProviderStatus: "ok", RawBody: raw,
	}}
	store := newFakeMallWeatherTaskStore(data_dao.FetchAttemptDispositionAcquired)
	store.recordErr = ErrMallWeatherAttemptSuperseded
	processor := newTestMallWeatherProcessor(t, provider, store)
	if err := processor.Process(context.Background(), job.TypeMallWeatherLifeIndex, job.MallTaskPayload{
		MallID: 7, TaskWindow: "life:7:2026072203",
	}); err != nil {
		t.Fatalf("Process() error=%v", err)
	}
	if !reflect.DeepEqual(store.events, []string{"start", "response"}) {
		t.Fatalf("events=%v", store.events)
	}
}

type fakeMallWeatherProvider struct {
	weatherResponse *caiyun.ProviderResponse
	weatherErr      error
	lifeResponse    *caiyun.ProviderResponse
	lifeErr         error
	weatherRequest  caiyun.WeatherRequest
	lifeRequest     caiyun.LifeIndexRequest
	weatherCalls    int
	lifeCalls       int
}

func (provider *fakeMallWeatherProvider) FetchWeather(_ context.Context, input caiyun.WeatherRequest) (*caiyun.ProviderResponse, error) {
	provider.weatherCalls++
	provider.weatherRequest = input
	return provider.weatherResponse, provider.weatherErr
}

func (provider *fakeMallWeatherProvider) FetchLifeIndices(_ context.Context, input caiyun.LifeIndexRequest) (*caiyun.ProviderResponse, error) {
	provider.lifeCalls++
	provider.lifeRequest = input
	return provider.lifeResponse, provider.lifeErr
}

type fakeMallWeatherTaskStore struct {
	execution  *mallWeatherExecution
	start      mallWeatherTaskStart
	events     []string
	response   *caiyun.ProviderResponse
	snapshot   *model.ProviderRawSnapshot
	failure    mallWeatherFailure
	batch      *mallWeatherModelBatch
	startErr   error
	recordErr  error
	failErr    error
	persistErr error
}

func newFakeMallWeatherTaskStore(disposition data_dao.FetchAttemptDisposition) *fakeMallWeatherTaskStore {
	longitude := 121.4551234
	latitude := 31.2285678
	return &fakeMallWeatherTaskStore{execution: &mallWeatherExecution{
		Mall: model.Mall{
			BaseModel: model.BaseModel{ID: 7}, Status: "active", GeocodeStatus: "confirmed",
			WeatherEnabled: true, WeatherProvider: weatherdomain.ProviderCaiyun,
			WeatherLongitude: &longitude, WeatherLatitude: &latitude, WeatherCoordinateSystem: "GCJ02",
		},
		Run: model.MallWeatherFetchRun{
			BaseModel: model.BaseModel{ID: 17}, MallID: 7, AttemptCount: 1, Status: "running",
		},
		Attempt: model.MallWeatherFetchAttempt{
			BaseModel: model.BaseModel{ID: 23}, FetchRunID: 17, AttemptNo: 1, Status: "running",
		},
		Disposition: disposition,
	}}
}

func (store *fakeMallWeatherTaskStore) Start(_ context.Context, input mallWeatherTaskStart, _ time.Time) (*mallWeatherExecution, error) {
	store.events = append(store.events, "start")
	store.start = input
	store.execution.Run.EndpointKind = input.Payload.EndpointKind
	store.execution.Run.TaskKind = input.TaskKind
	store.execution.Run.TaskWindow = input.Payload.TaskWindow
	return store.execution, store.startErr
}

func (store *fakeMallWeatherTaskStore) RecordResponse(_ context.Context, _ *mallWeatherExecution, response *caiyun.ProviderResponse, snapshot *model.ProviderRawSnapshot) error {
	store.events = append(store.events, "response")
	store.response = response
	store.snapshot = snapshot
	return store.recordErr
}

func (store *fakeMallWeatherTaskStore) Fail(_ context.Context, _ *mallWeatherExecution, failure mallWeatherFailure) error {
	store.events = append(store.events, "fail")
	store.failure = failure
	return store.failErr
}

func (store *fakeMallWeatherTaskStore) Persist(_ context.Context, _ *mallWeatherExecution, batch *mallWeatherModelBatch) error {
	store.events = append(store.events, "persist")
	store.batch = batch
	return store.persistErr
}

func newTestMallWeatherProcessor(t *testing.T, provider mallWeatherProvider, store mallWeatherTaskStore) *MallWeatherProcessor {
	t.Helper()
	weatherSnapshots, err := weatherdomain.NewRawSnapshotBuilder(weatherdomain.RawSnapshotConfig{
		SchemaVersion: weatherParserVersionV26,
	}, nil)
	if err != nil {
		t.Fatalf("NewRawSnapshotBuilder(weather) error=%v", err)
	}
	lifeSnapshots, err := weatherdomain.NewRawSnapshotBuilder(weatherdomain.RawSnapshotConfig{
		SchemaVersion: weatherParserVersionLifeV3,
	}, nil)
	if err != nil {
		t.Fatalf("NewRawSnapshotBuilder(life) error=%v", err)
	}
	current := time.Date(2026, 7, 22, 3, 0, 0, 0, time.UTC)
	processor, err := newMallWeatherProcessor(provider, store, weatherSnapshots, lifeSnapshots, MallWeatherProcessorConfig{
		FastHourlySteps: 24, FastDailySteps: 1, FullHourlySteps: 360, FullDailySteps: 15,
		LifeIndexDays: 15, Unit: "metric:v2", AlertEnabled: true,
		AttemptStaleAfter: 2 * time.Minute, FailureFinalizeTimeout: time.Second,
	}, func() time.Time {
		result := current
		current = current.Add(time.Second)
		return result
	})
	if err != nil {
		t.Fatalf("newMallWeatherProcessor() error=%v", err)
	}
	return processor
}

func readMallWeatherFixture(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error=%v", path, err)
	}
	return raw
}
