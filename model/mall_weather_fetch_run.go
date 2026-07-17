package model

import "time"

type ProviderRawSnapshot struct {
	BaseModel
	Provider         string     `gorm:"column:provider;size:16;not null;index" json:"provider"`
	EndpointKind     string     `gorm:"column:endpoint_kind;size:32;not null;index" json:"endpoint_kind"`
	MallID           *uint      `gorm:"column:mall_id;index" json:"mall_id"`
	ResponseChecksum string     `gorm:"column:response_checksum;type:char(64);not null;index" json:"response_checksum"`
	Compression      string     `gorm:"column:compression;size:16;not null;default:'gzip'" json:"compression"`
	ContentBlob      []byte     `gorm:"column:content_blob;type:longblob" json:"-"`
	ObjectKey        string     `gorm:"column:object_key;size:1024" json:"-"`
	ContentLength    int64      `gorm:"column:content_length;not null;default:0" json:"content_length"`
	SchemaVersion    string     `gorm:"column:schema_version;size:32;not null" json:"schema_version"`
	ExpiresAt        *time.Time `gorm:"column:expires_at;type:datetime(3);index" json:"expires_at"`
	WeatherTimestamps
}

func (ProviderRawSnapshot) TableName() string { return "provider_raw_snapshots" }

type MallWeatherFetchRun struct {
	BaseModel
	RunUUID              string     `gorm:"column:run_uuid;type:char(36);not null;uniqueIndex" json:"run_uuid"`
	MallID               uint       `gorm:"column:mall_id;not null;uniqueIndex:uk_fetch_task,priority:1;index" json:"mall_id"`
	TaskKind             string     `gorm:"column:task_kind;size:32;not null;uniqueIndex:uk_fetch_task,priority:3;index" json:"task_kind"`
	TaskWindow           string     `gorm:"column:task_window;size:128;not null;uniqueIndex:uk_fetch_task,priority:4" json:"task_window"`
	EndpointKind         string     `gorm:"column:endpoint_kind;size:32;not null;uniqueIndex:uk_fetch_task,priority:2;index" json:"endpoint_kind"`
	Provider             string     `gorm:"column:provider;size:16;not null;default:'caiyun'" json:"provider"`
	RequestedHourlySteps int        `gorm:"column:requested_hourly_steps;not null;default:0" json:"requested_hourly_steps"`
	RequestedDailySteps  int        `gorm:"column:requested_daily_steps;not null;default:0" json:"requested_daily_steps"`
	AttemptCount         int        `gorm:"column:attempt_count;not null;default:0" json:"attempt_count"`
	Status               string     `gorm:"column:status;size:32;not null;default:'pending';index" json:"status"`
	StartedAt            *time.Time `gorm:"column:started_at;type:datetime(3);index" json:"started_at"`
	FinishedAt           *time.Time `gorm:"column:finished_at;type:datetime(3)" json:"finished_at"`
	DurationMS           int64      `gorm:"column:duration_ms;not null;default:0" json:"duration_ms"`
	HTTPStatus           *int       `gorm:"column:http_status" json:"http_status"`
	ProviderStatus       string     `gorm:"column:provider_status;size:64" json:"provider_status"`
	ProviderServerTime   *time.Time `gorm:"column:provider_server_time;type:datetime(3)" json:"provider_server_time"`
	ResponseChecksum     string     `gorm:"column:response_checksum;type:char(64)" json:"response_checksum"`
	RawSnapshotID        *uint      `gorm:"column:raw_snapshot_id;index" json:"raw_snapshot_id"`
	RowCountsJSON        JSONText   `gorm:"column:row_counts_json;type:json" json:"row_counts_json"`
	ParseWarningsJSON    JSONText   `gorm:"column:parse_warnings_json;type:json" json:"parse_warnings_json"`
	ErrorClass           string     `gorm:"column:error_class;size:64;index" json:"error_class"`
	ErrorCode            string     `gorm:"column:error_code;size:64" json:"error_code"`
	ErrorMessageSafe     string     `gorm:"column:error_message_safe;type:text" json:"error_message_safe"`
	ParserVersion        string     `gorm:"column:parser_version;size:32" json:"parser_version"`
	WeatherTimestamps
}

func (MallWeatherFetchRun) TableName() string { return "mall_weather_fetch_runs" }

type MallWeatherFetchAttempt struct {
	BaseModel
	FetchRunID       uint       `gorm:"column:fetch_run_id;not null;uniqueIndex:uk_fetch_attempt,priority:1;index" json:"fetch_run_id"`
	AttemptNo        int        `gorm:"column:attempt_no;not null;uniqueIndex:uk_fetch_attempt,priority:2" json:"attempt_no"`
	StartedAt        time.Time  `gorm:"column:started_at;type:datetime(3);not null" json:"started_at"`
	FinishedAt       *time.Time `gorm:"column:finished_at;type:datetime(3)" json:"finished_at"`
	DurationMS       int64      `gorm:"column:duration_ms;not null;default:0" json:"duration_ms"`
	HTTPStatus       *int       `gorm:"column:http_status" json:"http_status"`
	ProviderStatus   string     `gorm:"column:provider_status;size:64" json:"provider_status"`
	RawSnapshotID    *uint      `gorm:"column:raw_snapshot_id;index" json:"raw_snapshot_id"`
	ResponseChecksum string     `gorm:"column:response_checksum;type:char(64)" json:"response_checksum"`
	Status           string     `gorm:"column:status;size:32;not null;index" json:"status"`
	ErrorClass       string     `gorm:"column:error_class;size:64;index" json:"error_class"`
	ErrorCode        string     `gorm:"column:error_code;size:64" json:"error_code"`
	ErrorMessageSafe string     `gorm:"column:error_message_safe;type:text" json:"error_message_safe"`
	WeatherTimestamps
}

func (MallWeatherFetchAttempt) TableName() string { return "mall_weather_fetch_attempts" }
