package data_svc

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"gin-biz-web-api/connector/feishu"
	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/job"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/credential"

	"github.com/google/uuid"
)

func TestNewMallWeatherFeishuOutboxContainsOnlyPipelineRunID(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 23, 8, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	traceID := uuid.NewString()
	row, err := newMallWeatherFeishuOutbox(22, traceID, now)
	if err != nil {
		t.Fatalf("newMallWeatherFeishuOutbox() error=%v", err)
	}
	if row.TaskKey != "mall:weather:feishu:"+traceID || row.TaskType != job.TypeMallWeatherFeishu ||
		row.QueueName != job.MallDeliveryQueueName || string(row.PayloadJSON) != `{"pipeline_run_id":22}` ||
		!row.AvailableAt.Equal(now.UTC()) {
		t.Fatalf("outbox=%+v", row)
	}
	lowerPayload := strings.ToLower(string(row.PayloadJSON))
	for _, forbidden := range []string{"secret", "token", "sheet", "profile", "filter", "url"} {
		if strings.Contains(lowerPayload, forbidden) {
			t.Fatalf("payload contains forbidden marker %q: %s", forbidden, row.PayloadJSON)
		}
	}
	for _, invalid := range []struct {
		runID uint
		trace string
		at    time.Time
	}{
		{runID: 0, trace: traceID, at: now},
		{runID: 22, trace: "not-a-uuid", at: now},
		{runID: 22, trace: traceID},
	} {
		if _, err := newMallWeatherFeishuOutbox(invalid.runID, invalid.trace, invalid.at); err == nil {
			t.Fatalf("newMallWeatherFeishuOutbox(%+v) accepted invalid identity", invalid)
		}
	}
}

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
		store:        &fakeMallWeatherFeishuPushStore{},
		runs:         fakeMallWeatherFeishuRunReader{},
		resources: fakeMallWeatherFeishuResourceResolver{values: map[string]string{
			credential.EnvFeishuWeatherSpreadsheetToken: "spreadsheet_abc",
			credential.EnvFeishuWeatherHourlySheetID:    "sheet_hourly",
		}},
		newSheets: func() (mallWeatherFeishuSheetsReader, error) { return sheets, nil },
		feishuEnabled: func() bool {
			return true
		},
		now: func() time.Time { return now },
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
		store:        &fakeMallWeatherFeishuPushStore{},
		runs:         fakeMallWeatherFeishuRunReader{},
		resources: fakeMallWeatherFeishuResourceResolver{values: map[string]string{
			credential.EnvFeishuWeatherSpreadsheetToken: "spreadsheet_abc",
			credential.EnvFeishuWeatherHourlySheetID:    "sheet_hourly",
		}},
		newSheets: func() (mallWeatherFeishuSheetsReader, error) { return sheets, nil },
		feishuEnabled: func() bool {
			return true
		},
		now: func() time.Time { return now },
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

func TestMallWeatherFeishuPushServiceCreatePersistsOnlySafeSnapshots(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 23, 8, 0, 0, 0, time.UTC)
	profile := mallWeatherFeishuTestProfile(t, now)
	destination := mallWeatherFeishuTestDestination(t)
	store := &fakeMallWeatherFeishuPushStore{result: &MallWeatherFeishuPushCreateResult{
		RunID: 41, TraceID: uuid.NewString(), Status: "PENDING", DestinationID: destination.ID,
		ProfileID: profile.ID, ProfileVersion: profile.Version, EstimatedRows: 10, CreatedBy: 19, CreatedAt: now,
	}}
	sheetsCalls := 0
	service, err := newMallWeatherFeishuPushService(mallWeatherFeishuPushDependencies{
		destinations: fakeMallWeatherFeishuDestinationReader{row: destination},
		profiles:     fakeMallWeatherExportProfileReader{row: profile},
		permissions:  fakeMallPermissionChecker{allowed: true},
		estimator:    &fakeMallWeatherExportEstimator{rows: 10},
		limits:       fakeMallWeatherExportLimitReader{},
		store:        store,
		runs:         fakeMallWeatherFeishuRunReader{},
		resources: fakeMallWeatherFeishuResourceResolver{values: map[string]string{
			credential.EnvFeishuWeatherSpreadsheetToken: "spreadsheet_private_value",
			credential.EnvFeishuWeatherHourlySheetID:    "sheet_private_value",
		}},
		newSheets: func() (mallWeatherFeishuSheetsReader, error) {
			sheetsCalls++
			return nil, errors.New("must not be called")
		},
		feishuEnabled: func() bool {
			return true
		},
		now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("newMallWeatherFeishuPushService() error=%v", err)
	}
	expectedVersion := profile.Version
	result, replayed, err := service.Create(
		context.Background(),
		19,
		"feishu-create-request-1",
		requestbody.MallWeatherFeishuPushRequest{
			DestinationID: destination.ID, ProfileID: profile.ID, ExpectedProfileVersion: &expectedVersion,
			Filters: &requestbody.MallWeatherExportFilters{Cities: []string{" Shanghai "}},
		},
	)
	if err != nil || replayed || result != store.result || store.calls != 1 || sheetsCalls != 0 {
		t.Fatalf("Create() result=%+v replayed=%t error=%v store=%+v sheetsCalls=%d", result, replayed, err, store, sheetsCalls)
	}
	command := store.command
	if command.ActorUserID != 19 || command.DestinationID != destination.ID || command.ProfileID != profile.ID ||
		command.ProfileVersion != profile.Version || command.EstimatedRows != 10 || command.RequestedAt != now ||
		len(command.KeyHash) != 64 || strings.Contains(command.KeyHash, "feishu-create-request-1") ||
		len(command.RequestHash) != 64 || uuid.Validate(command.TraceID) != nil {
		t.Fatalf("command identity=%+v", command)
	}
	allSnapshots := string(command.ProfileSnapshotJSON) + string(command.FiltersJSON) + string(command.DestinationSnapshotJSON)
	for _, secret := range []string{"spreadsheet_private_value", "sheet_private_value"} {
		if strings.Contains(allSnapshots, secret) {
			t.Fatalf("stored snapshots leak resolved resource %q: %s", secret, allSnapshots)
		}
	}
	if !strings.Contains(string(command.FiltersJSON), `"shanghai"`) ||
		!strings.Contains(string(command.DestinationSnapshotJSON), credential.EnvFeishuWeatherSpreadsheetToken) ||
		!strings.Contains(string(command.DestinationSnapshotJSON), credential.EnvFeishuWeatherHourlySheetID) {
		t.Fatalf("stored snapshots are incomplete: %s", allSnapshots)
	}
}

func TestMallWeatherFeishuPushServiceDisabledFailsClosedBeforeExternalReads(t *testing.T) {
	t.Parallel()

	sheetsCalls := 0
	store := &fakeMallWeatherFeishuPushStore{}
	service, err := newMallWeatherFeishuPushService(mallWeatherFeishuPushDependencies{
		destinations: fakeMallWeatherFeishuDestinationReader{},
		profiles:     fakeMallWeatherExportProfileReader{},
		permissions:  fakeMallPermissionChecker{allowed: true},
		estimator:    &fakeMallWeatherExportEstimator{rows: 10},
		limits:       fakeMallWeatherExportLimitReader{},
		store:        store,
		runs:         fakeMallWeatherFeishuRunReader{},
		resources:    fakeMallWeatherFeishuResourceResolver{},
		newSheets: func() (mallWeatherFeishuSheetsReader, error) {
			sheetsCalls++
			return &fakeMallWeatherFeishuSheetsReader{}, nil
		},
		feishuEnabled: func() bool {
			return false
		},
		now: func() time.Time {
			return time.Date(2026, 7, 23, 8, 0, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("newMallWeatherFeishuPushService() error=%v", err)
	}

	request := requestbody.MallWeatherFeishuPushRequest{DestinationID: 17, ProfileID: 9}
	if _, err := service.DryRun(context.Background(), 19, request); !errors.Is(err, ErrMallWeatherFeishuDisabled) {
		t.Fatalf("DryRun() error=%v, want disabled", err)
	}
	if _, _, err := service.Create(context.Background(), 19, "feishu-create-request-1", request); !errors.Is(err, ErrMallWeatherFeishuDisabled) {
		t.Fatalf("Create() error=%v, want disabled", err)
	}
	if sheetsCalls != 0 || store.calls != 0 {
		t.Fatalf("disabled service performed external work: sheetsCalls=%d store=%+v", sheetsCalls, store)
	}
}

func TestMallWeatherFeishuPushServiceGetReturnsOwnedSafeRun(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 23, 8, 0, 0, 0, time.UTC)
	record := &data_dao.MallWeatherFeishuRunRecord{
		Pipeline: model.PipelineRun{
			BaseModel: model.BaseModel{ID: 41}, TraceID: uuid.NewString(), RunType: "delivery",
			TriggerType: "api", DestinationID: 17, Status: "running",
		},
		Detail: model.MallWeatherFeishuRun{
			BaseModel: model.BaseModel{ID: 42}, PipelineRunID: 41, ProfileID: 9, ProfileVersion: 3,
			ProfileSnapshotJSON:     model.JSONText(`{"private":"profile"}`),
			FiltersJSON:             model.JSONText(`{"private":"filters"}`),
			DestinationSnapshotJSON: model.JSONText(`{"private":"destination"}`),
			RunToken:                "private-run-token", CreatedBy: 19,
			WeatherTimestamps: model.WeatherTimestamps{CreatedAt: now, UpdatedAt: now},
		},
	}
	service := &MallWeatherFeishuPushService{
		permissions: fakeMallPermissionChecker{allowed: true},
		runs:        fakeMallWeatherFeishuRunReader{record: record},
		now:         func() time.Time { return now },
	}
	result, err := service.Get(context.Background(), 19, 41)
	if err != nil || result == nil || result.Status != "PENDING" || result.RunID != 41 ||
		result.ProfileID != 9 || result.DestinationID != 17 || result.StartedAt != nil {
		t.Fatalf("Get() result=%+v error=%v", result, err)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal() error=%v", err)
	}
	for _, forbidden := range []string{"private", "run-token", "profileSnapshot", "filters"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("query response leaked %q: %s", forbidden, encoded)
		}
	}
	if _, err := service.Get(context.Background(), 20, 41); !errors.Is(err, data_dao.ErrMallWeatherFeishuRunNotFound) {
		t.Fatalf("Get() ownership error=%v", err)
	}
}

type fakeMallWeatherFeishuDestinationReader struct {
	row *model.DestinationDefinition
	err error
}

type fakeMallWeatherFeishuPushStore struct {
	command  mallWeatherFeishuPushCreateCommand
	result   *MallWeatherFeishuPushCreateResult
	replayed bool
	err      error
	calls    int
}

type fakeMallWeatherFeishuRunReader struct {
	record *data_dao.MallWeatherFeishuRunRecord
	err    error
}

func (reader fakeMallWeatherFeishuRunReader) FindByPipelineRunID(
	context.Context,
	uint,
) (*data_dao.MallWeatherFeishuRunRecord, error) {
	if reader.err != nil {
		return nil, reader.err
	}
	return reader.record, nil
}

func (store *fakeMallWeatherFeishuPushStore) Create(
	_ context.Context,
	command mallWeatherFeishuPushCreateCommand,
) (*MallWeatherFeishuPushCreateResult, bool, error) {
	store.calls++
	store.command = command
	return store.result, store.replayed, store.err
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
