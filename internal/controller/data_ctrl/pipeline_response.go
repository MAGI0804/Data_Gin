package data_ctrl

import (
	"encoding/json"
	"strings"

	"gin-biz-web-api/internal/configsecret"
	"gin-biz-web-api/internal/service/data_svc"
	"gin-biz-web-api/model"
)

const pipelineResponseTextLimit = 4 * 1024

type pipelineStageDetailResponse struct {
	Stage           model.PipelineStage           `json:"stage"`
	Steps           []methodStepDetailResponse    `json:"steps"`
	GeneratedConfig *stageGeneratedConfigResponse `json:"generated_config"`
}

type methodStepDetailResponse struct {
	Step    model.MethodStep     `json:"step"`
	Params  []model.MethodParam  `json:"params"`
	Outputs []model.MethodOutput `json:"outputs"`
}

type pipelineDetailResponse struct {
	Pipeline model.PipelineDefinition      `json:"pipeline"`
	Stages   []pipelineStageDetailResponse `json:"stages"`
	Steps    []methodStepDetailResponse    `json:"steps"`
}

type stageGeneratedConfigResponse struct {
	ID                  uint   `json:"id"`
	PipelineID          uint   `json:"pipeline_id"`
	StageID             uint   `json:"stage_id"`
	StageType           string `json:"stage_type"`
	GeneratedConfigJSON string `json:"generated_config_json"`
	TargetRefType       string `json:"target_ref_type"`
	TargetRefID         uint   `json:"target_ref_id"`
	Version             int    `json:"version"`
}

type stepRunResponse struct {
	ID                  uint              `json:"id"`
	RunID               uint              `json:"run_id"`
	PipelineID          uint              `json:"pipeline_id"`
	StepID              uint              `json:"step_id"`
	StepCode            string            `json:"step_code"`
	MethodType          string            `json:"method_type"`
	Status              string            `json:"status"`
	InputJSON           string            `json:"input_json"`
	OutputJSON          string            `json:"output_json"`
	GeneratedConfigJSON string            `json:"generated_config_json"`
	ErrorMessage        string            `json:"error_message"`
	StartedAt           *model.TimeNormal `json:"started_at"`
	FinishedAt          *model.TimeNormal `json:"finished_at"`
}

type pipelineRunResultResponse struct {
	RunID        uint   `json:"run_id"`
	TraceID      string `json:"trace_id"`
	SuccessCount int    `json:"success_count"`
	FailedCount  int    `json:"failed_count"`
}

func safePipelineDetail(detail *data_svc.PipelineDetail) pipelineDetailResponse {
	stages := make([]pipelineStageDetailResponse, 0, len(detail.Stages))
	for _, stage := range detail.Stages {
		stages = append(stages, safePipelineStageDetail(stage))
	}
	return pipelineDetailResponse{
		Pipeline: detail.Pipeline,
		Stages:   stages,
		Steps:    safeMethodStepDetails(detail.Steps),
	}
}

func safePipelineStageDetails(stages []data_svc.PipelineStageDetail) []pipelineStageDetailResponse {
	result := make([]pipelineStageDetailResponse, 0, len(stages))
	for _, stage := range stages {
		result = append(result, safePipelineStageDetail(stage))
	}
	return result
}

func safePipelineStageDetail(stage data_svc.PipelineStageDetail) pipelineStageDetailResponse {
	response := pipelineStageDetailResponse{Stage: stage.Stage, Steps: safeMethodStepDetails(stage.Steps)}
	if stage.GeneratedConfig != nil {
		config := safeStageGeneratedConfig(*stage.GeneratedConfig)
		response.GeneratedConfig = &config
	}
	return response
}

func safeMethodStepDetails(steps []data_svc.MethodStepDetail) []methodStepDetailResponse {
	result := make([]methodStepDetailResponse, 0, len(steps))
	for _, step := range steps {
		result = append(result, safeMethodStepDetail(step))
	}
	return result
}

func safeMethodStepDetail(detail data_svc.MethodStepDetail) methodStepDetailResponse {
	step := detail.Step
	step.GeneratedConfigJSON = redactPipelineJSON(step.GeneratedConfigJSON)
	params := make([]model.MethodParam, 0, len(detail.Params))
	for _, param := range detail.Params {
		param.Value = safeMethodParamValue(param)
		params = append(params, param)
	}
	return methodStepDetailResponse{Step: step, Params: params, Outputs: detail.Outputs}
}

func safeMethodParamValue(param model.MethodParam) string {
	if param.Secret || strings.EqualFold(strings.TrimSpace(param.ValueSource), "secret") || configsecret.SensitiveKey(param.Name) {
		return "[已隐藏]"
	}
	return boundPipelineResponseText(param.Value)
}

func safeStageGeneratedConfig(config model.StageGeneratedConfig) stageGeneratedConfigResponse {
	return stageGeneratedConfigResponse{
		ID: config.ID, PipelineID: config.PipelineID, StageID: config.StageID, StageType: config.StageType,
		GeneratedConfigJSON: redactPipelineJSON(config.GeneratedConfigJSON), TargetRefType: config.TargetRefType,
		TargetRefID: config.TargetRefID, Version: config.Version,
	}
}

func safePipelinePreview(preview map[string]interface{}) map[string]interface{} {
	redacted, _ := configsecret.RedactValue(preview, "")
	if result, ok := boundPipelineResponseValue(redacted).(map[string]interface{}); ok {
		return result
	}
	return map[string]interface{}{}
}

func safePipelineRunResult(result *data_svc.PipelineRunResult) pipelineRunResultResponse {
	return pipelineRunResultResponse{RunID: result.RunID, TraceID: result.TraceID, SuccessCount: result.SuccessCount, FailedCount: result.FailedCount}
}

func safeStepRuns(runs []model.StepRun) []stepRunResponse {
	result := make([]stepRunResponse, 0, len(runs))
	for _, run := range runs {
		result = append(result, stepRunResponse{
			ID: run.ID, RunID: run.RunID, PipelineID: run.PipelineID, StepID: run.StepID, StepCode: run.StepCode,
			MethodType: run.MethodType, Status: run.Status, InputJSON: redactPipelineJSON(run.InputJSON),
			OutputJSON: redactPipelineJSON(run.OutputJSON), GeneratedConfigJSON: redactPipelineJSON(run.GeneratedConfigJSON),
			ErrorMessage: safeDeliveryLogText(run.ErrorMessage), StartedAt: run.StartedAt, FinishedAt: run.FinishedAt,
		})
	}
	return result
}

func redactPipelineJSON(value string) string {
	if !json.Valid([]byte(value)) {
		return "{}"
	}
	var decoded interface{}
	if err := json.Unmarshal([]byte(value), &decoded); err != nil {
		return "{}"
	}
	redacted, _ := configsecret.RedactValue(decoded, "")
	encoded, err := json.Marshal(boundPipelineResponseValue(redacted))
	if err != nil {
		return "{}"
	}
	if len(encoded) > pipelineResponseTextLimit {
		return `{"summary":"内容过长，详情已隐藏。"}`
	}
	return string(encoded)
}

func boundPipelineResponseText(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= pipelineResponseTextLimit {
		return value
	}
	return value[:pipelineResponseTextLimit] + "…"
}

func boundPipelineResponseValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case string:
		return boundPipelineResponseText(typed)
	case map[string]interface{}:
		result := make(map[string]interface{}, len(typed))
		for key, child := range typed {
			result[key] = boundPipelineResponseValue(child)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(typed))
		for index, child := range typed {
			result[index] = boundPipelineResponseValue(child)
		}
		return result
	default:
		return value
	}
}
