package model

import "time"

type MallWeatherMinutely struct {
	BaseModel
	MallID            uint      `gorm:"column:mall_id;not null;uniqueIndex:uk_minutely_version,priority:1;index:idx_minutely_latest,priority:1" json:"mall_id"`
	Provider          string    `gorm:"column:provider;size:16;not null;uniqueIndex:uk_minutely_version,priority:2" json:"provider"`
	ForecastMinuteUTC time.Time `gorm:"column:forecast_minute_utc;type:datetime(3);not null;uniqueIndex:uk_minutely_version,priority:3;index:idx_minutely_latest,priority:2" json:"forecast_minute_utc"`
	IssuedAtUTC       time.Time `gorm:"column:issued_at_utc;type:datetime(3);not null;uniqueIndex:uk_minutely_version,priority:4;index:idx_minutely_latest,priority:3,sort:desc" json:"issued_at_utc"`
	FetchedAtUTC      time.Time `gorm:"column:fetched_at_utc;type:datetime(3);not null;index" json:"fetched_at_utc"`
	MinuteOffset      int       `gorm:"column:minute_offset;not null" json:"minute_offset"`
	PrecipitationMMH  *float64  `gorm:"column:precipitation_mm_h;type:decimal(10,4)" json:"precipitation_mm_h"`
	ProbabilityRatio  *float64  `gorm:"column:probability_ratio;type:decimal(8,6)" json:"probability_ratio"`
	ProbabilityWindow *int      `gorm:"column:probability_window" json:"probability_window"`
	Datasource        string    `gorm:"column:datasource;size:128" json:"datasource"`
	Description       string    `gorm:"column:description;type:text" json:"description"`
	ForecastKeypoint  string    `gorm:"column:forecast_keypoint;type:text" json:"forecast_keypoint"`
	WeatherQualityFields
	WeatherTimestamps
}

func (MallWeatherMinutely) TableName() string { return "mall_weather_minutely" }
