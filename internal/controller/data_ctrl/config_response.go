package data_ctrl

import (
	"strings"

	"gin-biz-web-api/internal/configsecret"
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

type destinationDefinitionResponse struct {
	ID              uint   `json:"id"`
	Name            string `json:"name"`
	Code            string `json:"code"`
	DestinationType string `json:"destination_type"`
	ConfigJSON      string `json:"config_json"`
	HasSecret       bool   `json:"has_secret"`
	Enabled         bool   `json:"enabled"`
}

type deliveryLogResponse struct {
	ID              uint   `json:"id"`
	TraceID         string `json:"trace_id"`
	RunID           uint   `json:"run_id"`
	SourceCode      string `json:"source_code"`
	DestinationCode string `json:"destination_code"`
	DestinationName string `json:"destination_name"`
	DestinationID   uint   `json:"destination_id"`
	CleanRecordID   uint   `json:"clean_record_id"`
	BusinessKey     string `json:"business_key"`
	HTTPStatus      int    `json:"http_status"`
	Success         bool   `json:"success"`
	ErrorMessage    string `json:"error_message"`
	ResponseSummary string `json:"response_summary"`
	RetryCount      int    `json:"retry_count"`
	SentAt          any    `json:"sent_at"`
}

// pipelineRunResponse is the public management projection. In particular it
// intentionally omits ErrorMessage, which can contain internal or third-party details.
type pipelineRunResponse struct {
	ID            uint              `json:"id"`
	TraceID       string            `json:"trace_id"`
	RunType       string            `json:"run_type"`
	TriggerType   string            `json:"trigger_type"`
	SourceID      uint              `json:"source_id"`
	DestinationID uint              `json:"destination_id"`
	Status        string            `json:"status"`
	TotalCount    int               `json:"total_count"`
	SuccessCount  int               `json:"success_count"`
	FailedCount   int               `json:"failed_count"`
	StartedAt     *model.TimeNormal `json:"started_at"`
	FinishedAt    *model.TimeNormal `json:"finished_at"`
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

func safeDestinationDefinition(destination model.DestinationDefinition) destinationDefinitionResponse {
	config, hasSecret := redactConfigJSON(destination.ConfigJSON)
	return destinationDefinitionResponse{ID: destination.ID, Name: destination.Name, Code: destination.Code, DestinationType: destination.DestinationType, ConfigJSON: config, HasSecret: hasSecret, Enabled: destination.Enabled}
}

func safeDestinationDefinitions(destinations []model.DestinationDefinition) []destinationDefinitionResponse {
	result := make([]destinationDefinitionResponse, 0, len(destinations))
	for _, destination := range destinations {
		result = append(result, safeDestinationDefinition(destination))
	}
	return result
}

func safeDeliveryLogs(logs []model.DeliveryLog) []deliveryLogResponse {
	result := make([]deliveryLogResponse, 0, len(logs))
	for _, log := range logs {
		result = append(result, deliveryLogResponse{ID: log.ID, TraceID: log.TraceID, RunID: log.RunID, SourceCode: log.SourceCode, DestinationCode: log.DestinationCode, DestinationName: log.DestinationName, DestinationID: log.DestinationID, CleanRecordID: log.CleanRecordID, BusinessKey: log.BusinessKey, HTTPStatus: log.HTTPStatus, Success: log.Success, ErrorMessage: safeDeliveryLogText(log.ErrorMessage), ResponseSummary: safeDeliveryLogText(log.ResponseSummary), RetryCount: log.RetryCount, SentAt: log.SentAt})
	}
	return result
}

func safePipelineRuns(runs []model.PipelineRun) []pipelineRunResponse {
	result := make([]pipelineRunResponse, 0, len(runs))
	for _, run := range runs {
		result = append(result, pipelineRunResponse{
			ID: run.ID, TraceID: run.TraceID, RunType: run.RunType, TriggerType: run.TriggerType,
			SourceID: run.SourceID, DestinationID: run.DestinationID, Status: run.Status,
			TotalCount: run.TotalCount, SuccessCount: run.SuccessCount, FailedCount: run.FailedCount,
			StartedAt: run.StartedAt, FinishedAt: run.FinishedAt,
		})
	}
	return result
}

func safeDeliveryLogText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	lowerValue := strings.ToLower(value)
	if strings.Contains(lowerValue, "token") || strings.Contains(lowerValue, "secret") || strings.Contains(lowerValue, "password") || strings.Contains(lowerValue, "authorization") || strings.Contains(lowerValue, "api_key") || strings.Contains(lowerValue, "private_key") {
		return "第三方响应包含敏感信息，详情已隐藏。"
	}
	if len(value) > 240 {
		return value[:240] + "…"
	}
	return value
}

func redactConfigJSON(config string) (string, bool) {
	return configsecret.RedactJSON(config)
}
