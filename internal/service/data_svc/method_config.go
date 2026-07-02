package data_svc

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"gin-biz-web-api/model"
)

var stepOutputBindingPattern = regexp.MustCompile(`^steps\.[A-Za-z0-9_-]+\.outputs\.[A-Za-z0-9_-]+$`)

type MethodStepDefinition struct {
	Step    model.MethodStep     `json:"step"`
	Params  []model.MethodParam  `json:"params"`
	Outputs []model.MethodOutput `json:"outputs"`
}

func BuildGeneratedStepConfig(def MethodStepDefinition) (string, error) {
	config, err := BuildGeneratedStepConfigMap(def)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("encode generated step config: %w", err)
	}
	return string(data), nil
}

func BuildGeneratedStepConfigMap(def MethodStepDefinition) (map[string]interface{}, error) {
	methodType := strings.TrimSpace(def.Step.MethodType)
	if methodType == "" {
		return nil, fmt.Errorf("method_type is required")
	}

	config := map[string]interface{}{
		"step_code":   strings.TrimSpace(def.Step.Code),
		"method_type": methodType,
		"timeout":     timeoutSeconds(def.Step.TimeoutSeconds),
	}

	switch methodType {
	case "request", "delivery":
		if err := applyRequestLikeConfig(config, def); err != nil {
			return nil, err
		}
	case "mapping":
		if err := applyMappingConfig(config, def); err != nil {
			return nil, err
		}
	case "extract":
		if err := applyExtractConfig(config, def); err != nil {
			return nil, err
		}
	default:
		if err := applyGenericConfig(config, def); err != nil {
			return nil, err
		}
	}

	if len(def.Outputs) > 0 {
		config["captures"] = outputCaptures(def.Outputs)
	}
	return config, nil
}

func applyRequestLikeConfig(config map[string]interface{}, def MethodStepDefinition) error {
	method := "GET"
	if param, ok := findParam(def.Params, "request", "method"); ok && strings.TrimSpace(param.Value) != "" {
		method = strings.ToUpper(strings.TrimSpace(param.Value))
	}
	if param, ok := findParam(def.Params, "method", "method"); ok && strings.TrimSpace(param.Value) != "" {
		method = strings.ToUpper(strings.TrimSpace(param.Value))
	}
	config["method"] = method

	urlParam, ok := findParam(def.Params, "url", "url")
	if !ok {
		return fmt.Errorf("request step %q requires url param", def.Step.Code)
	}
	value, err := paramValueSpec(urlParam)
	if err != nil {
		return err
	}
	config["url"] = value

	if params, err := buildNamedParams(def.Params, "query"); err != nil {
		return err
	} else if len(params) > 0 {
		config["query_params"] = params
	}
	if params, err := buildNamedParams(def.Params, "header"); err != nil {
		return err
	} else if len(params) > 0 {
		config["headers"] = params
	}
	if params, err := buildNamedParams(def.Params, "body"); err != nil {
		return err
	} else if len(params) > 0 {
		config["body_params"] = params
	}
	if param, ok := findParam(def.Params, "response", "records_path"); ok {
		config["records_path"] = strings.TrimSpace(param.Value)
	}
	if param, ok := findParam(def.Params, "response", "success_path"); ok {
		config["success_path"] = strings.TrimSpace(param.Value)
	}
	return nil
}

func applyMappingConfig(config map[string]interface{}, def MethodStepDefinition) error {
	fields := make([]map[string]interface{}, 0)
	for _, param := range def.Params {
		if param.Location != "field" {
			continue
		}
		field := map[string]interface{}{
			"name":        param.Name,
			"source_path": param.Value,
			"type":        defaultString(param.ValueType, "string"),
			"required":    param.Required,
		}
		fields = append(fields, field)
	}
	config["fields"] = fields
	if param, ok := findParam(def.Params, "mapping", "table_name"); ok {
		config["table_name"] = param.Value
	}
	if param, ok := findParam(def.Params, "mapping", "business_key_field"); ok {
		config["business_key_field"] = param.Value
	}
	return nil
}

func applyExtractConfig(config map[string]interface{}, def MethodStepDefinition) error {
	param, ok := findParam(def.Params, "response", "records_path")
	if !ok || strings.TrimSpace(param.Value) == "" {
		return fmt.Errorf("extract step %q requires response.records_path", def.Step.Code)
	}
	config["records_path"] = strings.TrimSpace(param.Value)
	return nil
}

func applyGenericConfig(config map[string]interface{}, def MethodStepDefinition) error {
	params := make([]map[string]interface{}, 0, len(def.Params))
	for _, param := range def.Params {
		value, err := paramValueSpec(param)
		if err != nil {
			return err
		}
		params = append(params, map[string]interface{}{
			"location": param.Location,
			"name":     param.Name,
			"value":    value,
		})
	}
	config["params"] = params
	return nil
}

func buildNamedParams(params []model.MethodParam, location string) ([]map[string]interface{}, error) {
	result := make([]map[string]interface{}, 0)
	for _, param := range params {
		if param.Location != location {
			continue
		}
		value, err := paramValueSpec(param)
		if err != nil {
			return nil, err
		}
		result = append(result, map[string]interface{}{
			"name":        param.Name,
			"value":       value,
			"required":    param.Required,
			"secret":      param.Secret,
			"description": param.Description,
		})
	}
	return result, nil
}

func paramValueSpec(param model.MethodParam) (interface{}, error) {
	source := strings.TrimSpace(param.ValueSource)
	if source == "" {
		source = "static"
	}
	value := strings.TrimSpace(param.Value)
	switch source {
	case "static":
		return convertParamValue(value, param.ValueType)
	case "binding":
		if !stepOutputBindingPattern.MatchString(value) {
			return nil, fmt.Errorf("invalid binding %q for param %q", value, param.Name)
		}
		return map[string]interface{}{"source": "binding", "path": value}, nil
	case "config":
		return map[string]interface{}{"source": "config", "path": value}, nil
	case "env":
		return map[string]interface{}{"source": "env", "name": value}, nil
	case "secret":
		return map[string]interface{}{"source": "secret", "name": value}, nil
	case "time":
		return map[string]interface{}{"source": "time", "format": value}, nil
	default:
		return nil, fmt.Errorf("unsupported value_source %q for param %q", source, param.Name)
	}
}

func convertParamValue(value, valueType string) (interface{}, error) {
	switch strings.ToLower(strings.TrimSpace(valueType)) {
	case "", "string":
		return value, nil
	case "int", "integer":
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse int value %q: %w", value, err)
		}
		return parsed, nil
	case "bool", "boolean":
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return nil, fmt.Errorf("parse bool value %q: %w", value, err)
		}
		return parsed, nil
	case "json", "object", "array":
		var parsed interface{}
		if err := json.Unmarshal([]byte(value), &parsed); err != nil {
			return nil, fmt.Errorf("parse json value for param: %w", err)
		}
		return parsed, nil
	default:
		return value, nil
	}
}

func outputCaptures(outputs []model.MethodOutput) []map[string]interface{} {
	captures := make([]map[string]interface{}, 0, len(outputs))
	for _, output := range outputs {
		captures = append(captures, map[string]interface{}{
			"name":        output.Name,
			"source_path": output.SourcePath,
			"value_type":  defaultString(output.ValueType, "string"),
			"required":    output.Required,
			"description": output.Description,
		})
	}
	return captures
}

func findParam(params []model.MethodParam, location, name string) (model.MethodParam, bool) {
	for _, param := range params {
		if param.Location == location && param.Name == name {
			return param, true
		}
	}
	return model.MethodParam{}, false
}

func timeoutSeconds(value int) int {
	if value <= 0 {
		return 30
	}
	return value
}
