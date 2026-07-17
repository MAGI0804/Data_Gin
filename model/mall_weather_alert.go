package model

import "time"

type MallWeatherAlert struct {
	BaseModel
	Provider            string     `gorm:"column:provider;size:16;not null;uniqueIndex:uk_weather_alert,priority:1" json:"provider"`
	AlertID             string     `gorm:"column:alert_id;size:255;not null;uniqueIndex:uk_weather_alert,priority:2" json:"alert_id"`
	Status              string     `gorm:"column:status;size:32;index" json:"status"`
	Code                string     `gorm:"column:code;size:64" json:"code"`
	AlertTypeCode       string     `gorm:"column:alert_type_code;size:64;index" json:"alert_type_code"`
	AlertLevelCode      string     `gorm:"column:alert_level_code;size:64;index" json:"alert_level_code"`
	AlertTypeName       string     `gorm:"column:alert_type_name;size:128" json:"alert_type_name"`
	AlertLevelName      string     `gorm:"column:alert_level_name;size:128" json:"alert_level_name"`
	Title               string     `gorm:"column:title;size:1000" json:"title"`
	Description         string     `gorm:"column:description;type:text" json:"description"`
	Source              string     `gorm:"column:source;size:255" json:"source"`
	PublishedAtUTC      *time.Time `gorm:"column:published_at_utc;type:datetime(3);index" json:"published_at_utc"`
	Province            string     `gorm:"column:province;size:128" json:"province"`
	City                string     `gorm:"column:city;size:128" json:"city"`
	County              string     `gorm:"column:county;size:128" json:"county"`
	Location            string     `gorm:"column:location;size:255" json:"location"`
	RegionID            string     `gorm:"column:region_id;size:64" json:"region_id"`
	Adcode              string     `gorm:"column:adcode;size:32;index" json:"adcode"`
	Latitude            *float64   `gorm:"column:latitude;type:decimal(10,7)" json:"latitude"`
	Longitude           *float64   `gorm:"column:longitude;type:decimal(10,7)" json:"longitude"`
	AdcodesJSON         JSONText   `gorm:"column:adcodes_json;type:json" json:"adcodes_json"`
	FirstSeenAt         time.Time  `gorm:"column:first_seen_at;type:datetime(3);not null" json:"first_seen_at"`
	LastSeenAt          time.Time  `gorm:"column:last_seen_at;type:datetime(3);not null;index" json:"last_seen_at"`
	EndedAt             *time.Time `gorm:"column:ended_at;type:datetime(3)" json:"ended_at"`
	ProviderPayloadJSON JSONText   `gorm:"column:provider_payload_json;type:json" json:"provider_payload_json"`
	FetchRunID          uint       `gorm:"column:fetch_run_id;not null;index" json:"fetch_run_id"`
	RawChecksum         string     `gorm:"column:raw_checksum;type:char(64);not null" json:"raw_checksum"`
	QualityStatus       string     `gorm:"column:quality_status;size:32;not null;default:'valid';index" json:"quality_status"`
	QualityFlagsJSON    JSONText   `gorm:"column:quality_flags_json;type:json" json:"quality_flags_json"`
	WeatherTimestamps
}

func (MallWeatherAlert) TableName() string { return "mall_weather_alerts" }

type MallWeatherAlertRelation struct {
	BaseModel
	MallID         uint      `gorm:"column:mall_id;not null;uniqueIndex:uk_weather_alert_relation,priority:1;index" json:"mall_id"`
	AlertPK        uint      `gorm:"column:alert_pk;not null;uniqueIndex:uk_weather_alert_relation,priority:2;index" json:"alert_pk"`
	RelationReason string    `gorm:"column:relation_reason;size:32;not null" json:"relation_reason"`
	FirstSeenAt    time.Time `gorm:"column:first_seen_at;type:datetime(3);not null" json:"first_seen_at"`
	LastSeenAt     time.Time `gorm:"column:last_seen_at;type:datetime(3);not null" json:"last_seen_at"`
	IsActive       bool      `gorm:"column:is_active;not null;default:true;index" json:"is_active"`
	WeatherTimestamps
}

func (MallWeatherAlertRelation) TableName() string { return "mall_weather_alert_relations" }
