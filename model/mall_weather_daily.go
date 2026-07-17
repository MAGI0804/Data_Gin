package model

import "time"

type MallWeatherDaily struct {
	BaseModel
	MallID            uint      `gorm:"column:mall_id;not null;uniqueIndex:uk_daily_version,priority:1;index" json:"mall_id"`
	Provider          string    `gorm:"column:provider;size:16;not null;uniqueIndex:uk_daily_version,priority:2" json:"provider"`
	ForecastDateLocal time.Time `gorm:"column:forecast_date_local;type:date;not null;uniqueIndex:uk_daily_version,priority:3;index" json:"forecast_date_local"`
	IssuedAtUTC       time.Time `gorm:"column:issued_at_utc;type:datetime(3);not null;uniqueIndex:uk_daily_version,priority:4;index" json:"issued_at_utc"`
	FetchedAtUTC      time.Time `gorm:"column:fetched_at_utc;type:datetime(3);not null;index" json:"fetched_at_utc"`

	TemperatureMaxC      *float64 `gorm:"column:temperature_max_c;type:decimal(6,2)" json:"temperature_max_c"`
	TemperatureMinC      *float64 `gorm:"column:temperature_min_c;type:decimal(6,2)" json:"temperature_min_c"`
	TemperatureAvgC      *float64 `gorm:"column:temperature_avg_c;type:decimal(6,2)" json:"temperature_avg_c"`
	DayTemperatureMaxC   *float64 `gorm:"column:day_temperature_max_c;type:decimal(6,2)" json:"day_temperature_max_c"`
	DayTemperatureMinC   *float64 `gorm:"column:day_temperature_min_c;type:decimal(6,2)" json:"day_temperature_min_c"`
	DayTemperatureAvgC   *float64 `gorm:"column:day_temperature_avg_c;type:decimal(6,2)" json:"day_temperature_avg_c"`
	NightTemperatureMaxC *float64 `gorm:"column:night_temperature_max_c;type:decimal(6,2)" json:"night_temperature_max_c"`
	NightTemperatureMinC *float64 `gorm:"column:night_temperature_min_c;type:decimal(6,2)" json:"night_temperature_min_c"`
	NightTemperatureAvgC *float64 `gorm:"column:night_temperature_avg_c;type:decimal(6,2)" json:"night_temperature_avg_c"`

	PrecipitationMaxMMH              *float64 `gorm:"column:precipitation_max_mm_h;type:decimal(10,4)" json:"precipitation_max_mm_h"`
	PrecipitationMinMMH              *float64 `gorm:"column:precipitation_min_mm_h;type:decimal(10,4)" json:"precipitation_min_mm_h"`
	PrecipitationAvgMMH              *float64 `gorm:"column:precipitation_avg_mm_h;type:decimal(10,4)" json:"precipitation_avg_mm_h"`
	PrecipitationProbabilityPct      *float64 `gorm:"column:precipitation_probability_pct;type:decimal(7,3)" json:"precipitation_probability_pct"`
	DayPrecipitationMaxMMH           *float64 `gorm:"column:day_precipitation_max_mm_h;type:decimal(10,4)" json:"day_precipitation_max_mm_h"`
	DayPrecipitationMinMMH           *float64 `gorm:"column:day_precipitation_min_mm_h;type:decimal(10,4)" json:"day_precipitation_min_mm_h"`
	DayPrecipitationAvgMMH           *float64 `gorm:"column:day_precipitation_avg_mm_h;type:decimal(10,4)" json:"day_precipitation_avg_mm_h"`
	DayPrecipitationProbabilityPct   *float64 `gorm:"column:day_precipitation_probability_pct;type:decimal(7,3)" json:"day_precipitation_probability_pct"`
	NightPrecipitationMaxMMH         *float64 `gorm:"column:night_precipitation_max_mm_h;type:decimal(10,4)" json:"night_precipitation_max_mm_h"`
	NightPrecipitationMinMMH         *float64 `gorm:"column:night_precipitation_min_mm_h;type:decimal(10,4)" json:"night_precipitation_min_mm_h"`
	NightPrecipitationAvgMMH         *float64 `gorm:"column:night_precipitation_avg_mm_h;type:decimal(10,4)" json:"night_precipitation_avg_mm_h"`
	NightPrecipitationProbabilityPct *float64 `gorm:"column:night_precipitation_probability_pct;type:decimal(7,3)" json:"night_precipitation_probability_pct"`

	WindMaxSpeedKPH          *float64 `gorm:"column:wind_max_speed_kph;type:decimal(10,4)" json:"wind_max_speed_kph"`
	WindMaxDirectionDeg      *float64 `gorm:"column:wind_max_direction_deg;type:decimal(7,3)" json:"wind_max_direction_deg"`
	WindMinSpeedKPH          *float64 `gorm:"column:wind_min_speed_kph;type:decimal(10,4)" json:"wind_min_speed_kph"`
	WindMinDirectionDeg      *float64 `gorm:"column:wind_min_direction_deg;type:decimal(7,3)" json:"wind_min_direction_deg"`
	WindAvgSpeedKPH          *float64 `gorm:"column:wind_avg_speed_kph;type:decimal(10,4)" json:"wind_avg_speed_kph"`
	WindAvgDirectionDeg      *float64 `gorm:"column:wind_avg_direction_deg;type:decimal(7,3)" json:"wind_avg_direction_deg"`
	DayWindMaxSpeedKPH       *float64 `gorm:"column:day_wind_max_speed_kph;type:decimal(10,4)" json:"day_wind_max_speed_kph"`
	DayWindMaxDirectionDeg   *float64 `gorm:"column:day_wind_max_direction_deg;type:decimal(7,3)" json:"day_wind_max_direction_deg"`
	DayWindMinSpeedKPH       *float64 `gorm:"column:day_wind_min_speed_kph;type:decimal(10,4)" json:"day_wind_min_speed_kph"`
	DayWindMinDirectionDeg   *float64 `gorm:"column:day_wind_min_direction_deg;type:decimal(7,3)" json:"day_wind_min_direction_deg"`
	DayWindAvgSpeedKPH       *float64 `gorm:"column:day_wind_avg_speed_kph;type:decimal(10,4)" json:"day_wind_avg_speed_kph"`
	DayWindAvgDirectionDeg   *float64 `gorm:"column:day_wind_avg_direction_deg;type:decimal(7,3)" json:"day_wind_avg_direction_deg"`
	NightWindMaxSpeedKPH     *float64 `gorm:"column:night_wind_max_speed_kph;type:decimal(10,4)" json:"night_wind_max_speed_kph"`
	NightWindMaxDirectionDeg *float64 `gorm:"column:night_wind_max_direction_deg;type:decimal(7,3)" json:"night_wind_max_direction_deg"`
	NightWindMinSpeedKPH     *float64 `gorm:"column:night_wind_min_speed_kph;type:decimal(10,4)" json:"night_wind_min_speed_kph"`
	NightWindMinDirectionDeg *float64 `gorm:"column:night_wind_min_direction_deg;type:decimal(7,3)" json:"night_wind_min_direction_deg"`
	NightWindAvgSpeedKPH     *float64 `gorm:"column:night_wind_avg_speed_kph;type:decimal(10,4)" json:"night_wind_avg_speed_kph"`
	NightWindAvgDirectionDeg *float64 `gorm:"column:night_wind_avg_direction_deg;type:decimal(7,3)" json:"night_wind_avg_direction_deg"`

	HumidityMaxRatio   *float64 `gorm:"column:humidity_max_ratio;type:decimal(8,6)" json:"humidity_max_ratio"`
	HumidityMinRatio   *float64 `gorm:"column:humidity_min_ratio;type:decimal(8,6)" json:"humidity_min_ratio"`
	HumidityAvgRatio   *float64 `gorm:"column:humidity_avg_ratio;type:decimal(8,6)" json:"humidity_avg_ratio"`
	CloudrateMaxRatio  *float64 `gorm:"column:cloudrate_max_ratio;type:decimal(8,6)" json:"cloudrate_max_ratio"`
	CloudrateMinRatio  *float64 `gorm:"column:cloudrate_min_ratio;type:decimal(8,6)" json:"cloudrate_min_ratio"`
	CloudrateAvgRatio  *float64 `gorm:"column:cloudrate_avg_ratio;type:decimal(8,6)" json:"cloudrate_avg_ratio"`
	PressureMaxPa      *float64 `gorm:"column:pressure_max_pa;type:decimal(12,3)" json:"pressure_max_pa"`
	PressureMinPa      *float64 `gorm:"column:pressure_min_pa;type:decimal(12,3)" json:"pressure_min_pa"`
	PressureAvgPa      *float64 `gorm:"column:pressure_avg_pa;type:decimal(12,3)" json:"pressure_avg_pa"`
	VisibilityMaxKM    *float64 `gorm:"column:visibility_max_km;type:decimal(10,3)" json:"visibility_max_km"`
	VisibilityMinKM    *float64 `gorm:"column:visibility_min_km;type:decimal(10,3)" json:"visibility_min_km"`
	VisibilityAvgKM    *float64 `gorm:"column:visibility_avg_km;type:decimal(10,3)" json:"visibility_avg_km"`
	DSWRFMaxWM2        *float64 `gorm:"column:dswrf_max_w_m2;type:decimal(12,3)" json:"dswrf_max_w_m2"`
	DSWRFMinWM2        *float64 `gorm:"column:dswrf_min_w_m2;type:decimal(12,3)" json:"dswrf_min_w_m2"`
	DSWRFAvgWM2        *float64 `gorm:"column:dswrf_avg_w_m2;type:decimal(12,3)" json:"dswrf_avg_w_m2"`
	PM25MaxUGM3        *float64 `gorm:"column:pm25_max_ug_m3;type:decimal(10,3)" json:"pm25_max_ug_m3"`
	PM25MinUGM3        *float64 `gorm:"column:pm25_min_ug_m3;type:decimal(10,3)" json:"pm25_min_ug_m3"`
	PM25AvgUGM3        *float64 `gorm:"column:pm25_avg_ug_m3;type:decimal(10,3)" json:"pm25_avg_ug_m3"`
	AQIMaxChn          *int     `gorm:"column:aqi_max_chn" json:"aqi_max_chn"`
	AQIMinChn          *int     `gorm:"column:aqi_min_chn" json:"aqi_min_chn"`
	AQIAvgChn          *int     `gorm:"column:aqi_avg_chn" json:"aqi_avg_chn"`
	AQIMaxUSA          *int     `gorm:"column:aqi_max_usa" json:"aqi_max_usa"`
	AQIMinUSA          *int     `gorm:"column:aqi_min_usa" json:"aqi_min_usa"`
	AQIAvgUSA          *int     `gorm:"column:aqi_avg_usa" json:"aqi_avg_usa"`
	Skycon             string   `gorm:"column:skycon;size:64" json:"skycon"`
	DaySkycon          string   `gorm:"column:day_skycon;size:64" json:"day_skycon"`
	NightSkycon        string   `gorm:"column:night_skycon;size:64" json:"night_skycon"`
	SunriseLocalTime   string   `gorm:"column:sunrise_local_time;size:16" json:"sunrise_local_time"`
	SunsetLocalTime    string   `gorm:"column:sunset_local_time;size:16" json:"sunset_local_time"`
	BasicLifeIndexJSON JSONText `gorm:"column:basic_life_index_json;type:json" json:"basic_life_index_json"`
	WeatherQualityFields
	WeatherTimestamps
}

func (MallWeatherDaily) TableName() string { return "mall_weather_daily" }
