package data_svc

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/credential"

	"github.com/google/uuid"
)

func TestPrepareMallWeatherFeishuExecutionRestoresOnlySnapshotResources(t *testing.T) {
	record, resources := validMallWeatherFeishuExecutionRecord(t)
	prepared, err := prepareMallWeatherFeishuExecution(record, resources)
	if err != nil {
		t.Fatalf("prepareMallWeatherFeishuExecution() error=%v", err)
	}
	if prepared.Destination.DestinationID != 17 || prepared.Destination.Code != "weather_feishu" ||
		prepared.Destination.SpreadsheetToken != "spreadsheet-secret-token" ||
		prepared.Destination.SheetIDs["hourly"] != "hourly-secret-sheet" ||
		prepared.Profile.ID != 9 || prepared.Profile.Version != 3 || prepared.Profile.Code != "mall_weather_full" ||
		len(prepared.Filter.MallIDs) != 1 || prepared.Filter.MallIDs[0] != 7 ||
		!prepared.SnapshotAt.Equal(record.Detail.CreatedAt) {
		t.Fatalf("unexpected prepared execution=%+v", prepared)
	}
	for _, secret := range []string{"spreadsheet-secret-token", "hourly-secret-sheet"} {
		if strings.Contains(string(record.Detail.DestinationSnapshotJSON), secret) {
			t.Fatalf("destination snapshot contains secret %q", secret)
		}
	}
}

func TestPrepareMallWeatherFeishuExecutionRejectsSnapshotDrift(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*data_dao.MallWeatherFeishuRunRecord)
	}{
		{
			name: "destination identity mismatch",
			mutate: func(record *data_dao.MallWeatherFeishuRunRecord) {
				record.Pipeline.DestinationID++
			},
		},
		{
			name: "unknown destination field",
			mutate: func(record *data_dao.MallWeatherFeishuRunRecord) {
				record.Detail.DestinationSnapshotJSON = model.JSONText(strings.TrimSuffix(
					string(record.Detail.DestinationSnapshotJSON),
					"}",
				) + `,"unknown":true}`)
			},
		},
		{
			name: "profile identity mismatch",
			mutate: func(record *data_dao.MallWeatherFeishuRunRecord) {
				record.Detail.ProfileVersion++
			},
		},
		{
			name: "non canonical filters",
			mutate: func(record *data_dao.MallWeatherFeishuRunRecord) {
				record.Detail.FiltersJSON = model.JSONText(`{"mallIds":[7,7]}`)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			record, resources := validMallWeatherFeishuExecutionRecord(t)
			test.mutate(&record)
			if _, err := prepareMallWeatherFeishuExecution(record, resources); err == nil {
				t.Fatal("prepareMallWeatherFeishuExecution() accepted drifted snapshot")
			}
		})
	}
}

func TestPrepareMallWeatherFeishuExecutionFailsClosedWhenResourceIsMissing(t *testing.T) {
	record, _ := validMallWeatherFeishuExecutionRecord(t)
	if _, err := prepareMallWeatherFeishuExecution(
		record,
		fakeMallWeatherFeishuResourceResolver{values: map[string]string{}},
	); err == nil {
		t.Fatal("prepareMallWeatherFeishuExecution() accepted missing resources")
	}
}

func validMallWeatherFeishuExecutionRecord(
	t *testing.T,
) (data_dao.MallWeatherFeishuRunRecord, fakeMallWeatherFeishuResourceResolver) {
	t.Helper()
	profileRequest := requestbody.MallWeatherExportProfileSaveRequest{
		Code: "mall_weather_full", Name: "Mall weather", TimeZone: "UTC", UnitSystem: "metric",
		DateFormat: defaultMallWeatherExportDateFormat, DateTimeFormat: defaultMallWeatherExportDateTimeFormat,
		FileNameTemplate: "mall-weather.xlsx",
		Filters:          requestbody.MallWeatherExportFilters{MallIDs: []uint{7}},
		Datasets:         []requestbody.MallWeatherExportDataset{{Kind: "hourly", SheetName: "Hourly"}},
	}
	_, profileConfig, err := normalizeMallWeatherExportProfile(profileRequest)
	if err != nil {
		t.Fatalf("normalizeMallWeatherExportProfile() error=%v", err)
	}
	profileSnapshot, err := json.Marshal(MallWeatherExportProfileSnapshot{
		ProfileID: 9, Code: profileRequest.Code, Name: profileRequest.Name, Version: 3, Config: profileConfig,
	})
	if err != nil {
		t.Fatalf("json.Marshal(profile snapshot) error=%v", err)
	}
	filters, err := json.Marshal(profileConfig.Filters)
	if err != nil {
		t.Fatalf("json.Marshal(filters) error=%v", err)
	}
	destinationConfigJSON, err := json.Marshal(MallWeatherFeishuDestinationConfig{
		SpreadsheetTokenEnv: credential.EnvFeishuWeatherSpreadsheetToken,
		SheetIDEnvMapping:   map[string]string{"hourly": credential.EnvFeishuWeatherHourlySheetID},
		WriteMode:           "append",
		BatchRows:           200,
		ProfileCode:         profileRequest.Code,
		TimeoutSeconds:      20,
	})
	if err != nil {
		t.Fatalf("json.Marshal(destination config) error=%v", err)
	}
	destinationConfig, err := parseMallWeatherFeishuDestinationConfig(string(destinationConfigJSON))
	if err != nil {
		t.Fatalf("parseMallWeatherFeishuDestinationConfig() error=%v", err)
	}
	destinationSnapshot, err := mallWeatherFeishuDestinationSnapshot(&MallWeatherFeishuResolvedDestination{
		DestinationID: 17,
		Code:          "weather_feishu",
		Config:        destinationConfig,
	})
	if err != nil {
		t.Fatalf("mallWeatherFeishuDestinationSnapshot() error=%v", err)
	}
	createdAt := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	record := data_dao.MallWeatherFeishuRunRecord{
		Pipeline: model.PipelineRun{
			BaseModel: model.BaseModel{ID: 41}, TraceID: uuid.NewString(), DestinationID: 17,
		},
		Detail: model.MallWeatherFeishuRun{
			BaseModel: model.BaseModel{ID: 42}, PipelineRunID: 41, ProfileID: 9, ProfileVersion: 3,
			ProfileSnapshotJSON: model.JSONText(profileSnapshot), FiltersJSON: model.JSONText(filters),
			DestinationSnapshotJSON: model.JSONText(destinationSnapshot),
			WeatherTimestamps:       model.WeatherTimestamps{CreatedAt: createdAt, UpdatedAt: createdAt},
		},
	}
	resources := fakeMallWeatherFeishuResourceResolver{values: map[string]string{
		credential.EnvFeishuWeatherSpreadsheetToken: "spreadsheet-secret-token",
		credential.EnvFeishuWeatherHourlySheetID:    "hourly-secret-sheet",
	}}
	return record, resources
}
