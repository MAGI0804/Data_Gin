package data_svc

import (
	"context"
	"errors"
	"testing"
	"time"

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
	if store.saved.Code != "mall_weather_full" || store.saved.CreatedBy != 17 || store.saved.Version != 1 ||
		result.UnitSystem != "imperial" || result.DateFormat != defaultMallWeatherExportDateFormat ||
		len(result.Filters.MallIDs) != 2 || result.Filters.MallIDs[0] != 7 || result.Filters.Cities[0] != "shanghai" ||
		result.Datasets[0].MaxRows != maxMallWeatherExportRows || result.Datasets[0].Latest == nil || !*result.Datasets[0].Latest ||
		result.Datasets[0].Columns[0].Format != "general" || result.Datasets[0].ConditionalFormats[0].BackgroundColor != "#ff0000" {
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
	if _, _, err := service.Save(context.Background(), 17, requestbody.MallWeatherExportProfileSaveRequest{}); !errors.Is(err, ErrMallForbidden) {
		t.Fatalf("Save() error=%v, want ErrMallForbidden", err)
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

type fakeMallWeatherExportProfileStore struct {
	saved *model.MallWeatherExportProfile
	rows  []model.MallWeatherExportProfile
	err   error
}

func (store *fakeMallWeatherExportProfileStore) Save(_ context.Context, row *model.MallWeatherExportProfile, _ *uint64) (bool, error) {
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

func (store *fakeMallWeatherExportProfileStore) List(context.Context, *bool) ([]model.MallWeatherExportProfile, error) {
	return append([]model.MallWeatherExportProfile(nil), store.rows...), store.err
}
