package requestbody

type SourceDefinitionCreateRequest struct {
	Name           string `json:"name" binding:"required,max=100"`
	Code           string `json:"code" binding:"required,max=100"`
	SourceType     string `json:"source_type" binding:"required,max=50"`
	Enabled        *bool  `json:"enabled"`
	AuthType       string `json:"auth_type" binding:"max=50"`
	ConfigJSON     string `json:"config_json"`
	SchemaJSON     string `json:"schema_json"`
	DedupeKeys     string `json:"dedupe_keys"`
	SourceQueryKey string `json:"source_query_key" binding:"max=100"`
}
