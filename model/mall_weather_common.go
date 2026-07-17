package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// JSONText stores validated JSON without turning an unset optional value into
// MySQL's invalid empty JSON string.
type JSONText string

func (j JSONText) Value() (driver.Value, error) {
	if strings.TrimSpace(string(j)) == "" {
		return nil, nil
	}
	if !json.Valid([]byte(j)) {
		return nil, fmt.Errorf("model: invalid json value")
	}
	return string(j), nil
}

func (j *JSONText) Scan(value interface{}) error {
	if value == nil {
		*j = ""
		return nil
	}
	var data []byte
	switch typed := value.(type) {
	case []byte:
		data = typed
	case string:
		data = []byte(typed)
	default:
		return fmt.Errorf("model: cannot scan json from %T", value)
	}
	if !json.Valid(data) {
		return fmt.Errorf("model: invalid json from database")
	}
	*j = JSONText(data)
	return nil
}

func (j JSONText) MarshalJSON() ([]byte, error) {
	if strings.TrimSpace(string(j)) == "" {
		return []byte("null"), nil
	}
	if !json.Valid([]byte(j)) {
		return nil, fmt.Errorf("model: invalid json value")
	}
	return []byte(j), nil
}

// WeatherTimestamps keeps weather-domain timestamps in UTC with millisecond
// precision. Unlike legacy models, these tables do not use Unix-second fields.
type WeatherTimestamps struct {
	CreatedAt time.Time `gorm:"column:created_at;type:datetime(3);not null;autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;type:datetime(3);not null;autoUpdateTime" json:"updated_at"`
}

// WeatherQualityFields is embedded by parsed provider business records.
type WeatherQualityFields struct {
	FetchRunID       uint      `gorm:"column:fetch_run_id;not null;index" json:"fetch_run_id"`
	QualityStatus    string    `gorm:"column:quality_status;size:32;not null;default:'valid';index" json:"quality_status"`
	QualityFlagsJSON JSONText  `gorm:"column:quality_flags_json;type:json" json:"quality_flags_json"`
	RawChecksum      string    `gorm:"column:raw_checksum;type:char(64);not null" json:"raw_checksum"`
	LastSeenAt       time.Time `gorm:"column:last_seen_at;type:datetime(3);not null;index" json:"last_seen_at"`
}
