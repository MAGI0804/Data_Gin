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

func TestMallWeatherRefreshServiceNormalizesAndAuthorizesForce(t *testing.T) {
	now := time.Date(2026, 7, 22, 4, 0, 0, 0, time.UTC)
	longitude, latitude := 121.455, 31.228
	permissions := &recordingMallPermissionChecker{allowed: map[string]bool{
		PermissionWeatherRefresh: true, PermissionWeatherConfigManage: true,
	}}
	store := &fakeMallWeatherRefreshStore{result: &MallWeatherRefreshResult{JobID: 9}}
	service, err := newMallWeatherRefreshService(fakeMallWeatherQueryMallReader{mall: &model.Mall{
		BaseModel: model.BaseModel{ID: 7}, Status: "active", WeatherEnabled: true, WeatherProvider: "caiyun",
		GeocodeStatus: "confirmed", WeatherLongitude: &longitude, WeatherLatitude: &latitude,
	}}, permissions, store, func() time.Time { return now })
	if err != nil {
		t.Fatalf("newMallWeatherRefreshService() error=%v", err)
	}
	result, _, err := service.Refresh(context.Background(), 17, 7, "refresh-key-1234", requestbody.MallWeatherRefreshRequest{
		Kinds: []string{"v26_full"}, Force: true, Reason: "  operator review  ",
	})
	if err != nil || result.JobID != 9 {
		t.Fatalf("Refresh() result=%+v error=%v", result, err)
	}
	if len(permissions.requested) != 2 || store.command.Reason != "operator review" || !store.command.Force ||
		len(store.command.Kinds) != 1 || store.command.Kinds[0] != MallWeatherRefreshKindV26Full ||
		len(store.command.KeyHash) != 64 || len(store.command.RequestHash) != 64 || len(store.command.TaskWindow) != 55 ||
		result.CorrelationID != store.command.TaskWindow {
		t.Fatalf("permissions=%v command=%+v", permissions.requested, store.command)
	}
}

func TestMallWeatherRefreshServiceRestoresCorrelationIDForLegacyReplay(t *testing.T) {
	now := time.Date(2026, 7, 22, 4, 0, 0, 0, time.UTC)
	longitude, latitude := 121.455, 31.228
	store := &fakeMallWeatherRefreshStore{
		result:   &MallWeatherRefreshResult{JobID: 9},
		replayed: true,
	}
	service, err := newMallWeatherRefreshService(fakeMallWeatherQueryMallReader{mall: &model.Mall{
		BaseModel: model.BaseModel{ID: 7}, Status: "active", WeatherEnabled: true, WeatherProvider: "caiyun",
		GeocodeStatus: "confirmed", WeatherLongitude: &longitude, WeatherLatitude: &latitude,
	}}, &recordingMallPermissionChecker{allowed: map[string]bool{PermissionWeatherRefresh: true}}, store, func() time.Time { return now })
	if err != nil {
		t.Fatalf("newMallWeatherRefreshService() error=%v", err)
	}
	result, replayed, err := service.Refresh(context.Background(), 17, 7, "refresh-key-1234", requestbody.MallWeatherRefreshRequest{
		Kinds: []string{MallWeatherRefreshKindV26Full}, Reason: "operator review",
	})
	if err != nil || !replayed || result == nil || result.CorrelationID == "" || result.CorrelationID != store.command.TaskWindow {
		t.Fatalf("Refresh() result=%+v replayed=%v command=%+v error=%v", result, replayed, store.command, err)
	}
}

func TestMallWeatherRefreshServiceRejectsStandaloneLifeIndexKind(t *testing.T) {
	service, err := newMallWeatherRefreshService(
		fakeMallWeatherQueryMallReader{},
		&recordingMallPermissionChecker{allowed: map[string]bool{PermissionWeatherRefresh: true}},
		&fakeMallWeatherRefreshStore{},
		time.Now,
	)
	if err != nil {
		t.Fatalf("newMallWeatherRefreshService() error=%v", err)
	}
	_, _, err = service.Refresh(context.Background(), 17, 7, "refresh-key-1234", requestbody.MallWeatherRefreshRequest{
		Kinds: []string{"V3_LIFE_INDEX"}, Reason: "operator review",
	})
	if !errors.Is(err, ErrMallInvalidInput) {
		t.Fatalf("Refresh() error=%v", err)
	}
}

func TestMallWeatherRefreshServiceRequiresAdminPermissionForForce(t *testing.T) {
	now := time.Now()
	permissions := &recordingMallPermissionChecker{allowed: map[string]bool{PermissionWeatherRefresh: true}}
	service, err := newMallWeatherRefreshService(fakeMallWeatherQueryMallReader{}, permissions, &fakeMallWeatherRefreshStore{}, func() time.Time { return now })
	if err != nil {
		t.Fatalf("newMallWeatherRefreshService() error=%v", err)
	}
	_, _, err = service.Refresh(context.Background(), 17, 7, "refresh-key-1234", requestbody.MallWeatherRefreshRequest{
		Kinds: []string{MallWeatherRefreshKindV26Full}, Force: true, Reason: "operator review",
	})
	if !errors.Is(err, ErrMallForbidden) || len(permissions.requested) != 2 {
		t.Fatalf("Refresh() permissions=%v error=%v", permissions.requested, err)
	}
}

func TestMallWeatherRefreshKindFreshChecksAllComprehensivePointers(t *testing.T) {
	now := time.Date(2026, 7, 22, 4, 0, 0, 0, time.UTC)
	latest := &fakeMallWeatherRefreshLatestReader{byKind: map[string]*model.MallWeatherLatest{}}
	for _, kind := range []string{model.MallWeatherDataKindRealtime, model.MallWeatherDataKindMinutely, model.MallWeatherDataKindHourly, model.MallWeatherDataKindDaily} {
		latest.byKind[kind] = &model.MallWeatherLatest{FetchedAtUTC: now.Add(-time.Minute), FreshnessStatus: model.MallWeatherFreshnessFresh}
	}
	latest.life = &model.MallWeatherLatest{FetchedAtUTC: now.Add(-time.Hour), FreshnessStatus: model.MallWeatherFreshnessFresh}
	if fresh, err := mallWeatherRefreshKindFresh(context.Background(), latest, 7, MallWeatherRefreshKindV26Full, now); err != nil || !fresh {
		t.Fatalf("v26 fresh=%v error=%v", fresh, err)
	}
	if latest.lifeSource != "v26_daily" {
		t.Fatalf("life source=%q", latest.lifeSource)
	}
	latest.byKind[model.MallWeatherDataKindDaily] = &model.MallWeatherLatest{FetchedAtUTC: now.Add(-13 * time.Hour), FreshnessStatus: model.MallWeatherFreshnessFresh}
	if fresh, err := mallWeatherRefreshKindFresh(context.Background(), latest, 7, MallWeatherRefreshKindV26Full, now); err != nil || fresh {
		t.Fatalf("stale v26 fresh=%v error=%v", fresh, err)
	}
}

func TestNewMallWeatherManualRefreshOutboxContainsOnlyMinimalPayload(t *testing.T) {
	now := time.Now().UTC()
	row, err := newMallWeatherManualRefreshOutbox(7, "manual:0123456789abcdef0123456789abcdef0123456789abcdef", MallWeatherRefreshKindV26Full, now)
	if err != nil {
		t.Fatalf("newMallWeatherManualRefreshOutbox() error=%v", err)
	}
	if row.TaskType != "mall:weather:manual" || row.QueueName != "weather" ||
		string(row.PayloadJSON) != `{"mall_id":7,"task_window":"manual:0123456789abcdef0123456789abcdef0123456789abcdef","endpoint_kind":"v26_weather"}` {
		t.Fatalf("outbox=%+v", row)
	}
}

type recordingMallPermissionChecker struct {
	allowed   map[string]bool
	requested []string
}

func (checker *recordingMallPermissionChecker) HasPermission(_ context.Context, _ uint, permission string, _ time.Time) (bool, error) {
	checker.requested = append(checker.requested, permission)
	return checker.allowed[permission], nil
}

type fakeMallWeatherRefreshStore struct {
	command  mallWeatherRefreshCommand
	result   *MallWeatherRefreshResult
	replayed bool
	err      error
}

func (store *fakeMallWeatherRefreshStore) Create(_ context.Context, command mallWeatherRefreshCommand) (*MallWeatherRefreshResult, bool, error) {
	store.command = command
	return store.result, store.replayed, store.err
}

type fakeMallWeatherRefreshLatestReader struct {
	byKind     map[string]*model.MallWeatherLatest
	life       *model.MallWeatherLatest
	lifeSource string
}

func (reader *fakeMallWeatherRefreshLatestReader) FindCurrentLatest(_ context.Context, _ uint, kind string) (*model.MallWeatherLatest, error) {
	row := reader.byKind[kind]
	if row == nil {
		return nil, data_dao.ErrMallWeatherLatestNotFound
	}
	return row, nil
}

func (reader *fakeMallWeatherRefreshLatestReader) FindCurrentLatestLifeSource(_ context.Context, _ uint, source string) (*model.MallWeatherLatest, error) {
	reader.lifeSource = source
	if reader.life == nil {
		return nil, data_dao.ErrMallWeatherLatestNotFound
	}
	return reader.life, nil
}
