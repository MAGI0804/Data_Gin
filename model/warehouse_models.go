package model

// SourceDefinition 通用数据源配置。
type SourceDefinition struct {
	BaseModel

	Name           string `gorm:"column:name;size:100;not null" json:"name"`
	Code           string `gorm:"column:code;size:100;not null;uniqueIndex" json:"code"`
	SourceType     string `gorm:"column:source_type;size:50;not null" json:"source_type"`
	Enabled        bool   `gorm:"column:enabled;default:true" json:"enabled"`
	AuthType       string `gorm:"column:auth_type;size:50;default:'none'" json:"auth_type"`
	ConfigJSON     string `gorm:"column:config_json;type:json" json:"config_json"`
	SchemaJSON     string `gorm:"column:schema_json;type:json" json:"schema_json"`
	DedupeKeys     string `gorm:"column:dedupe_keys;type:json" json:"dedupe_keys"`
	SourceQueryKey string `gorm:"column:source_query_key;size:100" json:"source_query_key"`

	CommonTimestampsField
}

func (SourceDefinition) TableName() string {
	return "source_definitions"
}

// RawRecord 通用原始记录，兼容 raw_data 逐步迁移。
type RawRecord struct {
	BaseModel

	SourceID     uint        `gorm:"column:source_id;default:0;index" json:"source_id"`
	SourceCode   string      `gorm:"column:source_code;size:100;index" json:"source_code"`
	ExternalID   string      `gorm:"column:external_id;size:255;index" json:"external_id"`
	DedupeHash   string      `gorm:"column:dedupe_hash;size:64;index" json:"dedupe_hash"`
	RawContent   string      `gorm:"column:raw_content;type:json;not null" json:"raw_content"`
	HeadersJSON  string      `gorm:"column:headers_json;type:json" json:"headers_json"`
	QueryJSON    string      `gorm:"column:query_json;type:json" json:"query_json"`
	MetadataJSON string      `gorm:"column:metadata_json;type:json" json:"metadata_json"`
	Status       string      `gorm:"column:status;type:enum('received','queued','cleaning','cleaned','failed');default:'received';index" json:"status"`
	ErrorMessage string      `gorm:"column:error_message;type:text" json:"error_message"`
	TraceID      string      `gorm:"column:trace_id;size:64;index" json:"trace_id"`
	ReceivedAt   *TimeNormal `gorm:"column:received_at" json:"received_at"`

	CommonTimestampsField
}

func (RawRecord) TableName() string {
	return "raw_records"
}

// CleanTableDefinition 清洗表配置。
type CleanTableDefinition struct {
	BaseModel

	SourceID        uint   `gorm:"column:source_id;not null;index" json:"source_id"`
	TableNameValue  string `gorm:"column:table_name;size:100;not null" json:"table_name"`
	DisplayName     string `gorm:"column:display_name;size:100;not null" json:"display_name"`
	PrimaryKeyField string `gorm:"column:primary_key_field;size:100" json:"primary_key_field"`
	FieldsJSON      string `gorm:"column:fields_json;type:json" json:"fields_json"`
	IndexesJSON     string `gorm:"column:indexes_json;type:json" json:"indexes_json"`
	Enabled         bool   `gorm:"column:enabled;default:true" json:"enabled"`

	CommonTimestampsField
}

func (CleanTableDefinition) TableName() string {
	return "clean_table_definitions"
}

// CleanRecord 通用清洗结果。
type CleanRecord struct {
	BaseModel

	RawRecordID      uint    `gorm:"column:raw_record_id;not null;index" json:"raw_record_id"`
	SourceID         uint    `gorm:"column:source_id;not null;index" json:"source_id"`
	LogicalTableName string  `gorm:"column:table_name;size:100;not null;index" json:"table_name"`
	BusinessKey      string  `gorm:"column:business_key;size:255;index" json:"business_key"`
	CleanContent     string  `gorm:"column:clean_content;type:json;not null" json:"clean_content"`
	QualityScore     float64 `gorm:"column:quality_score;type:decimal(5,2);default:100.00" json:"quality_score"`
	Status           string  `gorm:"column:status;type:enum('ready','invalid','delivered');default:'ready';index" json:"status"`

	CommonTimestampsField
}

func (CleanRecord) TableName() string {
	return "clean_records"
}

// TransformRule 通用清洗规则。
type TransformRule struct {
	BaseModel

	SourceID   uint   `gorm:"column:source_id;not null;index" json:"source_id"`
	Name       string `gorm:"column:name;size:100;not null" json:"name"`
	RuleType   string `gorm:"column:rule_type;type:enum('mapping','http_enrich','db_enrich','script','validator');not null;index" json:"rule_type"`
	OrderIndex int    `gorm:"column:order_index;default:0;index" json:"order_index"`
	ConfigJSON string `gorm:"column:config_json;type:json;not null" json:"config_json"`
	Enabled    bool   `gorm:"column:enabled;default:true;index" json:"enabled"`

	CommonTimestampsField
}

func (TransformRule) TableName() string {
	return "transform_rules"
}

// DestinationDefinition 通用推送目标配置。
type DestinationDefinition struct {
	BaseModel

	Name            string `gorm:"column:name;size:100;not null" json:"name"`
	Code            string `gorm:"column:code;size:100;not null;uniqueIndex" json:"code"`
	DestinationType string `gorm:"column:destination_type;size:50;not null" json:"destination_type"`
	ConfigJSON      string `gorm:"column:config_json;type:json;not null" json:"config_json"`
	Enabled         bool   `gorm:"column:enabled;default:true;index" json:"enabled"`

	CommonTimestampsField
}

func (DestinationDefinition) TableName() string {
	return "destination_definitions"
}

// DeliveryTask 通用推送任务。
type DeliveryTask struct {
	BaseModel

	Name            string `gorm:"column:name;size:100;not null" json:"name"`
	SourceID        uint   `gorm:"column:source_id;not null;index" json:"source_id"`
	CleanTable      string `gorm:"column:clean_table;size:100;not null;index" json:"clean_table"`
	DestinationID   uint   `gorm:"column:destination_id;not null;index" json:"destination_id"`
	TriggerType     string `gorm:"column:trigger_type;type:enum('manual','schedule','event');not null;index" json:"trigger_type"`
	CronExpr        string `gorm:"column:cron_expr;size:100" json:"cron_expr"`
	FilterJSON      string `gorm:"column:filter_json;type:json" json:"filter_json"`
	PayloadTemplate string `gorm:"column:payload_template;type:text" json:"payload_template"`
	Enabled         bool   `gorm:"column:enabled;default:true;index" json:"enabled"`

	CommonTimestampsField
}

func (DeliveryTask) TableName() string {
	return "delivery_tasks"
}

// PipelineRun 接收、清洗、推送运行记录。
type PipelineRun struct {
	BaseModel

	TraceID       string      `gorm:"column:trace_id;size:64;not null;index" json:"trace_id"`
	RunType       string      `gorm:"column:run_type;type:enum('fetch','ingest','transform','delivery');not null;index" json:"run_type"`
	TriggerType   string      `gorm:"column:trigger_type;type:enum('manual','schedule','event','api');not null" json:"trigger_type"`
	SourceID      uint        `gorm:"column:source_id;default:0;index" json:"source_id"`
	DestinationID uint        `gorm:"column:destination_id;default:0;index" json:"destination_id"`
	Status        string      `gorm:"column:status;type:enum('running','success','failed','partial_success');not null;index" json:"status"`
	TotalCount    int         `gorm:"column:total_count;default:0" json:"total_count"`
	SuccessCount  int         `gorm:"column:success_count;default:0" json:"success_count"`
	FailedCount   int         `gorm:"column:failed_count;default:0" json:"failed_count"`
	StartedAt     *TimeNormal `gorm:"column:started_at" json:"started_at"`
	FinishedAt    *TimeNormal `gorm:"column:finished_at" json:"finished_at"`
	ErrorMessage  string      `gorm:"column:error_message;type:text" json:"error_message"`

	CommonTimestampsField
}

func (PipelineRun) TableName() string {
	return "pipeline_runs"
}

// DeliveryLog 单条推送日志。
type DeliveryLog struct {
	BaseModel

	TraceID       string      `gorm:"column:trace_id;size:64;not null;index" json:"trace_id"`
	RunID         uint        `gorm:"column:run_id;default:0;index" json:"run_id"`
	CleanRecordID uint        `gorm:"column:clean_record_id;not null;index" json:"clean_record_id"`
	DestinationID uint        `gorm:"column:destination_id;not null;index" json:"destination_id"`
	BusinessKey   string      `gorm:"column:business_key;size:255;index" json:"business_key"`
	RequestBody   string      `gorm:"column:request_body;type:longtext" json:"request_body"`
	ResponseBody  string      `gorm:"column:response_body;type:longtext" json:"response_body"`
	HTTPStatus    int         `gorm:"column:http_status;default:0" json:"http_status"`
	Success       bool        `gorm:"column:success;default:false;index" json:"success"`
	ErrorMessage  string      `gorm:"column:error_message;type:text" json:"error_message"`
	RetryCount    int         `gorm:"column:retry_count;default:0" json:"retry_count"`
	SentAt        *TimeNormal `gorm:"column:sent_at" json:"sent_at"`

	CommonTimestampsField
}

func (DeliveryLog) TableName() string {
	return "delivery_logs"
}

// ExcelMatchJob 记录大文件 Excel 匹配导出任务。
type ExcelMatchJob struct {
	BaseModel

	SourceFileName string      `gorm:"column:source_file_name;size:255" json:"source_file_name"`
	SourceFilePath string      `gorm:"column:source_file_path;size:1024" json:"-"`
	ResultFilePath string      `gorm:"column:result_file_path;size:1024" json:"-"`
	WorkDir        string      `gorm:"column:work_dir;size:1024" json:"-"`
	ConfigJSON     string      `gorm:"column:config_json;type:json;not null" json:"config_json"`
	Status         string      `gorm:"column:status;size:30;not null;default:'pending';index" json:"status"`
	TotalRows      int         `gorm:"column:total_rows;default:0" json:"total_rows"`
	ProcessedRows  int         `gorm:"column:processed_rows;default:0" json:"processed_rows"`
	FilteredRows   int         `gorm:"column:filtered_rows;default:0" json:"filtered_rows"`
	MatchedRows    int         `gorm:"column:matched_rows;default:0" json:"matched_rows"`
	UnmatchedRows  int         `gorm:"column:unmatched_rows;default:0" json:"unmatched_rows"`
	ErrorMessage   string      `gorm:"column:error_message;type:text" json:"error_message"`
	StartedAt      *TimeNormal `gorm:"column:started_at" json:"started_at"`
	FinishedAt     *TimeNormal `gorm:"column:finished_at" json:"finished_at"`
	ExpiresAt      *TimeNormal `gorm:"column:expires_at;index" json:"expires_at"`

	CommonTimestampsField
}

func (ExcelMatchJob) TableName() string {
	return "excel_match_jobs"
}
