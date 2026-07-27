package data_svc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/job"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/storage"

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

func TestMallWeatherExportJobServiceCreatesWithFixedCompleteProfile(t *testing.T) {
	profile, err := fixedMallWeatherExportProfile()
	if err != nil {
		t.Fatalf("fixedMallWeatherExportProfile() error=%v", err)
	}
	profile.BaseModel = model.BaseModel{ID: 29}
	profile.Version = 4
	profile.WeatherTimestamps = model.WeatherTimestamps{CreatedAt: time.Now(), UpdatedAt: time.Now()}
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	estimator := &fakeMallWeatherExportEstimator{rowsByKind: map[string]int64{
		"malls": 1, "realtime": 1, "minutely": 120, "hourly": 360,
		"daily": 15, "alerts": 0, "life_indices": 60,
	}}
	store := &fakeMallWeatherExportJobStore{result: &MallWeatherExportCreateResult{
		JobID: uuid.NewString(), Status: "PENDING", ProfileID: 29, ProfileVersion: 4,
		EstimatedRows: 557, CreatedBy: 17, CreatedAt: now,
	}}
	profiles := &fakeMallWeatherExportProfileReader{systemRow: profile}
	service, err := newMallWeatherExportJobService(
		profiles,
		fakeMallPermissionChecker{allowed: true},
		estimator,
		&fakeMallWeatherExportJobReader{},
		fakeMallWeatherExportLimitReader{},
		store,
		func() time.Time { return now },
	)
	if err != nil {
		t.Fatalf("newMallWeatherExportJobService() error=%v", err)
	}
	result, replayed, err := service.Create(
		context.Background(),
		17,
		"fixed-export-key-1234",
		requestbody.MallWeatherExportCreateRequest{
			Filters: &requestbody.MallWeatherExportFilters{MallIDs: []uint{7}},
		},
	)
	if err != nil || replayed || result.ProfileID != 29 || result.ProfileVersion != 4 {
		t.Fatalf("Create() result=%+v replayed=%v error=%v", result, replayed, err)
	}
	if profiles.ensureCalls != 1 || store.command.ProfileID != 29 || store.command.EstimatedRows != 557 ||
		len(estimator.requests) != 7 || len(estimator.request.Filter.MallIDs) != 1 {
		t.Fatalf("profiles=%+v command=%+v estimates=%+v", profiles, store.command, estimator.requests)
	}
	var snapshot MallWeatherExportProfileSnapshot
	if err := json.Unmarshal([]byte(store.command.ProfileSnapshotJSON), &snapshot); err != nil {
		t.Fatalf("decode fixed profile snapshot: %v", err)
	}
	if snapshot.Code != fixedMallWeatherExportProfileCode || len(snapshot.Config.Datasets) != 7 {
		t.Fatalf("snapshot=%+v", snapshot)
	}
}

func TestMallWeatherExportFixedProfileUsesCurrentForecastWindows(t *testing.T) {
	snapshot := time.Date(2026, 7, 27, 12, 34, 56, 0, time.UTC)
	latest := true
	profile := MallWeatherExportProfileDTO{
		Code: fixedMallWeatherExportProfileCode, TimeZone: "Asia/Shanghai",
		Datasets: []requestbody.MallWeatherExportDataset{
			{Kind: "realtime", Latest: &latest},
			{Kind: "minutely", Latest: &latest},
			{Kind: "hourly", Latest: &latest},
			{Kind: "daily", Latest: &latest},
			{Kind: "life_indices", Latest: &latest},
		},
	}
	requests, err := mallWeatherExportEstimateRequests(
		profile,
		requestbody.MallWeatherExportFilters{MallIDs: []uint{7}},
		1000,
		snapshot,
	)
	if err != nil {
		t.Fatalf("mallWeatherExportEstimateRequests() error=%v", err)
	}
	if len(requests) != 5 {
		t.Fatalf("requests=%+v", requests)
	}
	byKind := make(map[string]data_dao.MallWeatherExportEstimateRequest, len(requests))
	for _, request := range requests {
		if len(request.Datasets) != 1 || request.Datasets[0].AsOfUTC == nil ||
			!request.Datasets[0].AsOfUTC.Equal(snapshot) {
			t.Fatalf("request=%+v", request)
		}
		byKind[request.Datasets[0].Kind] = request
	}
	minutely := byKind["minutely"].Filter
	if minutely.StartUTC == nil || minutely.EndUTC == nil ||
		!minutely.StartUTC.Equal(time.Date(2026, 7, 27, 12, 34, 0, 0, time.UTC)) ||
		!minutely.EndUTC.Equal(time.Date(2026, 7, 27, 14, 34, 0, 0, time.UTC)) {
		t.Fatalf("minutely filter=%+v", minutely)
	}
	hourly := byKind["hourly"].Filter
	if hourly.StartUTC == nil || hourly.EndUTC == nil ||
		!hourly.StartUTC.Equal(time.Date(2026, 7, 27, 13, 0, 0, 0, time.UTC)) ||
		!hourly.EndUTC.Equal(time.Date(2026, 8, 11, 13, 0, 0, 0, time.UTC)) {
		t.Fatalf("hourly filter=%+v", hourly)
	}
	for _, kind := range []string{"daily", "life_indices"} {
		filter := byKind[kind].Filter
		if filter.StartDate != "2026-07-27" || filter.EndDate != "2026-08-11" {
			t.Fatalf("kind=%s filter=%+v", kind, filter)
		}
	}
	if filter := byKind["realtime"].Filter; mallWeatherExportFilterHasRange(filter) {
		t.Fatalf("realtime filter=%+v", filter)
	}
}

func TestMallWeatherExportFixedProfilePreservesExplicitRange(t *testing.T) {
	latest := true
	profile := MallWeatherExportProfileDTO{
		Code: fixedMallWeatherExportProfileCode, TimeZone: "Asia/Shanghai",
		Datasets: []requestbody.MallWeatherExportDataset{
			{Kind: "hourly", Latest: &latest},
			{Kind: "daily", Latest: &latest},
		},
	}
	requests, err := mallWeatherExportEstimateRequests(
		profile,
		requestbody.MallWeatherExportFilters{
			MallIDs: []uint{7},
			Start:   "2026-07-01T01:02:03+08:00",
			End:     "2026-07-03T04:05:06+08:00",
		},
		1000,
		time.Date(2026, 7, 27, 12, 34, 56, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("mallWeatherExportEstimateRequests() error=%v", err)
	}
	if len(requests) != 1 || len(requests[0].Datasets) != 2 {
		t.Fatalf("requests=%+v", requests)
	}
	filter := requests[0].Filter
	wantStart := time.Date(2026, 6, 30, 17, 2, 3, 0, time.UTC)
	wantEnd := time.Date(2026, 7, 2, 20, 5, 6, 0, time.UTC)
	if filter.StartUTC == nil || filter.EndUTC == nil ||
		!filter.StartUTC.Equal(wantStart) || !filter.EndUTC.Equal(wantEnd) ||
		filter.StartDate != "2026-07-01" || filter.EndDate != "2026-07-03" {
		t.Fatalf("filter=%+v", filter)
	}
}

func TestMallWeatherExportJobServiceRejectsInvalidFixedProfileSelectors(t *testing.T) {
	version := uint64(1)
	requests := []requestbody.MallWeatherExportCreateRequest{
		{},
		{ExpectedProfileVersion: &version, Filters: &requestbody.MallWeatherExportFilters{MallIDs: []uint{7}}},
		{Filters: &requestbody.MallWeatherExportFilters{MallIDs: []uint{7, 8}}},
	}
	for _, request := range requests {
		service := &MallWeatherExportJobService{
			profiles: &fakeMallWeatherExportProfileReader{}, permissions: fakeMallPermissionChecker{allowed: true},
			estimator: &fakeMallWeatherExportEstimator{}, limits: fakeMallWeatherExportLimitReader{},
			store: &fakeMallWeatherExportJobStore{}, now: time.Now,
		}
		_, _, err := service.Create(context.Background(), 17, "fixed-export-key-1234", request)
		if !errors.Is(err, ErrMallWeatherExportInvalid) {
			t.Fatalf("Create() request=%+v error=%v", request, err)
		}
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
		&fakeMallWeatherExportProfileReader{row: &profile},
		fakeMallPermissionChecker{allowed: true},
		estimator,
		&fakeMallWeatherExportJobReader{},
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
		&fakeMallWeatherExportProfileReader{},
		fakeMallPermissionChecker{allowed: false},
		&fakeMallWeatherExportEstimator{},
		&fakeMallWeatherExportJobReader{},
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

func TestMallWeatherExportJobServiceGetsActorScopedSafeDTO(t *testing.T) {
	now := time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC)
	jobUUID := uuid.NewString()
	jobs := fakeMallWeatherExportJobReader{row: &model.MallWeatherExportJob{
		BaseModel: model.BaseModel{ID: 22}, JobUUID: jobUUID, ProfileID: 9, ProfileVersion: 3,
		Status: "running", TotalRows: 123, ProcessedRows: 45, CurrentSheet: "hourly",
		ResultObjectKey: "weather-exports/secret.xlsx", CreatedBy: 17,
		WeatherTimestamps: model.WeatherTimestamps{CreatedAt: now, UpdatedAt: now},
	}}
	service, err := newMallWeatherExportJobService(
		&fakeMallWeatherExportProfileReader{},
		fakeMallPermissionChecker{allowed: true},
		&fakeMallWeatherExportEstimator{},
		&jobs,
		fakeMallWeatherExportLimitReader{},
		&fakeMallWeatherExportJobStore{},
		time.Now,
	)
	if err != nil {
		t.Fatalf("newMallWeatherExportJobService() error=%v", err)
	}
	result, err := service.Get(context.Background(), 17, jobUUID)
	if err != nil {
		t.Fatalf("Get() error=%v", err)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal() error=%v", err)
	}
	if result.Status != "RUNNING" || result.ProcessedRows != 45 || jobs.actorUserID != 17 ||
		strings.Contains(string(encoded), "weather-exports/secret.xlsx") {
		t.Fatalf("result=%+v encoded=%s actor=%d", result, encoded, jobs.actorUserID)
	}
}

func TestMallWeatherExportJobServiceSignsActorScopedDownload(t *testing.T) {
	now := time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC)
	jobUUID := uuid.NewString()
	expiresAt := now.Add(10 * time.Minute)
	jobs := &fakeMallWeatherExportJobReader{download: &data_dao.MallWeatherExportDownloadJob{
		Status: "succeeded", ResultObjectKey: "mall-weather-exports/job/result.xlsx",
		FileSizeBytes: 4096, ExpiresAt: &expiresAt,
	}}
	service, err := newMallWeatherExportJobService(
		&fakeMallWeatherExportProfileReader{},
		fakeMallPermissionChecker{allowed: true},
		&fakeMallWeatherExportEstimator{},
		jobs,
		fakeMallWeatherExportLimitReader{},
		&fakeMallWeatherExportJobStore{},
		func() time.Time { return now },
	)
	if err != nil {
		t.Fatalf("newMallWeatherExportJobService() error=%v", err)
	}
	signer := &fakeMallWeatherExportDownloadSigner{url: "https://signed.example/result", objectSize: 4096}
	service.newSigner = func() (mallWeatherExportDownloadSigner, error) { return signer, nil }
	service.statTimeout = time.Second
	result, err := service.Download(t.Context(), 17, jobUUID)
	if err != nil {
		t.Fatalf("Download() error=%v", err)
	}
	if result.URL != signer.url || !result.ExpiresAt.Equal(now.Add(5*time.Minute)) ||
		jobs.actorUserID != 17 || signer.statObjectKey != jobs.download.ResultObjectKey ||
		signer.objectKey != jobs.download.ResultObjectKey || signer.expires != 5*time.Minute {
		t.Fatalf("result=%+v signer=%+v actor=%d", result, signer, jobs.actorUserID)
	}
	encoded, err := json.Marshal(result)
	if err != nil || strings.Contains(string(encoded), "mall-weather-exports") {
		t.Fatalf("encoded=%s error=%v", encoded, err)
	}
}

func TestMallWeatherExportJobServiceOpensActorScopedDownloadContent(t *testing.T) {
	now := time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC)
	jobUUID := uuid.NewString()
	expiresAt := now.Add(10 * time.Minute)
	jobs := &fakeMallWeatherExportJobReader{download: &data_dao.MallWeatherExportDownloadJob{
		Status: "succeeded", ResultObjectKey: "mall-weather-exports/job/result.xlsx",
		FileSizeBytes: 8, ExpiresAt: &expiresAt,
	}}
	service, err := newMallWeatherExportJobService(
		&fakeMallWeatherExportProfileReader{},
		fakeMallPermissionChecker{allowed: true},
		&fakeMallWeatherExportEstimator{},
		jobs,
		fakeMallWeatherExportLimitReader{},
		&fakeMallWeatherExportJobStore{},
		func() time.Time { return now },
	)
	if err != nil {
		t.Fatalf("newMallWeatherExportJobService() error=%v", err)
	}
	signer := &fakeMallWeatherExportDownloadSigner{downloadBody: "PK\x03\x04xlsx", objectSize: 8}
	service.newSigner = func() (mallWeatherExportDownloadSigner, error) { return signer, nil }

	result, err := service.OpenDownloadContent(t.Context(), 17, jobUUID)
	if err != nil {
		t.Fatalf("OpenDownloadContent() error=%v", err)
	}
	defer result.Body.Close()
	body, err := io.ReadAll(result.Body)
	if err != nil || string(body) != signer.downloadBody || result.Size != 8 ||
		result.FileName != "mall_weather_export_"+jobUUID+".xlsx" ||
		signer.openObjectKey != jobs.download.ResultObjectKey || jobs.actorUserID != 17 {
		t.Fatalf("result=%+v body=%q signer=%+v actor=%d error=%v", result, body, signer, jobs.actorUserID, err)
	}
}

func TestMallWeatherExportJobServiceRejectsInvalidDownloadContent(t *testing.T) {
	now := time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC)
	expiresAt := now.Add(10 * time.Minute)
	tests := []struct {
		name       string
		body       string
		objectSize int64
		openErr    error
		want       error
	}{
		{name: "object missing", openErr: storage.ErrOSSObjectNotFound, want: ErrMallWeatherExportArtifactMissing},
		{name: "storage unavailable", openErr: errors.New("access denied"), want: ErrMallWeatherExportStorageUnavailable},
		{name: "size mismatch", body: "PK\x03\x04bad", objectSize: 7, want: ErrMallWeatherExportArtifactMissing},
		{name: "invalid XLSX header", body: "NOPEdata", objectSize: 8, want: ErrMallWeatherExportArtifactMissing},
		{name: "truncated content", body: "PK", objectSize: 8, want: ErrMallWeatherExportArtifactMissing},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signer := &fakeMallWeatherExportDownloadSigner{
				downloadBody: tt.body, objectSize: tt.objectSize, openErr: tt.openErr,
			}
			service := &MallWeatherExportJobService{
				permissions: fakeMallPermissionChecker{allowed: true},
				jobs: &fakeMallWeatherExportJobReader{download: &data_dao.MallWeatherExportDownloadJob{
					Status: "succeeded", ResultObjectKey: "mall-weather-exports/job/result.xlsx",
					FileSizeBytes: 8, ExpiresAt: &expiresAt,
				}},
				newSigner:   func() (mallWeatherExportDownloadSigner, error) { return signer, nil },
				downloadTTL: 5 * time.Minute,
				now:         func() time.Time { return now },
			}
			result, err := service.OpenDownloadContent(t.Context(), 17, uuid.NewString())
			if result != nil || !errors.Is(err, tt.want) {
				t.Fatalf("OpenDownloadContent() result=%+v error=%v, want %v", result, err, tt.want)
			}
		})
	}
}

func TestMallWeatherExportJobServiceRejectsUnavailableDownload(t *testing.T) {
	now := time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC)
	jobUUID := uuid.NewString()
	tests := []struct {
		name   string
		status string
		expiry time.Time
		want   error
	}{
		{name: "still running", status: "running", expiry: now.Add(time.Hour), want: ErrMallWeatherExportNotReady},
		{name: "expired status", status: "expired", expiry: now.Add(time.Hour), want: ErrMallWeatherExportExpired},
		{name: "expired", status: "succeeded", expiry: now, want: ErrMallWeatherExportExpired},
		{name: "inside final minute", status: "succeeded", expiry: now.Add(30 * time.Second), want: ErrMallWeatherExportExpired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobs := &fakeMallWeatherExportJobReader{download: &data_dao.MallWeatherExportDownloadJob{
				Status: tt.status, ResultObjectKey: "mall-weather-exports/job/result.xlsx", ExpiresAt: &tt.expiry,
			}}
			service := &MallWeatherExportJobService{
				permissions: fakeMallPermissionChecker{allowed: true}, jobs: jobs,
				newSigner: func() (mallWeatherExportDownloadSigner, error) {
					return &fakeMallWeatherExportDownloadSigner{objectSize: 1}, nil
				},
				downloadTTL: 5 * time.Minute, statTimeout: time.Second, now: func() time.Time { return now },
			}
			_, err := service.Download(t.Context(), 17, jobUUID)
			if !errors.Is(err, tt.want) {
				t.Fatalf("Download() error=%v, want %v", err, tt.want)
			}
		})
	}
}

func TestMallWeatherExportJobServiceRejectsUnavailableArtifactBeforeSigning(t *testing.T) {
	now := time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC)
	expiresAt := now.Add(time.Hour)
	jobUUID := uuid.NewString()
	tests := []struct {
		name       string
		objectSize int64
		statErr    error
		want       error
	}{
		{name: "object missing", statErr: storage.ErrOSSObjectNotFound, want: ErrMallWeatherExportArtifactMissing},
		{name: "object size mismatch", objectSize: 2048, want: ErrMallWeatherExportArtifactMissing},
		{name: "storage unavailable", statErr: errors.New("access denied"), want: ErrMallWeatherExportStorageUnavailable},
		{name: "storage timeout", statErr: context.DeadlineExceeded, want: context.DeadlineExceeded},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobs := &fakeMallWeatherExportJobReader{download: &data_dao.MallWeatherExportDownloadJob{
				Status: "succeeded", ResultObjectKey: "mall-weather-exports/job/result.xlsx",
				FileSizeBytes: 4096, ExpiresAt: &expiresAt,
			}}
			signer := &fakeMallWeatherExportDownloadSigner{objectSize: tt.objectSize, statErr: tt.statErr}
			service := &MallWeatherExportJobService{
				permissions: fakeMallPermissionChecker{allowed: true}, jobs: jobs,
				newSigner:   func() (mallWeatherExportDownloadSigner, error) { return signer, nil },
				downloadTTL: 5 * time.Minute, statTimeout: time.Second, now: func() time.Time { return now },
			}
			_, err := service.Download(t.Context(), 17, jobUUID)
			if !errors.Is(err, tt.want) {
				t.Fatalf("Download() error=%v, want %v", err, tt.want)
			}
			if signer.objectKey != "" {
				t.Fatalf("PresignDownloadURL() called for unavailable artifact: %+v", signer)
			}
		})
	}
}

func TestMallWeatherExportJobServiceBoundsArtifactStat(t *testing.T) {
	now := time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC)
	expiresAt := now.Add(time.Hour)
	signer := &fakeMallWeatherExportDownloadSigner{waitForStatContext: true}
	service := &MallWeatherExportJobService{
		permissions: fakeMallPermissionChecker{allowed: true},
		jobs: &fakeMallWeatherExportJobReader{download: &data_dao.MallWeatherExportDownloadJob{
			Status: "succeeded", ResultObjectKey: "mall-weather-exports/job/result.xlsx",
			FileSizeBytes: 4096, ExpiresAt: &expiresAt,
		}},
		newSigner:   func() (mallWeatherExportDownloadSigner, error) { return signer, nil },
		downloadTTL: 5 * time.Minute,
		statTimeout: 5 * time.Millisecond,
		now:         func() time.Time { return now },
	}
	_, err := service.Download(t.Context(), 17, uuid.NewString())
	if !errors.Is(err, context.DeadlineExceeded) || signer.objectKey != "" {
		t.Fatalf("Download() error=%v signer=%+v", err, signer)
	}
}

func TestMallWeatherExportJobServiceMapsSignerFailuresToStorageUnavailable(t *testing.T) {
	now := time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC)
	expiresAt := now.Add(time.Hour)
	jobs := &fakeMallWeatherExportJobReader{download: &data_dao.MallWeatherExportDownloadJob{
		Status: "succeeded", ResultObjectKey: "mall-weather-exports/job/result.xlsx",
		FileSizeBytes: 4096, ExpiresAt: &expiresAt,
	}}
	tests := []struct {
		name       string
		factoryErr error
		presignErr error
		want       error
	}{
		{name: "client configuration", factoryErr: errors.New("invalid storage configuration"), want: ErrMallWeatherExportStorageUnavailable},
		{name: "presign credentials", presignErr: errors.New("credentials unavailable"), want: ErrMallWeatherExportStorageUnavailable},
		{name: "presign timeout", presignErr: context.DeadlineExceeded, want: context.DeadlineExceeded},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signer := &fakeMallWeatherExportDownloadSigner{objectSize: 4096, presignErr: tt.presignErr}
			service := &MallWeatherExportJobService{
				permissions: fakeMallPermissionChecker{allowed: true}, jobs: jobs,
				newSigner: func() (mallWeatherExportDownloadSigner, error) {
					if tt.factoryErr != nil {
						return nil, tt.factoryErr
					}
					return signer, nil
				},
				downloadTTL: 5 * time.Minute, statTimeout: time.Second, now: func() time.Time { return now },
			}
			_, err := service.Download(t.Context(), 17, uuid.NewString())
			if !errors.Is(err, tt.want) {
				t.Fatalf("Download() error=%v, want %v", err, tt.want)
			}
		})
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
		&fakeMallWeatherExportProfileReader{row: &profile},
		fakeMallPermissionChecker{allowed: true},
		estimator,
		&fakeMallWeatherExportJobReader{},
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
	row         *model.MallWeatherExportProfile
	systemRow   *model.MallWeatherExportProfile
	err         error
	ensureCalls int
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

func (reader *fakeMallWeatherExportProfileReader) EnsureSystemProfile(
	_ context.Context,
	_ *model.MallWeatherExportProfile,
) (*model.MallWeatherExportProfile, error) {
	reader.ensureCalls++
	if reader.err != nil {
		return nil, reader.err
	}
	copy := *reader.systemRow
	return &copy, nil
}

type fakeMallWeatherExportEstimator struct {
	request    data_dao.MallWeatherExportEstimateRequest
	requests   []data_dao.MallWeatherExportEstimateRequest
	rows       int64
	rowsByKind map[string]int64
	err        error
	calls      int
}

type fakeMallWeatherExportJobReader struct {
	row         *model.MallWeatherExportJob
	download    *data_dao.MallWeatherExportDownloadJob
	err         error
	actorUserID uint
}

func (reader *fakeMallWeatherExportJobReader) FindByUUIDAndActor(
	_ context.Context,
	_ string,
	actorUserID uint,
) (*model.MallWeatherExportJob, error) {
	reader.actorUserID = actorUserID
	if reader.err != nil {
		return nil, reader.err
	}
	copy := *reader.row
	return &copy, nil
}

func (reader *fakeMallWeatherExportJobReader) FindDownloadByUUIDAndActor(
	_ context.Context,
	_ string,
	actorUserID uint,
) (*data_dao.MallWeatherExportDownloadJob, error) {
	reader.actorUserID = actorUserID
	if reader.err != nil {
		return nil, reader.err
	}
	copy := *reader.download
	return &copy, nil
}

func (estimator *fakeMallWeatherExportEstimator) EstimateRows(
	_ context.Context,
	request data_dao.MallWeatherExportEstimateRequest,
) (int64, error) {
	estimator.calls++
	estimator.request = request
	estimator.requests = append(estimator.requests, request)
	if len(request.Datasets) == 1 && estimator.rowsByKind != nil {
		return estimator.rowsByKind[request.Datasets[0].Kind], estimator.err
	}
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

type fakeMallWeatherExportDownloadSigner struct {
	url                string
	objectSize         int64
	statErr            error
	presignErr         error
	openErr            error
	waitForStatContext bool
	statObjectKey      string
	objectKey          string
	openObjectKey      string
	downloadBody       string
	downloadName       string
	expires            time.Duration
}

func (signer *fakeMallWeatherExportDownloadSigner) StatDownloadObject(
	ctx context.Context,
	objectKey string,
) (storage.ObjectMetadata, error) {
	signer.statObjectKey = objectKey
	if signer.waitForStatContext {
		<-ctx.Done()
		return storage.ObjectMetadata{}, ctx.Err()
	}
	return storage.ObjectMetadata{Size: signer.objectSize}, signer.statErr
}

func (signer *fakeMallWeatherExportDownloadSigner) PresignDownloadURL(
	_ context.Context,
	objectKey string,
	downloadName string,
	expires time.Duration,
) (string, error) {
	signer.objectKey = objectKey
	signer.downloadName = downloadName
	signer.expires = expires
	return signer.url, signer.presignErr
}

func (signer *fakeMallWeatherExportDownloadSigner) OpenDownloadObject(
	_ context.Context,
	objectKey string,
) (storage.DownloadObject, error) {
	signer.openObjectKey = objectKey
	if signer.openErr != nil {
		return storage.DownloadObject{}, signer.openErr
	}
	return storage.DownloadObject{
		Body: io.NopCloser(strings.NewReader(signer.downloadBody)),
		Size: signer.objectSize,
	}, nil
}

func (store *fakeMallWeatherExportJobStore) Create(
	_ context.Context,
	command mallWeatherExportCreateCommand,
) (*MallWeatherExportCreateResult, bool, error) {
	store.calls++
	store.command = command
	return store.result, store.replayed, store.err
}
