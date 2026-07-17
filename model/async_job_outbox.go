package model

import "time"

// AsyncJobOutbox provides crash-safe handoff from committed database changes
// to Asynq's at-least-once delivery.
type AsyncJobOutbox struct {
	BaseModel
	TaskKey       string     `gorm:"column:task_key;size:255;not null;uniqueIndex" json:"task_key"`
	TaskType      string     `gorm:"column:task_type;size:128;not null;index" json:"task_type"`
	PayloadJSON   JSONText   `gorm:"column:payload_json;type:json;not null" json:"payload_json"`
	QueueName     string     `gorm:"column:queue_name;size:64;not null;index" json:"queue_name"`
	AvailableAt   time.Time  `gorm:"column:available_at;type:datetime(3);not null;index:idx_outbox_pending,priority:2" json:"available_at"`
	PublishedAt   *time.Time `gorm:"column:published_at;type:datetime(3);index:idx_outbox_pending,priority:1" json:"published_at"`
	Attempts      int        `gorm:"column:attempts;not null;default:0" json:"attempts"`
	LastErrorSafe string     `gorm:"column:last_error_safe;type:text" json:"last_error_safe"`
	LockedBy      string     `gorm:"column:locked_by;size:128" json:"locked_by"`
	LockedAt      *time.Time `gorm:"column:locked_at;type:datetime(3);index" json:"locked_at"`
	WeatherTimestamps
}

func (AsyncJobOutbox) TableName() string { return "async_job_outbox" }
