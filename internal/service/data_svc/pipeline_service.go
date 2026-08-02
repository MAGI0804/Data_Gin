package data_svc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strings"
	"time"

	destinationconnector "gin-biz-web-api/connector/destination"
	transformconnector "gin-biz-web-api/connector/transform"
	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/bojun"
	"gin-biz-web-api/pkg/config"
	"gin-biz-web-api/pkg/shanghaimall"
)

type PipelineService struct {
	pipelineDAO    *data_dao.PipelineDefinitionDAO
	stageDAO       *data_dao.PipelineStageDAO
	stepDAO        *data_dao.MethodStepDAO
	paramDAO       *data_dao.MethodParamDAO
	outputDAO      *data_dao.MethodOutputDAO
	stageConfigDAO *data_dao.StageGeneratedConfigDAO
	stepRunDAO     *data_dao.StepRunDAO
	pipelineRunDAO *data_dao.PipelineRunDAO
	sourceDAO      *data_dao.SourceDefinitionDAO
	transformDAO   *data_dao.TransformRuleDAO
	destinationDAO *data_dao.DestinationDefinitionDAO
	deliveryDAO    *data_dao.DeliveryTaskDAO
}

func NewPipelineService() *PipelineService {
	return &PipelineService{
		pipelineDAO:    data_dao.NewPipelineDefinitionDAO(),
		stageDAO:       data_dao.NewPipelineStageDAO(),
		stepDAO:        data_dao.NewMethodStepDAO(),
		paramDAO:       data_dao.NewMethodParamDAO(),
		outputDAO:      data_dao.NewMethodOutputDAO(),
		stageConfigDAO: data_dao.NewStageGeneratedConfigDAO(),
		stepRunDAO:     data_dao.NewStepRunDAO(),
		pipelineRunDAO: data_dao.NewPipelineRunDAO(),
		sourceDAO:      data_dao.NewSourceDefinitionDAO(),
		transformDAO:   data_dao.NewTransformRuleDAO(),
		destinationDAO: data_dao.NewDestinationDefinitionDAO(),
		deliveryDAO:    data_dao.NewDeliveryTaskDAO(),
	}
}

type PipelineStageDetail struct {
	Stage           model.PipelineStage         `json:"stage"`
	Steps           []MethodStepDetail          `json:"steps"`
	GeneratedConfig *model.StageGeneratedConfig `json:"generated_config"`
}

type MethodStepDetail struct {
	Step    model.MethodStep     `json:"step"`
	Params  []model.MethodParam  `json:"params"`
	Outputs []model.MethodOutput `json:"outputs"`
}

type PipelineDetail struct {
	Pipeline model.PipelineDefinition `json:"pipeline"`
	Stages   []PipelineStageDetail    `json:"stages"`
	Steps    []MethodStepDetail       `json:"steps"`
}

func (s *PipelineService) CreatePipeline(ctx context.Context, req *requestbody.PipelineCreateRequest) (*model.PipelineDefinition, error) {
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	pipeline := &model.PipelineDefinition{
		Name:        strings.TrimSpace(req.Name),
		Code:        strings.TrimSpace(req.Code),
		Description: strings.TrimSpace(req.Description),
		Enabled:     enabled,
	}
	_, err := s.pipelineDAO.Create(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	if err := s.ensureDefaultStages(ctx, pipeline.ID); err != nil {
		return nil, err
	}
	return pipeline, nil
}

func (s *PipelineService) ListPipelines(ctx context.Context) ([]model.PipelineDefinition, error) {
	return s.pipelineDAO.FindAll(ctx)
}

func (s *PipelineService) GetPipeline(ctx context.Context, id uint) (*PipelineDetail, error) {
	pipeline, err := s.pipelineDAO.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	steps, err := s.GetPipelineSteps(ctx, id)
	if err != nil {
		return nil, err
	}
	stages, err := s.GetPipelineStages(ctx, id)
	if err != nil {
		return nil, err
	}
	return &PipelineDetail{Pipeline: *pipeline, Stages: stages, Steps: steps}, nil
}

func (s *PipelineService) UpdatePipeline(ctx context.Context, id uint, req *requestbody.PipelineUpdateRequest) (*model.PipelineDefinition, error) {
	pipeline, err := s.pipelineDAO.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	pipeline.Name = strings.TrimSpace(req.Name)
	pipeline.Code = strings.TrimSpace(req.Code)
	pipeline.Description = strings.TrimSpace(req.Description)
	pipeline.Enabled = enabled
	return pipeline, s.pipelineDAO.Update(ctx, pipeline)
}

func (s *PipelineService) CreateStep(ctx context.Context, pipelineID uint, req *requestbody.MethodStepCreateRequest) (*MethodStepDetail, error) {
	if _, err := s.pipelineDAO.FindByID(ctx, pipelineID); err != nil {
		return nil, err
	}
	stageID := req.StageID
	if stageID == 0 {
		stage, err := s.findDefaultStageForMethod(ctx, pipelineID, req.MethodType)
		if err != nil {
			return nil, err
		}
		stageID = stage.ID
	}
	return s.createStepInStage(ctx, pipelineID, stageID, req)
}

func (s *PipelineService) CreateStepInStage(ctx context.Context, stageID uint, req *requestbody.MethodStepCreateRequest) (*MethodStepDetail, error) {
	return s.createStepInStage(ctx, 0, stageID, req)
}

func (s *PipelineService) createStepInStage(ctx context.Context, expectedPipelineID, stageID uint, req *requestbody.MethodStepCreateRequest) (*MethodStepDetail, error) {
	stage, err := s.stageDAO.FindByID(ctx, stageID)
	if err != nil {
		return nil, err
	}
	if expectedPipelineID != 0 && stage.PipelineID != expectedPipelineID {
		return nil, fmt.Errorf("stage %d does not belong to pipeline %d", stage.ID, expectedPipelineID)
	}
	if err := validateStageMethodType(stage.StageType, req.MethodType); err != nil {
		return nil, err
	}

	step, params, outputs, err := s.buildStepModel(stage.PipelineID, stage.ID, req)
	if err != nil {
		return nil, err
	}
	_, err = s.stepDAO.Create(ctx, step)
	if err != nil {
		return nil, err
	}
	if err := s.paramDAO.ReplaceByStepID(ctx, step.ID, params); err != nil {
		return nil, err
	}
	if err := s.outputDAO.ReplaceByStepID(ctx, step.ID, outputs); err != nil {
		return nil, err
	}
	return s.GetStep(ctx, step.ID)
}

func (s *PipelineService) UpdateStep(ctx context.Context, stepID uint, req *requestbody.MethodStepUpdateRequest) (*MethodStepDetail, error) {
	current, err := s.stepDAO.FindByID(ctx, stepID)
	if err != nil {
		return nil, err
	}
	stageID := current.StageID
	if req.StageID != 0 {
		stageID = req.StageID
	}
	if stageID != 0 {
		stage, err := s.stageDAO.FindByID(ctx, stageID)
		if err != nil {
			return nil, err
		}
		if err := validateStageMethodType(stage.StageType, req.MethodType); err != nil {
			return nil, err
		}
	}
	step, params, outputs, err := s.buildStepModel(current.PipelineID, stageID, (*requestbody.MethodStepCreateRequest)(req))
	if err != nil {
		return nil, err
	}
	step.ID = current.ID
	step.CreatedAt = current.CreatedAt
	if err := s.stepDAO.Update(ctx, step); err != nil {
		return nil, err
	}
	if err := s.paramDAO.ReplaceByStepID(ctx, step.ID, params); err != nil {
		return nil, err
	}
	if err := s.outputDAO.ReplaceByStepID(ctx, step.ID, outputs); err != nil {
		return nil, err
	}
	return s.GetStep(ctx, step.ID)
}

func (s *PipelineService) UpdateStepInPipeline(ctx context.Context, pipelineID, stepID uint, req *requestbody.MethodStepUpdateRequest) (*MethodStepDetail, error) {
	if _, err := s.pipelineDAO.FindByID(ctx, pipelineID); err != nil {
		return nil, err
	}
	current, err := s.stepDAO.FindByID(ctx, stepID)
	if err != nil {
		return nil, err
	}
	if current.PipelineID != pipelineID {
		return nil, fmt.Errorf("step %d does not belong to pipeline %d", current.ID, pipelineID)
	}
	if current.StageID != 0 && req.StageID != 0 && req.StageID != current.StageID {
		return nil, fmt.Errorf("moving a step between stages is not supported")
	}
	if current.StageID != 0 {
		return s.UpdateStep(ctx, stepID, req)
	}
	defaultStage, err := s.findDefaultStageForMethod(ctx, pipelineID, req.MethodType)
	if err != nil {
		return nil, err
	}
	if req.StageID != 0 && req.StageID != defaultStage.ID {
		return nil, fmt.Errorf("legacy step %d can only move to default stage %d", current.ID, defaultStage.ID)
	}
	normalizedRequest := *req
	normalizedRequest.StageID = defaultStage.ID
	return s.updateLegacyStep(ctx, current, defaultStage, &normalizedRequest)
}

func (s *PipelineService) UpdateStepInStage(ctx context.Context, stageID, stepID uint, req *requestbody.MethodStepUpdateRequest) (*MethodStepDetail, error) {
	stage, err := s.stageDAO.FindByID(ctx, stageID)
	if err != nil {
		return nil, err
	}
	current, err := s.stepDAO.FindByID(ctx, stepID)
	if err != nil {
		return nil, err
	}
	if current.PipelineID != stage.PipelineID || (current.StageID != 0 && current.StageID != stage.ID) {
		return nil, fmt.Errorf("step %d does not belong to stage %d", current.ID, stage.ID)
	}
	if req.StageID != 0 && req.StageID != stage.ID {
		return nil, fmt.Errorf("moving a step between stages is not supported")
	}
	if current.StageID == 0 && stage.StageType != defaultStageTypeForMethod(req.MethodType) {
		return nil, fmt.Errorf("legacy step %d belongs to default stage type %q", current.ID, defaultStageTypeForMethod(req.MethodType))
	}
	normalizedRequest := *req
	normalizedRequest.StageID = stage.ID
	if current.StageID == 0 {
		return s.updateLegacyStep(ctx, current, stage, &normalizedRequest)
	}
	return s.UpdateStep(ctx, stepID, &normalizedRequest)
}

func (s *PipelineService) updateLegacyStep(ctx context.Context, current *model.MethodStep, stage *model.PipelineStage, req *requestbody.MethodStepUpdateRequest) (*MethodStepDetail, error) {
	if current.StageID != 0 || current.PipelineID != stage.PipelineID {
		return nil, fmt.Errorf("legacy step %d does not belong to stage %d", current.ID, stage.ID)
	}
	if stage.StageType != defaultStageTypeForMethod(req.MethodType) {
		return nil, fmt.Errorf("legacy step %d belongs to default stage type %q", current.ID, defaultStageTypeForMethod(req.MethodType))
	}
	if err := validateStageMethodType(stage.StageType, req.MethodType); err != nil {
		return nil, err
	}
	step, params, outputs, err := s.buildStepModel(current.PipelineID, stage.ID, (*requestbody.MethodStepCreateRequest)(req))
	if err != nil {
		return nil, err
	}
	step.ID = current.ID
	step.CreatedAt = current.CreatedAt
	updated, err := s.stepDAO.UpdateLegacy(ctx, step)
	if err != nil {
		return nil, err
	}
	if !updated {
		return nil, fmt.Errorf("legacy step %d update conflict", current.ID)
	}
	if err := s.paramDAO.ReplaceByStepID(ctx, step.ID, params); err != nil {
		return nil, err
	}
	if err := s.outputDAO.ReplaceByStepID(ctx, step.ID, outputs); err != nil {
		return nil, err
	}
	return s.GetStep(ctx, step.ID)
}

func (s *PipelineService) buildStepModel(pipelineID, stageID uint, req *requestbody.MethodStepCreateRequest) (*model.MethodStep, []model.MethodParam, []model.MethodOutput, error) {
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	step := &model.MethodStep{
		PipelineID:     pipelineID,
		StageID:        stageID,
		Code:           strings.TrimSpace(req.Code),
		Name:           strings.TrimSpace(req.Name),
		MethodType:     strings.TrimSpace(req.MethodType),
		OrderIndex:     req.OrderIndex,
		Enabled:        enabled,
		TimeoutSeconds: req.TimeoutSeconds,
	}
	params := paramsFromRequest(req.Params)
	outputs := outputsFromRequest(req.Outputs)
	generated, err := BuildGeneratedStepConfig(MethodStepDefinition{Step: *step, Params: params, Outputs: outputs})
	if err != nil {
		return nil, nil, nil, err
	}
	step.GeneratedConfigJSON = generated
	return step, params, outputs, nil
}

func (s *PipelineService) CreateStage(ctx context.Context, pipelineID uint, req *requestbody.PipelineStageCreateRequest) (*model.PipelineStage, error) {
	if _, err := s.pipelineDAO.FindByID(ctx, pipelineID); err != nil {
		return nil, err
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	stage := &model.PipelineStage{
		PipelineID: pipelineID,
		StageType:  strings.TrimSpace(req.StageType),
		Name:       strings.TrimSpace(req.Name),
		OrderIndex: req.OrderIndex,
		Enabled:    enabled,
	}
	if _, err := s.stageDAO.Create(ctx, stage); err != nil {
		return nil, err
	}
	return stage, nil
}

func (s *PipelineService) UpdateStage(ctx context.Context, stageID uint, req *requestbody.PipelineStageUpdateRequest) (*model.PipelineStage, error) {
	stage, err := s.stageDAO.FindByID(ctx, stageID)
	if err != nil {
		return nil, err
	}
	steps, err := s.stepDAO.FindByStageID(ctx, stageID)
	if err != nil {
		return nil, err
	}
	for _, step := range steps {
		if err := validateStageMethodType(req.StageType, step.MethodType); err != nil {
			return nil, fmt.Errorf("stage type cannot change while step %d uses %q: %w", step.ID, step.MethodType, err)
		}
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	stage.StageType = strings.TrimSpace(req.StageType)
	stage.Name = strings.TrimSpace(req.Name)
	stage.OrderIndex = req.OrderIndex
	stage.Enabled = enabled
	return stage, s.stageDAO.Update(ctx, stage)
}

func (s *PipelineService) GetPipelineStages(ctx context.Context, pipelineID uint) ([]PipelineStageDetail, error) {
	if _, err := s.pipelineDAO.FindByID(ctx, pipelineID); err != nil {
		return nil, err
	}
	if err := s.ensureDefaultStages(ctx, pipelineID); err != nil {
		return nil, err
	}
	stages, err := s.stageDAO.FindByPipelineID(ctx, pipelineID)
	if err != nil {
		return nil, err
	}
	stepDetails, err := s.GetPipelineSteps(ctx, pipelineID)
	if err != nil {
		return nil, err
	}
	stepsByStage := map[uint][]MethodStepDetail{}
	for _, step := range stepDetails {
		stepsByStage[step.Step.StageID] = append(stepsByStage[step.Step.StageID], step)
	}
	configs, err := s.stageConfigDAO.FindByPipelineID(ctx, pipelineID)
	if err != nil {
		return nil, err
	}
	latestConfigByStage := map[uint]model.StageGeneratedConfig{}
	for _, cfg := range configs {
		if _, ok := latestConfigByStage[cfg.StageID]; !ok {
			latestConfigByStage[cfg.StageID] = cfg
		}
	}
	details := make([]PipelineStageDetail, 0, len(stages))
	for _, stage := range stages {
		var cfgPtr *model.StageGeneratedConfig
		if cfg, ok := latestConfigByStage[stage.ID]; ok {
			cfgCopy := cfg
			cfgPtr = &cfgCopy
		}
		details = append(details, PipelineStageDetail{
			Stage:           stage,
			Steps:           stepsByStage[stage.ID],
			GeneratedConfig: cfgPtr,
		})
	}
	return details, nil
}

func (s *PipelineService) GetPipelineSteps(ctx context.Context, pipelineID uint) ([]MethodStepDetail, error) {
	steps, err := s.stepDAO.FindByPipelineID(ctx, pipelineID)
	if err != nil {
		return nil, err
	}
	steps, err = s.normalizeLegacyStepStages(ctx, pipelineID, steps)
	if err != nil {
		return nil, err
	}
	return s.hydrateStepDetails(ctx, steps)
}

func (s *PipelineService) GetStep(ctx context.Context, stepID uint) (*MethodStepDetail, error) {
	step, err := s.stepDAO.FindByID(ctx, stepID)
	if err != nil {
		return nil, err
	}
	steps, err := s.normalizeLegacyStepStages(ctx, step.PipelineID, []model.MethodStep{*step})
	if err != nil {
		return nil, err
	}
	details, err := s.hydrateStepDetails(ctx, steps)
	if err != nil {
		return nil, err
	}
	if len(details) == 0 {
		return nil, fmt.Errorf("step %d not found", stepID)
	}
	return &details[0], nil
}

func (s *PipelineService) hydrateStepDetails(ctx context.Context, steps []model.MethodStep) ([]MethodStepDetail, error) {
	stepIDs := make([]uint, 0, len(steps))
	for _, step := range steps {
		stepIDs = append(stepIDs, step.ID)
	}
	params, err := s.paramDAO.FindByStepIDs(ctx, stepIDs)
	if err != nil {
		return nil, err
	}
	outputs, err := s.outputDAO.FindByStepIDs(ctx, stepIDs)
	if err != nil {
		return nil, err
	}
	paramsByStep := map[uint][]model.MethodParam{}
	for _, param := range params {
		paramsByStep[param.StepID] = append(paramsByStep[param.StepID], param)
	}
	outputsByStep := map[uint][]model.MethodOutput{}
	for _, output := range outputs {
		outputsByStep[output.StepID] = append(outputsByStep[output.StepID], output)
	}
	details := make([]MethodStepDetail, 0, len(steps))
	for _, step := range steps {
		details = append(details, MethodStepDetail{
			Step:    step,
			Params:  paramsByStep[step.ID],
			Outputs: outputsByStep[step.ID],
		})
	}
	return details, nil
}

func (s *PipelineService) PreviewPipelineJSON(ctx context.Context, pipelineID uint) (map[string]interface{}, error) {
	detail, err := s.GetPipeline(ctx, pipelineID)
	if err != nil {
		return nil, err
	}
	stages := make([]map[string]interface{}, 0, len(detail.Stages))
	for _, stage := range detail.Stages {
		stageConfig, err := buildStageGeneratedConfigMap(stage.Stage, stage.Steps)
		if err != nil {
			return nil, err
		}
		stages = append(stages, stageConfig)
	}
	steps := make([]map[string]interface{}, 0, len(detail.Steps))
	for _, step := range detail.Steps {
		cfg, err := BuildGeneratedStepConfigMap(MethodStepDefinition{Step: step.Step, Params: step.Params, Outputs: step.Outputs})
		if err != nil {
			return nil, err
		}
		steps = append(steps, cfg)
	}
	return map[string]interface{}{"pipeline": detail.Pipeline, "stages": stages, "steps": steps}, nil
}

func (s *PipelineService) GenerateStageConfig(ctx context.Context, stageID uint) (*model.StageGeneratedConfig, error) {
	stage, configJSON, err := s.buildCurrentStageConfigJSON(ctx, stageID)
	if err != nil {
		return nil, err
	}
	version, err := s.stageConfigDAO.NextVersion(ctx, stageID)
	if err != nil {
		return nil, err
	}
	cfg := &model.StageGeneratedConfig{
		PipelineID:          stage.PipelineID,
		StageID:             stage.ID,
		StageType:           stage.StageType,
		GeneratedConfigJSON: configJSON,
		TargetRefType:       targetRefTypeForStage(stage.StageType),
		Version:             version,
	}
	_, err = s.stageConfigDAO.Create(ctx, cfg)
	return cfg, err
}

func (s *PipelineService) buildCurrentStageConfigJSON(ctx context.Context, stageID uint) (*model.PipelineStage, string, error) {
	stage, err := s.stageDAO.FindByID(ctx, stageID)
	if err != nil {
		return nil, "", err
	}
	steps, err := s.GetStageSteps(ctx, stageID)
	if err != nil {
		return nil, "", err
	}
	configMap, err := buildStageGeneratedConfigMap(*stage, steps)
	if err != nil {
		return nil, "", err
	}
	configJSON, err := json.Marshal(configMap)
	if err != nil {
		return nil, "", err
	}
	return stage, string(configJSON), nil
}

func (s *PipelineService) PublishStageConfig(ctx context.Context, stageID uint) (*model.StageGeneratedConfig, error) {
	cfg, err := s.stageConfigDAO.FindLatestByStageID(ctx, stageID)
	hasSnapshot := err == nil
	if err != nil {
		cfg, err = s.GenerateStageConfig(ctx, stageID)
		if err != nil {
			return nil, err
		}
	}
	if cfg.TargetRefType == "" {
		cfg.TargetRefType = targetRefTypeForStage(cfg.StageType)
	}
	if cfg.TargetRefID != 0 || cfg.StageType == "log" {
		return cfg, nil
	}
	if hasSnapshot {
		_, currentConfigJSON, err := s.buildCurrentStageConfigJSON(ctx, stageID)
		if err != nil {
			return nil, err
		}
		if !sameJSONValue(cfg.GeneratedConfigJSON, currentConfigJSON) {
			return nil, fmt.Errorf("stage config %d is stale; regenerate before publishing", cfg.ID)
		}
	}
	targetRefType, targetRefID, err := s.publishStageConfigSnapshot(ctx, cfg)
	if err != nil {
		return nil, err
	}
	cfg.TargetRefType = targetRefType
	cfg.TargetRefID = targetRefID
	if err := s.stageConfigDAO.Update(ctx, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func sameJSONValue(left, right string) bool {
	var leftValue interface{}
	if err := json.Unmarshal([]byte(left), &leftValue); err != nil {
		return false
	}
	var rightValue interface{}
	if err := json.Unmarshal([]byte(right), &rightValue); err != nil {
		return false
	}
	return reflect.DeepEqual(leftValue, rightValue)
}

func (s *PipelineService) publishStageConfigSnapshot(ctx context.Context, cfg *model.StageGeneratedConfig) (string, uint, error) {
	switch cfg.StageType {
	case "fetch":
		source := buildSourceDefinitionFromStageConfig(cfg)
		id, err := s.sourceDAO.Create(ctx, source)
		return "source_definition", id, err
	case "process":
		rule := buildTransformRuleFromStageConfig(cfg)
		id, err := s.transformDAO.Create(ctx, rule)
		return "transform_rule", id, err
	case "push":
		destination := buildDestinationDefinitionFromStageConfig(cfg)
		destinationID, err := s.destinationDAO.Create(ctx, destination)
		if err != nil {
			return "destination_delivery_task", 0, err
		}
		task := buildDeliveryTaskFromStageConfig(cfg, destinationID)
		taskID, err := s.deliveryDAO.Create(ctx, task)
		return "destination_delivery_task", taskID, err
	default:
		return targetRefTypeForStage(cfg.StageType), 0, nil
	}
}

func (s *PipelineService) GetStageSteps(ctx context.Context, stageID uint) ([]MethodStepDetail, error) {
	stage, err := s.stageDAO.FindByID(ctx, stageID)
	if err != nil {
		return nil, err
	}
	steps, err := s.GetPipelineSteps(ctx, stage.PipelineID)
	if err != nil {
		return nil, err
	}
	result := make([]MethodStepDetail, 0, len(steps))
	for _, step := range steps {
		if step.Step.StageID == stage.ID {
			result = append(result, step)
		}
	}
	return result, nil
}

type PipelineRunResult struct {
	RunID        uint                   `json:"run_id"`
	TraceID      string                 `json:"trace_id"`
	StepOutputs  map[string]interface{} `json:"step_outputs"`
	SuccessCount int                    `json:"success_count"`
	FailedCount  int                    `json:"failed_count"`
}

func (s *PipelineService) RunPipeline(ctx context.Context, pipelineID uint) (*PipelineRunResult, error) {
	detail, err := s.GetPipeline(ctx, pipelineID)
	if err != nil {
		return nil, err
	}
	if !detail.Pipeline.Enabled {
		return nil, fmt.Errorf("pipeline %q is disabled", detail.Pipeline.Code)
	}
	traceID := newTraceID()
	now := time.Now()
	runID, err := s.pipelineRunDAO.Create(ctx, &model.PipelineRun{
		TraceID:     traceID,
		RunType:     "fetch",
		TriggerType: "manual",
		Status:      "running",
		TotalCount:  len(detail.Steps),
		StartedAt:   &model.TimeNormal{Time: now},
	})
	if err != nil {
		return nil, err
	}

	runtime := map[string]map[string]interface{}{}
	successCount := 0
	failedCount := 0
	disabledStageIDs := make(map[uint]bool, len(detail.Stages))
	for _, stage := range detail.Stages {
		if !stage.Stage.Enabled {
			disabledStageIDs[stage.Stage.ID] = true
		}
	}
	for _, step := range detail.Steps {
		if !step.Step.Enabled || disabledStageIDs[step.Step.StageID] {
			continue
		}
		outputs, runErr := s.runStep(ctx, runID, detail.Pipeline.ID, step, runtime)
		if runErr != nil {
			failedCount++
			_ = s.pipelineRunDAO.Finish(ctx, runID, "failed", successCount, failedCount, runErr.Error())
			return nil, runErr
		}
		runtime[step.Step.Code] = outputs
		successCount++
	}
	status := "success"
	if failedCount > 0 && successCount > 0 {
		status = "partial_success"
	} else if failedCount > 0 {
		status = "failed"
	}
	if err := s.pipelineRunDAO.Finish(ctx, runID, status, successCount, failedCount, ""); err != nil {
		return nil, err
	}
	return &PipelineRunResult{
		RunID:        runID,
		TraceID:      traceID,
		StepOutputs:  flattenRuntime(runtime),
		SuccessCount: successCount,
		FailedCount:  failedCount,
	}, nil
}

func (s *PipelineService) ListStepRuns(ctx context.Context, runID uint) ([]model.StepRun, error) {
	return s.stepRunDAO.FindByRunID(ctx, runID)
}

func (s *PipelineService) runStep(ctx context.Context, runID, pipelineID uint, detail MethodStepDetail, runtime map[string]map[string]interface{}) (map[string]interface{}, error) {
	inputs, err := resolveStepInputs(detail.Params, runtime)
	if err != nil {
		return nil, err
	}
	inputJSON, _ := json.Marshal(inputs)
	startedAt := time.Now()
	stepRun := &model.StepRun{
		RunID:               runID,
		PipelineID:          pipelineID,
		StepID:              detail.Step.ID,
		StepCode:            detail.Step.Code,
		MethodType:          detail.Step.MethodType,
		Status:              "running",
		InputJSON:           string(inputJSON),
		GeneratedConfigJSON: detail.Step.GeneratedConfigJSON,
		StartedAt:           &model.TimeNormal{Time: startedAt},
	}
	stepRunID, err := s.stepRunDAO.Create(ctx, stepRun)
	if err != nil {
		return nil, err
	}

	outputs, err := executeMethodStep(ctx, detail, inputs)
	outputJSON, _ := json.Marshal(outputs)
	if err != nil {
		_ = s.stepRunDAO.Finish(ctx, stepRunID, "failed", string(outputJSON), err.Error())
		return nil, err
	}
	if err := s.stepRunDAO.Finish(ctx, stepRunID, "success", string(outputJSON), ""); err != nil {
		return nil, err
	}
	return outputs, nil
}

func executeMethodStep(ctx context.Context, detail MethodStepDetail, inputs map[string]interface{}) (map[string]interface{}, error) {
	switch detail.Step.MethodType {
	case "request":
		return executeRequestStep(ctx, detail, inputs)
	case "bojun_signed_request":
		return executeBojunSignedRequestStep(ctx, inputs)
	case "extract":
		return executeExtractStep(detail, inputs)
	case "mapping":
		return executeMappingStep(detail, inputs)
	case "delivery":
		return executeDeliveryStep(ctx, detail, inputs)
	case "shanghai_mall_push":
		return executeShanghaiMallPushStep(ctx, inputs)
	default:
		return inputs, nil
	}
}

func executeBojunSignedRequestStep(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
	method := stringValue(inputs, "method", "")
	if method == "" {
		method = stringValue(scopedMap(inputs, "request"), "method", "")
	}
	if method == "" {
		return nil, fmt.Errorf("bojun signed request requires method")
	}
	return bojun.SendSignedRequest(ctx, method, scopedMap(inputs, "body"))
}

func executeRequestStep(ctx context.Context, detail MethodStepDetail, inputs map[string]interface{}) (map[string]interface{}, error) {
	requestURL, _ := inputs["url"].(string)
	if requestURL == "" {
		return nil, fmt.Errorf("request step %q requires url", detail.Step.Code)
	}
	method := strings.ToUpper(stringValue(inputs, "method", "GET"))
	parsedURL, err := url.Parse(requestURL)
	if err != nil {
		return nil, err
	}
	query := parsedURL.Query()
	for key, value := range scopedMap(inputs, "query") {
		query.Set(key, fmt.Sprintf("%v", value))
	}
	parsedURL.RawQuery = query.Encode()

	bodyBytes := []byte{}
	if body := scopedMap(inputs, "body"); len(body) > 0 {
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}
	timeout := time.Duration(timeoutSeconds(detail.Step.TimeoutSeconds)) * time.Second
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, method, parsedURL.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	for key, value := range scopedMap(inputs, "header") {
		req.Header.Set(key, fmt.Sprintf("%v", value))
	}
	if len(bodyBytes) > 0 && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := (&http.Client{Timeout: timeout}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	outputs := map[string]interface{}{
		"http_status":   resp.StatusCode,
		"response_body": string(respBody),
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return outputs, fmt.Errorf("request step %q returned http status %d", detail.Step.Code, resp.StatusCode)
	}
	var payload interface{}
	if len(respBody) > 0 && json.Unmarshal(respBody, &payload) == nil {
		captureOutputs(detail.Outputs, payload, outputs)
	}
	return outputs, nil
}

func executeExtractStep(detail MethodStepDetail, inputs map[string]interface{}) (map[string]interface{}, error) {
	payload := inputs["payload"]
	if payload == nil {
		payload = inputs["response_body"]
	}
	if text, ok := payload.(string); ok {
		var parsed interface{}
		if err := json.Unmarshal([]byte(text), &parsed); err == nil {
			payload = parsed
		}
	}
	outputs := map[string]interface{}{}
	captureOutputs(detail.Outputs, payload, outputs)
	return outputs, nil
}

func executeMappingStep(detail MethodStepDetail, inputs map[string]interface{}) (map[string]interface{}, error) {
	raw, ok := inputs["record"].(map[string]interface{})
	if !ok {
		raw = inputs
	}
	cfg, err := mappingConfigFromParams(detail.Params)
	if err != nil {
		return nil, err
	}
	mapped, err := transformconnector.ApplyMapping(raw, cfg)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"record": mapped}, nil
}

func executeDeliveryStep(ctx context.Context, detail MethodStepDetail, inputs map[string]interface{}) (map[string]interface{}, error) {
	requestURL, _ := inputs["url"].(string)
	if requestURL == "" {
		return map[string]interface{}{"skipped": true}, nil
	}
	body := "{}"
	if bodyMap := scopedMap(inputs, "body"); len(bodyMap) > 0 {
		bodyBytes, _ := json.Marshal(bodyMap)
		body = string(bodyBytes)
	}
	cfg := destinationconnector.Config{
		"url":              requestURL,
		"method":           stringValue(inputs, "method", http.MethodPost),
		"headers":          scopedMap(inputs, "header"),
		"payload_template": body,
		"timeout_seconds":  timeoutSeconds(detail.Step.TimeoutSeconds),
	}
	result, err := destinationconnector.HTTPPublisher{}.Publish(ctx, cfg, destinationconnector.CleanRecord{Content: inputs})
	outputs := map[string]interface{}{}
	if result != nil {
		outputs["http_status"] = result.HTTPStatus
		outputs["request_body"] = result.RequestBody
		outputs["response_body"] = result.ResponseBody
		outputs["success"] = result.Success
	}
	return outputs, err
}

func executeShanghaiMallPushStep(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
	target := stringValue(inputs, "target", "")
	if target == "" {
		target = stringValue(scopedMap(inputs, "request"), "target", "")
	}
	orderData := scopedMap(inputs, "order")
	if len(orderData) == 0 {
		orderData = inputs
	}
	order := shanghaimall.RetailOrder{
		DocNo:         firstString(orderData, "docno", "doc_no", "order_no"),
		OrderTypeCode: firstString(orderData, "order_type_code", "retailsaletype"),
		SaleTime:      firstString(orderData, "sale_time", "success_time", "tran_time"),
		Amount:        firstFloat(orderData, "amount", "tot_amt_actual", "total_amt_actual", "payment"),
		ListAmount:    firstFloat(orderData, "list_amount", "tot_amt_list", "total_amt_list", "total_fee"),
		Quantity:      firstInt(orderData, "quantity", "tot_qty", "total_qty"),
	}
	result, err := shanghaimall.Push(ctx, shanghaimall.Target(target), order)
	outputs := map[string]interface{}{}
	if result != nil {
		outputs["target"] = result.Target
		outputs["success"] = result.Success
		outputs["http_status"] = result.HTTPStatus
		outputs["request_body"] = result.RequestBody
		outputs["response_body"] = result.ResponseBody
		outputs["response_json"] = result.ResponseJSON
	}
	return outputs, err
}

func resolveStepInputs(params []model.MethodParam, runtime map[string]map[string]interface{}) (map[string]interface{}, error) {
	inputs := map[string]interface{}{}
	for _, param := range params {
		value, err := resolveParamRuntimeValue(param, runtime)
		if err != nil {
			return nil, err
		}
		if param.Location == "url" || param.Location == "request" || param.Location == "method" {
			inputs[param.Name] = value
			continue
		}
		if _, ok := inputs[param.Location]; !ok {
			inputs[param.Location] = map[string]interface{}{}
		}
		if scoped, ok := inputs[param.Location].(map[string]interface{}); ok {
			scoped[param.Name] = value
		}
	}
	return inputs, nil
}

func resolveParamRuntimeValue(param model.MethodParam, runtime map[string]map[string]interface{}) (interface{}, error) {
	switch strings.TrimSpace(param.ValueSource) {
	case "", "static":
		return convertParamValue(param.Value, param.ValueType)
	case "binding":
		return resolveBinding(param.Value, runtime)
	case "config":
		return config.Get(param.Value, ""), nil
	case "env":
		return os.Getenv(param.Value), nil
	case "secret":
		return "", nil
	case "time":
		format := param.Value
		if format == "" {
			format = time.RFC3339
		}
		return time.Now().Format(format), nil
	default:
		return nil, fmt.Errorf("unsupported value_source %q", param.ValueSource)
	}
}

func resolveBinding(path string, runtime map[string]map[string]interface{}) (interface{}, error) {
	if !stepOutputBindingPattern.MatchString(path) {
		return nil, fmt.Errorf("invalid binding %q", path)
	}
	parts := strings.Split(path, ".")
	outputs, ok := runtime[parts[1]]
	if !ok {
		return nil, fmt.Errorf("binding step %q has no outputs", parts[1])
	}
	value, ok := outputs[parts[3]]
	if !ok {
		return nil, fmt.Errorf("binding output %q not found", parts[3])
	}
	return value, nil
}

func mappingConfigFromParams(params []model.MethodParam) (transformconnector.MappingConfig, error) {
	cfg := transformconnector.MappingConfig{}
	for _, param := range params {
		switch param.Location {
		case "mapping":
			if param.Name == "table_name" {
				cfg.TableName = param.Value
			}
			if param.Name == "business_key_field" {
				cfg.BusinessKeyField = param.Value
			}
		case "field":
			cfg.Fields = append(cfg.Fields, transformconnector.FieldRule{
				Name:       param.Name,
				SourcePath: param.Value,
				Type:       defaultString(param.ValueType, "string"),
				Required:   param.Required,
			})
		}
	}
	return cfg, nil
}

func captureOutputs(outputs []model.MethodOutput, payload interface{}, target map[string]interface{}) {
	for _, output := range outputs {
		if output.SourcePath == "" {
			continue
		}
		if value := lookupPipelinePath(payload, strings.TrimPrefix(output.SourcePath, "$.")); value != nil {
			target[output.Name] = value
		}
	}
}

func lookupPipelinePath(payload interface{}, path string) interface{} {
	if path == "" {
		return payload
	}
	current := payload
	for _, part := range strings.Split(path, ".") {
		switch typed := current.(type) {
		case map[string]interface{}:
			current = typed[part]
		default:
			return nil
		}
	}
	return current
}

func scopedMap(input map[string]interface{}, key string) map[string]interface{} {
	if value, ok := input[key].(map[string]interface{}); ok {
		return value
	}
	return map[string]interface{}{}
}

func stringValue(input map[string]interface{}, key, fallback string) string {
	value, ok := input[key]
	if !ok || value == nil {
		return fallback
	}
	if text, ok := value.(string); ok && text != "" {
		return text
	}
	return fmt.Sprintf("%v", value)
}

func firstString(input map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value := stringFromAny(input[key]); value != "" {
			return value
		}
	}
	return ""
}

func firstFloat(input map[string]interface{}, keys ...string) float64 {
	for _, key := range keys {
		if value, ok := input[key]; ok && value != nil {
			return floatFromAny(value)
		}
	}
	return 0
}

func firstInt(input map[string]interface{}, keys ...string) int {
	for _, key := range keys {
		if value, ok := input[key]; ok && value != nil {
			return intFromAny(value)
		}
	}
	return 0
}

func flattenRuntime(runtime map[string]map[string]interface{}) map[string]interface{} {
	flattened := map[string]interface{}{}
	for stepCode, outputs := range runtime {
		flattened[stepCode] = outputs
	}
	return flattened
}

func paramsFromRequest(items []requestbody.MethodParamRequest) []model.MethodParam {
	params := make([]model.MethodParam, 0, len(items))
	for _, item := range items {
		params = append(params, model.MethodParam{
			Location:    strings.TrimSpace(item.Location),
			Name:        strings.TrimSpace(item.Name),
			ValueSource: strings.TrimSpace(item.ValueSource),
			Value:       item.Value,
			ValueType:   strings.TrimSpace(item.ValueType),
			Required:    item.Required,
			Secret:      item.Secret,
			Description: item.Description,
			OrderIndex:  item.OrderIndex,
		})
	}
	return params
}

func outputsFromRequest(items []requestbody.MethodOutputRequest) []model.MethodOutput {
	outputs := make([]model.MethodOutput, 0, len(items))
	for _, item := range items {
		outputs = append(outputs, model.MethodOutput{
			Name:        strings.TrimSpace(item.Name),
			SourcePath:  strings.TrimSpace(item.SourcePath),
			ValueType:   strings.TrimSpace(item.ValueType),
			Required:    item.Required,
			Description: item.Description,
			OrderIndex:  item.OrderIndex,
		})
	}
	return outputs
}

func (s *PipelineService) ensureDefaultStages(ctx context.Context, pipelineID uint) error {
	stages, err := s.stageDAO.FindByPipelineID(ctx, pipelineID)
	if err != nil {
		return err
	}
	existingStageTypes := make(map[string]bool, len(stages))
	for _, stage := range stages {
		existingStageTypes[stage.StageType] = true
	}
	for _, stage := range defaultPipelineStages(pipelineID) {
		if existingStageTypes[stage.StageType] {
			continue
		}
		if _, err := s.stageDAO.Create(ctx, &stage); err != nil {
			currentStages, findErr := s.stageDAO.FindByPipelineID(ctx, pipelineID)
			if findErr != nil || !containsStageType(currentStages, stage.StageType) {
				return err
			}
		}
		existingStageTypes[stage.StageType] = true
	}
	return nil
}

func containsStageType(stages []model.PipelineStage, stageType string) bool {
	for _, stage := range stages {
		if stage.StageType == stageType {
			return true
		}
	}
	return false
}

func (s *PipelineService) normalizeLegacyStepStages(ctx context.Context, pipelineID uint, steps []model.MethodStep) ([]model.MethodStep, error) {
	hasLegacyStep := false
	for _, step := range steps {
		if step.StageID == 0 {
			hasLegacyStep = true
			break
		}
	}
	if !hasLegacyStep {
		return steps, nil
	}
	if err := s.ensureDefaultStages(ctx, pipelineID); err != nil {
		return nil, err
	}
	stages, err := s.stageDAO.FindByPipelineID(ctx, pipelineID)
	if err != nil {
		return nil, err
	}
	defaultStageIDs := make(map[string]uint, len(stages))
	for _, stage := range stages {
		defaultStageIDs[stage.StageType] = stage.ID
	}
	normalized := append([]model.MethodStep(nil), steps...)
	for index := range normalized {
		if normalized[index].StageID != 0 {
			continue
		}
		stageType := defaultStageTypeForMethod(normalized[index].MethodType)
		stageID := defaultStageIDs[stageType]
		if stageID == 0 {
			return nil, fmt.Errorf("default stage %q not found for pipeline %d", stageType, pipelineID)
		}
		normalized[index].StageID = stageID
	}
	return normalized, nil
}

func (s *PipelineService) findDefaultStageForMethod(ctx context.Context, pipelineID uint, methodType string) (*model.PipelineStage, error) {
	if err := s.ensureDefaultStages(ctx, pipelineID); err != nil {
		return nil, err
	}
	stageType := defaultStageTypeForMethod(methodType)
	stages, err := s.stageDAO.FindByPipelineID(ctx, pipelineID)
	if err != nil {
		return nil, err
	}
	for _, stage := range stages {
		if stage.StageType == stageType {
			return &stage, nil
		}
	}
	return nil, fmt.Errorf("default stage %q not found for pipeline %d", stageType, pipelineID)
}

func defaultPipelineStages(pipelineID uint) []model.PipelineStage {
	return []model.PipelineStage{
		{PipelineID: pipelineID, StageType: "fetch", Name: "数据获取", OrderIndex: 1, Enabled: true},
		{PipelineID: pipelineID, StageType: "process", Name: "数据处理", OrderIndex: 2, Enabled: true},
		{PipelineID: pipelineID, StageType: "push", Name: "数据推送", OrderIndex: 3, Enabled: true},
		{PipelineID: pipelineID, StageType: "log", Name: "日志记录", OrderIndex: 4, Enabled: true},
	}
}

func defaultStageTypeForMethod(methodType string) string {
	switch methodType {
	case "request", "extract", "bojun_signed_request":
		return "fetch"
	case "delivery":
		return "push"
	case "shanghai_mall_push":
		return "push"
	case "log":
		return "log"
	default:
		return "process"
	}
}

func validateStageMethodType(stageType, methodType string) error {
	allowed := map[string]map[string]bool{
		"fetch": {
			"request":              true,
			"bojun_signed_request": true,
			"extract":              true,
			"db_query":             true,
		},
		"process": {
			"mapping":              true,
			"validate":             true,
			"db_query":             true,
			"db_write":             true,
			"template":             true,
			"request":              true,
			"bojun_signed_request": true,
		},
		"push": {
			"template":           true,
			"delivery":           true,
			"request":            true,
			"shanghai_mall_push": true,
		},
		"log": {
			"log":      true,
			"db_write": true,
			"delivery": true,
		},
	}
	if allowed[stageType] == nil {
		return fmt.Errorf("unsupported stage_type %q", stageType)
	}
	if !allowed[stageType][methodType] {
		return fmt.Errorf("method_type %q is not allowed in stage_type %q", methodType, stageType)
	}
	return nil
}

func buildStageGeneratedConfigMap(stage model.PipelineStage, steps []MethodStepDetail) (map[string]interface{}, error) {
	stepConfigs := make([]map[string]interface{}, 0, len(steps))
	for _, step := range steps {
		cfg, err := BuildGeneratedStepConfigMap(MethodStepDefinition{Step: step.Step, Params: step.Params, Outputs: step.Outputs})
		if err != nil {
			return nil, err
		}
		stepConfigs = append(stepConfigs, cfg)
	}
	return map[string]interface{}{
		"stage_id":        stage.ID,
		"stage_type":      stage.StageType,
		"stage_name":      stage.Name,
		"target_ref_type": targetRefTypeForStage(stage.StageType),
		"steps":           stepConfigs,
	}, nil
}

func targetRefTypeForStage(stageType string) string {
	switch stageType {
	case "fetch":
		return "source_definition"
	case "process":
		return "transform_rule"
	case "push":
		return "destination_delivery_task"
	case "log":
		return "pipeline_step_log"
	default:
		return ""
	}
}

func buildSourceDefinitionFromStageConfig(cfg *model.StageGeneratedConfig) *model.SourceDefinition {
	code := legacyConfigCode("source", cfg)
	return &model.SourceDefinition{
		Name:           legacyConfigName("数据获取", cfg),
		Code:           code,
		SourceType:     "api_poll",
		Enabled:        true,
		AuthType:       "pipeline_stage",
		ConfigJSON:     cfg.GeneratedConfigJSON,
		SchemaJSON:     "{}",
		DedupeKeys:     "[]",
		SourceQueryKey: code,
	}
}

func buildTransformRuleFromStageConfig(cfg *model.StageGeneratedConfig) *model.TransformRule {
	return &model.TransformRule{
		SourceID:   0,
		Name:       legacyConfigName("数据处理", cfg),
		RuleType:   "mapping",
		OrderIndex: cfg.Version,
		ConfigJSON: cfg.GeneratedConfigJSON,
		Enabled:    true,
	}
}

func buildDestinationDefinitionFromStageConfig(cfg *model.StageGeneratedConfig) *model.DestinationDefinition {
	return &model.DestinationDefinition{
		Name:            legacyConfigName("数据推送", cfg),
		Code:            legacyConfigCode("destination", cfg),
		DestinationType: "http",
		ConfigJSON:      cfg.GeneratedConfigJSON,
		Enabled:         true,
	}
}

func buildDeliveryTaskFromStageConfig(cfg *model.StageGeneratedConfig, destinationID uint) *model.DeliveryTask {
	return &model.DeliveryTask{
		Name:            legacyConfigName("推送任务", cfg),
		SourceID:        0,
		CleanTable:      fmt.Sprintf("pipeline_%d_stage_%d_clean", cfg.PipelineID, cfg.StageID),
		DestinationID:   destinationID,
		TriggerType:     "manual",
		FilterJSON:      "{}",
		PayloadTemplate: cfg.GeneratedConfigJSON,
		Enabled:         true,
	}
}

func legacyConfigCode(prefix string, cfg *model.StageGeneratedConfig) string {
	return fmt.Sprintf("%s_pipeline_%d_stage_%d_v%d", prefix, cfg.PipelineID, cfg.StageID, cfg.Version)
}

func legacyConfigName(label string, cfg *model.StageGeneratedConfig) string {
	return fmt.Sprintf("%s配置 P%d-S%d-v%d", label, cfg.PipelineID, cfg.StageID, cfg.Version)
}
