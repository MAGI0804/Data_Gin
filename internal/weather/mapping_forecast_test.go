package weather

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"gin-biz-web-api/connector/caiyun"
)

func TestMapForecastsMapsModelsAndDefensivelyCopiesValues(t *testing.T) {
	issuedAt := time.Date(2026, 7, 22, 2, 3, 47, 0, time.UTC)
	fetchedAt := issuedAt.Add(time.Minute)
	temperature := 31.5
	probability := 0.2
	window := 0
	hourlyTemperature := 32.0
	input := ForecastMappingInput{
		Metadata: MappingMetadata{MallID: 7, FetchRunID: 11, FetchedAtUTC: fetchedAt, RawChecksum: strings.Repeat("A", 64)},
		Weather: &caiyun.WeatherBundle{
			Metadata: caiyun.WeatherMetadata{ServerTimeUTC: issuedAt},
			Realtime: caiyun.RealtimeWeather{TemperatureC: &temperature, Skycon: "CLEAR_DAY"},
			Warnings: []caiyun.ParseWarning{{Code: "TEST_WARNING", Path: "result.realtime.temperature"}},
		},
		Minutely: &caiyun.MinutelyBundle{
			IssuedAtUTC: issuedAt,
			Forecasts: []caiyun.MinutelyForecast{{
				ForecastMinuteUTC: issuedAt.Truncate(time.Minute), MinuteOffset: 0,
				PrecipitationMMH: &probability, ProbabilityRatio: &probability, ProbabilityWindow: &window,
			}},
		},
		Hourly: &caiyun.HourlyBundle{
			IssuedAtUTC: issuedAt,
			Forecasts:   []caiyun.HourlyForecast{{ForecastTimeUTC: issuedAt.Add(time.Hour), TemperatureC: &hourlyTemperature}},
		},
	}
	batch, err := MapForecasts(input)
	if err != nil {
		t.Fatalf("MapForecasts() error=%v", err)
	}
	if batch.Realtime == nil || len(batch.Minutely) != 1 || len(batch.Hourly) != 1 ||
		batch.Realtime.MallID != 7 || batch.Realtime.FetchRunID != 11 || batch.Realtime.Provider != ProviderCaiyun ||
		batch.Realtime.RawChecksum != strings.Repeat("a", 64) || batch.Realtime.QualityStatus != QualityStatusWarning ||
		batch.Minutely[0].QualityStatus != QualityStatusValid || batch.Hourly[0].QualityStatus != QualityStatusValid {
		t.Fatalf("batch=%+v", batch)
	}
	if !json.Valid([]byte(batch.ParseWarningsJSON)) || !json.Valid([]byte(batch.RowCountsJSON)) || !json.Valid([]byte(batch.Realtime.QualityFlagsJSON)) {
		t.Fatalf("JSON warnings=%s counts=%s flags=%s", batch.ParseWarningsJSON, batch.RowCountsJSON, batch.Realtime.QualityFlagsJSON)
	}
	temperature = -99
	probability = 0.9
	hourlyTemperature = -88
	if *batch.Realtime.TemperatureC != 31.5 || *batch.Minutely[0].ProbabilityRatio != 0.2 || *batch.Hourly[0].TemperatureC != 32 {
		t.Fatalf("models alias parser DTO values: %+v", batch)
	}
}

func TestMapForecastsMarksMissingOptionalModules(t *testing.T) {
	issuedAt := time.Date(2026, 7, 22, 2, 3, 47, 0, time.UTC)
	batch, err := MapForecasts(ForecastMappingInput{
		Metadata: MappingMetadata{MallID: 1, FetchRunID: 2, FetchedAtUTC: issuedAt, RawChecksum: strings.Repeat("b", 64)},
		Weather:  &caiyun.WeatherBundle{Metadata: caiyun.WeatherMetadata{ServerTimeUTC: issuedAt}},
	})
	if err != nil {
		t.Fatalf("MapForecasts() error=%v", err)
	}
	var warnings []caiyun.ParseWarning
	if err := json.Unmarshal([]byte(batch.ParseWarningsJSON), &warnings); err != nil || warningCodeCount(warnings, "MODULE_NOT_PARSED") != 2 {
		t.Fatalf("warnings=%s error=%v", batch.ParseWarningsJSON, err)
	}
}

func TestMapForecastsPropagatesOnlyTopLevelWarningsAcrossModules(t *testing.T) {
	issuedAt := time.Date(2026, 7, 22, 2, 3, 47, 0, time.UTC)
	batch, err := MapForecasts(ForecastMappingInput{
		Metadata: MappingMetadata{MallID: 1, FetchRunID: 2, FetchedAtUTC: issuedAt, RawChecksum: strings.Repeat("d", 64)},
		Weather: &caiyun.WeatherBundle{
			Metadata: caiyun.WeatherMetadata{ServerTimeUTC: issuedAt},
			Warnings: []caiyun.ParseWarning{
				{Code: "TZSHIFT_MISMATCH", Path: "tzshift"},
				{Code: "INVALID_VALUE", Path: "result.realtime.humidity"},
			},
		},
		Minutely: &caiyun.MinutelyBundle{IssuedAtUTC: issuedAt, Forecasts: []caiyun.MinutelyForecast{{ForecastMinuteUTC: issuedAt}}},
		Hourly:   &caiyun.HourlyBundle{IssuedAtUTC: issuedAt, Forecasts: []caiyun.HourlyForecast{{ForecastTimeUTC: issuedAt}}},
	})
	if err != nil {
		t.Fatalf("MapForecasts() error=%v", err)
	}
	for _, flags := range []string{string(batch.Minutely[0].QualityFlagsJSON), string(batch.Hourly[0].QualityFlagsJSON)} {
		if !strings.Contains(flags, "TZSHIFT_MISMATCH") || strings.Contains(flags, "result.realtime.humidity") {
			t.Fatalf("cross-module flags=%s", flags)
		}
	}
}

func TestMapForecastsRejectsInvalidOrMismatchedInput(t *testing.T) {
	issuedAt := time.Date(2026, 7, 22, 2, 3, 47, 0, time.UTC)
	valid := ForecastMappingInput{
		Metadata: MappingMetadata{MallID: 1, FetchRunID: 2, FetchedAtUTC: issuedAt, RawChecksum: strings.Repeat("c", 64)},
		Weather:  &caiyun.WeatherBundle{Metadata: caiyun.WeatherMetadata{ServerTimeUTC: issuedAt}},
	}
	tests := []ForecastMappingInput{
		{},
		{Metadata: valid.Metadata},
		{Metadata: MappingMetadata{MallID: 1, FetchRunID: 2, FetchedAtUTC: issuedAt, RawChecksum: "bad"}, Weather: valid.Weather},
		{Metadata: valid.Metadata, Weather: valid.Weather, Minutely: &caiyun.MinutelyBundle{IssuedAtUTC: issuedAt.Add(time.Second)}},
	}
	for _, input := range tests {
		if _, err := MapForecasts(input); !errors.Is(err, ErrInvalidMappingInput) {
			t.Fatalf("MapForecasts() error=%v", err)
		}
	}
}

func warningCodeCount(warnings []caiyun.ParseWarning, code string) int {
	count := 0
	for _, warning := range warnings {
		if warning.Code == code {
			count++
		}
	}
	return count
}
