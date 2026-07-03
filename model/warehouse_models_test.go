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
		{"pipeline definition", func() {
			model := &PipelineDefinition{}
			model.ID = 1
			model.CreatedAt = 1
			model.UpdatedAt = 2
		}},
		{"pipeline stage", func() {
			model := &PipelineStage{}
			model.ID = 1
			model.CreatedAt = 1
			model.UpdatedAt = 2
		}},
		{"method step", func() {
			model := &MethodStep{}
			model.ID = 1
			model.CreatedAt = 1
			model.UpdatedAt = 2
		}},
		{"method param", func() {
			model := &MethodParam{}
			model.ID = 1
			model.CreatedAt = 1
			model.UpdatedAt = 2
		}},
		{"method output", func() {
			model := &MethodOutput{}
			model.ID = 1
			model.CreatedAt = 1
			model.UpdatedAt = 2
		}},
		{"stage generated config", func() {
			model := &StageGeneratedConfig{}
			model.ID = 1
			model.CreatedAt = 1
			model.UpdatedAt = 2
		}},
		{"step run", func() {
			model := &StepRun{}
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
