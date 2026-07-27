package data_svc

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/credential"
)

func TestMallWeatherSheetPushOptionServiceListsOnlyMatchedSafeOptions(t *testing.T) {
	t.Parallel()

	validConfig := mallWeatherSheetPushOptionTestConfig(t, "mall_weather_full")
	store := &fakeMallWeatherSheetPushOptionStore{
		destinations: []model.DestinationDefinition{
			{
				BaseModel: model.BaseModel{ID: 7}, Name: " 天气看板 ", Code: " weather_board ",
				DestinationType: mallWeatherFeishuDestinationType, ConfigJSON: validConfig, Enabled: true,
			},
			{
				BaseModel: model.BaseModel{ID: 8}, Name: "配置损坏", Code: "broken",
				DestinationType: mallWeatherFeishuDestinationType, ConfigJSON: `{"profile_code":"mall_weather_full","secret":"leak"}`, Enabled: true,
			},
			{
				BaseModel: model.BaseModel{ID: 9}, Name: "未匹配", Code: "unmatched",
				DestinationType: mallWeatherFeishuDestinationType,
				ConfigJSON:      mallWeatherSheetPushOptionTestConfig(t, "mall_weather_other"), Enabled: true,
			},
			{
				BaseModel: model.BaseModel{ID: 10}, Name: "已停用", Code: "disabled",
				DestinationType: mallWeatherFeishuDestinationType, ConfigJSON: validConfig, Enabled: false,
			},
			{
				BaseModel: model.BaseModel{ID: 11}, Name: "错误类型", Code: "wrong_type",
				DestinationType: "webhook", ConfigJSON: validConfig, Enabled: true,
			},
		},
		profiles: []model.MallWeatherExportProfile{
			{BaseModel: model.BaseModel{ID: 17}, Code: "mall_weather_full", Version: 3, Enabled: true},
			{BaseModel: model.BaseModel{ID: 18}, Code: "mall_weather_other", Version: 2, Enabled: false},
		},
	}
	permissions := &recordingMallWeatherSheetPushOptionPermission{allowed: true}
	now := time.Date(2026, 7, 27, 1, 2, 3, 0, time.UTC)
	service, err := newMallWeatherSheetPushOptionService(store, permissions, func() time.Time { return now })
	if err != nil {
		t.Fatalf("newMallWeatherSheetPushOptionService() error=%v", err)
	}

	result, err := service.List(context.Background(), 19)
	if err != nil {
		t.Fatalf("List() error=%v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("items=%+v", result.Items)
	}
	item := result.Items[0]
	if item.DestinationID != 7 || item.Name != "天气看板" || item.Code != "weather_board" ||
		item.ProfileID != 17 || item.ProfileCode != "mall_weather_full" || item.ProfileVersion != 3 {
		t.Fatalf("item=%+v", item)
	}
	if permissions.actorUserID != 19 || permissions.permission != PermissionWeatherFeishuPush ||
		!permissions.at.Equal(now) || store.destinationCalls != 1 || store.profileCalls != 1 {
		t.Fatalf("permissions=%+v store=%+v", permissions, store)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal() error=%v", err)
	}
	for _, forbidden := range []string{"config", "secret", "token", "sheetIdEnvMapping", "enabled", "destinationType"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("response leaked %q: %s", forbidden, encoded)
		}
	}
}

func TestMallWeatherSheetPushOptionServiceAuthorizesBeforeStoreReads(t *testing.T) {
	t.Parallel()

	store := &fakeMallWeatherSheetPushOptionStore{}
	service, err := newMallWeatherSheetPushOptionService(
		store,
		&recordingMallWeatherSheetPushOptionPermission{allowed: false},
		time.Now,
	)
	if err != nil {
		t.Fatalf("newMallWeatherSheetPushOptionService() error=%v", err)
	}
	if _, err := service.List(context.Background(), 19); !errors.Is(err, ErrMallForbidden) {
		t.Fatalf("List() error=%v, want forbidden", err)
	}
	if store.destinationCalls != 0 || store.profileCalls != 0 {
		t.Fatalf("unauthorized request read store: %+v", store)
	}
}

func TestMallWeatherSheetPushOptionServiceReturnsInitializedEmptyItems(t *testing.T) {
	t.Parallel()

	store := &fakeMallWeatherSheetPushOptionStore{destinations: []model.DestinationDefinition{{
		BaseModel: model.BaseModel{ID: 7}, Name: "损坏配置", Code: "broken",
		DestinationType: mallWeatherFeishuDestinationType, ConfigJSON: `{}`, Enabled: true,
	}}}
	service, err := newMallWeatherSheetPushOptionService(
		store,
		&recordingMallWeatherSheetPushOptionPermission{allowed: true},
		time.Now,
	)
	if err != nil {
		t.Fatalf("newMallWeatherSheetPushOptionService() error=%v", err)
	}
	result, err := service.List(context.Background(), 19)
	if err != nil {
		t.Fatalf("List() error=%v", err)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal() error=%v", err)
	}
	if string(encoded) != `{"items":[]}` || store.profileCalls != 0 {
		t.Fatalf("response=%s store=%+v", encoded, store)
	}
}

func TestMallWeatherSheetPushOptionServiceFailsClosedAtBound(t *testing.T) {
	t.Parallel()

	destinations := make([]model.DestinationDefinition, maxMallWeatherSheetPushOptions+1)
	store := &fakeMallWeatherSheetPushOptionStore{destinations: destinations}
	service, err := newMallWeatherSheetPushOptionService(
		store,
		&recordingMallWeatherSheetPushOptionPermission{allowed: true},
		time.Now,
	)
	if err != nil {
		t.Fatalf("newMallWeatherSheetPushOptionService() error=%v", err)
	}
	if _, err := service.List(context.Background(), 19); !errors.Is(err, ErrMallWeatherSheetPushOptionsUnavailable) {
		t.Fatalf("List() error=%v, want unavailable", err)
	}
	if store.destinationLimit != maxMallWeatherSheetPushOptions+1 || store.profileCalls != 0 {
		t.Fatalf("store=%+v", store)
	}
}

type fakeMallWeatherSheetPushOptionStore struct {
	destinations     []model.DestinationDefinition
	profiles         []model.MallWeatherExportProfile
	destinationErr   error
	profileErr       error
	destinationCalls int
	profileCalls     int
	destinationLimit int
}

func (store *fakeMallWeatherSheetPushOptionStore) ListEnabledDestinations(
	_ context.Context,
	destinationType string,
	limit int,
) ([]model.DestinationDefinition, error) {
	store.destinationCalls++
	store.destinationLimit = limit
	if destinationType != mallWeatherFeishuDestinationType {
		return nil, errors.New("unexpected destination type")
	}
	return store.destinations, store.destinationErr
}

func (store *fakeMallWeatherSheetPushOptionStore) ListEnabledProfilesByCodes(
	_ context.Context,
	codes []string,
) ([]model.MallWeatherExportProfile, error) {
	store.profileCalls++
	if len(codes) == 0 {
		return nil, errors.New("empty profile codes")
	}
	return store.profiles, store.profileErr
}

type recordingMallWeatherSheetPushOptionPermission struct {
	allowed     bool
	err         error
	actorUserID uint
	permission  string
	at          time.Time
}

func (checker *recordingMallWeatherSheetPushOptionPermission) HasPermission(
	_ context.Context,
	actorUserID uint,
	permission string,
	at time.Time,
) (bool, error) {
	checker.actorUserID = actorUserID
	checker.permission = permission
	checker.at = at
	return checker.allowed, checker.err
}

func mallWeatherSheetPushOptionTestConfig(t *testing.T, profileCode string) string {
	t.Helper()
	encoded, err := json.Marshal(MallWeatherFeishuDestinationConfig{
		SpreadsheetTokenEnv: credential.EnvFeishuWeatherSpreadsheetToken,
		SheetIDEnvMapping: map[string]string{
			"hourly": credential.EnvFeishuWeatherHourlySheetID,
		},
		WriteMode:   "append",
		ProfileCode: profileCode,
	})
	if err != nil {
		t.Fatalf("json.Marshal() error=%v", err)
	}
	return string(encoded)
}
