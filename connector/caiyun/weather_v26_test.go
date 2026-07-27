package caiyun

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestParseWeatherV26MapsMetadataAndRealtime(t *testing.T) {
	raw, err := os.ReadFile("testdata/weather_v26_realtime.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	bundle, err := ParseWeatherV26(raw)
	if err != nil {
		t.Fatalf("ParseWeatherV26() error=%v", err)
	}
	metadata := bundle.Metadata
	if metadata.Status != "ok" || metadata.APIVersion != "v2.6" || metadata.APIStatus != "active" || metadata.Language != "zh_CN" || metadata.Unit != "metric:v2" {
		t.Fatalf("metadata=%+v", metadata)
	}
	if metadata.ServerTimeUTC != time.Unix(1784688000, 0).UTC() || metadata.Timezone != "Asia/Shanghai" || metadata.TZShiftSeconds != 28800 {
		t.Fatalf("metadata time=%+v", metadata)
	}
	if metadata.Location.Latitude != 31.2285678 || metadata.Location.Longitude != 121.4551234 || metadata.Primary == nil || *metadata.Primary != 0 {
		t.Fatalf("metadata location=%+v primary=%v", metadata.Location, metadata.Primary)
	}
	realtime := bundle.Realtime
	if realtime.Status != "ok" || realtime.TemperatureC == nil || *realtime.TemperatureC != 33.2 ||
		realtime.WindDirectionDeg == nil || *realtime.WindDirectionDeg != 165.5 || realtime.Skycon != "PARTLY_CLOUDY_DAY" {
		t.Fatalf("realtime=%+v", realtime)
	}
	if realtime.LocalPrecipMMH == nil || *realtime.LocalPrecipMMH != 0.1 || realtime.AQIChn == nil || *realtime.AQIChn != 52 ||
		realtime.ComfortIndex == nil || *realtime.ComfortIndex != 5 || realtime.UltravioletDesc != "强" {
		t.Fatalf("nested realtime=%+v", realtime)
	}
	if len(realtime.ProviderJSON) == 0 || len(bundle.MinutelyJSON) == 0 || len(bundle.HourlyJSON) == 0 || len(bundle.DailyJSON) == 0 || len(bundle.AlertJSON) == 0 {
		t.Fatalf("raw modules were not preserved: %+v", bundle)
	}
	if len(bundle.Warnings) != 0 {
		t.Fatalf("warnings=%+v", bundle.Warnings)
	}
}

func TestParseWeatherV26ContainsBadRealtimeFields(t *testing.T) {
	raw := []byte(`{
		"status":"ok","api_version":"v2.6","api_status":"active","lang":"zh_CN","unit":"metric:v2",
		"tzshift":0,"timezone":"UTC","server_time":1784688000,"location":[31,121],
		"result":{"realtime":{
			"status":"degraded","temperature":33,"apparent_temperature":34,"humidity":1.2,"pressure":100000,
			"wind":{"speed":-1,"direction":361},"cloudrate":null,"visibility":10,"dswrf":100,"skycon":"CLEAR_DAY",
			"precipitation":{"local":{"status":"failed","datasource":"radar","intensity":-1}},
			"air_quality":{"pm25":"bad","aqi":{"chn":52.5}},"life_index":{"comfort":{"index":101,"desc":"hot"}}
		}}}`)
	bundle, err := ParseWeatherV26(raw)
	if err != nil {
		t.Fatalf("ParseWeatherV26() error=%v", err)
	}
	realtime := bundle.Realtime
	if realtime.HumidityRatio != nil || realtime.WindSpeedKPH != nil || realtime.WindDirectionDeg != nil || realtime.LocalPrecipMMH != nil || realtime.PM25UGM3 != nil || realtime.AQIChn != nil || realtime.ComfortIndex != nil {
		t.Fatalf("invalid values retained: %+v", realtime)
	}
	for _, code := range []string{"MODULE_STATUS_NOT_OK", "INVALID_VALUE", "MISSING_FIELD"} {
		if !hasParseWarning(bundle.Warnings, code) {
			t.Fatalf("warnings=%+v missing=%s", bundle.Warnings, code)
		}
	}
	if warningCount(bundle.Warnings, "MODULE_STATUS_NOT_OK") != 2 {
		t.Fatalf("warnings=%+v", bundle.Warnings)
	}
}

func TestParseWeatherV26WarnsOnTimezoneOffsetAndTruncatesText(t *testing.T) {
	longSkycon := strings.Repeat("天", 65)
	raw := []byte(`{
		"status":"ok","api_version":"v2.6","api_status":"active","lang":"zh_CN","unit":"metric:v2",
		"tzshift":0,"timezone":"Asia/Shanghai","server_time":1784688000,"location":[31,121],
		"result":{"forecast_keypoint":" keypoint ","realtime":{"status":"ok","temperature":1,"apparent_temperature":1,"humidity":0.5,"pressure":100000,
		"wind":{"speed":1,"direction":1},"cloudrate":0.5,"visibility":1,"dswrf":1,"skycon":"` + longSkycon + `"}}
	}`)
	bundle, err := ParseWeatherV26(raw)
	if err != nil {
		t.Fatalf("ParseWeatherV26() error=%v", err)
	}
	if bundle.Metadata.ForecastKeypoint != "keypoint" || len([]rune(bundle.Realtime.Skycon)) != 64 {
		t.Fatalf("bundle=%+v", bundle)
	}
	for _, code := range []string{"TZSHIFT_MISMATCH", "TEXT_TRUNCATED"} {
		if !hasParseWarning(bundle.Warnings, code) {
			t.Fatalf("warnings=%+v missing=%s", bundle.Warnings, code)
		}
	}
}

func TestParseWeatherV26RejectsUnsafeEnvelope(t *testing.T) {
	validMetadata := `"status":"ok","api_version":"v2.6","api_status":"active","lang":"zh_CN","unit":"metric:v2","tzshift":0,"timezone":"UTC","server_time":1784688000,"location":[31,121]`
	tests := []struct {
		name string
		raw  string
	}{
		{name: "malformed", raw: `not-json`},
		{name: "provider status", raw: `{"status":"failed","result":{}}`},
		{name: "missing realtime", raw: `{` + validMetadata + `,"result":{}}`},
		{name: "wrong API version", raw: `{"status":"ok","api_version":"v3","api_status":"active","lang":"zh_CN","unit":"metric:v2","tzshift":0,"timezone":"UTC","server_time":1784688000,"location":[31,121],"result":{"realtime":{}}}`},
		{name: "wrong unit", raw: `{"status":"ok","api_version":"v2.6","api_status":"active","lang":"zh_CN","unit":"imperial","tzshift":0,"timezone":"UTC","server_time":1784688000,"location":[31,121],"result":{"realtime":{}}}`},
		{name: "old server time", raw: `{"status":"ok","api_version":"v2.6","api_status":"active","lang":"zh_CN","unit":"metric:v2","tzshift":0,"timezone":"UTC","server_time":1,"location":[31,121],"result":{"realtime":{}}}`},
		{name: "unknown timezone", raw: `{"status":"ok","api_version":"v2.6","api_status":"active","lang":"zh_CN","unit":"metric:v2","tzshift":0,"timezone":"Invalid/Zone","server_time":1784688000,"location":[31,121],"result":{"realtime":{}}}`},
		{name: "longitude latitude request order", raw: `{"status":"ok","api_version":"v2.6","api_status":"active","lang":"zh_CN","unit":"metric:v2","tzshift":0,"timezone":"UTC","server_time":1784688000,"location":[121,31],"result":{"realtime":{}}}`},
		{name: "invalid response latitude", raw: `{"status":"ok","api_version":"v2.6","api_status":"active","lang":"zh_CN","unit":"metric:v2","tzshift":0,"timezone":"UTC","server_time":1784688000,"location":[91,121],"result":{"realtime":{}}}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseWeatherV26([]byte(test.raw))
			var parseError *ParseError
			if !errors.As(err, &parseError) || parseError.EndpointKind != EndpointWeatherV26 {
				t.Fatalf("ParseWeatherV26() error=%v", err)
			}
			if strings.Contains(err.Error(), test.raw) {
				t.Fatalf("error leaked response: %v", err)
			}
		})
	}
}
