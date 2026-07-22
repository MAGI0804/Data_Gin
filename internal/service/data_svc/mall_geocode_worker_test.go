package data_svc

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"gin-biz-web-api/connector/geocoder"
	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/job"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/providerhttp"
)

func TestMallGeocodeProcessorSkipsStaleTaskBeforeProviderCall(t *testing.T) {
	mall := geocodeTestMall()
	provider := &fakeGeocoder{}
	store := &fakeMallGeocodeStore{mall: mall}
	processor := newMallGeocodeProcessor(provider, store, time.Now)
	payload := geocodeTestPayload(mall)
	payload.MallVersion++

	if err := processor.Process(context.Background(), payload); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if provider.calls != 0 || store.persistCalls != 0 {
		t.Fatalf("provider calls=%d persist calls=%d", provider.calls, store.persistCalls)
	}
}

func TestMallGeocodeProcessorTreatsDeletedOrConcurrentlyChangedMallAsComplete(t *testing.T) {
	tests := []struct {
		name  string
		store *fakeMallGeocodeStore
	}{
		{name: "deleted before provider call", store: &fakeMallGeocodeStore{findErr: data_dao.ErrMallNotFound}},
		{name: "changed before persistence", store: &fakeMallGeocodeStore{mall: geocodeTestMall(), persistErr: ErrMallGeocodeStale}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mall := geocodeTestMall()
			if test.store.mall == nil && test.store.findErr == nil {
				test.store.mall = mall
			}
			processor := newMallGeocodeProcessor(&fakeGeocoder{response: &geocoder.Response{}}, test.store, time.Now)
			if err := processor.Process(context.Background(), geocodeTestPayload(mall)); err != nil {
				t.Fatalf("Process() error = %v", err)
			}
		})
	}
}

func TestMallGeocodeProcessorScoresAndPersistsProviderResponse(t *testing.T) {
	mall := geocodeTestMall()
	provider := &fakeGeocoder{response: &geocoder.Response{Candidates: []geocoder.Candidate{{
		FormattedAddress: "上海市黄浦区示例路1号示例商场", Province: "上海市", District: "黄浦区",
		Street: "示例路", StreetNumber: "1号", Level: "兴趣点", Longitude: 121.4, Latitude: 31.2,
	}}}}
	store := &fakeMallGeocodeStore{mall: mall}
	nowValues := []time.Time{time.Unix(10, 0), time.Unix(11, 0)}
	processor := newMallGeocodeProcessor(provider, store, func() time.Time {
		value := nowValues[0]
		nowValues = nowValues[1:]
		return value
	})

	if err := processor.Process(context.Background(), geocodeTestPayload(mall)); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if provider.calls != 1 || store.persistCalls != 1 || len(store.outcome.Scores) != 1 {
		t.Fatalf("provider calls=%d persist calls=%d outcome=%+v", provider.calls, store.persistCalls, store.outcome)
	}
	if provider.request.City != "上海市" || !strings.Contains(provider.request.Address, "示例商场") {
		t.Fatalf("provider request = %+v", provider.request)
	}
}

func TestMallGeocodeProcessorPersistsRetryableProviderFailure(t *testing.T) {
	mall := geocodeTestMall()
	providerFailure := &geocoder.ProviderError{Class: providerhttp.ErrorClassRateLimited, Code: "10014", Retryable: true}
	provider := &fakeGeocoder{err: providerFailure}
	store := &fakeMallGeocodeStore{mall: mall}
	processor := newMallGeocodeProcessor(provider, store, time.Now)

	err := processor.Process(context.Background(), geocodeTestPayload(mall))
	var processError *MallGeocodeProcessError
	if !errors.As(err, &processError) || !processError.Retryable {
		t.Fatalf("Process() error = %+v", err)
	}
	if store.persistCalls != 1 || store.outcome.ProviderErr != providerFailure {
		t.Fatalf("persist calls=%d outcome=%+v", store.persistCalls, store.outcome)
	}
	if strings.Contains(err.Error(), "10014") {
		t.Fatalf("public worker error leaked provider detail: %v", err)
	}
}

func TestMallGeocodeProcessorDoesNotPersistCancellation(t *testing.T) {
	mall := geocodeTestMall()
	store := &fakeMallGeocodeStore{mall: mall}
	processor := newMallGeocodeProcessor(&fakeGeocoder{err: context.Canceled}, store, time.Now)

	if err := processor.Process(context.Background(), geocodeTestPayload(mall)); !errors.Is(err, context.Canceled) {
		t.Fatalf("Process() error = %v", err)
	}
	if store.persistCalls != 0 {
		t.Fatalf("persist calls = %d", store.persistCalls)
	}
}

func TestGormMallGeocodeStoreBuildsAuditableAutoConfirmRows(t *testing.T) {
	mall := geocodeTestMall()
	payload := geocodeTestPayload(mall)
	raw := json.RawMessage(`{"status":"1","count":"1"}`)
	response := &geocoder.Response{ProviderStatus: "1", Infocode: "10000", Info: "OK", RawJSON: raw, Candidates: []geocoder.Candidate{{
		FormattedAddress: "上海市黄浦区示例路1号示例商场", Province: "上海市", District: "黄浦区",
		Street: "示例路", StreetNumber: "1号", Level: "兴趣点", CoordinateSystem: "GCJ02",
		Longitude: 121.4, Latitude: 31.2,
	}}}
	scores := geocoder.ScoreCandidates(geocoder.ScoreInput{
		Name: mall.NameCN, Province: mall.Province, City: mall.City, District: mall.District,
		Street: mall.Street, StreetNumber: mall.StreetNumber, Address: mall.AddressRaw,
	}, response.Candidates)
	store := &gormMallGeocodeStore{rawRetentionDays: 30}
	finishedAt := time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)
	run, candidates, snapshot, autoIndex, err := store.buildPersistenceRows(payload, mall, mallGeocodeOutcome{
		Response: response, Scores: scores, StartedAt: finishedAt.Add(-time.Second), FinishedAt: finishedAt,
	})
	if err != nil {
		t.Fatalf("buildPersistenceRows() error = %v", err)
	}
	if autoIndex != 0 || run.Status != "auto_confirmed" || len(candidates) != 1 || !candidates[0].IsSelected {
		t.Fatalf("run=%+v candidates=%+v autoIndex=%d", run, candidates, autoIndex)
	}
	if snapshot == nil || snapshot.ResponseChecksum == "" || snapshot.ContentLength != int64(len(raw)) || snapshot.ExpiresAt == nil {
		t.Fatalf("snapshot = %+v", snapshot)
	}
	reader, err := gzip.NewReader(bytes.NewReader(snapshot.ContentBlob))
	if err != nil {
		t.Fatalf("gzip.NewReader() error = %v", err)
	}
	decoded, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read compressed snapshot: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close compressed snapshot: %v", err)
	}
	if string(decoded) != string(raw) {
		t.Fatalf("decoded snapshot = %s", decoded)
	}
}

func TestNewInitialWeatherOutboxesBuildsIndependentMinimalTasks(t *testing.T) {
	now := time.Date(2026, 7, 22, 8, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	rows, err := newInitialWeatherOutboxes(7, 4, now)
	if err != nil {
		t.Fatalf("newInitialWeatherOutboxes() error = %v", err)
	}
	if len(rows) != 2 || rows[0].TaskType != job.TypeMallWeatherFull || rows[1].TaskType != job.TypeMallWeatherLifeIndex {
		t.Fatalf("rows = %+v", rows)
	}
	for _, row := range rows {
		if row.QueueName != job.MallWeatherQueueName || !strings.Contains(row.TaskKey, "7") || !strings.Contains(row.TaskKey, "v4") || !row.AvailableAt.Equal(now.UTC()) {
			t.Fatalf("row = %+v", row)
		}
		if string(row.PayloadJSON) != `{"mall_id":7}` {
			t.Fatalf("payload = %s", row.PayloadJSON)
		}
	}
}

type fakeGeocoder struct {
	response *geocoder.Response
	err      error
	calls    int
	request  geocoder.Request
}

func (provider *fakeGeocoder) Geocode(_ context.Context, request geocoder.Request) (*geocoder.Response, error) {
	provider.calls++
	provider.request = request
	return provider.response, provider.err
}

type fakeMallGeocodeStore struct {
	mall         *model.Mall
	findErr      error
	persistErr   error
	persistCalls int
	payload      job.MallGeocodeTaskPayload
	outcome      mallGeocodeOutcome
}

func (store *fakeMallGeocodeStore) FindMall(context.Context, uint) (*model.Mall, error) {
	return store.mall, store.findErr
}

func (store *fakeMallGeocodeStore) Persist(_ context.Context, payload job.MallGeocodeTaskPayload, _ *model.Mall, outcome mallGeocodeOutcome) error {
	store.persistCalls++
	store.payload = payload
	store.outcome = outcome
	return store.persistErr
}

func geocodeTestMall() *model.Mall {
	return &model.Mall{
		BaseModel: model.BaseModel{ID: 7}, MallCode: "SH-001", NameCN: "示例商场",
		Province: "上海市", City: "上海市", District: "黄浦区", AddressRaw: "示例路1号",
		Street: "示例路", StreetNumber: "1号", GeocodeStatus: "pending", Version: 3,
	}
}

func geocodeTestPayload(mall *model.Mall) job.MallGeocodeTaskPayload {
	return job.MallGeocodeTaskPayload{MallID: mall.ID, MallVersion: mall.Version, AddressHash: mallAddressHash(mall)}
}
