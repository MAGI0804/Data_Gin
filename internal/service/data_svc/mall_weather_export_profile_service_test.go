package data_svc

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/model"
)

func TestMallWeatherExportProfileServiceNormalizesAndSavesVersionedConfig(t *testing.T) {
	now := time.Date(2026, 7, 22, 4, 0, 0, 0, time.UTC)
	highTemperature := 35.0
	store := &fakeMallWeatherExportProfileStore{}
	service, err := newMallWeatherExportProfileService(
		store,
		fakeMallPermissionChecker{allowed: true},
		func() time.Time { return now },
	)
	if err != nil {
		t.Fatalf("newMallWeatherExportProfileService() error=%v", err)
	}
	result, created, err := service.Save(context.Background(), 17, requestbody.MallWeatherExportProfileSaveRequest{
		Code: " Mall_Weather_Full ", Name: " Full export ", TimeZone: "Asia/Shanghai",
		UnitSystem: " IMPERIAL ", FileNameTemplate: "mall_weather_{{date:20060102}}.xlsx",
		Filters: requestbody.MallWeatherExportFilters{
			MallIDs: []uint{9, 7, 9}, Cities: []string{" Shanghai ", "shanghai"},
		},
		Datasets: []requestbody.MallWeatherExportDataset{{
			Kind: "hourly", SheetName: "未来360小时", FreezeHeader: true, AutoFilter: true,
			Columns: []requestbody.MallWeatherExportColumn{{Field: "temperature_c", Title: "温度(℃)", Width: 12}},
			ConditionalFormats: []requestbody.MallWeatherExportConditionalFormat{{
				Field: "temperature_c", Operator: "greater_than", Value: &highTemperature, BackgroundColor: "#FF0000",
			}},
		}},
	})
	if err != nil || !created {
		t.Fatalf("Save() result=%+v created=%v error=%v", result, created, err)
	}
	invalidStoredProfile := store.saved.Code != "mall_weather_full" ||
		store.saved.CreatedBy != 17 || store.saved.Version != 1
	invalidPresentation := result.UnitSystem != "imperial" || result.DateFormat != defaultMallWeatherExportDateFormat
	invalidFilters := len(result.Filters.MallIDs) != 2 || len(result.Filters.Cities) != 1 ||
		result.Filters.MallIDs[0] != 7 ||
		result.Filters.Cities[0] != "shanghai"
	dataset := result.Datasets[0]
	invalidDataset := dataset.MaxRows != maxMallWeatherExportRows || dataset.Latest == nil || !*dataset.Latest ||
		dataset.Columns[0].Format != "general" || dataset.ConditionalFormats[0].BackgroundColor != "#ff0000"
	if invalidStoredProfile || invalidPresentation || invalidFilters || invalidDataset {
		t.Fatalf("saved=%+v result=%+v", store.saved, result)
	}
}

func TestNormalizeMallWeatherExportProfileRejectsUnsafeAndUnknownFields(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*requestbody.MallWeatherExportProfileSaveRequest)
	}{
		{name: "unsafe file name", mutate: func(request *requestbody.MallWeatherExportProfileSaveRequest) {
			request.FileNameTemplate = "../secret.xlsx"
		}},
		{name: "unsafe sheet name", mutate: func(request *requestbody.MallWeatherExportProfileSaveRequest) {
			request.Datasets[0].SheetName = "bad/name"
		}},
		{name: "unknown column", mutate: func(request *requestbody.MallWeatherExportProfileSaveRequest) {
			request.Datasets[0].Columns = []requestbody.MallWeatherExportColumn{{Field: "password", Title: "secret"}}
		}},
		{name: "unknown unit system", mutate: func(request *requestbody.MallWeatherExportProfileSaveRequest) {
			request.UnitSystem = "provider_default"
		}},
		{name: "invalid mall filter", mutate: func(request *requestbody.MallWeatherExportProfileSaveRequest) {
			request.Filters.MallIDs = []uint{0}
		}},
		{name: "condition targets unselected field", mutate: func(request *requestbody.MallWeatherExportProfileSaveRequest) {
			request.Datasets[0].Columns = []requestbody.MallWeatherExportColumn{{Field: "temperature_c", Title: "temperature"}}
			request.Datasets[0].ConditionalFormats = []requestbody.MallWeatherExportConditionalFormat{{
				Field: "humidity_pct", Operator: "equal", Value: float64Pointer(0.5), FontColor: "#000000",
			}}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := validMallWeatherExportProfileRequest()
			test.mutate(&request)
			if _, _, err := normalizeMallWeatherExportProfile(request); !errors.Is(err, ErrMallWeatherExportProfileInvalid) {
				t.Fatalf("normalizeMallWeatherExportProfile() request=%+v error=%v", request, err)
			}
		})
	}
}

func TestMallWeatherExportProfileServiceAuthorizationFailsClosed(t *testing.T) {
	service, err := newMallWeatherExportProfileService(
		&fakeMallWeatherExportProfileStore{},
		fakeMallPermissionChecker{allowed: false},
		time.Now,
	)
	if err != nil {
		t.Fatalf("newMallWeatherExportProfileService() error=%v", err)
	}
	_, _, err = service.Save(context.Background(), 17, requestbody.MallWeatherExportProfileSaveRequest{})
	if !errors.Is(err, ErrMallForbidden) {
		t.Fatalf("Save() error=%v, want ErrMallForbidden", err)
	}
}

func TestMallWeatherExportProfileServiceRejectsReservedFixedCode(t *testing.T) {
	store := &fakeMallWeatherExportProfileStore{}
	service, err := newMallWeatherExportProfileService(
		store,
		fakeMallPermissionChecker{allowed: true},
		time.Now,
	)
	if err != nil {
		t.Fatalf("newMallWeatherExportProfileService() error=%v", err)
	}
	for _, code := range []string{fixedMallWeatherExportProfileCodePrefix, fixedMallWeatherExportProfileCode, fixedMallWeatherExportProfileCodePrefix + "_v2"} {
		request := validMallWeatherExportProfileRequest()
		request.Code = code
		if _, _, err := service.Save(context.Background(), 17, request); !errors.Is(err, ErrMallWeatherExportProfileInvalid) {
			t.Fatalf("Save() code=%q error=%v, want reserved code rejection", code, err)
		}
	}
	if store.saved != nil {
		t.Fatalf("Save() wrote reserved profile: %+v", store.saved)
	}
}

func TestMallWeatherExportProfileServiceListsWithStableCursor(t *testing.T) {
	store := &fakeMallWeatherExportProfileStore{rows: []model.MallWeatherExportProfile{
		storedMallWeatherExportProfile(t, "alpha_profile", 1),
		storedMallWeatherExportProfile(t, "beta_profile", 2),
	}}
	service, err := newMallWeatherExportProfileService(
		store,
		fakeMallPermissionChecker{allowed: true},
		time.Now,
	)
	if err != nil {
		t.Fatalf("newMallWeatherExportProfileService() error=%v", err)
	}
	result, err := service.List(context.Background(), 17, nil, "", 1)
	if err != nil {
		t.Fatalf("List() error=%v", err)
	}
	if len(result.Items) != 1 || result.Items[0].Code != "alpha_profile" || result.Pagination.NextCursor == "" ||
		result.Pagination.PageSize != 1 || store.listQuery.Limit != 2 {
		t.Fatalf("result=%+v query=%+v", result, store.listQuery)
	}
	if _, err := service.List(context.Background(), 17, nil, result.Pagination.NextCursor, 1); err != nil {
		t.Fatalf("List() with cursor error=%v", err)
	}
	if store.listQuery.AfterCode != "alpha_profile" {
		t.Fatalf("query=%+v", store.listQuery)
	}
	_, err = service.List(context.Background(), 17, nil, "not-base64", 1)
	if !errors.Is(err, ErrMallWeatherExportProfileInvalid) {
		t.Fatalf("List() invalid cursor error=%v", err)
	}
}

func float64Pointer(value float64) *float64 {
	return &value
}

func validMallWeatherExportProfileRequest() requestbody.MallWeatherExportProfileSaveRequest {
	return requestbody.MallWeatherExportProfileSaveRequest{
		Code:             "valid_code",
		Name:             "name",
		TimeZone:         "UTC",
		FileNameTemplate: "safe.xlsx",
		Datasets: []requestbody.MallWeatherExportDataset{{
			Kind: "hourly", SheetName: "hourly",
		}},
	}
}

func storedMallWeatherExportProfile(t *testing.T, code string, id uint) model.MallWeatherExportProfile {
	t.Helper()
	config := MallWeatherExportProfileConfig{
		TimeZone:         "UTC",
		UnitSystem:       "metric",
		DateFormat:       defaultMallWeatherExportDateFormat,
		DateTimeFormat:   defaultMallWeatherExportDateTimeFormat,
		FileNameTemplate: "safe.xlsx",
		Datasets:         []requestbody.MallWeatherExportDataset{{Kind: "hourly", SheetName: "hourly"}},
	}
	profileJSON, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("json.Marshal() error=%v", err)
	}
	return model.MallWeatherExportProfile{
		BaseModel:         model.BaseModel{ID: id},
		Code:              code,
		Name:              code,
		Version:           1,
		ProfileJSON:       model.JSONText(profileJSON),
		Enabled:           true,
		WeatherTimestamps: model.WeatherTimestamps{CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}
}

type fakeMallWeatherExportProfileStore struct {
	saved     *model.MallWeatherExportProfile
	rows      []model.MallWeatherExportProfile
	listQuery data_dao.MallWeatherExportProfileListQuery
	err       error
}

func (store *fakeMallWeatherExportProfileStore) Save(
	_ context.Context,
	row *model.MallWeatherExportProfile,
	_ *uint64,
) (bool, error) {
	if store.err != nil {
		return false, store.err
	}
	copy := *row
	copy.BaseModel = model.BaseModel{ID: 9}
	copy.Version = 1
	copy.CreatedBy = row.UpdatedBy
	copy.WeatherTimestamps = model.WeatherTimestamps{CreatedAt: time.Now(), UpdatedAt: time.Now()}
	*row = copy
	store.saved = &copy
	return true, nil
}

func (store *fakeMallWeatherExportProfileStore) List(
	_ context.Context,
	query data_dao.MallWeatherExportProfileListQuery,
) ([]model.MallWeatherExportProfile, error) {
	store.listQuery = query
	return append([]model.MallWeatherExportProfile(nil), store.rows...), store.err
}
