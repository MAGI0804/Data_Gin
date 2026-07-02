package model

// PipelineDefinition describes a configurable business pipeline.
type PipelineDefinition struct {
	BaseModel

	Name        string `gorm:"column:name;size:100;not null" json:"name"`
	Code        string `gorm:"column:code;size:100;not null;uniqueIndex" json:"code"`
	Description string `gorm:"column:description;type:text" json:"description"`
	Enabled     bool   `gorm:"column:enabled;default:true;index" json:"enabled"`

	CommonTimestampsField
}

func (PipelineDefinition) TableName() string {
	return "pipeline_definitions"
}

// PipelineStage is a large business block built from small method steps.
type PipelineStage struct {
	BaseModel

	PipelineID uint   `gorm:"column:pipeline_id;not null;index:idx_pipeline_stage_type,unique" json:"pipeline_id"`
	StageType  string `gorm:"column:stage_type;size:50;not null;index:idx_pipeline_stage_type,unique" json:"stage_type"`
	Name       string `gorm:"column:name;size:100;not null" json:"name"`
	OrderIndex int    `gorm:"column:order_index;default:0;index" json:"order_index"`
	Enabled    bool   `gorm:"column:enabled;default:true;index" json:"enabled"`

	CommonTimestampsField
}

func (PipelineStage) TableName() string {
	return "pipeline_stages"
}

// MethodStep is one executable method inside a pipeline.
type MethodStep struct {
	BaseModel

	PipelineID          uint   `gorm:"column:pipeline_id;not null;index;uniqueIndex:idx_pipeline_step_code" json:"pipeline_id"`
	StageID             uint   `gorm:"column:stage_id;default:0;index" json:"stage_id"`
	Code                string `gorm:"column:code;size:100;not null;uniqueIndex:idx_pipeline_step_code" json:"code"`
	Name                string `gorm:"column:name;size:100;not null" json:"name"`
	MethodType          string `gorm:"column:method_type;size:50;not null;index" json:"method_type"`
	OrderIndex          int    `gorm:"column:order_index;default:0;index" json:"order_index"`
	Enabled             bool   `gorm:"column:enabled;default:true;index" json:"enabled"`
	TimeoutSeconds      int    `gorm:"column:timeout_seconds;default:30" json:"timeout_seconds"`
	GeneratedConfigJSON string `gorm:"column:generated_config_json;type:json" json:"generated_config_json"`

	CommonTimestampsField
}

func (MethodStep) TableName() string {
	return "method_steps"
}

// MethodParam is a single editable input parameter for a method step.
type MethodParam struct {
	BaseModel

	StepID      uint   `gorm:"column:step_id;not null;index" json:"step_id"`
	Location    string `gorm:"column:location;size:50;not null;index" json:"location"`
	Name        string `gorm:"column:name;size:100;not null" json:"name"`
	ValueSource string `gorm:"column:value_source;size:50;not null" json:"value_source"`
	Value       string `gorm:"column:value;type:text" json:"value"`
	ValueType   string `gorm:"column:value_type;size:50;default:'string'" json:"value_type"`
	Required    bool   `gorm:"column:required;default:false" json:"required"`
	Secret      bool   `gorm:"column:secret;default:false" json:"secret"`
	Description string `gorm:"column:description;type:text" json:"description"`
	OrderIndex  int    `gorm:"column:order_index;default:0;index" json:"order_index"`

	CommonTimestampsField
}

func (MethodParam) TableName() string {
	return "method_params"
}

// MethodOutput captures a value produced by a method step.
type MethodOutput struct {
	BaseModel

	StepID      uint   `gorm:"column:step_id;not null;index" json:"step_id"`
	Name        string `gorm:"column:name;size:100;not null" json:"name"`
	SourcePath  string `gorm:"column:source_path;size:255" json:"source_path"`
	ValueType   string `gorm:"column:value_type;size:50;default:'string'" json:"value_type"`
	Required    bool   `gorm:"column:required;default:false" json:"required"`
	Description string `gorm:"column:description;type:text" json:"description"`
	OrderIndex  int    `gorm:"column:order_index;default:0;index" json:"order_index"`

	CommonTimestampsField
}

func (MethodOutput) TableName() string {
	return "method_outputs"
}

// StageGeneratedConfig stores the generated large-block config for a stage.
type StageGeneratedConfig struct {
	BaseModel

	PipelineID          uint   `gorm:"column:pipeline_id;not null;index" json:"pipeline_id"`
	StageID             uint   `gorm:"column:stage_id;not null;index" json:"stage_id"`
	StageType           string `gorm:"column:stage_type;size:50;not null;index" json:"stage_type"`
	GeneratedConfigJSON string `gorm:"column:generated_config_json;type:json;not null" json:"generated_config_json"`
	TargetRefType       string `gorm:"column:target_ref_type;size:50" json:"target_ref_type"`
	TargetRefID         uint   `gorm:"column:target_ref_id;default:0" json:"target_ref_id"`
	Version             int    `gorm:"column:version;default:1" json:"version"`

	CommonTimestampsField
}

func (StageGeneratedConfig) TableName() string {
	return "stage_generated_configs"
}

// StepRun records the execution result for one method step.
type StepRun struct {
	BaseModel

	RunID               uint        `gorm:"column:run_id;not null;index" json:"run_id"`
	PipelineID          uint        `gorm:"column:pipeline_id;not null;index" json:"pipeline_id"`
	StepID              uint        `gorm:"column:step_id;not null;index" json:"step_id"`
	StepCode            string      `gorm:"column:step_code;size:100;not null;index" json:"step_code"`
	MethodType          string      `gorm:"column:method_type;size:50;not null" json:"method_type"`
	Status              string      `gorm:"column:status;type:enum('running','success','failed','skipped');not null;index" json:"status"`
	InputJSON           string      `gorm:"column:input_json;type:json" json:"input_json"`
	OutputJSON          string      `gorm:"column:output_json;type:json" json:"output_json"`
	GeneratedConfigJSON string      `gorm:"column:generated_config_json;type:json" json:"generated_config_json"`
	ErrorMessage        string      `gorm:"column:error_message;type:text" json:"error_message"`
	StartedAt           *TimeNormal `gorm:"column:started_at" json:"started_at"`
	FinishedAt          *TimeNormal `gorm:"column:finished_at" json:"finished_at"`

	CommonTimestampsField
}

func (StepRun) TableName() string {
	return "step_runs"
}
