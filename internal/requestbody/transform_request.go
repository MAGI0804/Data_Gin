package requestbody

type TransformRuleCreateRequest struct {
	SourceID   uint   `json:"source_id" binding:"required,min=1"`
	Name       string `json:"name" binding:"required,max=100"`
	RuleType   string `json:"rule_type" binding:"required,max=50"`
	OrderIndex int    `json:"order_index"`
	ConfigJSON string `json:"config_json" binding:"required"`
	Enabled    *bool  `json:"enabled"`
}

type TransformRuleUpdateRequest TransformRuleCreateRequest

type TransformRuleTestRequest struct {
	RawContent map[string]interface{} `json:"raw_content" binding:"required"`
	ConfigJSON string                 `json:"config_json" binding:"required"`
}
