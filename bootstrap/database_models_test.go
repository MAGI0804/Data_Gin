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
	if len(models) != 10 {
		t.Fatalf("reportCenterMigrationModels() count = %d, want 10", len(models))
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

func TestReportCenterMigrationVersionIncludesExportLease(t *testing.T) {
	if schemaMigrationVersion != "2026-08-12-report-center-v6" {
		t.Fatalf("schemaMigrationVersion = %q", schemaMigrationVersion)
	}
}
