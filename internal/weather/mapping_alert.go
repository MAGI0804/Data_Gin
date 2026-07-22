package weather

import (
	"time"

	"gin-biz-web-api/connector/caiyun"
	"gin-biz-web-api/model"
)

type AlertMappingInput struct {
	Metadata MappingMetadata
	Weather  *caiyun.WeatherBundle
	Alerts   *caiyun.AlertBundle
}

type AlertModelBatch struct {
	Alerts            []model.MallWeatherAlert
	ParseWarningsJSON model.JSONText
	RowCountsJSON     model.JSONText
}

func MapAlerts(input AlertMappingInput) (*AlertModelBatch, error) {
	metadata, err := validateMappingMetadata(input.Metadata)
	if err != nil || input.Weather == nil || !validMappingTime(input.Weather.Metadata.ServerTimeUTC) ||
		input.Alerts == nil || !validAlertRawJSON(input.Alerts) {
		return nil, ErrInvalidMappingInput
	}
	warnings := append(topLevelWeatherWarnings(input.Weather.Warnings), input.Alerts.Warnings...)
	qualityFlags, qualityStatus := qualityFields(warnings)
	rows := make([]model.MallWeatherAlert, len(input.Alerts.Alerts))
	for index, alert := range input.Alerts.Alerts {
		rows[index] = model.MallWeatherAlert{
			Provider: ProviderCaiyun, AlertID: alert.AlertID, Status: alert.Status, Code: alert.Code,
			AlertTypeCode: alert.AlertTypeCode, AlertLevelCode: alert.AlertLevelCode,
			AlertTypeName: alert.AlertTypeName, AlertLevelName: alert.AlertLevelName,
			Title: alert.Title, Description: alert.Description, Source: alert.Source,
			PublishedAtUTC: cloneTime(alert.PublishedAtUTC), Province: alert.Province, City: alert.City,
			County: alert.County, Location: alert.Location, RegionID: alert.RegionID, Adcode: alert.Adcode,
			Latitude: cloneFloat(alert.Latitude), Longitude: cloneFloat(alert.Longitude),
			AdcodesJSON: model.JSONText(string(alert.AdcodesJSON)),
			FirstSeenAt: metadata.FetchedAtUTC, LastSeenAt: metadata.FetchedAtUTC,
			ProviderPayloadJSON: model.JSONText(string(alert.ProviderJSON)),
			FetchRunID:          metadata.FetchRunID, RawChecksum: metadata.RawChecksum,
			QualityStatus: qualityStatus, QualityFlagsJSON: qualityFlags,
		}
	}
	warningsJSON, err := marshalJSONText(warnings)
	if err != nil {
		return nil, ErrInvalidMappingInput
	}
	rowCountsJSON, err := marshalJSONText(map[string]int{"alerts": len(rows)})
	if err != nil {
		return nil, ErrInvalidMappingInput
	}
	return &AlertModelBatch{Alerts: rows, ParseWarningsJSON: warningsJSON, RowCountsJSON: rowCountsJSON}, nil
}

func validAlertRawJSON(bundle *caiyun.AlertBundle) bool {
	for _, alert := range bundle.Alerts {
		if alert.AlertID == "" || !validOptionalJSON(alert.AdcodesJSON) || !validRequiredJSON(alert.ProviderJSON) {
			return false
		}
	}
	return true
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := value.UTC()
	return &cloned
}
