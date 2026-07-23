package data_svc

import (
	"encoding/json"
	"fmt"

	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/model"
)

func validateMallWeatherFeishuPreparedPush(prepared *mallWeatherFeishuPreparedPush) error {
	if prepared == nil || prepared.destinationRow == nil || prepared.destination == nil ||
		prepared.profileRow == nil || prepared.destinationRow.ID != prepared.destination.DestinationID ||
		prepared.profileRow.ID != prepared.profileDTO.ID || !prepared.profileDTO.Enabled ||
		prepared.profileDTO.Code != prepared.destination.Config.ProfileCode || len(prepared.profileDTO.Datasets) == 0 ||
		len(prepared.profileDTO.Datasets) != len(prepared.destination.Config.SheetIDEnvMapping) ||
		len(prepared.estimatedRows) != len(prepared.profileDTO.Datasets) {
		return ErrMallWeatherFeishuInvalid
	}
	seen := make(map[string]struct{}, len(prepared.profileDTO.Datasets))
	for _, dataset := range prepared.profileDTO.Datasets {
		if dataset.Kind == "" || dataset.SplitBy != "" {
			return ErrMallWeatherFeishuInvalid
		}
		if _, duplicate := seen[dataset.Kind]; duplicate {
			return ErrMallWeatherFeishuInvalid
		}
		seen[dataset.Kind] = struct{}{}
		if _, exists := prepared.destination.Config.SheetIDEnvMapping[dataset.Kind]; !exists {
			return ErrMallWeatherFeishuInvalid
		}
		if _, exists := prepared.destination.SheetIDs[dataset.Kind]; !exists {
			return ErrMallWeatherFeishuInvalid
		}
		columns, err := mallWeatherExportRenderColumns(dataset)
		if err != nil || len(columns) == 0 || len(columns) > maxMallWeatherFeishuColumns {
			return ErrMallWeatherFeishuInvalid
		}
		if err := validateMallWeatherFeishuPlannedUniqueKeys(
			prepared.destination.Config.WriteMode,
			columns,
			prepared.destination.Config.UniqueKeyFields[dataset.Kind],
		); err != nil {
			return ErrMallWeatherFeishuInvalid
		}
		if rows, exists := prepared.estimatedRows[dataset.Kind]; !exists || rows < 0 || rows > maxMallWeatherExportConfiguredRows {
			return ErrMallWeatherFeishuInvalid
		}
	}
	if len(seen) != len(prepared.destination.Config.SheetIDEnvMapping) {
		return ErrMallWeatherFeishuInvalid
	}
	return nil
}

func encodeMallWeatherFeishuPushSnapshots(
	prepared *mallWeatherFeishuPreparedPush,
) (model.JSONText, model.JSONText, model.JSONText, error) {
	if err := validateMallWeatherFeishuPreparedPush(prepared); err != nil {
		return "", "", "", err
	}
	profileConfig := MallWeatherExportProfileConfig{
		TimeZone: prepared.profileDTO.TimeZone, UnitSystem: prepared.profileDTO.UnitSystem,
		DateFormat: prepared.profileDTO.DateFormat, DateTimeFormat: prepared.profileDTO.DateTimeFormat,
		FileNameTemplate: prepared.profileDTO.FileNameTemplate, Filters: prepared.profileDTO.Filters,
		Datasets: prepared.profileDTO.Datasets,
	}
	profileSnapshot, err := json.Marshal(MallWeatherExportProfileSnapshot{
		ProfileID: prepared.profileRow.ID, Code: prepared.profileRow.Code, Name: prepared.profileRow.Name,
		Version: prepared.profileRow.Version, Config: profileConfig,
	})
	if err != nil {
		return "", "", "", fmt.Errorf("mall weather feishu push: encode profile snapshot: %w", err)
	}
	filtersJSON, err := json.Marshal(prepared.filters)
	if err != nil {
		return "", "", "", fmt.Errorf("mall weather feishu push: encode filters: %w", err)
	}
	destinationSnapshot, err := mallWeatherFeishuDestinationSnapshot(prepared.destination)
	if err != nil {
		return "", "", "", fmt.Errorf("mall weather feishu push: encode destination snapshot: %w", err)
	}
	return model.JSONText(profileSnapshot), model.JSONText(filtersJSON), model.JSONText(destinationSnapshot), nil
}

func mallWeatherFeishuPushRequestForHash(
	destinationID uint,
	profileID uint,
	expectedProfileVersion *uint64,
	filters requestbody.MallWeatherExportFilters,
) interface{} {
	return struct {
		DestinationID          uint                                 `json:"destinationId"`
		ProfileID              uint                                 `json:"profileId"`
		ExpectedProfileVersion *uint64                              `json:"expectedProfileVersion,omitempty"`
		Filters                requestbody.MallWeatherExportFilters `json:"filters"`
	}{
		DestinationID: destinationID, ProfileID: profileID,
		ExpectedProfileVersion: expectedProfileVersion, Filters: filters,
	}
}
