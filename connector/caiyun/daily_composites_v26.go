package caiyun

import (
	"encoding/json"
	"time"
)

func mergeDailyPrecipitation(
	raw json.RawMessage,
	path string,
	issuedAtUTC time.Time,
	zone *time.Location,
	rows map[time.Time]*DailyForecast,
	warnings *[]ParseWarning,
	precipitation func(*DailyForecast) *DailyPrecipitation,
) {
	mergeDailyItems(raw, path, false, issuedAtUTC, zone, rows, warnings, func(item dailyItem, itemPath string, row *DailyForecast) {
		value := precipitation(row)
		value.Maximum = decodeWeatherFloat(item.Maximum, itemPath+".max", 0, 2000, true, warnings)
		value.Minimum = decodeWeatherFloat(item.Minimum, itemPath+".min", 0, 2000, true, warnings)
		value.Average = decodeWeatherFloat(item.Average, itemPath+".avg", 0, 2000, true, warnings)
		value.ProbabilityPct = decodeWeatherFloat(item.Probability, itemPath+".probability", 0, 100, false, warnings)
		warnDailyMetricOrder(value.DailyMetric, itemPath, warnings)
	})
}

func mergeDailyWind(
	raw json.RawMessage,
	path string,
	issuedAtUTC time.Time,
	zone *time.Location,
	rows map[time.Time]*DailyForecast,
	warnings *[]ParseWarning,
	wind func(*DailyForecast) *DailyWind,
) {
	mergeDailyItems(raw, path, false, issuedAtUTC, zone, rows, warnings, func(item dailyItem, itemPath string, row *DailyForecast) {
		value := wind(row)
		value.Maximum = decodeDailyWindValue(item.Maximum, itemPath+".max", warnings)
		value.Minimum = decodeDailyWindValue(item.Minimum, itemPath+".min", warnings)
		value.Average = decodeDailyWindValue(item.Average, itemPath+".avg", warnings)
		warnDailyWindOrder(*value, itemPath, warnings)
	})
}

func decodeDailyWindValue(raw json.RawMessage, path string, warnings *[]ParseWarning) DailyWindValue {
	var payload struct {
		Speed     json.RawMessage `json:"speed"`
		Direction json.RawMessage `json:"direction"`
	}
	if !decodeWeatherObject(raw, path, true, &payload, warnings) {
		return DailyWindValue{}
	}
	return DailyWindValue{
		SpeedKPH:     decodeWeatherFloat(payload.Speed, path+".speed", 0, 500, true, warnings),
		DirectionDeg: decodeWeatherFloat(payload.Direction, path+".direction", 0, 360, true, warnings),
	}
}

func warnDailyWindOrder(wind DailyWind, path string, warnings *[]ParseWarning) {
	if wind.Maximum.SpeedKPH != nil && wind.Minimum.SpeedKPH != nil && *wind.Maximum.SpeedKPH < *wind.Minimum.SpeedKPH {
		*warnings = append(*warnings, ParseWarning{Code: "INVALID_RANGE_ORDER", Path: path})
	}
	if (wind.Average.SpeedKPH != nil && wind.Minimum.SpeedKPH != nil && *wind.Average.SpeedKPH < *wind.Minimum.SpeedKPH) ||
		(wind.Average.SpeedKPH != nil && wind.Maximum.SpeedKPH != nil && *wind.Average.SpeedKPH > *wind.Maximum.SpeedKPH) {
		*warnings = append(*warnings, ParseWarning{Code: "INVALID_AVERAGE", Path: path + ".avg.speed"})
	}
}

func mergeDailyAirQuality(raw json.RawMessage, issuedAtUTC time.Time, zone *time.Location, rows map[time.Time]*DailyForecast, warnings *[]ParseWarning) {
	var payload struct {
		PM25 json.RawMessage `json:"pm25"`
		AQI  json.RawMessage `json:"aqi"`
	}
	if !decodeWeatherObject(raw, "result.daily.air_quality", false, &payload, warnings) {
		return
	}
	mergeDailyMetric(payload.PM25, "result.daily.air_quality.pm25", false, 0, 10000, issuedAtUTC, zone, rows, warnings,
		func(row *DailyForecast) *DailyMetric { return &row.AirQuality.PM25 })
	const path = "result.daily.air_quality.aqi"
	mergeDailyItems(payload.AQI, path, false, issuedAtUTC, zone, rows, warnings, func(item dailyItem, itemPath string, row *DailyForecast) {
		maximum := decodeDailyAQIValue(item.Maximum, itemPath+".max", warnings)
		minimum := decodeDailyAQIValue(item.Minimum, itemPath+".min", warnings)
		average := decodeDailyAQIValue(item.Average, itemPath+".avg", warnings)
		row.AirQuality.AQIChn = DailyIntegerMetric{Maximum: maximum.Chn, Minimum: minimum.Chn, Average: average.Chn}
		row.AirQuality.AQIUSA = DailyIntegerMetric{Maximum: maximum.USA, Minimum: minimum.USA, Average: average.USA}
		warnDailyIntegerMetricOrder(row.AirQuality.AQIChn, itemPath, "chn", warnings)
		warnDailyIntegerMetricOrder(row.AirQuality.AQIUSA, itemPath, "usa", warnings)
	})
}

type dailyAQIValue struct {
	Chn *int
	USA *int
}

func decodeDailyAQIValue(raw json.RawMessage, path string, warnings *[]ParseWarning) dailyAQIValue {
	var payload struct {
		Chn json.RawMessage `json:"chn"`
		USA json.RawMessage `json:"usa"`
	}
	if !decodeWeatherObject(raw, path, true, &payload, warnings) {
		return dailyAQIValue{}
	}
	return dailyAQIValue{
		Chn: decodeWeatherInt(payload.Chn, path+".chn", 0, 5000, false, warnings),
		USA: decodeWeatherInt(payload.USA, path+".usa", 0, 5000, false, warnings),
	}
}

func warnDailyIntegerMetricOrder(metric DailyIntegerMetric, path, standard string, warnings *[]ParseWarning) {
	if metric.Maximum != nil && metric.Minimum != nil && *metric.Maximum < *metric.Minimum {
		*warnings = append(*warnings, ParseWarning{Code: "INVALID_RANGE_ORDER", Path: path + "." + standard})
	}
	if (metric.Average != nil && metric.Minimum != nil && *metric.Average < *metric.Minimum) ||
		(metric.Average != nil && metric.Maximum != nil && *metric.Average > *metric.Maximum) {
		*warnings = append(*warnings, ParseWarning{Code: "INVALID_AVERAGE", Path: path + ".avg." + standard})
	}
}
