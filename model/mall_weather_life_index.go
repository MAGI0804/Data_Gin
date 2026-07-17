package model

import "time"

type MallWeatherLifeIndex struct {
	BaseModel
	MallID              uint      `gorm:"column:mall_id;not null;uniqueIndex:uk_life_version,priority:1;index" json:"mall_id"`
	Provider            string    `gorm:"column:provider;size:16;not null;uniqueIndex:uk_life_version,priority:2" json:"provider"`
	SourceAPI           string    `gorm:"column:source_api;size:32;not null;uniqueIndex:uk_life_version,priority:3" json:"source_api"`
	ForecastDateLocal   time.Time `gorm:"column:forecast_date_local;type:date;not null;uniqueIndex:uk_life_version,priority:4;index" json:"forecast_date_local"`
	IndexType           int       `gorm:"column:index_type;not null;uniqueIndex:uk_life_version,priority:5" json:"index_type"`
	IssuedAtUTC         time.Time `gorm:"column:issued_at_utc;type:datetime(3);not null;uniqueIndex:uk_life_version,priority:6;index" json:"issued_at_utc"`
	FetchedAtUTC        time.Time `gorm:"column:fetched_at_utc;type:datetime(3);not null;index" json:"fetched_at_utc"`
	IndexCode           string    `gorm:"column:index_code;size:128;not null;index" json:"index_code"`
	IndexName           string    `gorm:"column:index_name;size:255" json:"index_name"`
	Level               *int      `gorm:"column:level" json:"level"`
	ShortDesc           string    `gorm:"column:short_desc;size:1000" json:"short_desc"`
	Detail              string    `gorm:"column:detail;type:text" json:"detail"`
	IsUnknownType       bool      `gorm:"column:is_unknown_type;not null;default:false;index" json:"is_unknown_type"`
	ProviderPayloadJSON JSONText  `gorm:"column:provider_payload_json;type:json" json:"provider_payload_json"`
	WeatherQualityFields
	WeatherTimestamps
}

func (MallWeatherLifeIndex) TableName() string { return "mall_weather_life_indices" }

type MallWeatherLatest struct {
	BaseModel
	MallID          uint       `gorm:"column:mall_id;not null;uniqueIndex:uk_weather_latest,priority:1;index" json:"mall_id"`
	DataKind        string     `gorm:"column:data_kind;size:32;not null;uniqueIndex:uk_weather_latest,priority:2;index" json:"data_kind"`
	BusinessKey     string     `gorm:"column:business_key;size:255;not null;uniqueIndex:uk_weather_latest,priority:3" json:"business_key"`
	BusinessTime    *time.Time `gorm:"column:business_time;type:datetime(3);index" json:"business_time"`
	BusinessDate    *time.Time `gorm:"column:business_date;type:date;index" json:"business_date"`
	Subtype         string     `gorm:"column:subtype;size:128" json:"subtype"`
	SourceRowID     uint       `gorm:"column:source_row_id;not null" json:"source_row_id"`
	IssuedAtUTC     time.Time  `gorm:"column:issued_at_utc;type:datetime(3);not null" json:"issued_at_utc"`
	FetchedAtUTC    time.Time  `gorm:"column:fetched_at_utc;type:datetime(3);not null" json:"fetched_at_utc"`
	FreshnessStatus string     `gorm:"column:freshness_status;size:16;not null;index" json:"freshness_status"`
	WeatherTimestamps
}

func (MallWeatherLatest) TableName() string { return "mall_weather_latest" }
