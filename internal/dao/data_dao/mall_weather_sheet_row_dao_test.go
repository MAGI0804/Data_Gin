package data_dao

import (
	"context"
	"strings"
	"testing"
	"time"

	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/credential"

	"gorm.io/gorm"
)

func TestMallWeatherSheetRowDAOValidatesInputsBeforeDatabase(t *testing.T) {
	dao := &MallWeatherSheetRowDAO{}
	checksum := strings.Repeat("a", 64)
	now := time.Now()
	if _, err := dao.FindByBusinessKeys(context.Background(), 1, "hourly", []string{"sha256:key"}); err == nil {
		t.Fatal("FindByBusinessKeys() accepted unavailable database")
	}
	if _, err := dao.IsInitialized(
		context.Background(), 1, "hourly", credential.EnvFeishuWeatherHourlySheetID, checksum,
	); err == nil {
		t.Fatal("IsInitialized() accepted unavailable database")
	}
	if err := dao.UpsertMappings(
		context.Background(), 1, "hourly", credential.EnvFeishuWeatherHourlySheetID,
		[]MallWeatherSheetRowMapping{{BusinessKey: "sha256:key", RowNumber: 2, Checksum: checksum}}, now,
	); err == nil {
		t.Fatal("UpsertMappings() accepted unavailable database")
	}
	if err := dao.CreateScannedMappings(
		context.Background(), 1, "hourly", credential.EnvFeishuWeatherHourlySheetID,
		[]MallWeatherSheetRowMapping{{BusinessKey: "sha256:key", RowNumber: 2, Checksum: checksum}}, now,
	); err == nil {
		t.Fatal("CreateScannedMappings() accepted unavailable database")
	}
	if err := dao.ResetMappings(context.Background(), 1, "hourly"); err == nil {
		t.Fatal("ResetMappings() accepted unavailable database")
	}
	if err := dao.MarkInitialized(
		context.Background(), 1, "hourly", credential.EnvFeishuWeatherHourlySheetID, checksum, now,
	); err == nil {
		t.Fatal("MarkInitialized() accepted unavailable database")
	}
}

func TestMallWeatherSheetRowDAOAcceptsBoundedMappings(t *testing.T) {
	dao := NewMallWeatherSheetRowDAO(dryRunMallWeatherSheetRowDB(t))
	checksum := strings.Repeat("a", 64)
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	mappings := []MallWeatherSheetRowMapping{
		{BusinessKey: "sha256:first", RowNumber: 2, Checksum: checksum},
		{BusinessKey: "sha256:second", RowNumber: 3, Checksum: strings.Repeat("b", 64)},
	}
	if err := dao.UpsertMappings(
		t.Context(), 17, "hourly", credential.EnvFeishuWeatherHourlySheetID, mappings, now,
	); err != nil {
		t.Fatalf("UpsertMappings() error=%v", err)
	}
	if err := dao.MarkInitialized(
		t.Context(), 17, "hourly", credential.EnvFeishuWeatherHourlySheetID, checksum, now,
	); err != nil {
		t.Fatalf("MarkInitialized() error=%v", err)
	}
	if err := dao.ResetMappings(t.Context(), 17, "hourly"); err != nil {
		t.Fatalf("ResetMappings() error=%v", err)
	}
	initialized, err := dao.IsInitialized(
		t.Context(), 17, "hourly", credential.EnvFeishuWeatherHourlySheetID, checksum,
	)
	if err != nil || initialized {
		t.Fatalf("IsInitialized() initialized=%v error=%v", initialized, err)
	}
	rows, err := dao.FindByBusinessKeys(
		t.Context(), 17, "hourly", []string{"sha256:first", "sha256:second"},
	)
	if err != nil || len(rows) != 0 {
		t.Fatalf("FindByBusinessKeys() rows=%v error=%v", rows, err)
	}
}

func TestMallWeatherSheetRowDAORejectsAmbiguousMappings(t *testing.T) {
	dao := NewMallWeatherSheetRowDAO(dryRunMallWeatherSheetRowDB(t))
	checksum := strings.Repeat("a", 64)
	now := time.Now()
	tests := []struct {
		name     string
		mappings []MallWeatherSheetRowMapping
	}{
		{
			name: "duplicate business key",
			mappings: []MallWeatherSheetRowMapping{
				{BusinessKey: "sha256:key", RowNumber: 2, Checksum: checksum},
				{BusinessKey: "sha256:key", RowNumber: 3, Checksum: checksum},
			},
		},
		{
			name: "duplicate remote row",
			mappings: []MallWeatherSheetRowMapping{
				{BusinessKey: "sha256:first", RowNumber: 2, Checksum: checksum},
				{BusinessKey: "sha256:second", RowNumber: 2, Checksum: checksum},
			},
		},
		{
			name:     "reserved state key",
			mappings: []MallWeatherSheetRowMapping{{BusinessKey: mallWeatherSheetMappingStateKey, RowNumber: 2, Checksum: checksum}},
		},
		{
			name:     "invalid checksum",
			mappings: []MallWeatherSheetRowMapping{{BusinessKey: "sha256:key", RowNumber: 2, Checksum: "bad"}},
		},
		{
			name:     "header row",
			mappings: []MallWeatherSheetRowMapping{{BusinessKey: "sha256:key", RowNumber: 1, Checksum: checksum}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := dao.UpsertMappings(
				t.Context(), 17, "hourly", credential.EnvFeishuWeatherHourlySheetID, test.mappings, now,
			); err == nil {
				t.Fatal("UpsertMappings() accepted ambiguous mappings")
			}
		})
	}
	if _, err := dao.FindByBusinessKeys(
		t.Context(), 17, "hourly", []string{"sha256:key", "sha256:key"},
	); err == nil {
		t.Fatal("FindByBusinessKeys() accepted duplicate keys")
	}
}

func TestValidStoredMallWeatherSheetRowRejectsStateMarker(t *testing.T) {
	now := time.Now()
	row := model.MallWeatherSheetRow{
		BaseModel: model.BaseModel{ID: 1}, DestinationID: 17, DatasetKind: "hourly",
		BusinessKey: "sha256:key", SheetIDEnv: credential.EnvFeishuWeatherHourlySheetID,
		RowNumber: 2, Checksum: strings.Repeat("a", 64), LastSyncedAt: now,
	}
	if !validStoredMallWeatherSheetRow(row, 17, "hourly") {
		t.Fatal("validStoredMallWeatherSheetRow() rejected valid row")
	}
	row.BusinessKey = mallWeatherSheetMappingStateKey
	if validStoredMallWeatherSheetRow(row, 17, "hourly") {
		t.Fatal("validStoredMallWeatherSheetRow() accepted state marker")
	}
}

func dryRunMallWeatherSheetRowDB(t *testing.T) *gorm.DB {
	t.Helper()
	return dryRunWeatherDAOTestDB(t).Session(&gorm.Session{SkipDefaultTransaction: true})
}
