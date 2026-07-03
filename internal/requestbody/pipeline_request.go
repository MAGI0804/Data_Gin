package requestbody

type PipelineCreateRequest struct {
	Name        string `json:"name" binding:"required,max=100"`
	Code        string `json:"code" binding:"required,max=100"`
	Description string `json:"description"`
	Enabled     *bool  `json:"enabled"`
}

type PipelineUpdateRequest PipelineCreateRequest

type PipelineStageCreateRequest struct {
	StageType  string `json:"stage_type" binding:"required,max=50"`
	Name       string `json:"name" binding:"required,max=100"`
	OrderIndex int    `json:"order_index"`
	Enabled    *bool  `json:"enabled"`
}

type PipelineStageUpdateRequest PipelineStageCreateRequest

type MethodParamRequest struct {
	Location    string `json:"location" binding:"required,max=50"`
	Name        string `json:"name" binding:"required,max=100"`
	ValueSource string `json:"value_source" binding:"required,max=50"`
	Value       string `json:"value"`
	ValueType   string `json:"value_type" binding:"max=50"`
	Required    bool   `json:"required"`
	Secret      bool   `json:"secret"`
	Description string `json:"description"`
	OrderIndex  int    `json:"order_index"`
}

type MethodOutputRequest struct {
	Name        string `json:"name" binding:"required,max=100"`
	SourcePath  string `json:"source_path" binding:"max=255"`
	ValueType   string `json:"value_type" binding:"max=50"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
	OrderIndex  int    `json:"order_index"`
}

type MethodStepCreateRequest struct {
	StageID        uint                  `json:"stage_id"`
	Code           string                `json:"code" binding:"required,max=100"`
	Name           string                `json:"name" binding:"required,max=100"`
	MethodType     string                `json:"method_type" binding:"required,max=50"`
	OrderIndex     int                   `json:"order_index"`
	Enabled        *bool                 `json:"enabled"`
	TimeoutSeconds int                   `json:"timeout_seconds"`
	Params         []MethodParamRequest  `json:"params"`
	Outputs        []MethodOutputRequest `json:"outputs"`
}

type MethodStepUpdateRequest MethodStepCreateRequest
