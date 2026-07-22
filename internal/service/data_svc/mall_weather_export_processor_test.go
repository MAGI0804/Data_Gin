package data_svc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/model"

	"github.com/google/uuid"
)

func TestPrepareMallWeatherExportJobStrictlyValidatesSnapshot(t *testing.T) {
	now := time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC)
	row := validMallWeatherExportProcessorJob(t)
	prepared, err := prepareMallWeatherExportJob(row, now)
	if err != nil {
		t.Fatalf("prepareMallWeatherExportJob() error=%v", err)
	}
	if prepared.FileName != "商场天气_20260722_160000.xlsx" || prepared.Config.TimeZone != "Asia/Shanghai" {
		t.Fatalf("prepared=%+v", prepared)
	}
	if len(prepared.Filter.MallIDs) != 1 || prepared.Filter.MallIDs[0] != 7 {
		t.Fatalf("filter=%+v", prepared.Filter)
	}

	invalid := row
	invalid.ProfileSnapshotJSON = model.JSONText(strings.TrimSuffix(string(row.ProfileSnapshotJSON), "}") + `,"unknown":true}`)
	if _, err := prepareMallWeatherExportJob(invalid, now); err == nil {
		t.Fatal("prepareMallWeatherExportJob() accepted unknown snapshot field")
	}
	invalid = row
	invalid.ProfileVersion++
	if _, err := prepareMallWeatherExportJob(invalid, now); err == nil {
		t.Fatal("prepareMallWeatherExportJob() accepted mismatched snapshot identity")
	}
	var snapshot MallWeatherExportProfileSnapshot
	if err := json.Unmarshal([]byte(row.ProfileSnapshotJSON), &snapshot); err != nil {
		t.Fatalf("Unmarshal(snapshot) error=%v", err)
	}
	snapshot.Code = " MALL_WEATHER "
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("Marshal(invalid snapshot) error=%v", err)
	}
	invalid = row
	invalid.ProfileSnapshotJSON = model.JSONText(encoded)
	if _, err := prepareMallWeatherExportJob(invalid, now); err == nil {
		t.Fatal("prepareMallWeatherExportJob() accepted non-canonical profile identity")
	}
}

func TestMallWeatherExportWorkDirAndArtifactArePrivate(t *testing.T) {
	token := uuid.NewString()
	workDir, err := createMallWeatherExportWorkDir(t.TempDir(), 17, token)
	if err != nil {
		t.Fatalf("createMallWeatherExportWorkDir() error=%v", err)
	}
	info, err := os.Stat(workDir)
	if err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("work directory mode=%v error=%v", info.Mode(), err)
	}
	artifact := filepath.Join(workDir, "result.xlsx")
	if err := os.WriteFile(artifact, []byte("weather export"), 0o666); err != nil {
		t.Fatalf("WriteFile() error=%v", err)
	}
	checksum, size, err := inspectMallWeatherExportArtifact(artifact)
	if err != nil {
		t.Fatalf("inspectMallWeatherExportArtifact() error=%v", err)
	}
	if checksum != "81365898446568dbe62cb15eee6a246958c405bc08d8cf61e56fb090df5a68ac" || size != 14 {
		t.Fatalf("checksum=%s size=%d", checksum, size)
	}
	info, err = os.Stat(artifact)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("artifact mode=%v error=%v", info.Mode(), err)
	}
}

func TestMallWeatherExportObjectKeyIsRunSpecific(t *testing.T) {
	jobUUID, runToken := uuid.NewString(), uuid.NewString()
	key, err := mallWeatherExportObjectKey(jobUUID, runToken, time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("mallWeatherExportObjectKey() error=%v", err)
	}
	if !strings.Contains(key, jobUUID+"/"+runToken+"/result.xlsx") || strings.HasPrefix(key, "/") {
		t.Fatalf("object key=%q", key)
	}
}

func validMallWeatherExportProcessorJob(t *testing.T) model.MallWeatherExportJob {
	t.Helper()
	request := requestbody.MallWeatherExportProfileSaveRequest{
		Code: "mall_weather", Name: "商场天气", TimeZone: "Asia/Shanghai", UnitSystem: "metric",
		DateFormat: "2006-01-02", DateTimeFormat: "2006-01-02 15:04:05",
		FileNameTemplate: "商场天气_{{date:20060102_150405}}.xlsx",
		Filters:          requestbody.MallWeatherExportFilters{MallIDs: []uint{7}},
		Datasets:         []requestbody.MallWeatherExportDataset{{Kind: "malls", SheetName: "商场"}},
	}
	_, config, err := normalizeMallWeatherExportProfile(request)
	if err != nil {
		t.Fatalf("normalizeMallWeatherExportProfile() error=%v", err)
	}
	snapshot, err := json.Marshal(MallWeatherExportProfileSnapshot{
		ProfileID: 9, Code: request.Code, Name: request.Name, Version: 3, Config: config,
	})
	if err != nil {
		t.Fatalf("Marshal(snapshot) error=%v", err)
	}
	filters, err := json.Marshal(config.Filters)
	if err != nil {
		t.Fatalf("Marshal(filters) error=%v", err)
	}
	return model.MallWeatherExportJob{
		BaseModel:           model.BaseModel{ID: 17},
		JobUUID:             uuid.NewString(),
		ProfileID:           9,
		ProfileVersion:      3,
		TotalRows:           1,
		ProfileSnapshotJSON: model.JSONText(snapshot),
		FiltersJSON:         model.JSONText(filters),
	}
}
