package caiyun

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

const (
	maximumHourlySeriesItems      = 720
	hourlyHistoryTolerance        = 2 * time.Hour
	hourlyForecastWindow          = 360 * time.Hour
	hourlyFutureTolerance         = 2 * time.Hour
	maximumHourlyDescriptionRunes = 16000
)

type HourlyForecast struct {
	ForecastTimeUTC      time.Time
	TemperatureC         *float64
	ApparentTemperatureC *float64
	PressurePa           *float64
	HumidityRatio        *float64
	WindSpeedKPH         *float64
	WindDirectionDeg     *float64
	PrecipitationMMH     *float64
	PrecipProbabilityPct *float64
	CloudrateRatio       *float64
	DSWRFWM2             *float64
	VisibilityKM         *float64
	Skycon               string
	PM25UGM3             *float64
	AQIChn               *int
	AQIUSA               *int
	HourlyDescription    string
	ForecastKeypoint     string
}

type HourlyBundle struct {
	Status       string
	IssuedAtUTC  time.Time
	Forecasts    []HourlyForecast
	Warnings     []ParseWarning
	ProviderJSON json.RawMessage
}

type hourlyPayload struct {
	Status              json.RawMessage `json:"status"`
	Description         json.RawMessage `json:"description"`
	Temperature         json.RawMessage `json:"temperature"`
	ApparentTemperature json.RawMessage `json:"apparent_temperature"`
	Pressure            json.RawMessage `json:"pressure"`
	Humidity            json.RawMessage `json:"humidity"`
	Wind                json.RawMessage `json:"wind"`
	Precipitation       json.RawMessage `json:"precipitation"`
	Cloudrate           json.RawMessage `json:"cloudrate"`
	DSWRF               json.RawMessage `json:"dswrf"`
	Visibility          json.RawMessage `json:"visibility"`
	Skycon              json.RawMessage `json:"skycon"`
	AirQuality          json.RawMessage `json:"air_quality"`
}

type hourlyItem struct {
	Datetime    json.RawMessage `json:"datetime"`
	Value       json.RawMessage `json:"value"`
	Speed       json.RawMessage `json:"speed"`
	Direction   json.RawMessage `json:"direction"`
	Probability json.RawMessage `json:"probability"`
}

func ParseHourlyV26(weather *WeatherBundle) (*HourlyBundle, error) {
	if weather == nil || !isJSONObject(weather.HourlyJSON) || weather.Metadata.ServerTimeUTC.IsZero() {
		return nil, weatherParseError()
	}
	issuedAtUTC := weather.Metadata.ServerTimeUTC.UTC()
	if unixTime := issuedAtUTC.Unix(); unixTime < minimumWeatherUnixTime || unixTime >= maximumWeatherUnixTime {
		return nil, weatherParseError()
	}
	var payload hourlyPayload
	if err := json.Unmarshal(weather.HourlyJSON, &payload); err != nil {
		return nil, weatherParseError()
	}
	warnings := make([]ParseWarning, 0)
	status := decodeWeatherString(payload.Status, "result.hourly.status", 32, true, &warnings)
	if status != "" && status != "ok" {
		warnings = append(warnings, ParseWarning{Code: "MODULE_STATUS_NOT_OK", Path: "result.hourly.status"})
	}
	description := decodeWeatherString(payload.Description, "result.hourly.description", maximumHourlyDescriptionRunes, false, &warnings)
	rows := make(map[time.Time]*HourlyForecast)

	mergeHourlyFloat(payload.Temperature, "result.hourly.temperature", true, -100, 100, issuedAtUTC, rows, &warnings,
		func(row *HourlyForecast, value *float64) { row.TemperatureC = value })
	mergeHourlyFloat(payload.ApparentTemperature, "result.hourly.apparent_temperature", false, -150, 150, issuedAtUTC, rows, &warnings,
		func(row *HourlyForecast, value *float64) { row.ApparentTemperatureC = value })
	mergeHourlyFloat(payload.Pressure, "result.hourly.pressure", false, 20000, 120000, issuedAtUTC, rows, &warnings,
		func(row *HourlyForecast, value *float64) { row.PressurePa = value })
	mergeHourlyFloat(payload.Humidity, "result.hourly.humidity", false, 0, 1, issuedAtUTC, rows, &warnings,
		func(row *HourlyForecast, value *float64) { row.HumidityRatio = value })
	mergeHourlyFloat(payload.Cloudrate, "result.hourly.cloudrate", false, 0, 1, issuedAtUTC, rows, &warnings,
		func(row *HourlyForecast, value *float64) { row.CloudrateRatio = value })
	mergeHourlyFloat(payload.DSWRF, "result.hourly.dswrf", false, 0, 2000, issuedAtUTC, rows, &warnings,
		func(row *HourlyForecast, value *float64) { row.DSWRFWM2 = value })
	mergeHourlyFloat(payload.Visibility, "result.hourly.visibility", false, 0, 1000, issuedAtUTC, rows, &warnings,
		func(row *HourlyForecast, value *float64) { row.VisibilityKM = value })
	mergeHourlySkycon(payload.Skycon, issuedAtUTC, rows, &warnings)
	mergeHourlyWind(payload.Wind, issuedAtUTC, rows, &warnings)
	mergeHourlyPrecipitation(payload.Precipitation, issuedAtUTC, rows, &warnings)
	mergeHourlyAirQuality(payload.AirQuality, issuedAtUTC, rows, &warnings)
	if len(rows) == 0 {
		return nil, weatherParseError()
	}

	times := make([]time.Time, 0, len(rows))
	for forecastTime := range rows {
		times = append(times, forecastTime)
	}
	sort.Slice(times, func(left, right int) bool { return times[left].Before(times[right]) })
	forecasts := make([]HourlyForecast, 0, len(times))
	missingTemperature := 0
	missingSkycon := 0
	irregularInterval := false
	for index, forecastTime := range times {
		row := rows[forecastTime]
		row.HourlyDescription = description
		row.ForecastKeypoint = weather.Metadata.ForecastKeypoint
		if row.TemperatureC == nil {
			missingTemperature++
		}
		if row.Skycon == "" {
			missingSkycon++
		}
		if index > 0 && forecastTime.Sub(times[index-1]) != time.Hour {
			irregularInterval = true
		}
		forecasts = append(forecasts, *row)
	}
	if missingTemperature*2 >= len(forecasts) {
		warnings = append(warnings, ParseWarning{Code: "CORE_FIELD_COVERAGE_LOW", Path: "result.hourly.temperature"})
	}
	if missingSkycon*2 >= len(forecasts) {
		warnings = append(warnings, ParseWarning{Code: "CORE_FIELD_COVERAGE_LOW", Path: "result.hourly.skycon"})
	}
	if irregularInterval {
		warnings = append(warnings, ParseWarning{Code: "IRREGULAR_TIME_INTERVAL", Path: "result.hourly"})
	}
	return &HourlyBundle{
		Status: status, IssuedAtUTC: issuedAtUTC, Forecasts: forecasts,
		Warnings: warnings, ProviderJSON: cloneRawMessage(weather.HourlyJSON),
	}, nil
}

func mergeHourlyFloat(
	raw json.RawMessage,
	path string,
	required bool,
	minimum float64,
	maximum float64,
	issuedAtUTC time.Time,
	rows map[time.Time]*HourlyForecast,
	warnings *[]ParseWarning,
	set func(*HourlyForecast, *float64),
) {
	mergeHourlyItems(raw, path, required, issuedAtUTC, rows, warnings, func(item hourlyItem, itemPath string, row *HourlyForecast) {
		set(row, decodeWeatherFloat(item.Value, itemPath+".value", minimum, maximum, true, warnings))
	})
}

func mergeHourlySkycon(raw json.RawMessage, issuedAtUTC time.Time, rows map[time.Time]*HourlyForecast, warnings *[]ParseWarning) {
	const path = "result.hourly.skycon"
	mergeHourlyItems(raw, path, true, issuedAtUTC, rows, warnings, func(item hourlyItem, itemPath string, row *HourlyForecast) {
		row.Skycon = decodeWeatherString(item.Value, itemPath+".value", 64, true, warnings)
	})
}

func mergeHourlyWind(raw json.RawMessage, issuedAtUTC time.Time, rows map[time.Time]*HourlyForecast, warnings *[]ParseWarning) {
	const path = "result.hourly.wind"
	mergeHourlyItems(raw, path, false, issuedAtUTC, rows, warnings, func(item hourlyItem, itemPath string, row *HourlyForecast) {
		row.WindSpeedKPH = decodeWeatherFloat(item.Speed, itemPath+".speed", 0, 500, true, warnings)
		row.WindDirectionDeg = decodeWeatherFloat(item.Direction, itemPath+".direction", 0, 360, true, warnings)
	})
}

func mergeHourlyPrecipitation(raw json.RawMessage, issuedAtUTC time.Time, rows map[time.Time]*HourlyForecast, warnings *[]ParseWarning) {
	const path = "result.hourly.precipitation"
	mergeHourlyItems(raw, path, false, issuedAtUTC, rows, warnings, func(item hourlyItem, itemPath string, row *HourlyForecast) {
		row.PrecipitationMMH = decodeWeatherFloat(item.Value, itemPath+".value", 0, 2000, true, warnings)
		row.PrecipProbabilityPct = decodeWeatherFloat(item.Probability, itemPath+".probability", 0, 100, false, warnings)
	})
}

func mergeHourlyAirQuality(raw json.RawMessage, issuedAtUTC time.Time, rows map[time.Time]*HourlyForecast, warnings *[]ParseWarning) {
	var payload struct {
		PM25 json.RawMessage `json:"pm25"`
		AQI  json.RawMessage `json:"aqi"`
	}
	if !decodeWeatherObject(raw, "result.hourly.air_quality", false, &payload, warnings) {
		return
	}
	mergeHourlyFloat(payload.PM25, "result.hourly.air_quality.pm25", false, 0, 10000, issuedAtUTC, rows, warnings,
		func(row *HourlyForecast, value *float64) { row.PM25UGM3 = value })
	const path = "result.hourly.air_quality.aqi"
	mergeHourlyItems(payload.AQI, path, false, issuedAtUTC, rows, warnings, func(item hourlyItem, itemPath string, row *HourlyForecast) {
		var value struct {
			Chn json.RawMessage `json:"chn"`
			USA json.RawMessage `json:"usa"`
		}
		if !decodeWeatherObject(item.Value, itemPath+".value", true, &value, warnings) {
			row.AQIChn = nil
			row.AQIUSA = nil
			return
		}
		row.AQIChn = decodeWeatherInt(value.Chn, itemPath+".value.chn", 0, 5000, false, warnings)
		row.AQIUSA = decodeWeatherInt(value.USA, itemPath+".value.usa", 0, 5000, false, warnings)
	})
}

func mergeHourlyItems(
	raw json.RawMessage,
	path string,
	required bool,
	issuedAtUTC time.Time,
	rows map[time.Time]*HourlyForecast,
	warnings *[]ParseWarning,
	merge func(hourlyItem, string, *HourlyForecast),
) {
	if len(raw) == 0 || string(raw) == "null" {
		if required {
			*warnings = append(*warnings, ParseWarning{Code: "MISSING_FIELD", Path: path})
		}
		return
	}
	var items []json.RawMessage
	if !isJSONArray(raw) || json.Unmarshal(raw, &items) != nil || len(items) > maximumHourlySeriesItems {
		*warnings = append(*warnings, ParseWarning{Code: "INVALID_FIELD", Path: path})
		return
	}
	if required && len(items) == 0 {
		*warnings = append(*warnings, ParseWarning{Code: "MISSING_FIELD", Path: path})
		return
	}
	seen := make(map[time.Time]struct{}, len(items))
	for index, rawItem := range items {
		itemPath := fmt.Sprintf("%s[%d]", path, index)
		var item hourlyItem
		if !isJSONObject(rawItem) || json.Unmarshal(rawItem, &item) != nil {
			*warnings = append(*warnings, ParseWarning{Code: "INVALID_ITEM", Path: itemPath})
			continue
		}
		forecastTime, ok := parseHourlyTime(item.Datetime, itemPath+".datetime", issuedAtUTC, warnings)
		if !ok {
			continue
		}
		if _, duplicate := seen[forecastTime]; duplicate {
			*warnings = append(*warnings, ParseWarning{Code: "DUPLICATE_DATETIME", Path: itemPath + ".datetime"})
		}
		seen[forecastTime] = struct{}{}
		row := rows[forecastTime]
		if row == nil {
			row = &HourlyForecast{ForecastTimeUTC: forecastTime}
			rows[forecastTime] = row
		}
		merge(item, itemPath, row)
	}
}

func parseHourlyTime(raw json.RawMessage, path string, issuedAtUTC time.Time, warnings *[]ParseWarning) (time.Time, bool) {
	var value string
	if json.Unmarshal(raw, &value) != nil {
		*warnings = append(*warnings, ParseWarning{Code: "INVALID_DATETIME", Path: path})
		return time.Time{}, false
	}
	forecastTime, err := parseCaiyunISOTime(value)
	if err != nil {
		*warnings = append(*warnings, ParseWarning{Code: "INVALID_DATETIME", Path: path})
		return time.Time{}, false
	}
	forecastTime = forecastTime.UTC()
	if forecastTime.Before(issuedAtUTC.Add(-hourlyHistoryTolerance)) || forecastTime.After(issuedAtUTC.Add(hourlyForecastWindow+hourlyFutureTolerance)) {
		*warnings = append(*warnings, ParseWarning{Code: "FORECAST_TIME_OUT_OF_RANGE", Path: path})
		return time.Time{}, false
	}
	return forecastTime, true
}
