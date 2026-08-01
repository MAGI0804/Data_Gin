package requestbody

// IngestRequest 数据接收请求
type IngestRequest struct {
	DataSource string                   `json:"data_source" binding:"required,max=100"`
	DataType   string                   `json:"data_type" binding:"required,max=50"`
	Data       []map[string]interface{} `json:"data" binding:"required,min=1,max=1000"`
}

// BatchIngestRequest 批量数据接收请求
type BatchIngestRequest struct {
	BatchID string `json:"batch_id" binding:"required,max=100"`
	Items   []struct {
		DataSource string                   `json:"data_source" binding:"required,max=100"`
		DataType   string                   `json:"data_type" binding:"required,max=50"`
		Data       []map[string]interface{} `json:"data" binding:"required,min=1,max=1000"`
	} `json:"items" binding:"required,min=1,max=100"`
}

// CollectRequest 数据采集请求
type CollectRequest struct {
	SourceID uint `json:"source_id" binding:"required,min=1"`
}

// CollectStatusRequest 数据采集状态查询请求
type CollectStatusRequest struct {
	JobID string `json:"job_id" binding:"required,max=100"`
}

// RawDataQueryRequest 原始数据查询请求
type RawDataQueryRequest struct {
	DataType     string `form:"data_type" binding:"max=50"`
	DataSourceID uint   `form:"data_source_id" binding:"min=0"`
	Status       string `form:"status" binding:"max=20"`
	Limit        int    `form:"limit" binding:"min=1,max=1000"`
}

// ProcessedDataQueryRequest 处理后数据查询请求
type ProcessedDataQueryRequest struct {
	DataType string `form:"data_type" binding:"max=50"`
	Limit    int    `form:"limit" binding:"min=1,max=1000"`
}

// ProcessedDataListQueryRequest is the paginated query contract for legacy
// processed_data records. Business-key filtering belongs to clean_records and
// is intentionally not accepted here.
type ProcessedDataListQueryRequest struct {
	Page        int      `form:"page" binding:"min=1"`
	PageSize    int      `form:"page_size" binding:"min=1,max=100"`
	DataType    string   `form:"data_type" binding:"max=50"`
	MinQuality  *float64 `form:"min_quality" binding:"omitempty,min=0,max=100"`
	MaxQuality  *float64 `form:"max_quality" binding:"omitempty,min=0,max=100"`
	CreatedFrom int64    `form:"created_from" binding:"min=0"`
	CreatedTo   int64    `form:"created_to" binding:"min=0"`
}

// CleanRecordListQueryRequest is the paginated query contract for clean_records.
// It is deliberately separate from legacy processed_data because only clean
// records have a business key, source and delivery status.
type CleanRecordListQueryRequest struct {
	Page        int      `form:"page" binding:"min=1"`
	PageSize    int      `form:"page_size" binding:"min=1,max=100"`
	SourceID    uint     `form:"source_id" binding:"min=0"`
	TableName   string   `form:"table_name" binding:"max=100"`
	BusinessKey string   `form:"business_key" binding:"max=255"`
	Status      string   `form:"status" binding:"omitempty,oneof=ready invalid delivered"`
	MinQuality  *float64 `form:"min_quality" binding:"omitempty,min=0,max=100"`
	MaxQuality  *float64 `form:"max_quality" binding:"omitempty,min=0,max=100"`
	CreatedFrom int64    `form:"created_from" binding:"min=0"`
	CreatedTo   int64    `form:"created_to" binding:"min=0"`
}

// StatisticsQueryRequest 统计数据查询请求
type StatisticsQueryRequest struct {
	StartDate string `form:"start_date" binding:"max=10"`
	EndDate   string `form:"end_date" binding:"max=10"`
	DataType  string `form:"data_type" binding:"max=50"`
}

// RawIngestRequest 原始数据接收请求（用于接收任意格式的数据）
type RawIngestRequest struct {
	DataSource string      `json:"data_source" binding:"max=100"`
	DataType   string      `json:"data_type" binding:"max=50"`
	RawContent interface{} `json:"raw_content"`
	Remark     string      `form:"remark" binding:"max=255"`
}

// RawDataListQueryRequest 原始数据列表查询请求
type RawDataListQueryRequest struct {
	Page      int    `json:"page" binding:"min=1"`
	PageSize  int    `json:"page_size" binding:"min=1,max=100"`
	Source    string `json:"source" binding:"max=100"`
	StartTime string `json:"start_time" binding:"max=20"`
	EndTime   string `json:"end_time" binding:"max=20"`
	Origin    string `json:"origin" binding:"omitempty,oneof=receive pull"`
}

// RawRecordListQueryRequest is the safe pagination contract for warehouse
// raw_records. It deliberately accepts only metadata that is safe to list;
// raw content, request headers and processing errors are never query fields.
type RawRecordListQueryRequest struct {
	Page      int    `form:"page" binding:"omitempty,min=1,max=1000000"`
	PageSize  int    `form:"page_size" binding:"omitempty,min=1,max=100"`
	Source    string `form:"source" binding:"max=100"`
	Status    string `form:"status" binding:"omitempty,oneof=received queued cleaning cleaned failed"`
	TraceID   string `form:"trace_id" binding:"max=64"`
	StartTime string `form:"start_time" binding:"max=19"`
	EndTime   string `form:"end_time" binding:"max=19"`
	Origin    string `form:"origin" binding:"omitempty,oneof=receive pull"`
}
