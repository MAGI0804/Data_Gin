package caiyun

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestParseAlertsV26MapsSortsAndUsesLastDuplicate(t *testing.T) {
	weather := alertWeatherWithPayload(t, map[string]interface{}{
		"status": "ok", "request_status": "ok",
		"adcodes": []interface{}{map[string]interface{}{"adcode": "310000", "name": "上海市"}},
		"content": []interface{}{
			alertItem("b-alert", "0902", "旧标题", 1784688000, []float64{31.23, 121.47}),
			alertItem("a-alert", "0204", "暴雨红色预警", 1784688100, []float64{31.22, 121.46}),
			alertItem("b-alert", "0902", "雷电黄色预警", 1784688200, []float64{31.24, 121.48}),
		},
	})
	bundle, err := ParseAlertsV26(weather)
	if err != nil {
		t.Fatalf("ParseAlertsV26() error=%v", err)
	}
	if bundle.Status != "ok" || bundle.RequestStatus != "ok" || len(bundle.Alerts) != 2 || len(bundle.ProviderJSON) == 0 ||
		len(bundle.Adcodes) != 1 || bundle.Adcodes[0] != "310000" {
		t.Fatalf("bundle=%+v", bundle)
	}
	if bundle.Alerts[0].AlertID != "a-alert" || bundle.Alerts[1].AlertID != "b-alert" || bundle.Alerts[1].Title != "雷电黄色预警" {
		t.Fatalf("alerts=%+v", bundle.Alerts)
	}
	alert := bundle.Alerts[1]
	if alert.AlertTypeCode != "09" || alert.AlertLevelCode != "02" || alert.AlertTypeName != "雷电" || alert.AlertLevelName != "黄色" ||
		alert.PublishedAtUTC == nil || *alert.PublishedAtUTC != time.Unix(1784688200, 0).UTC() ||
		alert.Latitude == nil || *alert.Latitude != 31.24 || alert.Longitude == nil || *alert.Longitude != 121.48 ||
		!json.Valid(alert.AdcodesJSON) || !json.Valid(alert.ProviderJSON) {
		t.Fatalf("alert=%+v", alert)
	}
	if !hasParseWarning(bundle.Warnings, "DUPLICATE_ALERT_ID") {
		t.Fatalf("warnings=%+v", bundle.Warnings)
	}
}

func TestParseAlertsV26ContainsBadItemsWithoutLosingValidAlerts(t *testing.T) {
	weather := alertWeatherWithPayload(t, map[string]interface{}{
		"status": "degraded", "request_status": "failed", "adcodes": map[string]interface{}{},
		"content": []interface{}{
			map[string]interface{}{"alertId": "bad\nidentifier", "code": "0902"},
			map[string]interface{}{
				"alertId": "valid", "code": 902, "pubtimestamp": 1, "latlon": []float64{91, 181},
				"title": strings.Repeat("告", 1001),
			},
		},
	})
	bundle, err := ParseAlertsV26(weather)
	if err != nil {
		t.Fatalf("ParseAlertsV26() error=%v", err)
	}
	if len(bundle.Alerts) != 1 || bundle.Alerts[0].AlertID != "valid" || len([]rune(bundle.Alerts[0].Title)) != 1000 ||
		bundle.Alerts[0].PublishedAtUTC != nil || bundle.Alerts[0].Latitude != nil || bundle.Alerts[0].Code != "" {
		t.Fatalf("bundle=%+v", bundle)
	}
	for _, code := range []string{"MODULE_STATUS_NOT_OK", "REQUEST_STATUS_NOT_OK", "INVALID_FIELD", "INVALID_ALERT_ID", "INVALID_VALUE", "TEXT_TRUNCATED"} {
		if !hasParseWarning(bundle.Warnings, code) {
			t.Fatalf("warnings=%+v missing=%s", bundle.Warnings, code)
		}
	}
}

func TestParseAlertsV26AcceptsEmptyContentAndRejectsMissingContent(t *testing.T) {
	empty, err := ParseAlertsV26(alertWeatherWithPayload(t, map[string]interface{}{
		"status": "ok", "request_status": "ok", "content": []interface{}{},
	}))
	if err != nil || len(empty.Alerts) != 0 {
		t.Fatalf("empty bundle=%+v error=%v", empty, err)
	}
	tests := []*WeatherBundle{
		nil,
		{AlertJSON: []byte(`{"status":"ok"}`)},
		{AlertJSON: []byte(`{"status":"ok","content":{}}`)},
	}
	for _, weather := range tests {
		_, err := ParseAlertsV26(weather)
		var parseError *ParseError
		if !errors.As(err, &parseError) || parseError.EndpointKind != EndpointWeatherV26 {
			t.Fatalf("ParseAlertsV26() error=%v", err)
		}
	}
}

func alertWeatherWithPayload(t *testing.T, payload map[string]interface{}) *WeatherBundle {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal alert fixture: %v", err)
	}
	return &WeatherBundle{AlertJSON: raw}
}

func alertItem(alertID, code, title string, publishedAt int64, latlon []float64) map[string]interface{} {
	return map[string]interface{}{
		"alertId": alertID, "status": "预警中", "code": code, "title": title,
		"description": "请注意防范", "source": "国家预警信息发布中心", "pubtimestamp": publishedAt,
		"province": "上海市", "city": "上海市", "county": "浦东新区", "location": "浦东新区",
		"regionId": "101020600", "adcode": "310115", "latlon": latlon,
	}
}
