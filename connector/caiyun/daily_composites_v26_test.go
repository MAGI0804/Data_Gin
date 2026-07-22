package caiyun

import (
	"testing"
	"time"
)

func TestParseDailyV26MapsPrecipitationWindAndAirQuality(t *testing.T) {
	issuedAt := time.Date(2026, 7, 22, 2, 3, 47, 0, time.UTC)
	date := "2026-07-22"
	weather := dailyWeatherWithPayload(t, issuedAt, map[string]interface{}{
		"status":                "ok",
		"temperature":           []interface{}{dailyMetricItem(date, 34, 26, 30)},
		"skycon":                []interface{}{map[string]interface{}{"date": date, "value": "CLEAR_DAY"}},
		"precipitation":         []interface{}{dailyPrecipitationItem(date, 5, 0, 1.5, 35)},
		"precipitation_08h_20h": []interface{}{dailyPrecipitationItem(date, 3, 0, 1, 25)},
		"precipitation_20h_32h": []interface{}{dailyPrecipitationItem(date, 2, 0, 0.5, 15)},
		"wind":                  []interface{}{dailyWindItem(date, 30, 180, 5, 90, 15, 135)},
		"wind_08h_20h":          []interface{}{dailyWindItem(date, 25, 170, 6, 100, 14, 140)},
		"wind_20h_32h":          []interface{}{dailyWindItem(date, 20, 160, 4, 80, 10, 120)},
		"air_quality": map[string]interface{}{
			"pm25": []interface{}{dailyMetricItem(date, 50, 10, 25)},
			"aqi": []interface{}{map[string]interface{}{
				"date": date,
				"max":  map[string]interface{}{"chn": 80, "usa": 70},
				"min":  map[string]interface{}{"chn": 20, "usa": 18},
				"avg":  map[string]interface{}{"chn": 45, "usa": 40},
			}},
		},
	})
	bundle, err := ParseDailyV26(weather)
	if err != nil {
		t.Fatalf("ParseDailyV26() error=%v", err)
	}
	if len(bundle.Forecasts) != 1 || len(bundle.Warnings) != 0 {
		t.Fatalf("bundle=%+v", bundle)
	}
	forecast := bundle.Forecasts[0]
	if forecast.Precipitation.Maximum == nil || *forecast.Precipitation.Maximum != 5 || forecast.Precipitation.ProbabilityPct == nil || *forecast.Precipitation.ProbabilityPct != 35 ||
		forecast.DayPrecipitation.Average == nil || *forecast.DayPrecipitation.Average != 1 || forecast.NightPrecipitation.ProbabilityPct == nil || *forecast.NightPrecipitation.ProbabilityPct != 15 {
		t.Fatalf("precipitation=%+v", forecast)
	}
	if forecast.Wind.Maximum.SpeedKPH == nil || *forecast.Wind.Maximum.SpeedKPH != 30 || forecast.Wind.Average.DirectionDeg == nil || *forecast.Wind.Average.DirectionDeg != 135 ||
		forecast.DayWind.Minimum.SpeedKPH == nil || *forecast.DayWind.Minimum.SpeedKPH != 6 || forecast.NightWind.Average.SpeedKPH == nil || *forecast.NightWind.Average.SpeedKPH != 10 {
		t.Fatalf("wind=%+v", forecast)
	}
	if forecast.AirQuality.PM25.Average == nil || *forecast.AirQuality.PM25.Average != 25 || forecast.AirQuality.AQIChn.Maximum == nil || *forecast.AirQuality.AQIChn.Maximum != 80 ||
		forecast.AirQuality.AQIUSA.Average == nil || *forecast.AirQuality.AQIUSA.Average != 40 {
		t.Fatalf("air quality=%+v", forecast.AirQuality)
	}
}

func TestParseDailyV26ContainsInvalidCompositeValues(t *testing.T) {
	issuedAt := time.Date(2026, 7, 22, 2, 3, 47, 0, time.UTC)
	date := "2026-07-22"
	weather := dailyWeatherWithPayload(t, issuedAt, map[string]interface{}{
		"status":        "ok",
		"temperature":   []interface{}{dailyMetricItem(date, 34, 26, 30)},
		"skycon":        []interface{}{map[string]interface{}{"date": date, "value": "CLEAR_DAY"}},
		"precipitation": []interface{}{dailyPrecipitationItem(date, 1, 2, 3, 101)},
		"wind":          []interface{}{dailyWindItem(date, -1, 361, 5, 90, 600, 135)},
		"air_quality": map[string]interface{}{
			"aqi": []interface{}{map[string]interface{}{
				"date": date,
				"max":  map[string]interface{}{"chn": 10, "usa": 10},
				"min":  map[string]interface{}{"chn": 20, "usa": 20},
				"avg":  map[string]interface{}{"chn": 30, "usa": 30},
			}},
		},
	})
	bundle, err := ParseDailyV26(weather)
	if err != nil {
		t.Fatalf("ParseDailyV26() error=%v", err)
	}
	forecast := bundle.Forecasts[0]
	if forecast.Precipitation.ProbabilityPct != nil || forecast.Wind.Maximum.SpeedKPH != nil || forecast.Wind.Maximum.DirectionDeg != nil || forecast.Wind.Average.SpeedKPH != nil {
		t.Fatalf("invalid values retained: %+v", forecast)
	}
	for _, code := range []string{"INVALID_VALUE", "INVALID_RANGE_ORDER", "INVALID_AVERAGE"} {
		if !hasParseWarning(bundle.Warnings, code) {
			t.Fatalf("warnings=%+v missing=%s", bundle.Warnings, code)
		}
	}
}

func dailyPrecipitationItem(date string, maximum, minimum, average, probability float64) map[string]interface{} {
	return map[string]interface{}{
		"date": date, "max": maximum, "min": minimum, "avg": average, "probability": probability,
	}
}

func dailyWindItem(date string, maximumSpeed, maximumDirection, minimumSpeed, minimumDirection, averageSpeed, averageDirection float64) map[string]interface{} {
	return map[string]interface{}{
		"date": date,
		"max":  map[string]interface{}{"speed": maximumSpeed, "direction": maximumDirection},
		"min":  map[string]interface{}{"speed": minimumSpeed, "direction": minimumDirection},
		"avg":  map[string]interface{}{"speed": averageSpeed, "direction": averageDirection},
	}
}
