package model

import "time"

type MallWeatherExportProfile struct {
	BaseModel
	Code        string   `gorm:"column:code;size:100;not null;uniqueIndex:uk_weather_export_profile" json:"code"`
	Name        string   `gorm:"column:name;size:255;not null" json:"name"`
	Version     uint64   `gorm:"column:version;not null;default:1" json:"version"`
	ProfileJSON JSONText `gorm:"column:profile_json;type:json;not null" json:"profile_json"`
	Enabled     bool     `gorm:"column:enabled;not null;default:true;index" json:"enabled"`
	CreatedBy   uint     `gorm:"column:created_by;default:0" json:"created_by"`
	UpdatedBy   uint     `gorm:"column:updated_by;default:0" json:"updated_by"`
	WeatherTimestamps
}

func (MallWeatherExportProfile) TableName() string { return "mall_weather_export_profiles" }

type MallWeatherExportJob struct {
	BaseModel
	JobUUID             string     `gorm:"column:job_uuid;type:char(36);not null;uniqueIndex" json:"job_uuid"`
	ProfileID           uint       `gorm:"column:profile_id;not null;index" json:"profile_id"`
	ProfileVersion      uint64     `gorm:"column:profile_version;not null" json:"profile_version"`
	ProfileSnapshotJSON JSONText   `gorm:"column:profile_snapshot_json;type:json;not null" json:"profile_snapshot_json"`
	FiltersJSON         JSONText   `gorm:"column:filters_json;type:json" json:"filters_json"`
	IdempotencyKey      string     `gorm:"column:idempotency_key;size:255;not null;uniqueIndex" json:"idempotency_key"`
	Status              string     `gorm:"column:status;size:32;not null;default:'pending';index" json:"status"`
	TotalRows           int64      `gorm:"column:total_rows;not null;default:0" json:"total_rows"`
	ProcessedRows       int64      `gorm:"column:processed_rows;not null;default:0" json:"processed_rows"`
	CurrentSheet        string     `gorm:"column:current_sheet;size:255" json:"current_sheet"`
	LastCursorJSON      JSONText   `gorm:"column:last_cursor_json;type:json" json:"last_cursor_json"`
	CancelRequested     bool       `gorm:"column:cancel_requested;not null;default:false;index" json:"cancel_requested"`
	ResultObjectKey     string     `gorm:"column:result_object_key;size:1024" json:"-"`
	ResultChecksum      string     `gorm:"column:result_checksum;type:char(64)" json:"result_checksum"`
	FileSizeBytes       int64      `gorm:"column:file_size_bytes;not null;default:0" json:"file_size_bytes"`
	ErrorMessageSafe    string     `gorm:"column:error_message_safe;type:text" json:"error_message_safe"`
	StartedAt           *time.Time `gorm:"column:started_at;type:datetime(3)" json:"started_at"`
	FinishedAt          *time.Time `gorm:"column:finished_at;type:datetime(3)" json:"finished_at"`
	ExpiresAt           *time.Time `gorm:"column:expires_at;type:datetime(3);index" json:"expires_at"`
	CreatedBy           uint       `gorm:"column:created_by;default:0" json:"created_by"`
	WeatherTimestamps
}

func (MallWeatherExportJob) TableName() string { return "mall_weather_export_jobs" }

// MallWeatherFeishuRun stores the immutable, non-secret inputs and the
// internal worker lease for a weather Sheet push. The public lifecycle and
// counters remain on the associated PipelineRun.
type MallWeatherFeishuRun struct {
	BaseModel
	PipelineRunID           uint     `gorm:"column:pipeline_run_id;not null;uniqueIndex:uk_weather_feishu_run" json:"pipeline_run_id"`
	ProfileID               uint     `gorm:"column:profile_id;not null;index" json:"profile_id"`
	ProfileVersion          uint64   `gorm:"column:profile_version;not null" json:"profile_version"`
	ProfileSnapshotJSON     JSONText `gorm:"column:profile_snapshot_json;type:json;not null" json:"-"`
	FiltersJSON             JSONText `gorm:"column:filters_json;type:json;not null" json:"-"`
	DestinationSnapshotJSON JSONText `gorm:"column:destination_snapshot_json;type:json;not null" json:"-"`
	RunToken                string   `gorm:"column:run_token;type:char(36);not null;default:''" json:"-"`
	CreatedBy               uint     `gorm:"column:created_by;not null" json:"created_by"`
	WeatherTimestamps
}

func (MallWeatherFeishuRun) TableName() string { return "mall_weather_feishu_runs" }

type MallWeatherSheetRow struct {
	BaseModel
	DestinationID uint      `gorm:"column:destination_id;not null;uniqueIndex:uk_weather_sheet_row,priority:1;uniqueIndex:uk_weather_sheet_row_number,priority:1;index" json:"destination_id"`
	DatasetKind   string    `gorm:"column:dataset_kind;size:32;not null;uniqueIndex:uk_weather_sheet_row,priority:2;uniqueIndex:uk_weather_sheet_row_number,priority:2" json:"dataset_kind"`
	BusinessKey   string    `gorm:"column:business_key;size:512;not null;uniqueIndex:uk_weather_sheet_row,priority:3" json:"business_key"`
	SheetIDEnv    string    `gorm:"column:sheet_id_env;size:128;not null" json:"sheet_id_env"`
	RowNumber     int64     `gorm:"column:row_number;not null;uniqueIndex:uk_weather_sheet_row_number,priority:3" json:"row_number"`
	Checksum      string    `gorm:"column:checksum;type:char(64);not null" json:"checksum"`
	LastSyncedAt  time.Time `gorm:"column:last_synced_at;type:datetime(3);not null;index" json:"last_synced_at"`
	WeatherTimestamps
}

func (MallWeatherSheetRow) TableName() string { return "mall_weather_sheet_rows" }
