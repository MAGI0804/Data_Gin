package model

import "testing"

func TestWarehouseModelsHaveInitializedCommonFields(t *testing.T) {
	tests := []struct {
		name string
		set  func()
	}{
		{"source definition", func() {
			model := &SourceDefinition{}
			model.ID = 1
			model.CreatedAt = 1
			model.UpdatedAt = 2
		}},
		{"raw record", func() {
			model := &RawRecord{}
			model.ID = 1
			model.CreatedAt = 1
			model.UpdatedAt = 2
		}},
		{"transform rule", func() {
			model := &TransformRule{}
			model.ID = 1
			model.CreatedAt = 1
			model.UpdatedAt = 2
		}},
		{"delivery task", func() {
			model := &DeliveryTask{}
			model.ID = 1
			model.CreatedAt = 1
			model.UpdatedAt = 2
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Fatalf("setting common fields panicked: %v", recovered)
				}
			}()
			tt.set()
		})
	}
}
