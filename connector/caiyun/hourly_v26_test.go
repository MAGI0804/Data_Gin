package caiyun

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestParseHourlyV26MergesUnionSortsAndUsesLastDuplicate(t *testing.T) {
	issuedAt := time.Date(2026, 7, 22, 2, 3, 47, 0, time.UTC)
	t0 := "2026-07-22T11:00+08:00"
	t1 := "2026-07-22T12:00+08:00"
	t2 := "2026-07-22T13:00+08:00"
	weather := hourlyWeatherWithPayload(t, issuedAt, map[string]interface{}{
		"status": "ok", "description": " 未来天气稳定 ",
		"temperature": []interface{}{
			map[string]interface{}{"datetime": t1, "value": 31.0},
			map[string]interface{}{"datetime": t0, "value": 30.0},
			map[string]interface{}{"datetime": t1, "value": 32.0},
		},
		"apparent_temperature": []interface{}{map[string]interface{}{"datetime": t0, "value": 33.0}},
		"pressure":             []interface{}{map[string]interface{}{"datetime": t0, "value": 100000.0}},
		"humidity":             []interface{}{map[string]interface{}{"datetime": t0, "value": 0.6}},
		"cloudrate":            []interface{}{map[string]interface{}{"datetime": t0, "value": 0.4}},
		"dswrf":                []interface{}{map[string]interface{}{"datetime": t0, "value": 500.0}},
		"visibility":           []interface{}{map[string]interface{}{"datetime": t0, "value": 20.0}},
		"skycon": []interface{}{
			map[string]interface{}{"datetime": t0, "value": "CLEAR_DAY"},
			map[string]interface{}{"datetime": t1, "value": "PARTLY_CLOUDY_DAY"},
			map[string]interface{}{"datetime": t2, "value": "CLOUDY"},
		},
		"wind":          []interface{}{map[string]interface{}{"datetime": t0, "speed": 12.0, "direction": 180.0}},
		"precipitation": []interface{}{map[string]interface{}{"datetime": t0, "value": 0.2, "probability": 25.0}},
		"air_quality": map[string]interface{}{
			"pm25": []interface{}{map[string]interface{}{"datetime": t0, "value": 18.0}},
			"aqi":  []interface{}{map[string]interface{}{"datetime": t0, "value": map[string]interface{}{"chn": 52, "usa": 48}}},
		},
	})
	bundle, err := ParseHourlyV26(weather)
	if err != nil {
		t.Fatalf("ParseHourlyV26() error=%v", err)
	}
	if bundle.Status != "ok" || bundle.IssuedAtUTC != issuedAt || len(bundle.Forecasts) != 3 || len(bundle.ProviderJSON) == 0 {
		t.Fatalf("bundle=%+v", bundle)
	}
	first := bundle.Forecasts[0]
	if first.ForecastTimeUTC != time.Date(2026, 7, 22, 3, 0, 0, 0, time.UTC) || first.TemperatureC == nil || *first.TemperatureC != 30 ||
		first.WindSpeedKPH == nil || *first.WindSpeedKPH != 12 || first.PrecipProbabilityPct == nil || *first.PrecipProbabilityPct != 25 ||
		first.AQIChn == nil || *first.AQIChn != 52 || first.HourlyDescription != "未来天气稳定" || first.ForecastKeypoint != "短期无明显变化" {
		t.Fatalf("first=%+v", first)
	}
	if bundle.Forecasts[1].TemperatureC == nil || *bundle.Forecasts[1].TemperatureC != 32 || bundle.Forecasts[2].TemperatureC != nil || bundle.Forecasts[2].Skycon != "CLOUDY" {
		t.Fatalf("forecasts=%+v", bundle.Forecasts)
	}
	if !hasParseWarning(bundle.Warnings, "DUPLICATE_DATETIME") {
		t.Fatalf("warnings=%+v", bundle.Warnings)
	}
}

func TestParseHourlyV26Preserves72HourTemperatureContract(t *testing.T) {
	issuedAt := time.Date(2026, 7, 22, 2, 30, 0, 0, time.UTC)
	temperatures := make([]interface{}, 72)
	skycons := make([]interface{}, 72)
	for index := range temperatures {
		forecastTime := issuedAt.Truncate(time.Hour).Add(time.Duration(index+1) * time.Hour).Format(time.RFC3339)
		temperatures[index] = map[string]interface{}{"datetime": forecastTime, "value": 20.0 + float64(index)/10}
		skycons[index] = map[string]interface{}{"datetime": forecastTime, "value": "CLEAR_DAY"}
	}

	bundle, err := ParseHourlyV26(hourlyWeatherWithPayload(t, issuedAt, map[string]interface{}{
		"status": "ok", "temperature": temperatures, "skycon": skycons,
	}))
	if err != nil {
		t.Fatalf("ParseHourlyV26() error=%v", err)
	}
	if len(bundle.Forecasts) != 72 {
		t.Fatalf("forecast count=%d want=72", len(bundle.Forecasts))
	}
	for index := range bundle.Forecasts {
		if bundle.Forecasts[index].TemperatureC == nil {
			t.Fatalf("forecast[%d] temperature=nil", index)
		}
	}
}

func TestParseHourlyV26ContainsBadItemsAndFlagsCoverage(t *testing.T) {
	issuedAt := time.Date(2026, 7, 22, 2, 30, 0, 0, time.UTC)
	weather := hourlyWeatherWithPayload(t, issuedAt, map[string]interface{}{
		"status": "degraded",
		"temperature": []interface{}{
			map[string]interface{}{"datetime": "bad", "value": 30},
			map[string]interface{}{"datetime": "2026-07-22T03:00:00Z", "value": 200},
			map[string]interface{}{"datetime": "2026-08-20T03:00:00Z", "value": 20},
		},
		"skycon": []interface{}{
			map[string]interface{}{"datetime": "2026-07-22T03:00:00Z", "value": "CLEAR_DAY"},
			map[string]interface{}{"datetime": "2026-07-22T05:00:00Z", "value": 123},
		},
	})
	bundle, err := ParseHourlyV26(weather)
	if err != nil {
		t.Fatalf("ParseHourlyV26() error=%v", err)
	}
	if len(bundle.Forecasts) != 2 || bundle.Forecasts[0].TemperatureC != nil || bundle.Forecasts[1].Skycon != "" {
		t.Fatalf("forecasts=%+v", bundle.Forecasts)
	}
	for _, code := range []string{"MODULE_STATUS_NOT_OK", "INVALID_DATETIME", "FORECAST_TIME_OUT_OF_RANGE", "INVALID_VALUE", "INVALID_FIELD", "CORE_FIELD_COVERAGE_LOW", "IRREGULAR_TIME_INTERVAL"} {
		if !hasParseWarning(bundle.Warnings, code) {
			t.Fatalf("warnings=%+v missing=%s", bundle.Warnings, code)
		}
	}
}

func TestParseHourlyV26RejectsMissingOrEmptyModule(t *testing.T) {
	issuedAt := time.Date(2026, 7, 22, 2, 30, 0, 0, time.UTC)
	tests := []struct {
		name    string
		weather *WeatherBundle
	}{
		{name: "nil bundle"},
		{name: "missing module", weather: &WeatherBundle{Metadata: WeatherMetadata{ServerTimeUTC: issuedAt}}},
		{name: "empty module", weather: hourlyWeatherWithPayload(t, issuedAt, map[string]interface{}{"status": "ok"})},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseHourlyV26(test.weather)
			var parseError *ParseError
			if !errors.As(err, &parseError) || parseError.EndpointKind != EndpointWeatherV26 {
				t.Fatalf("ParseHourlyV26() error=%v", err)
			}
			if test.weather != nil && len(test.weather.HourlyJSON) > 0 && strings.Contains(err.Error(), string(test.weather.HourlyJSON)) {
				t.Fatalf("error leaked response: %v", err)
			}
		})
	}
}

func hourlyWeatherWithPayload(t *testing.T, issuedAt time.Time, payload map[string]interface{}) *WeatherBundle {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal hourly fixture: %v", err)
	}
	return &WeatherBundle{
		Metadata:   WeatherMetadata{ServerTimeUTC: issuedAt, ForecastKeypoint: "短期无明显变化"},
		HourlyJSON: raw,
	}
}
