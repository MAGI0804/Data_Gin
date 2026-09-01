package model

import "time"

const (
	OfficeMessageSourceEdited = "EDITED"
	OfficeMessageSourceOracle = "ORACLE"

	OfficePushChannelFeishu = "FEISHU"

	OfficePushRunStatusQueued    = "QUEUED"
	OfficePushRunStatusRunning   = "RUNNING"
	OfficePushRunStatusSucceeded = "SUCCEEDED"
	OfficePushRunStatusFailed    = "FAILED"
	OfficePushRunStatusUnknown   = "UNKNOWN"
)

// OfficeMessage describes either operator-authored text or an Oracle result
// table that is exported when a push starts.
type OfficeMessage struct {
	BaseModel
	Name              string    `gorm:"column:name;size:128;not null;index" json:"name"`
	SourceType        string    `gorm:"column:source_type;size:16;not null;index" json:"sourceType"`
	Content           string    `gorm:"column:content;type:text" json:"content"`
	ProcedureOwner    string    `gorm:"column:procedure_owner;size:128" json:"procedureOwner"`
	PackageName       string    `gorm:"column:package_name;size:128" json:"packageName"`
	ProcedureName     string    `gorm:"column:procedure_name;size:128" json:"procedureName"`
	ProcedureOverload string    `gorm:"column:procedure_overload;size:32" json:"procedureOverload"`
	ResultTableOwner  string    `gorm:"column:result_table_owner;size:128" json:"resultTableOwner"`
	ResultTableName   string    `gorm:"column:result_table_name;size:128" json:"resultTableName"`
	ColumnMappingJSON JSONText  `gorm:"column:column_mapping_json;type:json" json:"columnMapping"`
	Enabled           bool      `gorm:"column:enabled;not null;default:true;index" json:"enabled"`
	LockVersion       uint64    `gorm:"column:lock_version;not null;default:1" json:"lockVersion"`
	CreatedBy         uint      `gorm:"column:created_by;not null;index" json:"createdBy"`
	UpdatedBy         uint      `gorm:"column:updated_by;not null" json:"updatedBy"`
	CreatedAt         time.Time `gorm:"column:created_at;type:datetime(3);not null;autoCreateTime" json:"createdAt"`
	UpdatedAt         time.Time `gorm:"column:updated_at;type:datetime(3);not null;autoUpdateTime" json:"updatedAt"`
}

func (OfficeMessage) TableName() string { return "office_messages" }

// OfficePushTarget binds one message to one Feishu bot recipient.
type OfficePushTarget struct {
	BaseModel
	Name          string    `gorm:"column:name;size:128;not null;index" json:"name"`
	MessageID     uint      `gorm:"column:message_id;not null;index" json:"messageId"`
	Channel       string    `gorm:"column:channel;size:16;not null;default:'FEISHU';index" json:"channel"`
	ReceiveIDType string    `gorm:"column:receive_id_type;size:32;not null" json:"receiveIdType"`
	ReceiveID     string    `gorm:"column:receive_id;size:255;not null" json:"receiveId"`
	Enabled       bool      `gorm:"column:enabled;not null;default:true;index" json:"enabled"`
	LockVersion   uint64    `gorm:"column:lock_version;not null;default:1" json:"lockVersion"`
	CreatedBy     uint      `gorm:"column:created_by;not null;index" json:"createdBy"`
	UpdatedBy     uint      `gorm:"column:updated_by;not null" json:"updatedBy"`
	CreatedAt     time.Time `gorm:"column:created_at;type:datetime(3);not null;autoCreateTime" json:"createdAt"`
	UpdatedAt     time.Time `gorm:"column:updated_at;type:datetime(3);not null;autoUpdateTime" json:"updatedAt"`
}

func (OfficePushTarget) TableName() string { return "office_push_targets" }

// OfficePushRun is the durable, idempotent execution record consumed by the
// asynchronous worker. RunUUID is also sent to Feishu as its idempotency key.
type OfficePushRun struct {
	BaseModel
	RunUUID          string     `gorm:"column:run_uuid;size:64;not null;uniqueIndex" json:"runUuid"`
	TargetID         uint       `gorm:"column:target_id;not null;index" json:"targetId"`
	MessageID        uint       `gorm:"column:message_id;not null;index" json:"messageId"`
	Status           string     `gorm:"column:status;size:16;not null;index" json:"status"`
	AttemptCount     int        `gorm:"column:attempt_count;not null;default:0" json:"attemptCount"`
	RowCount         int64      `gorm:"column:row_count;not null;default:0" json:"rowCount"`
	FeishuMessageID  string     `gorm:"column:feishu_message_id;size:128" json:"feishuMessageId,omitempty"`
	ErrorCode        string     `gorm:"column:error_code;size:64" json:"errorCode,omitempty"`
	ErrorMessageSafe string     `gorm:"column:error_message_safe;size:500" json:"errorMessage,omitempty"`
	RequestedBy      uint       `gorm:"column:requested_by;not null;index" json:"requestedBy"`
	StartedAt        *time.Time `gorm:"column:started_at;type:datetime(3)" json:"startedAt,omitempty"`
	FinishedAt       *time.Time `gorm:"column:finished_at;type:datetime(3)" json:"finishedAt,omitempty"`
	CreatedAt        time.Time  `gorm:"column:created_at;type:datetime(3);not null;autoCreateTime;index" json:"createdAt"`
	UpdatedAt        time.Time  `gorm:"column:updated_at;type:datetime(3);not null;autoUpdateTime" json:"updatedAt"`
}

func (OfficePushRun) TableName() string { return "office_push_runs" }
