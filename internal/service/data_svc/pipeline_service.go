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
	"strings"
	"time"

	destinationconnector "gin-biz-web-api/connector/destination"
	transformconnector "gin-biz-web-api/connector/transform"
	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/config"
)

type PipelineService struct {
	pipelineDAO    *data_dao.PipelineDefinitionDAO
	stepDAO        *data_dao.MethodStepDAO
	paramDAO       *data_dao.MethodParamDAO
	outputDAO      *data_dao.MethodOutputDAO
	stepRunDAO     *data_dao.StepRunDAO
	pipelineRunDAO *data_dao.PipelineRunDAO
}

func NewPipelineService() *PipelineService {
	return &PipelineService{
		pipelineDAO:    data_dao.NewPipelineDefinitionDAO(),
		stepDAO:        data_dao.NewMethodStepDAO(),
		paramDAO:       data_dao.NewMethodParamDAO(),
		outputDAO:      data_dao.NewMethodOutputDAO(),
		stepRunDAO:     data_dao.NewStepRunDAO(),
		pipelineRunDAO: data_dao.NewPipelineRunDAO(),
	}
}

type MethodStepDetail struct {
	Step    model.MethodStep     `json:"step"`
	Params  []model.MethodParam  `json:"params"`
	Outputs []model.MethodOutput `json:"outputs"`
}

type PipelineDetail struct {
	Pipeline model.PipelineDefinition `json:"pipeline"`
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
	return pipeline, err
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
	return &PipelineDetail{Pipeline: *pipeline, Steps: steps}, nil
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
	step, params, outputs, err := s.buildStepModel(pipelineID, req)
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
	step, params, outputs, err := s.buildStepModel(current.PipelineID, (*requestbody.MethodStepCreateRequest)(req))
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

func (s *PipelineService) buildStepModel(pipelineID uint, req *requestbody.MethodStepCreateRequest) (*model.MethodStep, []model.MethodParam, []model.MethodOutput, error) {
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	step := &model.MethodStep{
		PipelineID:     pipelineID,
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

func (s *PipelineService) GetPipelineSteps(ctx context.Context, pipelineID uint) ([]MethodStepDetail, error) {
	steps, err := s.stepDAO.FindByPipelineID(ctx, pipelineID)
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
	details, err := s.hydrateStepDetails(ctx, []model.MethodStep{*step})
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
	steps := make([]map[string]interface{}, 0, len(detail.Steps))
	for _, step := range detail.Steps {
		cfg, err := BuildGeneratedStepConfigMap(MethodStepDefinition{Step: step.Step, Params: step.Params, Outputs: step.Outputs})
		if err != nil {
			return nil, err
		}
		steps = append(steps, cfg)
	}
	return map[string]interface{}{"pipeline": detail.Pipeline, "steps": steps}, nil
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
	for _, step := range detail.Steps {
		if !step.Step.Enabled {
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
	case "extract":
		return executeExtractStep(detail, inputs)
	case "mapping":
		return executeMappingStep(detail, inputs)
	case "delivery":
		return executeDeliveryStep(ctx, detail, inputs)
	default:
		return inputs, nil
	}
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
