package caiyun

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestParseDailyV26MergesCoreGroupsAndAstro(t *testing.T) {
	issuedAt := time.Date(2026, 7, 22, 2, 3, 47, 0, time.UTC)
	weather := dailyWeatherWithPayload(t, issuedAt, map[string]interface{}{
		"status": "ok",
		"temperature": []interface{}{
			dailyMetricItem("2026-07-23", 35, 27, 31),
			dailyMetricItem("2026-07-22", 34, 26, 30),
			dailyMetricItem("2026-07-23", 36, 28, 32),
		},
		"temperature_08h_20h": []interface{}{dailyMetricItem("2026-07-22", 34, 29, 31)},
		"temperature_20h_32h": []interface{}{dailyMetricItem("2026-07-22", 29, 26, 27)},
		"humidity":            []interface{}{dailyMetricItem("2026-07-22", 0.8, 0.4, 0.6)},
		"cloudrate":           []interface{}{dailyMetricItem("2026-07-22", 0.9, 0.2, 0.5)},
		"pressure":            []interface{}{dailyMetricItem("2026-07-22", 101000, 99000, 100000)},
		"visibility":          []interface{}{dailyMetricItem("2026-07-22", 30, 10, 20)},
		"dswrf":               []interface{}{dailyMetricItem("2026-07-22", 800, 0, 400)},
		"skycon": []interface{}{
			map[string]interface{}{"date": "2026-07-22", "value": "PARTLY_CLOUDY_DAY"},
			map[string]interface{}{"date": "2026-07-23", "value": "CLOUDY"},
		},
		"skycon_08h_20h": []interface{}{map[string]interface{}{"date": "2026-07-22", "value": "CLEAR_DAY"}},
		"skycon_20h_32h": []interface{}{map[string]interface{}{"date": "2026-07-22", "value": "PARTLY_CLOUDY_NIGHT"}},
		"astro": []interface{}{map[string]interface{}{
			"date": "2026-07-22", "sunrise": map[string]interface{}{"time": "05:08"}, "sunset": map[string]interface{}{"time": "18:55:00"},
		}},
	})
	bundle, err := ParseDailyV26(weather)
	if err != nil {
		t.Fatalf("ParseDailyV26() error=%v", err)
	}
	if bundle.Status != "ok" || bundle.IssuedAtUTC != issuedAt || len(bundle.Forecasts) != 2 || len(bundle.ProviderJSON) == 0 {
		t.Fatalf("bundle=%+v", bundle)
	}
	first := bundle.Forecasts[0]
	if first.ForecastDateLocal.Format("2006-01-02") != "2026-07-22" || first.Temperature.Maximum == nil || *first.Temperature.Maximum != 34 ||
		first.DayTemperature.Average == nil || *first.DayTemperature.Average != 31 || first.NightTemperature.Minimum == nil || *first.NightTemperature.Minimum != 26 ||
		first.Humidity.Average == nil || *first.Humidity.Average != 0.6 || first.Skycon != "PARTLY_CLOUDY_DAY" ||
		first.DaySkycon != "CLEAR_DAY" || first.NightSkycon != "PARTLY_CLOUDY_NIGHT" || first.SunriseLocalTime != "05:08" || first.SunsetLocalTime != "18:55:00" {
		t.Fatalf("first=%+v", first)
	}
	if bundle.Forecasts[1].Temperature.Maximum == nil || *bundle.Forecasts[1].Temperature.Maximum != 36 || !hasParseWarning(bundle.Warnings, "DUPLICATE_DATE") {
		t.Fatalf("forecasts=%+v warnings=%+v", bundle.Forecasts, bundle.Warnings)
	}
}

func TestParseDailyV26AcceptsOfficialTimezoneDate(t *testing.T) {
	issuedAt := time.Date(2026, 7, 22, 2, 3, 47, 0, time.UTC)
	providerDate := "2026-07-22T00:00+08:00"
	weather := dailyWeatherWithPayload(t, issuedAt, map[string]interface{}{
		"status":      "ok",
		"temperature": []interface{}{dailyMetricItem(providerDate, 34, 26, 30)},
		"skycon":      []interface{}{map[string]interface{}{"date": providerDate, "value": "CLEAR_DAY"}},
	})
	bundle, err := ParseDailyV26(weather)
	if err != nil {
		t.Fatalf("ParseDailyV26() error=%v", err)
	}
	if len(bundle.Forecasts) != 1 || bundle.Forecasts[0].ForecastDateLocal.Format("2006-01-02") != "2026-07-22" {
		t.Fatalf("forecasts=%+v", bundle.Forecasts)
	}
}

func TestParseDailyV26Preserves15DayForecastAndLifeIndexContract(t *testing.T) {
	issuedAt := time.Date(2026, 7, 22, 2, 30, 0, 0, time.UTC)
	temperatures := make([]interface{}, 15)
	skycons := make([]interface{}, 15)
	comfort := make([]interface{}, 15)
	for index := range temperatures {
		forecastDate := issuedAt.In(time.FixedZone("CST", 8*60*60)).AddDate(0, 0, index).Format("2006-01-02")
		temperatures[index] = dailyMetricItem(forecastDate, 34, 26, 30)
		skycons[index] = map[string]interface{}{"date": forecastDate, "value": "CLEAR_DAY"}
		comfort[index] = map[string]interface{}{"date": forecastDate, "index": "4", "desc": "舒适"}
	}

	bundle, err := ParseDailyV26(dailyWeatherWithPayload(t, issuedAt, map[string]interface{}{
		"status":      "ok",
		"temperature": temperatures,
		"skycon":      skycons,
		"life_index":  map[string]interface{}{"comfort": comfort},
	}))
	if err != nil {
		t.Fatalf("ParseDailyV26() error=%v", err)
	}
	if len(bundle.Forecasts) != 15 {
		t.Fatalf("forecast count=%d want=15", len(bundle.Forecasts))
	}
	for index := range bundle.Forecasts {
		if len(bundle.Forecasts[index].BasicLifeIndices) != 1 {
			t.Fatalf("forecast[%d] life indices=%+v", index, bundle.Forecasts[index].BasicLifeIndices)
		}
	}
}

func TestParseDailyV26ContainsBadDatesAndMetricValues(t *testing.T) {
	issuedAt := time.Date(2026, 7, 22, 2, 3, 47, 0, time.UTC)
	weather := dailyWeatherWithPayload(t, issuedAt, map[string]interface{}{
		"status": "degraded",
		"temperature": []interface{}{
			dailyMetricItem("bad", 30, 20, 25),
			dailyMetricItem("2026-07-22", 20, 30, 40),
			dailyMetricItem("2026-09-01", 30, 20, 25),
		},
		"skycon": []interface{}{
			map[string]interface{}{"date": "2026-07-22", "value": 123},
			map[string]interface{}{"date": "2026-07-24", "value": "CLEAR_DAY"},
		},
		"astro": []interface{}{map[string]interface{}{
			"date": "2026-07-22", "sunrise": map[string]interface{}{"time": "25:00"},
		}},
	})
	bundle, err := ParseDailyV26(weather)
	if err != nil {
		t.Fatalf("ParseDailyV26() error=%v", err)
	}
	if len(bundle.Forecasts) != 2 || bundle.Forecasts[0].SunriseLocalTime != "" {
		t.Fatalf("forecasts=%+v", bundle.Forecasts)
	}
	for _, code := range []string{"MODULE_STATUS_NOT_OK", "INVALID_DATE", "FORECAST_DATE_OUT_OF_RANGE", "INVALID_RANGE_ORDER", "INVALID_AVERAGE", "INVALID_FIELD", "INVALID_VALUE", "CORE_FIELD_COVERAGE_LOW", "IRREGULAR_DATE_INTERVAL"} {
		if !hasParseWarning(bundle.Warnings, code) {
			t.Fatalf("warnings=%+v missing=%s", bundle.Warnings, code)
		}
	}
}

func TestParseDailyV26RejectsMissingOrEmptyModule(t *testing.T) {
	issuedAt := time.Date(2026, 7, 22, 2, 3, 47, 0, time.UTC)
	tests := []struct {
		name    string
		weather *WeatherBundle
	}{
		{name: "nil bundle"},
		{name: "missing module", weather: &WeatherBundle{Metadata: WeatherMetadata{ServerTimeUTC: issuedAt, Timezone: "Asia/Shanghai"}}},
		{name: "empty module", weather: dailyWeatherWithPayload(t, issuedAt, map[string]interface{}{"status": "ok"})},
		{name: "unknown timezone", weather: &WeatherBundle{Metadata: WeatherMetadata{ServerTimeUTC: issuedAt, Timezone: "Invalid/Zone"}, DailyJSON: []byte(`{"status":"ok"}`)}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseDailyV26(test.weather)
			var parseError *ParseError
			if !errors.As(err, &parseError) || parseError.EndpointKind != EndpointWeatherV26 {
				t.Fatalf("ParseDailyV26() error=%v", err)
			}
			if test.weather != nil && len(test.weather.DailyJSON) > 0 && strings.Contains(err.Error(), string(test.weather.DailyJSON)) {
				t.Fatalf("error leaked response: %v", err)
			}
		})
	}
}

func dailyMetricItem(date string, maximum, minimum, average float64) map[string]interface{} {
	return map[string]interface{}{"date": date, "max": maximum, "min": minimum, "avg": average}
}

func dailyWeatherWithPayload(t *testing.T, issuedAt time.Time, payload map[string]interface{}) *WeatherBundle {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal daily fixture: %v", err)
	}
	return &WeatherBundle{
		Metadata:  WeatherMetadata{ServerTimeUTC: issuedAt, Timezone: "Asia/Shanghai"},
		DailyJSON: raw,
	}
}
