package data_ctrl

import (
	"encoding/json"
	"strings"

	"gin-biz-web-api/model"
)

type sourceDefinitionResponse struct {
	ID             uint   `json:"id"`
	Name           string `json:"name"`
	Code           string `json:"code"`
	SourceType     string `json:"source_type"`
	Enabled        bool   `json:"enabled"`
	AuthType       string `json:"auth_type"`
	ConfigJSON     string `json:"config_json"`
	HasSecret      bool   `json:"has_secret"`
	SchemaJSON     string `json:"schema_json"`
	DedupeKeys     string `json:"dedupe_keys"`
	SourceQueryKey string `json:"source_query_key"`
}

type transformRuleResponse struct {
	ID         uint   `json:"id"`
	SourceID   uint   `json:"source_id"`
	Name       string `json:"name"`
	RuleType   string `json:"rule_type"`
	OrderIndex int    `json:"order_index"`
	ConfigJSON string `json:"config_json"`
	HasSecret  bool   `json:"has_secret"`
	Enabled    bool   `json:"enabled"`
}

func safeSourceDefinition(source model.SourceDefinition) sourceDefinitionResponse {
	config, hasSecret := redactConfigJSON(source.ConfigJSON)
	return sourceDefinitionResponse{ID: source.ID, Name: source.Name, Code: source.Code, SourceType: source.SourceType, Enabled: source.Enabled, AuthType: source.AuthType, ConfigJSON: config, HasSecret: hasSecret, SchemaJSON: source.SchemaJSON, DedupeKeys: source.DedupeKeys, SourceQueryKey: source.SourceQueryKey}
}

func safeSourceDefinitions(sources []model.SourceDefinition) []sourceDefinitionResponse {
	result := make([]sourceDefinitionResponse, 0, len(sources))
	for _, source := range sources {
		result = append(result, safeSourceDefinition(source))
	}
	return result
}

func safeTransformRule(rule model.TransformRule) transformRuleResponse {
	config, hasSecret := redactConfigJSON(rule.ConfigJSON)
	return transformRuleResponse{ID: rule.ID, SourceID: rule.SourceID, Name: rule.Name, RuleType: rule.RuleType, OrderIndex: rule.OrderIndex, ConfigJSON: config, HasSecret: hasSecret, Enabled: rule.Enabled}
}

func safeTransformRules(rules []model.TransformRule) []transformRuleResponse {
	result := make([]transformRuleResponse, 0, len(rules))
	for _, rule := range rules {
		result = append(result, safeTransformRule(rule))
	}
	return result
}

func redactConfigJSON(config string) (string, bool) {
	if !json.Valid([]byte(config)) {
		return "{}", true
	}
	var value interface{}
	if json.Unmarshal([]byte(config), &value) != nil {
		return "{}", true
	}
	redacted, hasSecret := redactConfigValue(value, "")
	encoded, err := json.Marshal(redacted)
	if err != nil {
		return "{}", true
	}
	return string(encoded), hasSecret
}

func redactConfigValue(value interface{}, key string) (interface{}, bool) {
	if sensitiveConfigKey(key) {
		return "[已隐藏]", true
	}
	switch typed := value.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{}, len(typed))
		hasSecret := false
		for childKey, childValue := range typed {
			clean, hidden := redactConfigValue(childValue, childKey)
			result[childKey] = clean
			hasSecret = hasSecret || hidden
		}
		return result, hasSecret
	case []interface{}:
		result := make([]interface{}, len(typed))
		hasSecret := false
		for index, childValue := range typed {
			clean, hidden := redactConfigValue(childValue, key)
			result[index] = clean
			hasSecret = hasSecret || hidden
		}
		return result, hasSecret
	default:
		return value, false
	}
}

func sensitiveConfigKey(key string) bool {
	key = strings.ToLower(strings.ReplaceAll(key, "-", "_"))
	return strings.Contains(key, "secret") || strings.Contains(key, "token") || strings.Contains(key, "password") || strings.Contains(key, "authorization") || strings.Contains(key, "api_key") || strings.Contains(key, "private_key")
}
