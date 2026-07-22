package caiyun

import (
	"encoding/json"
	"fmt"
	"math"
	"time"
)

const (
	minutelyForecastMinutes         = 120
	minutelyConsistencyMinutes      = 60
	minutelyProbabilityWindows      = 4
	maximumMinutelySeriesItems      = 240
	maximumMinutelyProbabilityItems = 16
	maximumMinutelyDescriptionRunes = 16000
)

type MinutelyForecast struct {
	ForecastMinuteUTC time.Time
	MinuteOffset      int
	PrecipitationMMH  *float64
	ProbabilityRatio  *float64
	ProbabilityWindow *int
	Datasource        string
	Description       string
	ForecastKeypoint  string
}

type MinutelyBundle struct {
	Status       string
	IssuedAtUTC  time.Time
	Forecasts    []MinutelyForecast
	Warnings     []ParseWarning
	ProviderJSON json.RawMessage
}

type minutelyPayload struct {
	Status          json.RawMessage `json:"status"`
	Datasource      json.RawMessage `json:"datasource"`
	Precipitation2H json.RawMessage `json:"precipitation_2h"`
	Precipitation   json.RawMessage `json:"precipitation"`
	Probability     json.RawMessage `json:"probability"`
	Description     json.RawMessage `json:"description"`
}

func ParseMinutelyV26(weather *WeatherBundle) (*MinutelyBundle, error) {
	if weather == nil || !isJSONObject(weather.MinutelyJSON) || weather.Metadata.ServerTimeUTC.IsZero() {
		return nil, weatherParseError()
	}
	issuedAtUTC := weather.Metadata.ServerTimeUTC.UTC()
	if unixTime := issuedAtUTC.Unix(); unixTime < minimumWeatherUnixTime || unixTime >= maximumWeatherUnixTime {
		return nil, weatherParseError()
	}
	var payload minutelyPayload
	if err := json.Unmarshal(weather.MinutelyJSON, &payload); err != nil {
		return nil, weatherParseError()
	}

	warnings := make([]ParseWarning, 0)
	status := decodeWeatherString(payload.Status, "result.minutely.status", 32, true, &warnings)
	if status != "" && status != "ok" {
		warnings = append(warnings, ParseWarning{Code: "MODULE_STATUS_NOT_OK", Path: "result.minutely.status"})
	}
	datasource := decodeWeatherString(payload.Datasource, "result.minutely.datasource", 128, true, &warnings)
	description := decodeWeatherString(payload.Description, "result.minutely.description", maximumMinutelyDescriptionRunes, false, &warnings)

	precipitation2H, itemCount, err := parseMinutelySeries(
		payload.Precipitation2H, "result.minutely.precipitation_2h",
		minutelyForecastMinutes, maximumMinutelySeriesItems, 0, 2000, true, &warnings,
	)
	if err != nil || itemCount == 0 {
		return nil, weatherParseError()
	}
	precipitation, _, err := parseMinutelySeries(
		payload.Precipitation, "result.minutely.precipitation",
		minutelyConsistencyMinutes, maximumMinutelySeriesItems, 0, 2000, true, &warnings,
	)
	if err != nil {
		return nil, weatherParseError()
	}
	probabilities, _, err := parseMinutelySeries(
		payload.Probability, "result.minutely.probability",
		minutelyProbabilityWindows, maximumMinutelyProbabilityItems, 0, 1, true, &warnings,
	)
	if err != nil {
		return nil, weatherParseError()
	}
	if minutelySeriesMismatch(precipitation2H[:minutelyConsistencyMinutes], precipitation) {
		warnings = append(warnings, ParseWarning{Code: "PRECIPITATION_SERIES_MISMATCH", Path: "result.minutely.precipitation"})
	}

	issuedMinuteUTC := issuedAtUTC.Truncate(time.Minute)
	forecasts := make([]MinutelyForecast, minutelyForecastMinutes)
	for offset := range forecasts {
		forecast := MinutelyForecast{
			ForecastMinuteUTC: issuedMinuteUTC.Add(time.Duration(offset) * time.Minute),
			MinuteOffset:      offset,
			PrecipitationMMH:  precipitation2H[offset],
			Datasource:        datasource,
			Description:       description,
			ForecastKeypoint:  weather.Metadata.ForecastKeypoint,
		}
		window := offset / 30
		if probability := probabilities[window]; probability != nil {
			probabilityValue := *probability
			windowValue := window
			forecast.ProbabilityRatio = &probabilityValue
			forecast.ProbabilityWindow = &windowValue
		}
		forecasts[offset] = forecast
	}
	return &MinutelyBundle{
		Status: status, IssuedAtUTC: issuedAtUTC, Forecasts: forecasts,
		Warnings: warnings, ProviderJSON: cloneRawMessage(weather.MinutelyJSON),
	}, nil
}

func parseMinutelySeries(
	raw json.RawMessage,
	path string,
	expectedItems int,
	maximumItems int,
	minimumValue float64,
	maximumValue float64,
	required bool,
	warnings *[]ParseWarning,
) ([]*float64, int, error) {
	values := make([]*float64, expectedItems)
	if len(raw) == 0 || string(raw) == "null" {
		if required {
			*warnings = append(*warnings, ParseWarning{Code: "MISSING_FIELD", Path: path})
		}
		return values, 0, nil
	}
	var items []json.RawMessage
	if !isJSONArray(raw) || json.Unmarshal(raw, &items) != nil || len(items) > maximumItems {
		return nil, 0, weatherParseError()
	}
	if len(items) != expectedItems {
		*warnings = append(*warnings, ParseWarning{Code: "UNEXPECTED_ARRAY_LENGTH", Path: path})
	}
	limit := min(len(items), expectedItems)
	for index := 0; index < limit; index++ {
		values[index] = decodeWeatherFloat(
			items[index], fmt.Sprintf("%s[%d]", path, index), minimumValue, maximumValue, true, warnings,
		)
	}
	return values, len(items), nil
}

func minutelySeriesMismatch(first, second []*float64) bool {
	for index := range first {
		if first[index] == nil || second[index] == nil {
			continue
		}
		if math.Abs(*first[index]-*second[index]) > 1e-9 {
			return true
		}
	}
	return false
}
