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
	if len(models) != 9 {
		t.Fatalf("reportCenterMigrationModels() count = %d, want 9", len(models))
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
