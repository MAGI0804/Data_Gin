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
}
