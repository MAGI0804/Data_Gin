package caiyun

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestParseMinutelyV26ExpandsForecastTimesAndProbabilities(t *testing.T) {
	weather := testMinutelyWeather(t, minutelyForecastMinutes, minutelyConsistencyMinutes, minutelyProbabilityWindows)
	bundle, err := ParseMinutelyV26(weather)
	if err != nil {
		t.Fatalf("ParseMinutelyV26() error=%v", err)
	}
	if bundle.Status != "ok" || len(bundle.Forecasts) != minutelyForecastMinutes || len(bundle.ProviderJSON) == 0 {
		t.Fatalf("bundle=%+v", bundle)
	}
	issuedAtUTC := time.Date(2026, 7, 22, 10, 3, 47, 0, time.UTC)
	if bundle.IssuedAtUTC != issuedAtUTC || bundle.Forecasts[0].ForecastMinuteUTC != issuedAtUTC.Truncate(time.Minute) ||
		bundle.Forecasts[119].ForecastMinuteUTC != issuedAtUTC.Truncate(time.Minute).Add(119*time.Minute) {
		t.Fatalf("forecast times first=%v last=%v issued=%v", bundle.Forecasts[0].ForecastMinuteUTC, bundle.Forecasts[119].ForecastMinuteUTC, bundle.IssuedAtUTC)
	}
	for _, offset := range []int{0, 29, 30, 59, 60, 89, 90, 119} {
		forecast := bundle.Forecasts[offset]
		wantWindow := offset / 30
		wantProbability := float64(wantWindow+1) / 10
		if forecast.MinuteOffset != offset || forecast.PrecipitationMMH == nil || *forecast.PrecipitationMMH != float64(offset)/100 ||
			forecast.ProbabilityWindow == nil || *forecast.ProbabilityWindow != wantWindow ||
			forecast.ProbabilityRatio == nil || *forecast.ProbabilityRatio != wantProbability {
			t.Fatalf("forecast[%d]=%+v", offset, forecast)
		}
		if forecast.Datasource != "radar" || forecast.Description != "未来两小时无明显降水" || forecast.ForecastKeypoint != "未来两小时无降水" {
			t.Fatalf("forecast text[%d]=%+v", offset, forecast)
		}
	}
	if len(bundle.Warnings) != 0 {
		t.Fatalf("warnings=%+v", bundle.Warnings)
	}
}

func TestParseMinutelyV26ContainsBadItemsAndReportsSeriesMismatch(t *testing.T) {
	weather := testMinutelyWeather(t, 2, 1, 2)
	var payload map[string]interface{}
	if err := json.Unmarshal(weather.MinutelyJSON, &payload); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	payload["status"] = "degraded"
	payload["precipitation_2h"] = []interface{}{0.0, "bad"}
	payload["precipitation"] = []float64{1.0}
	payload["probability"] = []interface{}{0.2, 2.0}
	weather.MinutelyJSON = mustMarshalMinutely(t, payload)

	bundle, err := ParseMinutelyV26(weather)
	if err != nil {
		t.Fatalf("ParseMinutelyV26() error=%v", err)
	}
	if len(bundle.Forecasts) != minutelyForecastMinutes || bundle.Forecasts[1].PrecipitationMMH != nil ||
		bundle.Forecasts[60].ProbabilityRatio != nil || bundle.Forecasts[0].ProbabilityRatio == nil || *bundle.Forecasts[0].ProbabilityRatio != 0.2 {
		t.Fatalf("bundle=%+v", bundle)
	}
	for _, code := range []string{"MODULE_STATUS_NOT_OK", "UNEXPECTED_ARRAY_LENGTH", "INVALID_VALUE", "PRECIPITATION_SERIES_MISMATCH"} {
		if !hasParseWarning(bundle.Warnings, code) {
			t.Fatalf("warnings=%+v missing=%s", bundle.Warnings, code)
		}
	}
}

func TestParseMinutelyV26RejectsMissingOrUnboundedPrimarySeries(t *testing.T) {
	tooMany := make([]float64, maximumMinutelySeriesItems+1)
	tests := []struct {
		name    string
		weather *WeatherBundle
	}{
		{name: "nil bundle"},
		{name: "missing module", weather: &WeatherBundle{Metadata: WeatherMetadata{ServerTimeUTC: time.Now().UTC()}}},
		{name: "missing primary series", weather: minutelyWeatherWithPayload(t, map[string]interface{}{
			"status": "ok", "datasource": "radar", "precipitation": []float64{}, "probability": []float64{},
		})},
		{name: "unbounded primary series", weather: minutelyWeatherWithPayload(t, map[string]interface{}{
			"status": "ok", "datasource": "radar", "precipitation_2h": tooMany,
		})},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseMinutelyV26(test.weather)
			var parseError *ParseError
			if !errors.As(err, &parseError) || parseError.EndpointKind != EndpointWeatherV26 {
				t.Fatalf("ParseMinutelyV26() error=%v", err)
			}
			if test.weather != nil && len(test.weather.MinutelyJSON) > 0 && strings.Contains(err.Error(), string(test.weather.MinutelyJSON)) {
				t.Fatalf("error leaked response: %v", err)
			}
		})
	}
}

func testMinutelyWeather(t *testing.T, precipitation2HItems, precipitationItems, probabilityItems int) *WeatherBundle {
	t.Helper()
	precipitation2H := make([]float64, precipitation2HItems)
	for index := range precipitation2H {
		precipitation2H[index] = float64(index) / 100
	}
	precipitation := make([]float64, precipitationItems)
	copy(precipitation, precipitation2H)
	probabilities := make([]float64, probabilityItems)
	for index := range probabilities {
		probabilities[index] = float64(index+1) / 10
	}
	return minutelyWeatherWithPayload(t, map[string]interface{}{
		"status": "ok", "datasource": "radar", "description": " 未来两小时无明显降水 ",
		"precipitation_2h": precipitation2H, "precipitation": precipitation, "probability": probabilities,
	})
}

func minutelyWeatherWithPayload(t *testing.T, payload map[string]interface{}) *WeatherBundle {
	t.Helper()
	return &WeatherBundle{
		Metadata: WeatherMetadata{
			ServerTimeUTC:    time.Date(2026, 7, 22, 10, 3, 47, 0, time.UTC),
			ForecastKeypoint: "未来两小时无降水",
		},
		MinutelyJSON: mustMarshalMinutely(t, payload),
	}
}

func mustMarshalMinutely(t *testing.T, value interface{}) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal minutely fixture: %v", err)
	}
	return raw
}
