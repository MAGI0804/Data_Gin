package requestbody

type DestinationCreateRequest struct {
	Name            string `json:"name" binding:"required,max=100"`
	Code            string `json:"code" binding:"required,max=100"`
	DestinationType string `json:"destination_type" binding:"required,max=50"`
	ConfigJSON      string `json:"config_json" binding:"required"`
	Enabled         *bool  `json:"enabled"`
}

type DeliveryTaskCreateRequest struct {
	Name            string `json:"name" binding:"required,max=100"`
	SourceID        uint   `json:"source_id" binding:"required,min=1"`
	CleanTable      string `json:"clean_table" binding:"required,max=100"`
	DestinationID   uint   `json:"destination_id" binding:"required,min=1"`
	TriggerType     string `json:"trigger_type" binding:"required,max=50"`
	CronExpr        string `json:"cron_expr" binding:"max=100"`
	FilterJSON      string `json:"filter_json"`
	PayloadTemplate string `json:"payload_template"`
	Enabled         *bool  `json:"enabled"`
}

type DestinationUpdateRequest DestinationCreateRequest

type DeliveryTaskUpdateRequest DeliveryTaskCreateRequest
