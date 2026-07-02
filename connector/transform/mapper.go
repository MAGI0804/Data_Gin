package transform

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type FieldRule struct {
	Name       string        `json:"name"`
	SourcePath string        `json:"source_path"`
	Type       string        `json:"type"`
	Required   bool          `json:"required"`
	Default    interface{}   `json:"default"`
	Transform  string        `json:"transform"`
	Enum       []interface{} `json:"enum"`
}

type MappingConfig struct {
	Fields []FieldRule `json:"fields"`
}

func ApplyMapping(raw map[string]interface{}, cfg MappingConfig) (map[string]interface{}, error) {
	clean := make(map[string]interface{}, len(cfg.Fields))

	for _, field := range cfg.Fields {
		value := Lookup(raw, field.SourcePath)
		if value == nil {
			value = field.Default
		}

		if field.Required && isEmpty(value) {
			return nil, fmt.Errorf("field %q is required", field.Name)
		}
		if isEmpty(value) {
			clean[field.Name] = value
			continue
		}

		converted, err := convertValue(value, field.Type)
		if err != nil {
			return nil, fmt.Errorf("convert field %q: %w", field.Name, err)
		}

		transformed, err := applyTransform(converted, field.Transform)
		if err != nil {
			return nil, fmt.Errorf("transform field %q: %w", field.Name, err)
		}

		if len(field.Enum) > 0 && !containsEnum(field.Enum, transformed) {
			return nil, fmt.Errorf("field %q value %v is not allowed", field.Name, transformed)
		}

		clean[field.Name] = transformed
	}

	return clean, nil
}

func Lookup(raw map[string]interface{}, path string) interface{} {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "$.")
	if path == "" {
		return raw
	}

	current := interface{}(raw)
	for _, part := range strings.Split(path, ".") {
		object, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		current = object[part]
	}
	return current
}

func convertValue(value interface{}, valueType string) (interface{}, error) {
	switch strings.ToLower(valueType) {
	case "", "any":
		return value, nil
	case "string":
		return fmt.Sprintf("%v", value), nil
	case "int", "integer":
		return toInt(value)
	case "decimal", "float", "number":
		return toFloat(value)
	case "bool", "boolean":
		return toBool(value)
	default:
		return nil, fmt.Errorf("unsupported type %q", valueType)
	}
}

func applyTransform(value interface{}, transform string) (interface{}, error) {
	transform = strings.TrimSpace(transform)
	if transform == "" {
		return value, nil
	}

	divideMatch := regexp.MustCompile(`^divide:(\d+(\.\d+)?)$`).FindStringSubmatch(transform)
	if len(divideMatch) > 0 {
		number, err := toFloat(value)
		if err != nil {
			return nil, err
		}
		divisor, err := strconv.ParseFloat(divideMatch[1], 64)
		if err != nil {
			return nil, err
		}
		if divisor == 0 {
			return nil, fmt.Errorf("divide by zero")
		}
		return number / divisor, nil
	}

	return nil, fmt.Errorf("unsupported transform %q", transform)
}

func toInt(value interface{}) (int64, error) {
	switch typed := value.(type) {
	case int:
		return int64(typed), nil
	case int64:
		return typed, nil
	case float64:
		return int64(typed), nil
	case string:
		return strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to int", value)
	}
}

func toFloat(value interface{}) (float64, error) {
	switch typed := value.(type) {
	case int:
		return float64(typed), nil
	case int64:
		return float64(typed), nil
	case float64:
		return typed, nil
	case string:
		return strconv.ParseFloat(strings.TrimSpace(typed), 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to float", value)
	}
}

func toBool(value interface{}) (bool, error) {
	switch typed := value.(type) {
	case bool:
		return typed, nil
	case string:
		return strconv.ParseBool(strings.TrimSpace(typed))
	default:
		return false, fmt.Errorf("cannot convert %T to bool", value)
	}
}

func isEmpty(value interface{}) bool {
	if value == nil {
		return true
	}
	if str, ok := value.(string); ok {
		return strings.TrimSpace(str) == ""
	}
	return false
}

func containsEnum(values []interface{}, target interface{}) bool {
	targetText := fmt.Sprintf("%v", target)
	for _, value := range values {
		if fmt.Sprintf("%v", value) == targetText {
			return true
		}
	}
	return false
}
