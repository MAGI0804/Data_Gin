package data_svc

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/job"
	"gin-biz-web-api/model"

	"github.com/google/uuid"
)

func TestMallWeatherExportJobServiceCreatesBoundedSnapshotJob(t *testing.T) {
	profile := storedMallWeatherExportProfile(t, "full_profile", 9)
	profile.Version = 3
	now := time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC)
	estimator := &fakeMallWeatherExportEstimator{rows: 123}
	store := &fakeMallWeatherExportJobStore{result: &MallWeatherExportCreateResult{
		JobID: uuid.NewString(), Status: "PENDING", ProfileID: 9, ProfileVersion: 3,
		EstimatedRows: 123, CreatedBy: 17, CreatedAt: now,
	}}
	service := newTestMallWeatherExportJobService(t, profile, estimator, store, now)
	version := uint64(3)
	result, replayed, err := service.Create(
		context.Background(),
		17,
		"export-key-1234",
		requestbody.MallWeatherExportCreateRequest{
			ProfileID: 9, ExpectedProfileVersion: &version,
			Filters: &requestbody.MallWeatherExportFilters{MallIDs: []uint{7, 7}},
		},
	)
	if err != nil || replayed {
		t.Fatalf("Create() result=%+v replayed=%v error=%v", result, replayed, err)
	}
	if estimator.request.StopAfter != defaultMallWeatherExportMaxEstimatedRows ||
		len(estimator.request.Datasets) != 1 || !estimator.request.Datasets[0].Latest ||
		len(estimator.request.Filter.MallIDs) != 1 || estimator.request.Filter.MallIDs[0] != 7 {
		t.Fatalf("estimate request=%+v", estimator.request)
	}
	command := store.command
	if command.ProfileVersion != 3 || command.EstimatedRows != 123 || len(command.KeyHash) != 64 ||
		len(command.JobIdempotencyHash) != 64 || strings.Contains(string(command.ProfileSnapshotJSON), "export-key") ||
		strings.Contains(command.JobIdempotencyHash, "export-key") {
		t.Fatalf("command=%+v", command)
	}
	var snapshot MallWeatherExportProfileSnapshot
	if err := json.Unmarshal([]byte(command.ProfileSnapshotJSON), &snapshot); err != nil {
		t.Fatalf("decode profile snapshot: %v", err)
	}
	if snapshot.ProfileID != 9 || snapshot.Version != 3 || snapshot.Code != "full_profile" {
		t.Fatalf("snapshot=%+v", snapshot)
	}
}

func TestMallWeatherExportJobServiceRejectsUnboundedHistory(t *testing.T) {
	profile := storedMallWeatherExportProfile(t, "history_profile", 9)
	config := MallWeatherExportProfileConfig{
		TimeZone: "UTC", UnitSystem: "metric", DateFormat: defaultMallWeatherExportDateFormat,
		DateTimeFormat: defaultMallWeatherExportDateTimeFormat, FileNameTemplate: "history.xlsx",
		Datasets: []requestbody.MallWeatherExportDataset{{
			Kind: "fetch_runs", SheetName: "runs",
		}},
	}
	profile.ProfileJSON = mustMallWeatherExportJSON(t, config)
	estimator := &fakeMallWeatherExportEstimator{rows: 1}
	store := &fakeMallWeatherExportJobStore{}
	service := newTestMallWeatherExportJobService(t, profile, estimator, store, time.Now().UTC())
	_, _, err := service.Create(
		context.Background(),
		17,
		"export-key-1234",
		requestbody.MallWeatherExportCreateRequest{ProfileID: 9},
	)
	if !errors.Is(err, ErrMallWeatherExportInvalid) || estimator.calls != 0 || store.calls != 0 {
		t.Fatalf("Create() error=%v estimatorCalls=%d storeCalls=%d", err, estimator.calls, store.calls)
	}
}

func TestMallWeatherExportJobServiceRejectsEstimateOverRuntimeLimit(t *testing.T) {
	profile := storedMallWeatherExportProfile(t, "full_profile", 9)
	estimator := &fakeMallWeatherExportEstimator{rows: 101}
	store := &fakeMallWeatherExportJobStore{}
	service, err := newMallWeatherExportJobService(
		fakeMallWeatherExportProfileReader{row: &profile},
		fakeMallPermissionChecker{allowed: true},
		estimator,
		fakeMallWeatherExportLimitReader{
			value:  `{"maxEstimatedRows":100,"maxRangeDays":30,"estimateTimeoutSeconds":2}`,
			exists: true,
		},
		store,
		time.Now,
	)
	if err != nil {
		t.Fatalf("newMallWeatherExportJobService() error=%v", err)
	}
	_, _, err = service.Create(
		context.Background(),
		17,
		"export-key-1234",
		requestbody.MallWeatherExportCreateRequest{ProfileID: 9},
	)
	if !errors.Is(err, ErrMallWeatherExportTooLarge) || store.calls != 0 {
		t.Fatalf("Create() error=%v storeCalls=%d", err, store.calls)
	}
}

func TestMallWeatherExportJobServiceAuthorizationFailsClosed(t *testing.T) {
	service, err := newMallWeatherExportJobService(
		fakeMallWeatherExportProfileReader{},
		fakeMallPermissionChecker{allowed: false},
		&fakeMallWeatherExportEstimator{},
		fakeMallWeatherExportLimitReader{},
		&fakeMallWeatherExportJobStore{},
		time.Now,
	)
	if err != nil {
		t.Fatalf("newMallWeatherExportJobService() error=%v", err)
	}
	_, _, err = service.Create(
		context.Background(),
		17,
		"export-key-1234",
		requestbody.MallWeatherExportCreateRequest{ProfileID: 9},
	)
	if !errors.Is(err, ErrMallForbidden) {
		t.Fatalf("Create() error=%v, want ErrMallForbidden", err)
	}
}

func TestMallWeatherExportJobServiceRejectsMalformedRuntimeLimits(t *testing.T) {
	service := &MallWeatherExportJobService{
		limits: fakeMallWeatherExportLimitReader{
			value:  `{"maxEstimatedRows":100} {}`,
			exists: true,
		},
	}
	if _, err := service.loadLimits(context.Background()); err == nil {
		t.Fatal("loadLimits() accepted trailing JSON")
	}
}

func TestNewMallWeatherExportOutboxContainsOnlyJobID(t *testing.T) {
	now := time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC)
	jobUUID := uuid.NewString()
	row, err := newMallWeatherExportOutbox(22, jobUUID, now)
	if err != nil {
		t.Fatalf("newMallWeatherExportOutbox() error=%v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(row.PayloadJSON), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if row.TaskType != job.TypeMallWeatherExport || row.QueueName != job.MallExportQueueName ||
		len(payload) != 1 || payload["export_job_id"] != float64(22) || strings.Contains(string(row.PayloadJSON), jobUUID) {
		t.Fatalf("row=%+v payload=%v", row, payload)
	}
}

func newTestMallWeatherExportJobService(
	t *testing.T,
	profile model.MallWeatherExportProfile,
	estimator *fakeMallWeatherExportEstimator,
	store *fakeMallWeatherExportJobStore,
	now time.Time,
) *MallWeatherExportJobService {
	t.Helper()
	service, err := newMallWeatherExportJobService(
		fakeMallWeatherExportProfileReader{row: &profile},
		fakeMallPermissionChecker{allowed: true},
		estimator,
		fakeMallWeatherExportLimitReader{},
		store,
		func() time.Time { return now },
	)
	if err != nil {
		t.Fatalf("newMallWeatherExportJobService() error=%v", err)
	}
	return service
}

func mustMallWeatherExportJSON(t *testing.T, value interface{}) model.JSONText {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error=%v", err)
	}
	return model.JSONText(data)
}

type fakeMallWeatherExportProfileReader struct {
	row *model.MallWeatherExportProfile
	err error
}

func (reader fakeMallWeatherExportProfileReader) FindByID(
	context.Context,
	uint,
) (*model.MallWeatherExportProfile, error) {
	if reader.err != nil {
		return nil, reader.err
	}
	copy := *reader.row
	return &copy, nil
}

type fakeMallWeatherExportEstimator struct {
	request data_dao.MallWeatherExportEstimateRequest
	rows    int64
	err     error
	calls   int
}

func (estimator *fakeMallWeatherExportEstimator) EstimateRows(
	_ context.Context,
	request data_dao.MallWeatherExportEstimateRequest,
) (int64, error) {
	estimator.calls++
	estimator.request = request
	return estimator.rows, estimator.err
}

type fakeMallWeatherExportLimitReader struct {
	value  string
	exists bool
	err    error
}

func (reader fakeMallWeatherExportLimitReader) GetValue(context.Context, string) (string, bool, error) {
	return reader.value, reader.exists, reader.err
}

type fakeMallWeatherExportJobStore struct {
	command  mallWeatherExportCreateCommand
	result   *MallWeatherExportCreateResult
	replayed bool
	err      error
	calls    int
}

func (store *fakeMallWeatherExportJobStore) Create(
	_ context.Context,
	command mallWeatherExportCreateCommand,
) (*MallWeatherExportCreateResult, bool, error) {
	store.calls++
	store.command = command
	return store.result, store.replayed, store.err
}
