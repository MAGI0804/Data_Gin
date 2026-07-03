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
		{"fetch bojun signed request allowed", "fetch", "bojun_signed_request", false},
		{"process mapping allowed", "process", "mapping", false},
		{"push delivery allowed", "push", "delivery", false},
		{"push shanghai mall allowed", "push", "shanghai_mall_push", false},
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
	if got := defaultStageTypeForMethod("bojun_signed_request"); got != "fetch" {
		t.Fatalf("bojun signed request stage = %s", got)
	}
	if got := defaultStageTypeForMethod("delivery"); got != "push" {
		t.Fatalf("delivery stage = %s", got)
	}
	if got := defaultStageTypeForMethod("shanghai_mall_push"); got != "push" {
		t.Fatalf("shanghai mall push stage = %s", got)
	}
	if got := defaultStageTypeForMethod("mapping"); got != "process" {
		t.Fatalf("mapping stage = %s", got)
	}
}

func TestBuildLegacyConfigModelsFromStageConfig(t *testing.T) {
	cfg := &model.StageGeneratedConfig{
		PipelineID:          7,
		StageID:             9,
		StageType:           "fetch",
		GeneratedConfigJSON: `{"stage_type":"fetch","steps":[]}`,
		Version:             3,
	}

	source := buildSourceDefinitionFromStageConfig(cfg)
	if source.Code != "source_pipeline_7_stage_9_v3" {
		t.Fatalf("source code = %s", source.Code)
	}
	if source.ConfigJSON != cfg.GeneratedConfigJSON {
		t.Fatalf("source config json = %s", source.ConfigJSON)
	}
	if source.SourceType != "api_poll" || source.AuthType != "pipeline_stage" {
		t.Fatalf("source type/auth = %s/%s", source.SourceType, source.AuthType)
	}

	rule := buildTransformRuleFromStageConfig(cfg)
	if rule.RuleType != "mapping" || rule.OrderIndex != 3 {
		t.Fatalf("rule type/order = %s/%d", rule.RuleType, rule.OrderIndex)
	}

	destination := buildDestinationDefinitionFromStageConfig(cfg)
	if destination.Code != "destination_pipeline_7_stage_9_v3" {
		t.Fatalf("destination code = %s", destination.Code)
	}
	if destination.DestinationType != "http" {
		t.Fatalf("destination type = %s", destination.DestinationType)
	}

	task := buildDeliveryTaskFromStageConfig(cfg, 22)
	if task.DestinationID != 22 || task.TriggerType != "manual" {
		t.Fatalf("task destination/trigger = %d/%s", task.DestinationID, task.TriggerType)
	}
	if task.PayloadTemplate != cfg.GeneratedConfigJSON {
		t.Fatalf("task payload template = %s", task.PayloadTemplate)
	}
}
