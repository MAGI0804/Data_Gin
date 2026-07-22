package data_svc

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"gin-biz-web-api/connector/feishu"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/credential"
)

func TestMallWeatherFeishuPushServiceDryRunOrchestratesReadOnlyPlan(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC)
	profile := mallWeatherFeishuTestProfile(t, now)
	destination := mallWeatherFeishuTestDestination(t)
	estimator := &fakeMallWeatherExportEstimator{rows: 10}
	sheets := &fakeMallWeatherFeishuSheetsReader{
		metadata: &feishu.SpreadsheetMetadata{
			SpreadsheetToken: "spreadsheet_abc",
			Sheets: []feishu.SheetMetadata{{
				SheetID: "sheet_hourly", Title: "Hourly", Index: 0, ResourceType: "sheet",
				GridProperties: feishu.SheetGridProperties{RowCount: 100, ColumnCount: 10},
			}},
		},
		values: matchedMallWeatherFeishuHeader(),
	}
	service, err := newMallWeatherFeishuPushService(mallWeatherFeishuPushDependencies{
		destinations: fakeMallWeatherFeishuDestinationReader{row: destination},
		profiles:     fakeMallWeatherExportProfileReader{row: profile},
		permissions:  fakeMallPermissionChecker{allowed: true},
		estimator:    estimator,
		limits:       fakeMallWeatherExportLimitReader{},
		resources: fakeMallWeatherFeishuResourceResolver{values: map[string]string{
			credential.EnvFeishuWeatherSpreadsheetToken: "spreadsheet_abc",
			credential.EnvFeishuWeatherHourlySheetID:    "sheet_hourly",
		}},
		newSheets: func() (mallWeatherFeishuSheetsReader, error) { return sheets, nil },
		now:       func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("newMallWeatherFeishuPushService() error=%v", err)
	}
	expectedVersion := uint64(3)
	result, err := service.DryRun(context.Background(), 19, requestbody.MallWeatherFeishuPushRequest{
		DestinationID: 17, ProfileID: 9, ExpectedProfileVersion: &expectedVersion,
		Filters: &requestbody.MallWeatherExportFilters{Cities: []string{" Shanghai "}},
	})
	if err != nil {
		t.Fatalf("DryRun() error=%v", err)
	}
	if result == nil || !result.CanExecute || result.TotalEstimatedRows != 10 || len(result.Datasets) != 1 ||
		estimator.calls != 1 || len(estimator.request.Filter.Cities) != 1 ||
		estimator.request.Filter.Cities[0] != "shanghai" || sheets.inspectCalls != 1 || sheets.readCalls != 1 ||
		sheets.lastRange.SheetID != "sheet_hourly" || sheets.lastRange.EndColumn != 3 {
		t.Fatalf("result=%+v estimator=%+v sheets=%+v", result, estimator, sheets)
	}
}

func TestMallWeatherFeishuPushServiceDryRunFailsClosedBeforeExternalReads(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC)
	profile := mallWeatherFeishuTestProfile(t, now)
	destination := mallWeatherFeishuTestDestination(t)
	estimator := &fakeMallWeatherExportEstimator{rows: 10}
	sheets := &fakeMallWeatherFeishuSheetsReader{}
	service, err := newMallWeatherFeishuPushService(mallWeatherFeishuPushDependencies{
		destinations: fakeMallWeatherFeishuDestinationReader{row: destination},
		profiles:     fakeMallWeatherExportProfileReader{row: profile},
		permissions:  fakeMallPermissionChecker{allowed: true},
		estimator:    estimator,
		limits:       fakeMallWeatherExportLimitReader{},
		resources: fakeMallWeatherFeishuResourceResolver{values: map[string]string{
			credential.EnvFeishuWeatherSpreadsheetToken: "spreadsheet_abc",
			credential.EnvFeishuWeatherHourlySheetID:    "sheet_hourly",
		}},
		newSheets: func() (mallWeatherFeishuSheetsReader, error) { return sheets, nil },
		now:       func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("newMallWeatherFeishuPushService() error=%v", err)
	}
	wrongVersion := uint64(2)
	_, err = service.DryRun(context.Background(), 19, requestbody.MallWeatherFeishuPushRequest{
		DestinationID: 17, ProfileID: 9, ExpectedProfileVersion: &wrongVersion,
	})
	if !errors.Is(err, ErrMallWeatherExportProfileConflict) || estimator.calls != 0 ||
		sheets.inspectCalls != 0 || sheets.readCalls != 0 {
		t.Fatalf("DryRun() error=%v estimator=%+v sheets=%+v", err, estimator, sheets)
	}
}

type fakeMallWeatherFeishuDestinationReader struct {
	row *model.DestinationDefinition
	err error
}

func (reader fakeMallWeatherFeishuDestinationReader) FindByID(
	context.Context,
	uint,
) (*model.DestinationDefinition, error) {
	if reader.err != nil {
		return nil, reader.err
	}
	copy := *reader.row
	return &copy, nil
}

type fakeMallWeatherFeishuSheetsReader struct {
	metadata     *feishu.SpreadsheetMetadata
	values       *feishu.SheetValues
	err          error
	inspectCalls int
	readCalls    int
	lastRange    feishu.SheetRange
}

func (reader *fakeMallWeatherFeishuSheetsReader) Inspect(
	context.Context,
	string,
	[]string,
) (*feishu.SpreadsheetMetadata, error) {
	reader.inspectCalls++
	return reader.metadata, reader.err
}

func (reader *fakeMallWeatherFeishuSheetsReader) ReadRange(
	_ context.Context,
	_ string,
	readRange feishu.SheetRange,
) (*feishu.SheetValues, error) {
	reader.readCalls++
	reader.lastRange = readRange
	return reader.values, reader.err
}

func mallWeatherFeishuTestDestination(t *testing.T) *model.DestinationDefinition {
	t.Helper()
	config, err := json.Marshal(MallWeatherFeishuDestinationConfig{
		SpreadsheetTokenEnv: credential.EnvFeishuWeatherSpreadsheetToken,
		SheetIDEnvMapping:   map[string]string{"hourly": credential.EnvFeishuWeatherHourlySheetID},
		WriteMode:           "upsert",
		ProfileCode:         "mall_weather_full",
		UniqueKeyFields:     map[string][]string{"hourly": {"mall_code", "forecast_time", "issued_at"}},
	})
	if err != nil {
		t.Fatalf("json.Marshal() error=%v", err)
	}
	return &model.DestinationDefinition{
		BaseModel: model.BaseModel{ID: 17}, Code: "weather_feishu", DestinationType: mallWeatherFeishuDestinationType,
		ConfigJSON: string(config), Enabled: true,
	}
}

func mallWeatherFeishuTestProfile(t *testing.T, now time.Time) *model.MallWeatherExportProfile {
	t.Helper()
	config := MallWeatherExportProfileConfig{
		TimeZone: "UTC", UnitSystem: "metric", DateFormat: defaultMallWeatherExportDateFormat,
		DateTimeFormat: defaultMallWeatherExportDateTimeFormat, FileNameTemplate: "safe.xlsx",
		Datasets: []requestbody.MallWeatherExportDataset{{
			Kind: "hourly", SheetName: "Hourly", Columns: []requestbody.MallWeatherExportColumn{
				{Field: "mall_code", Title: "Mall"},
				{Field: "forecast_time", Title: "Forecast"},
				{Field: "issued_at", Title: "Issued"},
			},
		}},
	}
	encoded, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("json.Marshal() error=%v", err)
	}
	return &model.MallWeatherExportProfile{
		BaseModel: model.BaseModel{ID: 9}, Code: "mall_weather_full", Name: "Full", Version: 3,
		ProfileJSON: model.JSONText(encoded), Enabled: true,
		WeatherTimestamps: model.WeatherTimestamps{CreatedAt: now, UpdatedAt: now},
	}
}
