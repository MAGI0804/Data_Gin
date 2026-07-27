package caiyun

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

const maximumDailySeriesItems = 31

type DailyMetric struct {
	Maximum *float64
	Minimum *float64
	Average *float64
}

type DailyWindValue struct {
	SpeedKPH     *float64
	DirectionDeg *float64
}

type DailyWind struct {
	Maximum DailyWindValue
	Minimum DailyWindValue
	Average DailyWindValue
}

type DailyPrecipitation struct {
	DailyMetric
	ProbabilityPct *float64
}

type DailyIntegerMetric struct {
	Maximum *int
	Minimum *int
	Average *int
}

type DailyAirQuality struct {
	PM25   DailyMetric
	AQIChn DailyIntegerMetric
	AQIUSA DailyIntegerMetric
}

type DailyForecast struct {
	ForecastDateLocal  time.Time
	Temperature        DailyMetric
	DayTemperature     DailyMetric
	NightTemperature   DailyMetric
	Precipitation      DailyPrecipitation
	DayPrecipitation   DailyPrecipitation
	NightPrecipitation DailyPrecipitation
	Wind               DailyWind
	DayWind            DailyWind
	NightWind          DailyWind
	Humidity           DailyMetric
	Cloudrate          DailyMetric
	Pressure           DailyMetric
	Visibility         DailyMetric
	DSWRF              DailyMetric
	AirQuality         DailyAirQuality
	Skycon             string
	DaySkycon          string
	NightSkycon        string
	SunriseLocalTime   string
	SunsetLocalTime    string
	BasicLifeIndices   []LifeIndexItem
	BasicLifeIndexJSON json.RawMessage
	basicLifeIndexRaw  map[string]json.RawMessage
}

type DailyBundle struct {
	Status       string
	IssuedAtUTC  time.Time
	Forecasts    []DailyForecast
	Warnings     []ParseWarning
	ProviderJSON json.RawMessage
}

type dailyPayload struct {
	Status             json.RawMessage `json:"status"`
	Astro              json.RawMessage `json:"astro"`
	Temperature        json.RawMessage `json:"temperature"`
	DayTemperature     json.RawMessage `json:"temperature_08h_20h"`
	NightTemperature   json.RawMessage `json:"temperature_20h_32h"`
	Precipitation      json.RawMessage `json:"precipitation"`
	DayPrecipitation   json.RawMessage `json:"precipitation_08h_20h"`
	NightPrecipitation json.RawMessage `json:"precipitation_20h_32h"`
	Wind               json.RawMessage `json:"wind"`
	DayWind            json.RawMessage `json:"wind_08h_20h"`
	NightWind          json.RawMessage `json:"wind_20h_32h"`
	Humidity           json.RawMessage `json:"humidity"`
	Cloudrate          json.RawMessage `json:"cloudrate"`
	Pressure           json.RawMessage `json:"pressure"`
	Visibility         json.RawMessage `json:"visibility"`
	DSWRF              json.RawMessage `json:"dswrf"`
	AirQuality         json.RawMessage `json:"air_quality"`
	Skycon             json.RawMessage `json:"skycon"`
	DaySkycon          json.RawMessage `json:"skycon_08h_20h"`
	NightSkycon        json.RawMessage `json:"skycon_20h_32h"`
	LifeIndex          json.RawMessage `json:"life_index"`
}

type dailyItem struct {
	Date         json.RawMessage `json:"date"`
	Maximum      json.RawMessage `json:"max"`
	Minimum      json.RawMessage `json:"min"`
	Average      json.RawMessage `json:"avg"`
	Value        json.RawMessage `json:"value"`
	Probability  json.RawMessage `json:"probability"`
	Sunrise      json.RawMessage `json:"sunrise"`
	Sunset       json.RawMessage `json:"sunset"`
	Index        json.RawMessage `json:"index"`
	Description  json.RawMessage `json:"desc"`
	ProviderJSON json.RawMessage `json:"-"`
}

func ParseDailyV26(weather *WeatherBundle) (*DailyBundle, error) {
	if weather == nil || !isJSONObject(weather.DailyJSON) || weather.Metadata.ServerTimeUTC.IsZero() || weather.Metadata.Timezone == "" {
		return nil, weatherParseError()
	}
	issuedAtUTC := weather.Metadata.ServerTimeUTC.UTC()
	if unixTime := issuedAtUTC.Unix(); unixTime < minimumWeatherUnixTime || unixTime >= maximumWeatherUnixTime {
		return nil, weatherParseError()
	}
	zone, err := time.LoadLocation(weather.Metadata.Timezone)
	if err != nil {
		return nil, weatherParseError()
	}
	var payload dailyPayload
	if err := json.Unmarshal(weather.DailyJSON, &payload); err != nil {
		return nil, weatherParseError()
	}
	warnings := make([]ParseWarning, 0)
	status := decodeWeatherString(payload.Status, "result.daily.status", 32, true, &warnings)
	if status != "" && status != "ok" {
		warnings = append(warnings, ParseWarning{Code: "MODULE_STATUS_NOT_OK", Path: "result.daily.status"})
	}
	rows := make(map[time.Time]*DailyForecast)

	mergeDailyMetric(payload.Temperature, "result.daily.temperature", true, -100, 100, issuedAtUTC, zone, rows, &warnings,
		func(row *DailyForecast) *DailyMetric { return &row.Temperature })
	mergeDailyMetric(payload.DayTemperature, "result.daily.temperature_08h_20h", false, -100, 100, issuedAtUTC, zone, rows, &warnings,
		func(row *DailyForecast) *DailyMetric { return &row.DayTemperature })
	mergeDailyMetric(payload.NightTemperature, "result.daily.temperature_20h_32h", false, -100, 100, issuedAtUTC, zone, rows, &warnings,
		func(row *DailyForecast) *DailyMetric { return &row.NightTemperature })
	mergeDailyMetric(payload.Humidity, "result.daily.humidity", false, 0, 1, issuedAtUTC, zone, rows, &warnings,
		func(row *DailyForecast) *DailyMetric { return &row.Humidity })
	mergeDailyMetric(payload.Cloudrate, "result.daily.cloudrate", false, 0, 1, issuedAtUTC, zone, rows, &warnings,
		func(row *DailyForecast) *DailyMetric { return &row.Cloudrate })
	mergeDailyMetric(payload.Pressure, "result.daily.pressure", false, 20000, 120000, issuedAtUTC, zone, rows, &warnings,
		func(row *DailyForecast) *DailyMetric { return &row.Pressure })
	mergeDailyMetric(payload.Visibility, "result.daily.visibility", false, 0, 1000, issuedAtUTC, zone, rows, &warnings,
		func(row *DailyForecast) *DailyMetric { return &row.Visibility })
	mergeDailyMetric(payload.DSWRF, "result.daily.dswrf", false, 0, 2000, issuedAtUTC, zone, rows, &warnings,
		func(row *DailyForecast) *DailyMetric { return &row.DSWRF })
	mergeDailyPrecipitation(payload.Precipitation, "result.daily.precipitation", issuedAtUTC, zone, rows, &warnings,
		func(row *DailyForecast) *DailyPrecipitation { return &row.Precipitation })
	mergeDailyPrecipitation(payload.DayPrecipitation, "result.daily.precipitation_08h_20h", issuedAtUTC, zone, rows, &warnings,
		func(row *DailyForecast) *DailyPrecipitation { return &row.DayPrecipitation })
	mergeDailyPrecipitation(payload.NightPrecipitation, "result.daily.precipitation_20h_32h", issuedAtUTC, zone, rows, &warnings,
		func(row *DailyForecast) *DailyPrecipitation { return &row.NightPrecipitation })
	mergeDailyWind(payload.Wind, "result.daily.wind", issuedAtUTC, zone, rows, &warnings,
		func(row *DailyForecast) *DailyWind { return &row.Wind })
	mergeDailyWind(payload.DayWind, "result.daily.wind_08h_20h", issuedAtUTC, zone, rows, &warnings,
		func(row *DailyForecast) *DailyWind { return &row.DayWind })
	mergeDailyWind(payload.NightWind, "result.daily.wind_20h_32h", issuedAtUTC, zone, rows, &warnings,
		func(row *DailyForecast) *DailyWind { return &row.NightWind })
	mergeDailyAirQuality(payload.AirQuality, issuedAtUTC, zone, rows, &warnings)
	mergeDailyBasicLifeIndices(payload.LifeIndex, issuedAtUTC, zone, rows, &warnings)
	mergeDailySkycon(payload.Skycon, "result.daily.skycon", true, issuedAtUTC, zone, rows, &warnings,
		func(row *DailyForecast, value string) { row.Skycon = value })
	mergeDailySkycon(payload.DaySkycon, "result.daily.skycon_08h_20h", false, issuedAtUTC, zone, rows, &warnings,
		func(row *DailyForecast, value string) { row.DaySkycon = value })
	mergeDailySkycon(payload.NightSkycon, "result.daily.skycon_20h_32h", false, issuedAtUTC, zone, rows, &warnings,
		func(row *DailyForecast, value string) { row.NightSkycon = value })
	mergeDailyAstro(payload.Astro, issuedAtUTC, zone, rows, &warnings)
	if len(rows) == 0 {
		return nil, weatherParseError()
	}

	dates := make([]time.Time, 0, len(rows))
	for date := range rows {
		dates = append(dates, date)
	}
	sort.Slice(dates, func(left, right int) bool { return dates[left].Before(dates[right]) })
	forecasts := make([]DailyForecast, 0, len(dates))
	missingTemperature := 0
	missingSkycon := 0
	irregularInterval := false
	for index, date := range dates {
		row := rows[date]
		if err := finalizeDailyLifeIndices(row); err != nil {
			return nil, weatherParseError()
		}
		if row.Temperature.Maximum == nil && row.Temperature.Minimum == nil && row.Temperature.Average == nil {
			missingTemperature++
		}
		if row.Skycon == "" {
			missingSkycon++
		}
		if index > 0 && date.Sub(dates[index-1]) != 24*time.Hour {
			irregularInterval = true
		}
		forecasts = append(forecasts, *row)
	}
	if missingTemperature*2 >= len(forecasts) {
		warnings = append(warnings, ParseWarning{Code: "CORE_FIELD_COVERAGE_LOW", Path: "result.daily.temperature"})
	}
	if missingSkycon*2 >= len(forecasts) {
		warnings = append(warnings, ParseWarning{Code: "CORE_FIELD_COVERAGE_LOW", Path: "result.daily.skycon"})
	}
	if irregularInterval {
		warnings = append(warnings, ParseWarning{Code: "IRREGULAR_DATE_INTERVAL", Path: "result.daily"})
	}
	return &DailyBundle{
		Status: status, IssuedAtUTC: issuedAtUTC, Forecasts: forecasts,
		Warnings: warnings, ProviderJSON: cloneRawMessage(weather.DailyJSON),
	}, nil
}

func mergeDailyMetric(
	raw json.RawMessage,
	path string,
	required bool,
	minimum float64,
	maximum float64,
	issuedAtUTC time.Time,
	zone *time.Location,
	rows map[time.Time]*DailyForecast,
	warnings *[]ParseWarning,
	metric func(*DailyForecast) *DailyMetric,
) {
	mergeDailyItems(raw, path, required, issuedAtUTC, zone, rows, warnings, func(item dailyItem, itemPath string, row *DailyForecast) {
		value := metric(row)
		value.Maximum = decodeWeatherFloat(item.Maximum, itemPath+".max", minimum, maximum, true, warnings)
		value.Minimum = decodeWeatherFloat(item.Minimum, itemPath+".min", minimum, maximum, true, warnings)
		value.Average = decodeWeatherFloat(item.Average, itemPath+".avg", minimum, maximum, true, warnings)
		warnDailyMetricOrder(*value, itemPath, warnings)
	})
}

func mergeDailySkycon(
	raw json.RawMessage,
	path string,
	required bool,
	issuedAtUTC time.Time,
	zone *time.Location,
	rows map[time.Time]*DailyForecast,
	warnings *[]ParseWarning,
	set func(*DailyForecast, string),
) {
	mergeDailyItems(raw, path, required, issuedAtUTC, zone, rows, warnings, func(item dailyItem, itemPath string, row *DailyForecast) {
		set(row, decodeWeatherString(item.Value, itemPath+".value", 64, true, warnings))
	})
}

func mergeDailyAstro(raw json.RawMessage, issuedAtUTC time.Time, zone *time.Location, rows map[time.Time]*DailyForecast, warnings *[]ParseWarning) {
	const path = "result.daily.astro"
	mergeDailyItems(raw, path, false, issuedAtUTC, zone, rows, warnings, func(item dailyItem, itemPath string, row *DailyForecast) {
		row.SunriseLocalTime = decodeDailyClock(item.Sunrise, itemPath+".sunrise", warnings)
		row.SunsetLocalTime = decodeDailyClock(item.Sunset, itemPath+".sunset", warnings)
	})
}

func decodeDailyClock(raw json.RawMessage, path string, warnings *[]ParseWarning) string {
	var payload struct {
		Time json.RawMessage `json:"time"`
	}
	if !decodeWeatherObject(raw, path, false, &payload, warnings) {
		return ""
	}
	value := decodeWeatherString(payload.Time, path+".time", 16, false, warnings)
	if value == "" {
		return ""
	}
	if _, err := time.Parse("15:04", value); err == nil {
		return value
	}
	if parsed, err := time.Parse("15:04:05", value); err == nil {
		return parsed.Format("15:04:05")
	}
	*warnings = append(*warnings, ParseWarning{Code: "INVALID_VALUE", Path: path + ".time"})
	return ""
}

func mergeDailyItems(
	raw json.RawMessage,
	path string,
	required bool,
	issuedAtUTC time.Time,
	zone *time.Location,
	rows map[time.Time]*DailyForecast,
	warnings *[]ParseWarning,
	merge func(dailyItem, string, *DailyForecast),
) {
	if len(raw) == 0 || string(raw) == "null" {
		if required {
			*warnings = append(*warnings, ParseWarning{Code: "MISSING_FIELD", Path: path})
		}
		return
	}
	var items []json.RawMessage
	if !isJSONArray(raw) || json.Unmarshal(raw, &items) != nil || len(items) > maximumDailySeriesItems {
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
		var item dailyItem
		if !isJSONObject(rawItem) || json.Unmarshal(rawItem, &item) != nil {
			*warnings = append(*warnings, ParseWarning{Code: "INVALID_ITEM", Path: itemPath})
			continue
		}
		item.ProviderJSON = cloneRawMessage(rawItem)
		date, ok := parseDailyDate(item.Date, itemPath+".date", issuedAtUTC, zone, warnings)
		if !ok {
			continue
		}
		if _, duplicate := seen[date]; duplicate {
			*warnings = append(*warnings, ParseWarning{Code: "DUPLICATE_DATE", Path: itemPath + ".date"})
		}
		seen[date] = struct{}{}
		row := rows[date]
		if row == nil {
			row = &DailyForecast{ForecastDateLocal: date}
			rows[date] = row
		}
		merge(item, itemPath, row)
	}
}

func parseDailyDate(raw json.RawMessage, path string, issuedAtUTC time.Time, zone *time.Location, warnings *[]ParseWarning) (time.Time, bool) {
	var value string
	if json.Unmarshal(raw, &value) != nil {
		*warnings = append(*warnings, ParseWarning{Code: "INVALID_DATE", Path: path})
		return time.Time{}, false
	}
	value = strings.TrimSpace(value)
	date, err := time.Parse("2006-01-02", value)
	if err != nil {
		providerTime, providerErr := parseCaiyunISOTime(value)
		if providerErr != nil {
			*warnings = append(*warnings, ParseWarning{Code: "INVALID_DATE", Path: path})
			return time.Time{}, false
		}
		localTime := providerTime.In(zone)
		date = time.Date(localTime.Year(), localTime.Month(), localTime.Day(), 0, 0, 0, 0, time.UTC)
	}
	issuedLocal := issuedAtUTC.In(zone)
	issuedDate := time.Date(issuedLocal.Year(), issuedLocal.Month(), issuedLocal.Day(), 0, 0, 0, 0, time.UTC)
	if date.Before(issuedDate.AddDate(0, 0, -1)) || date.After(issuedDate.AddDate(0, 0, 16)) {
		*warnings = append(*warnings, ParseWarning{Code: "FORECAST_DATE_OUT_OF_RANGE", Path: path})
		return time.Time{}, false
	}
	return date, true
}

func warnDailyMetricOrder(metric DailyMetric, path string, warnings *[]ParseWarning) {
	if metric.Maximum != nil && metric.Minimum != nil && *metric.Maximum < *metric.Minimum {
		*warnings = append(*warnings, ParseWarning{Code: "INVALID_RANGE_ORDER", Path: path})
	}
	if (metric.Average != nil && metric.Minimum != nil && *metric.Average < *metric.Minimum) ||
		(metric.Average != nil && metric.Maximum != nil && *metric.Average > *metric.Maximum) {
		*warnings = append(*warnings, ParseWarning{Code: "INVALID_AVERAGE", Path: path + ".avg"})
	}
}
