package caiyun

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestParseDailyV26MapsBasicLifeIndicesToUnifiedTypes(t *testing.T) {
	issuedAt := time.Date(2026, 7, 22, 2, 3, 47, 0, time.UTC)
	date := "2026-07-22"
	weather := dailyWeatherWithPayload(t, issuedAt, map[string]interface{}{
		"status":      "ok",
		"temperature": []interface{}{dailyMetricItem(date, 34, 26, 30)},
		"skycon":      []interface{}{map[string]interface{}{"date": date, "value": "CLEAR_DAY"}},
		"life_index": map[string]interface{}{
			"ultraviolet": []interface{}{basicLifeIndexItem(date, "4", "强")},
			"carWashing":  []interface{}{basicLifeIndexItem(date, 2, "较适宜")},
			"dressing":    []interface{}{basicLifeIndexItem(date, 3, "炎热")},
			"comfort":     []interface{}{basicLifeIndexItem(date, 5, "闷热")},
			"coldRisk":    []interface{}{basicLifeIndexItem(date, 1, "少发")},
		},
	})
	bundle, err := ParseDailyV26(weather)
	if err != nil {
		t.Fatalf("ParseDailyV26() error=%v", err)
	}
	forecast := bundle.Forecasts[0]
	if len(forecast.BasicLifeIndices) != 5 || !json.Valid(forecast.BasicLifeIndexJSON) {
		t.Fatalf("forecast=%+v", forecast)
	}
	wantTypes := []int{6, 7, 8, 10, 26}
	for index, wantType := range wantTypes {
		item := forecast.BasicLifeIndices[index]
		if item.Type != wantType || item.Code == "" || item.Name == "" || item.Level == nil || len(item.ProviderJSON) == 0 {
			t.Fatalf("item[%d]=%+v", index, item)
		}
	}
	if *forecast.BasicLifeIndices[4].Level != 4 || forecast.BasicLifeIndices[4].Code != "ULTRAVIOLET" {
		t.Fatalf("ultraviolet=%+v", forecast.BasicLifeIndices[4])
	}
	var rawGroups map[string]json.RawMessage
	if err := json.Unmarshal(forecast.BasicLifeIndexJSON, &rawGroups); err != nil || len(rawGroups) != 5 || len(rawGroups["comfort"]) == 0 {
		t.Fatalf("basic raw=%s error=%v", forecast.BasicLifeIndexJSON, err)
	}
}

func TestParseDailyV26ContainsInvalidBasicLifeIndexLevelsAndUnknownGroups(t *testing.T) {
	issuedAt := time.Date(2026, 7, 22, 2, 3, 47, 0, time.UTC)
	date := "2026-07-22"
	longDescription := strings.Repeat("热", maximumLifeIndexDescRunes+1)
	weather := dailyWeatherWithPayload(t, issuedAt, map[string]interface{}{
		"status":      "ok",
		"temperature": []interface{}{dailyMetricItem(date, 34, 26, 30)},
		"skycon":      []interface{}{map[string]interface{}{"date": date, "value": "CLEAR_DAY"}},
		"life_index": map[string]interface{}{
			"comfort": []interface{}{
				basicLifeIndexItem(date, 1, "first"),
				basicLifeIndexItem(date, "bad", longDescription),
			},
			"ultraviolet": []interface{}{map[string]interface{}{"date": date, "desc": "missing"}},
			"futureIndex": []interface{}{basicLifeIndexItem(date, 1, "future")},
		},
	})
	bundle, err := ParseDailyV26(weather)
	if err != nil {
		t.Fatalf("ParseDailyV26() error=%v", err)
	}
	forecast := bundle.Forecasts[0]
	if len(forecast.BasicLifeIndices) != 2 || forecast.BasicLifeIndices[0].Type != 8 || forecast.BasicLifeIndices[0].Level != nil ||
		len([]rune(forecast.BasicLifeIndices[0].Description)) != maximumLifeIndexDescRunes || forecast.BasicLifeIndices[1].Level != nil {
		t.Fatalf("indices=%+v", forecast.BasicLifeIndices)
	}
	var rawGroups map[string]json.RawMessage
	if err := json.Unmarshal(forecast.BasicLifeIndexJSON, &rawGroups); err != nil || len(rawGroups["futureIndex"]) == 0 {
		t.Fatalf("basic raw=%s error=%v", forecast.BasicLifeIndexJSON, err)
	}
	for _, code := range []string{"DUPLICATE_DATE", "INVALID_LEVEL", "MISSING_LEVEL", "TEXT_TRUNCATED", "UNKNOWN_LIFE_INDEX_GROUP"} {
		if !hasParseWarning(bundle.Warnings, code) {
			t.Fatalf("warnings=%+v missing=%s", bundle.Warnings, code)
		}
	}
}

func basicLifeIndexItem(date string, index interface{}, description string) map[string]interface{} {
	return map[string]interface{}{"date": date, "index": index, "desc": description}
}
