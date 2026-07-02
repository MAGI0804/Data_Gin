package data_svc

import (
	"testing"

	"gin-biz-web-api/model"
)

func TestValidateStageMethodTypeEnforcesStageBoundaries(t *testing.T) {
	tests := []struct {
		name       string
		stageType  string
		methodType string
		wantErr    bool
	}{
		{"fetch request allowed", "fetch", "request", false},
		{"process mapping allowed", "process", "mapping", false},
		{"push delivery allowed", "push", "delivery", false},
		{"log log allowed", "log", "log", false},
		{"fetch delivery rejected", "fetch", "delivery", true},
		{"push mapping rejected", "push", "mapping", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateStageMethodType(tt.stageType, tt.methodType)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateStageMethodType() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBuildStageGeneratedConfigMapKeepsLargeStageAndSteps(t *testing.T) {
	config, err := buildStageGeneratedConfigMap(
		model.PipelineStage{BaseModel: model.BaseModel{ID: 11}, StageType: "fetch", Name: "数据获取"},
		[]MethodStepDetail{
			{
				Step: model.MethodStep{Code: "fetch_orders", MethodType: "request", TimeoutSeconds: 30},
				Params: []model.MethodParam{
					{Location: "url", Name: "url", ValueSource: "config", Value: "cfg.youzan.orders_url"},
				},
				Outputs: []model.MethodOutput{
					{Name: "records", SourcePath: "data.items", ValueType: "array"},
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("buildStageGeneratedConfigMap returned error: %v", err)
	}

	if config["stage_type"] != "fetch" {
		t.Fatalf("stage_type = %v", config["stage_type"])
	}
	if config["target_ref_type"] != "source_definition" {
		t.Fatalf("target_ref_type = %v", config["target_ref_type"])
	}
	steps := config["steps"].([]map[string]interface{})
	if steps[0]["step_code"] != "fetch_orders" {
		t.Fatalf("step_code = %v", steps[0]["step_code"])
	}
}

func TestDefaultStageTypeForMethod(t *testing.T) {
	if got := defaultStageTypeForMethod("request"); got != "fetch" {
		t.Fatalf("request stage = %s", got)
	}
	if got := defaultStageTypeForMethod("delivery"); got != "push" {
		t.Fatalf("delivery stage = %s", got)
	}
	if got := defaultStageTypeForMethod("mapping"); got != "process" {
		t.Fatalf("mapping stage = %s", got)
	}
}
