package model

import "time"

type MallWeatherRealtime struct {
	BaseModel
	MallID                  uint      `gorm:"column:mall_id;not null;uniqueIndex:uk_realtime_version,priority:1;index:idx_realtime_latest,priority:1" json:"mall_id"`
	Provider                string    `gorm:"column:provider;size:16;not null;uniqueIndex:uk_realtime_version,priority:2" json:"provider"`
	SnapshotAtUTC           time.Time `gorm:"column:snapshot_at_utc;type:datetime(3);not null;uniqueIndex:uk_realtime_version,priority:3;index:idx_realtime_latest,priority:2,sort:desc" json:"snapshot_at_utc"`
	ProviderServerTimeUTC   time.Time `gorm:"column:provider_server_time_utc;type:datetime(3);not null" json:"provider_server_time_utc"`
	FetchedAtUTC            time.Time `gorm:"column:fetched_at_utc;type:datetime(3);not null;index" json:"fetched_at_utc"`
	TemperatureC            *float64  `gorm:"column:temperature_c;type:decimal(6,2)" json:"temperature_c"`
	ApparentTemperatureC    *float64  `gorm:"column:apparent_temperature_c;type:decimal(6,2)" json:"apparent_temperature_c"`
	HumidityRatio           *float64  `gorm:"column:humidity_ratio;type:decimal(8,6)" json:"humidity_ratio"`
	PressurePa              *float64  `gorm:"column:pressure_pa;type:decimal(12,3)" json:"pressure_pa"`
	WindSpeedKPH            *float64  `gorm:"column:wind_speed_kph;type:decimal(10,4)" json:"wind_speed_kph"`
	WindDirectionDeg        *float64  `gorm:"column:wind_direction_deg;type:decimal(7,3)" json:"wind_direction_deg"`
	CloudrateRatio          *float64  `gorm:"column:cloudrate_ratio;type:decimal(8,6)" json:"cloudrate_ratio"`
	VisibilityKM            *float64  `gorm:"column:visibility_km;type:decimal(10,3)" json:"visibility_km"`
	DSWRFWM2                *float64  `gorm:"column:dswrf_w_m2;type:decimal(12,3)" json:"dswrf_w_m2"`
	Skycon                  string    `gorm:"column:skycon;size:64;index" json:"skycon"`
	LocalPrecipStatus       string    `gorm:"column:local_precip_status;size:32" json:"local_precip_status"`
	LocalPrecipMMH          *float64  `gorm:"column:local_precip_mm_h;type:decimal(10,4)" json:"local_precip_mm_h"`
	LocalPrecipDatasource   string    `gorm:"column:local_precip_datasource;size:128" json:"local_precip_datasource"`
	NearestPrecipStatus     string    `gorm:"column:nearest_precip_status;size:32" json:"nearest_precip_status"`
	NearestPrecipDistanceKM *float64  `gorm:"column:nearest_precip_distance_km;type:decimal(10,3)" json:"nearest_precip_distance_km"`
	NearestPrecipMMH        *float64  `gorm:"column:nearest_precip_mm_h;type:decimal(10,4)" json:"nearest_precip_mm_h"`
	PM25UGM3                *float64  `gorm:"column:pm25_ug_m3;type:decimal(10,3)" json:"pm25_ug_m3"`
	PM10UGM3                *float64  `gorm:"column:pm10_ug_m3;type:decimal(10,3)" json:"pm10_ug_m3"`
	O3UGM3                  *float64  `gorm:"column:o3_ug_m3;type:decimal(10,3)" json:"o3_ug_m3"`
	SO2UGM3                 *float64  `gorm:"column:so2_ug_m3;type:decimal(10,3)" json:"so2_ug_m3"`
	NO2UGM3                 *float64  `gorm:"column:no2_ug_m3;type:decimal(10,3)" json:"no2_ug_m3"`
	COMGM3                  *float64  `gorm:"column:co_mg_m3;type:decimal(10,3)" json:"co_mg_m3"`
	AQIChn                  *int      `gorm:"column:aqi_chn;index" json:"aqi_chn"`
	AQIUSA                  *int      `gorm:"column:aqi_usa" json:"aqi_usa"`
	AQIDescChn              string    `gorm:"column:aqi_desc_chn;size:64" json:"aqi_desc_chn"`
	AQIDescUSA              string    `gorm:"column:aqi_desc_usa;size:64" json:"aqi_desc_usa"`
	ComfortIndex            *int      `gorm:"column:comfort_index" json:"comfort_index"`
	ComfortDesc             string    `gorm:"column:comfort_desc;size:255" json:"comfort_desc"`
	UltravioletIndex        *int      `gorm:"column:ultraviolet_index" json:"ultraviolet_index"`
	UltravioletDesc         string    `gorm:"column:ultraviolet_desc;size:255" json:"ultraviolet_desc"`
	WeatherQualityFields
	WeatherTimestamps
}

func (MallWeatherRealtime) TableName() string { return "mall_weather_realtime" }
