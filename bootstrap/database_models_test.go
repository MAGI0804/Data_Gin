package bootstrap

import (
	"reflect"
	"testing"
)

func TestMallWeatherMigrationModelsAreUnique(t *testing.T) {
	models := mallWeatherMigrationModels()
	if len(models) != 18 {
		t.Fatalf("mallWeatherMigrationModels() count = %d, want 18", len(models))
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
