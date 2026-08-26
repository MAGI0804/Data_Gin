package bootstrap

import (
	"reflect"
	"testing"
)

func TestMallWeatherMigrationModelsAreUnique(t *testing.T) {
	models := mallWeatherMigrationModels()
	if len(models) != 24 {
		t.Fatalf("mallWeatherMigrationModels() count = %d, want 24", len(models))
	}

	seen := make(map[reflect.Type]struct{}, len(models))
	for _, model := range models {
		modelType := reflect.TypeOf(model)
		if _, exists := seen[modelType]; exists {
			t.Fatalf("duplicate migration model %v", modelType)
		}
		seen[modelType] = struct{}{}
	}
}

func TestReportCenterMigrationModelsAreUnique(t *testing.T) {
	models := reportCenterMigrationModels()
	if len(models) != 12 {
		t.Fatalf("reportCenterMigrationModels() count = %d, want 12", len(models))
	}

	seen := make(map[reflect.Type]struct{}, len(models))
	for _, model := range models {
		modelType := reflect.TypeOf(model)
		if _, exists := seen[modelType]; exists {
			t.Fatalf("duplicate migration model %v", modelType)
		}
		seen[modelType] = struct{}{}
	}
}

func TestSchemaMigrationVersionIncludesBojunOracleModels(t *testing.T) {
	if schemaMigrationVersion != "2026-08-26-bojun-oracle-v13" {
		t.Fatalf("schemaMigrationVersion = %q", schemaMigrationVersion)
	}
	if previousSchemaMigrationVersion != "2026-08-26-bojun-oracle-v12" {
		t.Fatalf("previousSchemaMigrationVersion = %q", previousSchemaMigrationVersion)
	}
}

func TestRunPendingSchemaMigrationUsesIncrementalPathFromPreviousVersion(t *testing.T) {
	incrementalCalls, fullCalls := 0, 0
	err := runPendingSchemaMigration(true, func() error {
		incrementalCalls++
		return nil
	}, func() error {
		fullCalls++
		return nil
	})
	if err != nil || incrementalCalls != 1 || fullCalls != 0 {
		t.Fatalf("runPendingSchemaMigration() error=%v incremental=%d full=%d", err, incrementalCalls, fullCalls)
	}
}

func TestRunPendingSchemaMigrationUsesFullPathWithoutPreviousVersion(t *testing.T) {
	incrementalCalls, fullCalls := 0, 0
	err := runPendingSchemaMigration(false, func() error {
		incrementalCalls++
		return nil
	}, func() error {
		fullCalls++
		return nil
	})
	if err != nil || incrementalCalls != 0 || fullCalls != 1 {
		t.Fatalf("runPendingSchemaMigration() error=%v incremental=%d full=%d", err, incrementalCalls, fullCalls)
	}
}
