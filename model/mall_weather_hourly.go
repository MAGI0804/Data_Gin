package model

import "time"

type MallWeatherHourly struct {
	BaseModel
	MallID               uint      `gorm:"column:mall_id;not null;uniqueIndex:uk_hourly_version,priority:1;index:idx_hourly_latest,priority:1;index:idx_hourly_city_query,priority:2" json:"mall_id"`
	Provider             string    `gorm:"column:provider;size:16;not null;uniqueIndex:uk_hourly_version,priority:2" json:"provider"`
	ForecastTimeUTC      time.Time `gorm:"column:forecast_time_utc;type:datetime(3);not null;uniqueIndex:uk_hourly_version,priority:3;index:idx_hourly_latest,priority:2;index:idx_hourly_city_query,priority:1" json:"forecast_time_utc"`
	IssuedAtUTC          time.Time `gorm:"column:issued_at_utc;type:datetime(3);not null;uniqueIndex:uk_hourly_version,priority:4;index:idx_hourly_latest,priority:3,sort:desc" json:"issued_at_utc"`
	FetchedAtUTC         time.Time `gorm:"column:fetched_at_utc;type:datetime(3);not null;index" json:"fetched_at_utc"`
	TemperatureC         *float64  `gorm:"column:temperature_c;type:decimal(6,2)" json:"temperature_c"`
	ApparentTemperatureC *float64  `gorm:"column:apparent_temperature_c;type:decimal(6,2)" json:"apparent_temperature_c"`
	PressurePa           *float64  `gorm:"column:pressure_pa;type:decimal(12,3)" json:"pressure_pa"`
	HumidityRatio        *float64  `gorm:"column:humidity_ratio;type:decimal(8,6)" json:"humidity_ratio"`
	WindSpeedKPH         *float64  `gorm:"column:wind_speed_kph;type:decimal(10,4)" json:"wind_speed_kph"`
	WindDirectionDeg     *float64  `gorm:"column:wind_direction_deg;type:decimal(7,3)" json:"wind_direction_deg"`
	PrecipitationMMH     *float64  `gorm:"column:precipitation_mm_h;type:decimal(10,4)" json:"precipitation_mm_h"`
	PrecipProbabilityPct *float64  `gorm:"column:precip_probability_pct;type:decimal(7,3)" json:"precip_probability_pct"`
	CloudrateRatio       *float64  `gorm:"column:cloudrate_ratio;type:decimal(8,6)" json:"cloudrate_ratio"`
	DSWRFWM2             *float64  `gorm:"column:dswrf_w_m2;type:decimal(12,3)" json:"dswrf_w_m2"`
	VisibilityKM         *float64  `gorm:"column:visibility_km;type:decimal(10,3)" json:"visibility_km"`
	Skycon               string    `gorm:"column:skycon;size:64;index" json:"skycon"`
	PM25UGM3             *float64  `gorm:"column:pm25_ug_m3;type:decimal(10,3)" json:"pm25_ug_m3"`
	AQIChn               *int      `gorm:"column:aqi_chn" json:"aqi_chn"`
	AQIUSA               *int      `gorm:"column:aqi_usa" json:"aqi_usa"`
	HourlyDescription    string    `gorm:"column:hourly_description;type:text" json:"hourly_description"`
	ForecastKeypoint     string    `gorm:"column:forecast_keypoint;type:text" json:"forecast_keypoint"`
	WeatherQualityFields
	WeatherTimestamps
}

func (MallWeatherHourly) TableName() string { return "mall_weather_hourly" }
