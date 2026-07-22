package caiyun

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	maximumAlertItems            = 256
	maximumAlertIDRunes          = 255
	maximumAlertDescriptionRunes = 16000
)

type WeatherAlert struct {
	AlertID        string
	Status         string
	Code           string
	AlertTypeCode  string
	AlertLevelCode string
	AlertTypeName  string
	AlertLevelName string
	Title          string
	Description    string
	Source         string
	PublishedAtUTC *time.Time
	Province       string
	City           string
	County         string
	Location       string
	RegionID       string
	Adcode         string
	Latitude       *float64
	Longitude      *float64
	AdcodesJSON    json.RawMessage
	ProviderJSON   json.RawMessage
}

type AlertBundle struct {
	Status        string
	RequestStatus string
	Alerts        []WeatherAlert
	Adcodes       []string
	Warnings      []ParseWarning
	ProviderJSON  json.RawMessage
}

type alertPayload struct {
	Status        json.RawMessage   `json:"status"`
	RequestStatus json.RawMessage   `json:"request_status"`
	Content       []json.RawMessage `json:"content"`
	Adcodes       json.RawMessage   `json:"adcodes"`
}

type alertItemPayload struct {
	AlertID      json.RawMessage `json:"alertId"`
	Status       json.RawMessage `json:"status"`
	Code         json.RawMessage `json:"code"`
	Title        json.RawMessage `json:"title"`
	Description  json.RawMessage `json:"description"`
	Source       json.RawMessage `json:"source"`
	PubTimestamp json.RawMessage `json:"pubtimestamp"`
	Province     json.RawMessage `json:"province"`
	City         json.RawMessage `json:"city"`
	County       json.RawMessage `json:"county"`
	Location     json.RawMessage `json:"location"`
	RegionID     json.RawMessage `json:"regionId"`
	Adcode       json.RawMessage `json:"adcode"`
	LatLon       json.RawMessage `json:"latlon"`
}

var alertCodePattern = regexp.MustCompile(`^[0-9]{4,64}$`)

func ParseAlertsV26(weather *WeatherBundle) (*AlertBundle, error) {
	if weather == nil || !isJSONObject(weather.AlertJSON) {
		return nil, weatherParseError()
	}
	var payload alertPayload
	if err := json.Unmarshal(weather.AlertJSON, &payload); err != nil || payload.Content == nil || len(payload.Content) > maximumAlertItems {
		return nil, weatherParseError()
	}
	warnings := make([]ParseWarning, 0)
	status := decodeWeatherString(payload.Status, "result.alert.status", 32, true, &warnings)
	if status != "" && status != "ok" {
		warnings = append(warnings, ParseWarning{Code: "MODULE_STATUS_NOT_OK", Path: "result.alert.status"})
	}
	requestStatus := decodeWeatherString(payload.RequestStatus, "result.alert.request_status", 32, false, &warnings)
	if requestStatus != "" && requestStatus != "ok" {
		warnings = append(warnings, ParseWarning{Code: "REQUEST_STATUS_NOT_OK", Path: "result.alert.request_status"})
	}
	adcodesJSON, adcodes := parseAlertAdcodes(payload.Adcodes, &warnings)
	alertsByID := make(map[string]WeatherAlert, len(payload.Content))
	for index, rawItem := range payload.Content {
		path := fmt.Sprintf("result.alert.content[%d]", index)
		alert, ok := parseAlertItem(rawItem, path, adcodesJSON, &warnings)
		if !ok {
			continue
		}
		if _, duplicate := alertsByID[alert.AlertID]; duplicate {
			warnings = append(warnings, ParseWarning{Code: "DUPLICATE_ALERT_ID", Path: path + ".alertId"})
		}
		alertsByID[alert.AlertID] = alert
	}
	alertIDs := make([]string, 0, len(alertsByID))
	for alertID := range alertsByID {
		alertIDs = append(alertIDs, alertID)
	}
	sort.Strings(alertIDs)
	alerts := make([]WeatherAlert, 0, len(alertIDs))
	for _, alertID := range alertIDs {
		alerts = append(alerts, alertsByID[alertID])
	}
	return &AlertBundle{
		Status: status, RequestStatus: requestStatus, Alerts: alerts, Adcodes: adcodes,
		Warnings: warnings, ProviderJSON: cloneRawMessage(weather.AlertJSON),
	}, nil
}

func parseAlertItem(raw json.RawMessage, path string, adcodesJSON json.RawMessage, warnings *[]ParseWarning) (WeatherAlert, bool) {
	var payload alertItemPayload
	if !isJSONObject(raw) || json.Unmarshal(raw, &payload) != nil {
		*warnings = append(*warnings, ParseWarning{Code: "INVALID_ITEM", Path: path})
		return WeatherAlert{}, false
	}
	alertID := decodeAlertIdentifier(payload.AlertID)
	if alertID == "" {
		*warnings = append(*warnings, ParseWarning{Code: "INVALID_ALERT_ID", Path: path + ".alertId"})
		return WeatherAlert{}, false
	}
	code := decodeWeatherString(payload.Code, path+".code", 64, false, warnings)
	if code != "" && !alertCodePattern.MatchString(code) {
		*warnings = append(*warnings, ParseWarning{Code: "INVALID_VALUE", Path: path + ".code"})
	}
	alertTypeCode, alertLevelCode, alertTypeName, alertLevelName := alertCodeIdentity(code)
	latitude, longitude := parseAlertLatLon(payload.LatLon, path+".latlon", warnings)
	return WeatherAlert{
		AlertID: alertID,
		Status:  decodeWeatherString(payload.Status, path+".status", 32, false, warnings),
		Code:    code, AlertTypeCode: alertTypeCode, AlertLevelCode: alertLevelCode,
		AlertTypeName: alertTypeName, AlertLevelName: alertLevelName,
		Title:          decodeWeatherString(payload.Title, path+".title", 1000, false, warnings),
		Description:    decodeWeatherString(payload.Description, path+".description", maximumAlertDescriptionRunes, false, warnings),
		Source:         decodeWeatherString(payload.Source, path+".source", 255, false, warnings),
		PublishedAtUTC: parseAlertPublishedAt(payload.PubTimestamp, path+".pubtimestamp", warnings),
		Province:       decodeWeatherString(payload.Province, path+".province", 128, false, warnings),
		City:           decodeWeatherString(payload.City, path+".city", 128, false, warnings),
		County:         decodeWeatherString(payload.County, path+".county", 128, false, warnings),
		Location:       decodeWeatherString(payload.Location, path+".location", 255, false, warnings),
		RegionID:       decodeWeatherString(payload.RegionID, path+".regionId", 64, false, warnings),
		Adcode:         decodeWeatherString(payload.Adcode, path+".adcode", 32, false, warnings),
		Latitude:       latitude, Longitude: longitude,
		AdcodesJSON: cloneRawMessage(adcodesJSON), ProviderJSON: cloneRawMessage(raw),
	}, true
}

func decodeAlertIdentifier(raw json.RawMessage) string {
	var value string
	if json.Unmarshal(raw, &value) != nil {
		return ""
	}
	value = strings.TrimSpace(value)
	if value == "" || utf8.RuneCountInString(value) > maximumAlertIDRunes {
		return ""
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return ""
		}
	}
	return value
}

func parseAlertPublishedAt(raw json.RawMessage, path string, warnings *[]ParseWarning) *time.Time {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var unixTime int64
	if json.Unmarshal(raw, &unixTime) != nil || unixTime < minimumWeatherUnixTime || unixTime >= maximumWeatherUnixTime {
		*warnings = append(*warnings, ParseWarning{Code: "INVALID_VALUE", Path: path})
		return nil
	}
	publishedAtUTC := time.Unix(unixTime, 0).UTC()
	return &publishedAtUTC
}

func parseAlertLatLon(raw json.RawMessage, path string, warnings *[]ParseWarning) (*float64, *float64) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var values []float64
	if json.Unmarshal(raw, &values) != nil || len(values) != 2 || !validLatitude(values[0]) || !validLongitude(values[1]) {
		*warnings = append(*warnings, ParseWarning{Code: "INVALID_VALUE", Path: path})
		return nil, nil
	}
	latitude := values[0]
	longitude := values[1]
	return &latitude, &longitude
}

func parseAlertAdcodes(raw json.RawMessage, warnings *[]ParseWarning) (json.RawMessage, []string) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var values []json.RawMessage
	if !isJSONArray(raw) || json.Unmarshal(raw, &values) != nil || len(values) > maximumAlertItems {
		*warnings = append(*warnings, ParseWarning{Code: "INVALID_FIELD", Path: "result.alert.adcodes"})
		return nil, nil
	}
	unique := make(map[string]struct{}, len(values))
	for index, value := range values {
		path := fmt.Sprintf("result.alert.adcodes[%d]", index)
		var adcode string
		if json.Unmarshal(value, &adcode) != nil {
			var item struct {
				Adcode json.RawMessage `json:"adcode"`
			}
			if !isJSONObject(value) || json.Unmarshal(value, &item) != nil || json.Unmarshal(item.Adcode, &adcode) != nil {
				*warnings = append(*warnings, ParseWarning{Code: "INVALID_ITEM", Path: path})
				continue
			}
		}
		adcode = strings.TrimSpace(adcode)
		if adcode == "" || len(adcode) > 32 || !safeProviderStatusPattern.MatchString(adcode) {
			*warnings = append(*warnings, ParseWarning{Code: "INVALID_VALUE", Path: path})
			continue
		}
		unique[adcode] = struct{}{}
	}
	adcodes := make([]string, 0, len(unique))
	for adcode := range unique {
		adcodes = append(adcodes, adcode)
	}
	sort.Strings(adcodes)
	return cloneRawMessage(raw), adcodes
}

func alertCodeIdentity(code string) (string, string, string, string) {
	if !alertCodePattern.MatchString(code) {
		return "", "", "", ""
	}
	typeCode := code[:2]
	levelCode := code[len(code)-2:]
	return typeCode, levelCode, alertTypeNames[typeCode], alertLevelNames[levelCode]
}

var alertTypeNames = map[string]string{
	"01": "台风", "02": "暴雨", "03": "暴雪", "04": "寒潮", "05": "大风",
	"06": "沙尘暴", "07": "高温", "08": "干旱", "09": "雷电", "10": "冰雹",
	"11": "霜冻", "12": "大雾", "13": "霾", "14": "道路结冰",
}

var alertLevelNames = map[string]string{
	"01": "蓝色", "02": "黄色", "03": "橙色", "04": "红色",
}
