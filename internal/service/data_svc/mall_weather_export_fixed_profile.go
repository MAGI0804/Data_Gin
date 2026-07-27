package data_svc

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/model"
)

const (
	fixedMallWeatherExportProfileCodePrefix = "mall_weather_excel_fixed"
	fixedMallWeatherExportProfileCode       = fixedMallWeatherExportProfileCodePrefix + "_v1"
)

func reservedFixedMallWeatherExportProfileCode(code string) bool {
	return code == fixedMallWeatherExportProfileCodePrefix ||
		strings.HasPrefix(code, fixedMallWeatherExportProfileCodePrefix+"_")
}

func fixedMallWeatherExportProfile() (*model.MallWeatherExportProfile, error) {
	enabled := true
	latest := true
	request := requestbody.MallWeatherExportProfileSaveRequest{
		Code:             fixedMallWeatherExportProfileCode,
		Name:             "商场天气完整导出",
		Enabled:          &enabled,
		TimeZone:         "Asia/Shanghai",
		UnitSystem:       "metric",
		DateFormat:       "2006-01-02",
		DateTimeFormat:   "2006-01-02 15:04:05",
		FileNameTemplate: "商场天气_{{date:20060102_150405}}.xlsx",
		Datasets: []requestbody.MallWeatherExportDataset{
			{Kind: "malls", SheetName: "商场", FreezeHeader: true, AutoFilter: true},
			{Kind: "realtime", SheetName: "实时天气", Latest: &latest, FreezeHeader: true, AutoFilter: true},
			{Kind: "minutely", SheetName: "约1公里分钟降水", Latest: &latest, FreezeHeader: true, AutoFilter: true},
			{Kind: "hourly", SheetName: "小时预报", Latest: &latest, FreezeHeader: true, AutoFilter: true},
			{Kind: "daily", SheetName: "每日预报", Latest: &latest, FreezeHeader: true, AutoFilter: true},
			{Kind: "alerts", SheetName: "气象预警", Latest: &latest, FreezeHeader: true, AutoFilter: true},
			{Kind: "life_indices", SheetName: "生活指数", Latest: &latest, FreezeHeader: true, AutoFilter: true},
		},
	}
	normalized, config, err := normalizeMallWeatherExportProfile(request)
	if err != nil {
		return nil, fmt.Errorf("mall weather export: normalize fixed profile: %w", err)
	}
	profileJSON, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("mall weather export: encode fixed profile: %w", err)
	}
	return &model.MallWeatherExportProfile{
		Code: fixedMallWeatherExportProfileCode, Name: normalized.Name,
		ProfileJSON: model.JSONText(profileJSON), Enabled: true,
	}, nil
}

func sameFixedMallWeatherExportProfileJSON(left, right model.JSONText) bool {
	var leftValue interface{}
	var rightValue interface{}
	if json.Unmarshal([]byte(left), &leftValue) != nil || json.Unmarshal([]byte(right), &rightValue) != nil {
		return false
	}
	return reflect.DeepEqual(leftValue, rightValue)
}
