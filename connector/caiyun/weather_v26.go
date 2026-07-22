package caiyun

import (
	"encoding/json"
	"math"
	"strings"
	"time"
)

const (
	maximumMetadataTextRunes = 64
	maximumKeypointRunes     = 2000
	minimumWeatherUnixTime   = int64(946684800)  // 2000-01-01T00:00:00Z
	maximumWeatherUnixTime   = int64(4102444800) // 2100-01-01T00:00:00Z
)

type WeatherLocation struct {
	Latitude  float64
	Longitude float64
}

type WeatherMetadata struct {
	Status           string
	APIVersion       string
	APIStatus        string
	Language         string
	Unit             string
	TZShiftSeconds   int
	Timezone         string
	ServerTimeUTC    time.Time
	Location         WeatherLocation
	Primary          *int
	ForecastKeypoint string
}

type RealtimeWeather struct {
	Status                  string
	TemperatureC            *float64
	ApparentTemperatureC    *float64
	HumidityRatio           *float64
	PressurePa              *float64
	WindSpeedKPH            *float64
	WindDirectionDeg        *float64
	CloudrateRatio          *float64
	VisibilityKM            *float64
	DSWRFWM2                *float64
	Skycon                  string
	LocalPrecipStatus       string
	LocalPrecipMMH          *float64
	LocalPrecipDatasource   string
	NearestPrecipStatus     string
	NearestPrecipDistanceKM *float64
	NearestPrecipMMH        *float64
	PM25UGM3                *float64
	PM10UGM3                *float64
	O3UGM3                  *float64
	SO2UGM3                 *float64
	NO2UGM3                 *float64
	COMGM3                  *float64
	AQIChn                  *int
	AQIUSA                  *int
	AQIDescChn              string
	AQIDescUSA              string
	ComfortIndex            *int
	ComfortDesc             string
	UltravioletIndex        *int
	UltravioletDesc         string
	ProviderJSON            json.RawMessage
}

type WeatherBundle struct {
	Metadata     WeatherMetadata
	Realtime     RealtimeWeather
	MinutelyJSON json.RawMessage
	HourlyJSON   json.RawMessage
	DailyJSON    json.RawMessage
	AlertJSON    json.RawMessage
	Warnings     []ParseWarning
}

type weatherEnvelope struct {
	Status           string          `json:"status"`
	APIVersion       json.RawMessage `json:"api_version"`
	APIStatus        json.RawMessage `json:"api_status"`
	Language         json.RawMessage `json:"lang"`
	Unit             json.RawMessage `json:"unit"`
	TZShift          json.RawMessage `json:"tzshift"`
	Timezone         json.RawMessage `json:"timezone"`
	ServerTime       json.RawMessage `json:"server_time"`
	Location         json.RawMessage `json:"location"`
	Primary          json.RawMessage `json:"primary"`
	ForecastKeypoint json.RawMessage `json:"forecast_keypoint"`
	Result           json.RawMessage `json:"result"`
}

type weatherResult struct {
	Realtime json.RawMessage `json:"realtime"`
	Minutely json.RawMessage `json:"minutely"`
	Hourly   json.RawMessage `json:"hourly"`
	Daily    json.RawMessage `json:"daily"`
	Alert    json.RawMessage `json:"alert"`
}

type realtimePayload struct {
	Status              json.RawMessage `json:"status"`
	Temperature         json.RawMessage `json:"temperature"`
	ApparentTemperature json.RawMessage `json:"apparent_temperature"`
	Humidity            json.RawMessage `json:"humidity"`
	Pressure            json.RawMessage `json:"pressure"`
	Wind                json.RawMessage `json:"wind"`
	Cloudrate           json.RawMessage `json:"cloudrate"`
	Visibility          json.RawMessage `json:"visibility"`
	DSWRF               json.RawMessage `json:"dswrf"`
	Skycon              json.RawMessage `json:"skycon"`
	Precipitation       json.RawMessage `json:"precipitation"`
	AirQuality          json.RawMessage `json:"air_quality"`
	LifeIndex           json.RawMessage `json:"life_index"`
}

func ParseWeatherV26(raw []byte) (*WeatherBundle, error) {
	if len(raw) == 0 || int64(len(raw)) > defaultMaxResponseBytes {
		return nil, weatherParseError()
	}
	var envelope weatherEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil || envelope.Status != "ok" || !isJSONObject(envelope.Result) {
		return nil, weatherParseError()
	}

	metadata, warnings, err := parseWeatherMetadata(envelope)
	if err != nil {
		return nil, err
	}
	var result weatherResult
	if err := json.Unmarshal(envelope.Result, &result); err != nil || !isJSONObject(result.Realtime) {
		return nil, weatherParseError()
	}
	realtime, realtimeWarnings, err := parseRealtimeWeather(result.Realtime)
	if err != nil {
		return nil, err
	}
	warnings = append(warnings, realtimeWarnings...)
	return &WeatherBundle{
		Metadata: metadata, Realtime: realtime,
		MinutelyJSON: cloneRawMessage(result.Minutely), HourlyJSON: cloneRawMessage(result.Hourly),
		DailyJSON: cloneRawMessage(result.Daily), AlertJSON: cloneRawMessage(result.Alert),
		Warnings: warnings,
	}, nil
}

func parseWeatherMetadata(envelope weatherEnvelope) (WeatherMetadata, []ParseWarning, error) {
	warnings := make([]ParseWarning, 0)
	apiVersion := decodeWeatherString(envelope.APIVersion, "api_version", maximumMetadataTextRunes, true, &warnings)
	apiStatus := decodeWeatherString(envelope.APIStatus, "api_status", maximumMetadataTextRunes, true, &warnings)
	language := decodeWeatherString(envelope.Language, "lang", maximumMetadataTextRunes, true, &warnings)
	unit := decodeWeatherString(envelope.Unit, "unit", maximumMetadataTextRunes, true, &warnings)
	timezoneName := decodeWeatherString(envelope.Timezone, "timezone", maximumMetadataTextRunes, true, &warnings)
	if apiVersion != "v2.6" || apiStatus == "" || language == "" || unit != "metric:v2" || timezoneName == "" {
		return WeatherMetadata{}, nil, weatherParseError()
	}
	if apiStatus != "active" {
		warnings = append(warnings, ParseWarning{Code: "API_STATUS_NOT_ACTIVE", Path: "api_status"})
	}

	var serverTime int64
	var tzShift int
	var location []float64
	if json.Unmarshal(envelope.ServerTime, &serverTime) != nil || serverTime < minimumWeatherUnixTime || serverTime >= maximumWeatherUnixTime ||
		json.Unmarshal(envelope.TZShift, &tzShift) != nil || tzShift < -18*60*60 || tzShift > 18*60*60 ||
		json.Unmarshal(envelope.Location, &location) != nil || len(location) != 2 ||
		!validLatitude(location[0]) || !validLongitude(location[1]) {
		return WeatherMetadata{}, nil, weatherParseError()
	}
	zone, err := time.LoadLocation(timezoneName)
	if err != nil {
		return WeatherMetadata{}, nil, weatherParseError()
	}
	serverTimeUTC := time.Unix(serverTime, 0).UTC()
	_, actualOffset := serverTimeUTC.In(zone).Zone()
	if actualOffset != tzShift {
		warnings = append(warnings, ParseWarning{Code: "TZSHIFT_MISMATCH", Path: "tzshift"})
	}

	var primary *int
	if len(envelope.Primary) != 0 && string(envelope.Primary) != "null" {
		var value int
		if err := json.Unmarshal(envelope.Primary, &value); err != nil {
			warnings = append(warnings, ParseWarning{Code: "INVALID_FIELD", Path: "primary"})
		} else {
			primary = &value
		}
	}
	keypoint := decodeWeatherString(envelope.ForecastKeypoint, "forecast_keypoint", maximumKeypointRunes, false, &warnings)
	return WeatherMetadata{
		Status: envelope.Status, APIVersion: apiVersion, APIStatus: apiStatus,
		Language: language, Unit: unit, TZShiftSeconds: tzShift, Timezone: timezoneName,
		ServerTimeUTC: serverTimeUTC,
		Location:      WeatherLocation{Latitude: location[0], Longitude: location[1]},
		Primary:       primary, ForecastKeypoint: keypoint,
	}, warnings, nil
}

func parseRealtimeWeather(raw json.RawMessage) (RealtimeWeather, []ParseWarning, error) {
	var payload realtimePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return RealtimeWeather{}, nil, weatherParseError()
	}
	warnings := make([]ParseWarning, 0)
	realtime := RealtimeWeather{ProviderJSON: cloneRawMessage(raw)}
	realtime.Status = decodeWeatherString(payload.Status, "result.realtime.status", 32, true, &warnings)
	if realtime.Status != "" && realtime.Status != "ok" {
		warnings = append(warnings, ParseWarning{Code: "MODULE_STATUS_NOT_OK", Path: "result.realtime.status"})
	}

	realtime.TemperatureC = decodeWeatherFloat(payload.Temperature, "result.realtime.temperature", -100, 100, true, &warnings)
	realtime.ApparentTemperatureC = decodeWeatherFloat(payload.ApparentTemperature, "result.realtime.apparent_temperature", -150, 150, true, &warnings)
	realtime.HumidityRatio = decodeWeatherFloat(payload.Humidity, "result.realtime.humidity", 0, 1, true, &warnings)
	realtime.PressurePa = decodeWeatherFloat(payload.Pressure, "result.realtime.pressure", 20000, 120000, true, &warnings)
	realtime.CloudrateRatio = decodeWeatherFloat(payload.Cloudrate, "result.realtime.cloudrate", 0, 1, true, &warnings)
	realtime.VisibilityKM = decodeWeatherFloat(payload.Visibility, "result.realtime.visibility", 0, 1000, true, &warnings)
	realtime.DSWRFWM2 = decodeWeatherFloat(payload.DSWRF, "result.realtime.dswrf", 0, 2000, true, &warnings)
	realtime.Skycon = decodeWeatherString(payload.Skycon, "result.realtime.skycon", 64, true, &warnings)

	parseRealtimeWind(payload.Wind, &realtime, &warnings)
	parseRealtimePrecipitation(payload.Precipitation, &realtime, &warnings)
	parseRealtimeAirQuality(payload.AirQuality, &realtime, &warnings)
	parseRealtimeLifeIndex(payload.LifeIndex, &realtime, &warnings)
	return realtime, warnings, nil
}

func parseRealtimeWind(raw json.RawMessage, realtime *RealtimeWeather, warnings *[]ParseWarning) {
	var payload struct {
		Speed     json.RawMessage `json:"speed"`
		Direction json.RawMessage `json:"direction"`
	}
	if !decodeWeatherObject(raw, "result.realtime.wind", true, &payload, warnings) {
		return
	}
	realtime.WindSpeedKPH = decodeWeatherFloat(payload.Speed, "result.realtime.wind.speed", 0, 500, true, warnings)
	realtime.WindDirectionDeg = decodeWeatherFloat(payload.Direction, "result.realtime.wind.direction", 0, 360, true, warnings)
}

func parseRealtimePrecipitation(raw json.RawMessage, realtime *RealtimeWeather, warnings *[]ParseWarning) {
	var payload struct {
		Local   json.RawMessage `json:"local"`
		Nearest json.RawMessage `json:"nearest"`
	}
	if !decodeWeatherObject(raw, "result.realtime.precipitation", false, &payload, warnings) {
		return
	}
	var local struct {
		Status     json.RawMessage `json:"status"`
		Datasource json.RawMessage `json:"datasource"`
		Intensity  json.RawMessage `json:"intensity"`
	}
	if decodeWeatherObject(payload.Local, "result.realtime.precipitation.local", false, &local, warnings) {
		realtime.LocalPrecipStatus = decodeWeatherString(local.Status, "result.realtime.precipitation.local.status", 32, true, warnings)
		realtime.LocalPrecipDatasource = decodeWeatherString(local.Datasource, "result.realtime.precipitation.local.datasource", 128, false, warnings)
		realtime.LocalPrecipMMH = decodeWeatherFloat(local.Intensity, "result.realtime.precipitation.local.intensity", 0, 2000, false, warnings)
		warnModuleStatus(realtime.LocalPrecipStatus, "result.realtime.precipitation.local.status", warnings)
	}
	var nearest struct {
		Status    json.RawMessage `json:"status"`
		Distance  json.RawMessage `json:"distance"`
		Intensity json.RawMessage `json:"intensity"`
	}
	if decodeWeatherObject(payload.Nearest, "result.realtime.precipitation.nearest", false, &nearest, warnings) {
		realtime.NearestPrecipStatus = decodeWeatherString(nearest.Status, "result.realtime.precipitation.nearest.status", 32, true, warnings)
		realtime.NearestPrecipDistanceKM = decodeWeatherFloat(nearest.Distance, "result.realtime.precipitation.nearest.distance", 0, 100000, false, warnings)
		realtime.NearestPrecipMMH = decodeWeatherFloat(nearest.Intensity, "result.realtime.precipitation.nearest.intensity", 0, 2000, false, warnings)
		warnModuleStatus(realtime.NearestPrecipStatus, "result.realtime.precipitation.nearest.status", warnings)
	}
}

func parseRealtimeAirQuality(raw json.RawMessage, realtime *RealtimeWeather, warnings *[]ParseWarning) {
	var payload struct {
		PM25        json.RawMessage `json:"pm25"`
		PM10        json.RawMessage `json:"pm10"`
		O3          json.RawMessage `json:"o3"`
		SO2         json.RawMessage `json:"so2"`
		NO2         json.RawMessage `json:"no2"`
		CO          json.RawMessage `json:"co"`
		AQI         json.RawMessage `json:"aqi"`
		Description json.RawMessage `json:"description"`
	}
	if !decodeWeatherObject(raw, "result.realtime.air_quality", false, &payload, warnings) {
		return
	}
	realtime.PM25UGM3 = decodeWeatherFloat(payload.PM25, "result.realtime.air_quality.pm25", 0, 10000, false, warnings)
	realtime.PM10UGM3 = decodeWeatherFloat(payload.PM10, "result.realtime.air_quality.pm10", 0, 10000, false, warnings)
	realtime.O3UGM3 = decodeWeatherFloat(payload.O3, "result.realtime.air_quality.o3", 0, 10000, false, warnings)
	realtime.SO2UGM3 = decodeWeatherFloat(payload.SO2, "result.realtime.air_quality.so2", 0, 10000, false, warnings)
	realtime.NO2UGM3 = decodeWeatherFloat(payload.NO2, "result.realtime.air_quality.no2", 0, 10000, false, warnings)
	realtime.COMGM3 = decodeWeatherFloat(payload.CO, "result.realtime.air_quality.co", 0, 1000, false, warnings)
	var aqi struct {
		Chn json.RawMessage `json:"chn"`
		USA json.RawMessage `json:"usa"`
	}
	if decodeWeatherObject(payload.AQI, "result.realtime.air_quality.aqi", false, &aqi, warnings) {
		realtime.AQIChn = decodeWeatherInt(aqi.Chn, "result.realtime.air_quality.aqi.chn", 0, 5000, false, warnings)
		realtime.AQIUSA = decodeWeatherInt(aqi.USA, "result.realtime.air_quality.aqi.usa", 0, 5000, false, warnings)
	}
	var description struct {
		Chn json.RawMessage `json:"chn"`
		USA json.RawMessage `json:"usa"`
	}
	if decodeWeatherObject(payload.Description, "result.realtime.air_quality.description", false, &description, warnings) {
		realtime.AQIDescChn = decodeWeatherString(description.Chn, "result.realtime.air_quality.description.chn", 64, false, warnings)
		realtime.AQIDescUSA = decodeWeatherString(description.USA, "result.realtime.air_quality.description.usa", 64, false, warnings)
	}
}

func parseRealtimeLifeIndex(raw json.RawMessage, realtime *RealtimeWeather, warnings *[]ParseWarning) {
	var payload struct {
		Comfort     json.RawMessage `json:"comfort"`
		Ultraviolet json.RawMessage `json:"ultraviolet"`
	}
	if !decodeWeatherObject(raw, "result.realtime.life_index", false, &payload, warnings) {
		return
	}
	parseRealtimeIndex(payload.Comfort, "result.realtime.life_index.comfort", &realtime.ComfortIndex, &realtime.ComfortDesc, warnings)
	parseRealtimeIndex(payload.Ultraviolet, "result.realtime.life_index.ultraviolet", &realtime.UltravioletIndex, &realtime.UltravioletDesc, warnings)
}

func parseRealtimeIndex(raw json.RawMessage, path string, index **int, description *string, warnings *[]ParseWarning) {
	var payload struct {
		Index json.RawMessage `json:"index"`
		Desc  json.RawMessage `json:"desc"`
	}
	if !decodeWeatherObject(raw, path, false, &payload, warnings) {
		return
	}
	*index = decodeWeatherInt(payload.Index, path+".index", -100, 100, false, warnings)
	*description = decodeWeatherString(payload.Desc, path+".desc", 255, false, warnings)
}

func decodeWeatherObject(raw json.RawMessage, path string, required bool, destination interface{}, warnings *[]ParseWarning) bool {
	if len(raw) == 0 || string(raw) == "null" {
		if required {
			*warnings = append(*warnings, ParseWarning{Code: "MISSING_FIELD", Path: path})
		}
		return false
	}
	if !isJSONObject(raw) || json.Unmarshal(raw, destination) != nil {
		*warnings = append(*warnings, ParseWarning{Code: "INVALID_FIELD", Path: path})
		return false
	}
	return true
}

func decodeWeatherString(raw json.RawMessage, path string, maximumRunes int, required bool, warnings *[]ParseWarning) string {
	if len(raw) == 0 || string(raw) == "null" {
		if required {
			*warnings = append(*warnings, ParseWarning{Code: "MISSING_FIELD", Path: path})
		}
		return ""
	}
	var value string
	if json.Unmarshal(raw, &value) != nil {
		*warnings = append(*warnings, ParseWarning{Code: "INVALID_FIELD", Path: path})
		return ""
	}
	value, truncated := truncateRunes(strings.TrimSpace(value), maximumRunes)
	if truncated {
		*warnings = append(*warnings, ParseWarning{Code: "TEXT_TRUNCATED", Path: path})
	}
	if required && value == "" {
		*warnings = append(*warnings, ParseWarning{Code: "MISSING_FIELD", Path: path})
	}
	return value
}

func decodeWeatherFloat(raw json.RawMessage, path string, minimum, maximum float64, required bool, warnings *[]ParseWarning) *float64 {
	if len(raw) == 0 || string(raw) == "null" {
		if required {
			*warnings = append(*warnings, ParseWarning{Code: "MISSING_FIELD", Path: path})
		}
		return nil
	}
	var value float64
	if json.Unmarshal(raw, &value) != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < minimum || value > maximum {
		*warnings = append(*warnings, ParseWarning{Code: "INVALID_VALUE", Path: path})
		return nil
	}
	return &value
}

func decodeWeatherInt(raw json.RawMessage, path string, minimum, maximum int, required bool, warnings *[]ParseWarning) *int {
	if len(raw) == 0 || string(raw) == "null" {
		if required {
			*warnings = append(*warnings, ParseWarning{Code: "MISSING_FIELD", Path: path})
		}
		return nil
	}
	var value int
	if json.Unmarshal(raw, &value) != nil || value < minimum || value > maximum {
		*warnings = append(*warnings, ParseWarning{Code: "INVALID_VALUE", Path: path})
		return nil
	}
	return &value
}

func warnModuleStatus(status, path string, warnings *[]ParseWarning) {
	if status != "" && status != "ok" {
		*warnings = append(*warnings, ParseWarning{Code: "MODULE_STATUS_NOT_OK", Path: path})
	}
}

func validLatitude(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= -90 && value <= 90
}

func validLongitude(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= -180 && value <= 180
}

func cloneRawMessage(raw json.RawMessage) json.RawMessage {
	return append(json.RawMessage(nil), raw...)
}

func weatherParseError() error {
	return &ParseError{EndpointKind: EndpointWeatherV26}
}
