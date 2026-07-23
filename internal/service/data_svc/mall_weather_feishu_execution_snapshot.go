package data_svc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/model"

	"github.com/google/uuid"
)

type mallWeatherFeishuPreparedExecution struct {
	Destination *MallWeatherFeishuResolvedDestination
	Profile     MallWeatherExportProfileDTO
	Filter      data_dao.MallWeatherExportEstimateFilter
	SnapshotAt  time.Time
}

func prepareMallWeatherFeishuExecution(
	record data_dao.MallWeatherFeishuRunRecord,
	resources mallWeatherFeishuResourceResolver,
) (*mallWeatherFeishuPreparedExecution, error) {
	if resources == nil || record.Pipeline.ID == 0 || record.Detail.ID == 0 ||
		record.Detail.PipelineRunID != record.Pipeline.ID || record.Pipeline.DestinationID == 0 ||
		record.Detail.ProfileID == 0 || record.Detail.ProfileVersion == 0 ||
		uuid.Validate(record.Pipeline.TraceID) != nil || record.Detail.CreatedAt.IsZero() {
		return nil, errors.New("mall weather feishu execution: invalid stored identity")
	}
	destination, err := restoreMallWeatherFeishuDestination(
		record.Detail.DestinationSnapshotJSON,
		record.Pipeline.DestinationID,
		resources,
	)
	if err != nil {
		return nil, err
	}
	profile, err := restoreMallWeatherFeishuProfile(
		record.Detail.ProfileSnapshotJSON,
		record.Detail.ProfileID,
		record.Detail.ProfileVersion,
	)
	if err != nil {
		return nil, err
	}
	if err := validateMallWeatherFeishuExecutionPlan(destination, profile); err != nil {
		return nil, err
	}
	filters, err := restoreMallWeatherFeishuFilters(record.Detail.FiltersJSON)
	if err != nil {
		return nil, err
	}
	if err := validateMallWeatherExportJobRange(
		profile.Datasets,
		filters,
		profile.TimeZone,
		mallWeatherExportLimits{MaxRangeDays: maxMallWeatherExportConfiguredRangeDays},
	); err != nil {
		return nil, fmt.Errorf("mall weather feishu execution: invalid stored range: %w", err)
	}
	estimate, err := mallWeatherExportEstimateRequest(profile, filters, 1)
	if err != nil {
		return nil, fmt.Errorf("mall weather feishu execution: build data filter: %w", err)
	}
	return &mallWeatherFeishuPreparedExecution{
		Destination: destination,
		Profile:     profile,
		Filter:      estimate.Filter,
		SnapshotAt:  record.Detail.CreatedAt.UTC(),
	}, nil
}

func restoreMallWeatherFeishuDestination(
	raw model.JSONText,
	destinationID uint,
	resources mallWeatherFeishuResourceResolver,
) (*MallWeatherFeishuResolvedDestination, error) {
	var snapshot mallWeatherFeishuDestinationSnapshotData
	if err := decodeMallWeatherExportStoredJSON(raw, &snapshot); err != nil {
		return nil, fmt.Errorf("mall weather feishu execution: decode destination snapshot: %w", err)
	}
	if snapshot.DestinationID == 0 || snapshot.DestinationID != destinationID ||
		strings.TrimSpace(snapshot.Code) == "" || len(snapshot.Sheets) == 0 ||
		len(snapshot.Sheets) > len(mallWeatherFeishuDatasetKinds) {
		return nil, errors.New("mall weather feishu execution: invalid destination snapshot identity")
	}
	mapping := make(map[string]string, len(snapshot.Sheets))
	for _, reference := range snapshot.Sheets {
		if reference.Dataset == "" || reference.Env == "" {
			return nil, errors.New("mall weather feishu execution: invalid destination sheet reference")
		}
		if _, exists := mapping[reference.Dataset]; exists {
			return nil, errors.New("mall weather feishu execution: duplicate destination dataset")
		}
		mapping[reference.Dataset] = reference.Env
	}
	config := MallWeatherFeishuDestinationConfig{
		SpreadsheetTokenEnv: snapshot.SpreadsheetTokenEnv,
		SheetIDEnvMapping:   mapping,
		WriteMode:           snapshot.WriteMode,
		BatchRows:           snapshot.BatchRows,
		ProfileCode:         snapshot.ProfileCode,
		UniqueKeyFields:     snapshot.UniqueKeyFields,
		TimeoutSeconds:      snapshot.TimeoutSeconds,
		CreateIfMissing:     snapshot.CreateIfMissing,
		AllowHeaderRewrite:  snapshot.AllowHeaderRewrite,
	}
	configJSON, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("mall weather feishu execution: encode destination config: %w", err)
	}
	normalizedConfig, err := parseMallWeatherFeishuDestinationConfig(string(configJSON))
	if err != nil {
		return nil, fmt.Errorf("mall weather feishu execution: validate destination snapshot: %w", err)
	}
	canonicalSnapshot, err := mallWeatherFeishuDestinationSnapshot(&MallWeatherFeishuResolvedDestination{
		DestinationID: snapshot.DestinationID,
		Code:          strings.TrimSpace(snapshot.Code),
		Config:        normalizedConfig,
	})
	if err != nil {
		return nil, err
	}
	decodedSnapshot, err := json.Marshal(snapshot)
	if err != nil || !bytes.Equal(decodedSnapshot, canonicalSnapshot) {
		return nil, errors.New("mall weather feishu execution: destination snapshot is not canonical")
	}
	return resolveMallWeatherFeishuDestination(&model.DestinationDefinition{
		BaseModel:       model.BaseModel{ID: snapshot.DestinationID},
		Code:            strings.TrimSpace(snapshot.Code),
		DestinationType: mallWeatherFeishuDestinationType,
		ConfigJSON:      string(configJSON),
		Enabled:         true,
	}, resources)
}

func restoreMallWeatherFeishuProfile(
	raw model.JSONText,
	profileID uint,
	profileVersion uint64,
) (MallWeatherExportProfileDTO, error) {
	var snapshot MallWeatherExportProfileSnapshot
	if err := decodeMallWeatherExportStoredJSON(raw, &snapshot); err != nil {
		return MallWeatherExportProfileDTO{}, fmt.Errorf("mall weather feishu execution: decode profile snapshot: %w", err)
	}
	if snapshot.ProfileID == 0 || snapshot.ProfileID != profileID || snapshot.Version == 0 ||
		snapshot.Version != profileVersion {
		return MallWeatherExportProfileDTO{}, errors.New("mall weather feishu execution: profile snapshot identity mismatch")
	}
	originalConfig, err := json.Marshal(snapshot.Config)
	if err != nil {
		return MallWeatherExportProfileDTO{}, fmt.Errorf("mall weather feishu execution: encode profile snapshot: %w", err)
	}
	normalizedProfile, normalizedConfig, err := normalizeMallWeatherExportProfile(
		requestbody.MallWeatherExportProfileSaveRequest{
			Code:             snapshot.Code,
			Name:             snapshot.Name,
			TimeZone:         snapshot.Config.TimeZone,
			UnitSystem:       snapshot.Config.UnitSystem,
			DateFormat:       snapshot.Config.DateFormat,
			DateTimeFormat:   snapshot.Config.DateTimeFormat,
			FileNameTemplate: snapshot.Config.FileNameTemplate,
			Filters:          snapshot.Config.Filters,
			Datasets:         snapshot.Config.Datasets,
		},
	)
	if err != nil {
		return MallWeatherExportProfileDTO{}, fmt.Errorf("mall weather feishu execution: validate profile snapshot: %w", err)
	}
	normalizedConfigJSON, encodeErr := json.Marshal(normalizedConfig)
	if encodeErr != nil || normalizedProfile.Code != snapshot.Code || normalizedProfile.Name != snapshot.Name ||
		!bytes.Equal(originalConfig, normalizedConfigJSON) {
		return MallWeatherExportProfileDTO{}, errors.New("mall weather feishu execution: profile snapshot is not canonical")
	}
	return MallWeatherExportProfileDTO{
		ID:               snapshot.ProfileID,
		Code:             snapshot.Code,
		Name:             snapshot.Name,
		Version:          snapshot.Version,
		Enabled:          true,
		TimeZone:         normalizedConfig.TimeZone,
		UnitSystem:       normalizedConfig.UnitSystem,
		DateFormat:       normalizedConfig.DateFormat,
		DateTimeFormat:   normalizedConfig.DateTimeFormat,
		FileNameTemplate: normalizedConfig.FileNameTemplate,
		Filters:          normalizedConfig.Filters,
		Datasets:         normalizedConfig.Datasets,
	}, nil
}

func restoreMallWeatherFeishuFilters(raw model.JSONText) (requestbody.MallWeatherExportFilters, error) {
	var filters requestbody.MallWeatherExportFilters
	if err := decodeMallWeatherExportStoredJSON(raw, &filters); err != nil {
		return filters, fmt.Errorf("mall weather feishu execution: decode filters snapshot: %w", err)
	}
	original, err := json.Marshal(filters)
	if err != nil {
		return filters, fmt.Errorf("mall weather feishu execution: encode filters snapshot: %w", err)
	}
	normalized, err := normalizeMallWeatherExportFilters(filters)
	if err != nil {
		return filters, fmt.Errorf("mall weather feishu execution: validate filters snapshot: %w", err)
	}
	normalizedJSON, encodeErr := json.Marshal(normalized)
	if encodeErr != nil || !bytes.Equal(original, normalizedJSON) {
		return filters, errors.New("mall weather feishu execution: filters snapshot is not canonical")
	}
	return normalized, nil
}

func validateMallWeatherFeishuExecutionPlan(
	destination *MallWeatherFeishuResolvedDestination,
	profile MallWeatherExportProfileDTO,
) error {
	if destination == nil || destination.DestinationID == 0 || destination.Code == "" ||
		destination.SpreadsheetToken == "" || profile.ID == 0 || profile.Version == 0 || !profile.Enabled ||
		profile.Code == "" || profile.Code != destination.Config.ProfileCode || len(profile.Datasets) == 0 ||
		len(profile.Datasets) != len(destination.Config.SheetIDEnvMapping) ||
		len(profile.Datasets) != len(destination.SheetIDs) {
		return errors.New("mall weather feishu execution: snapshot plans do not match")
	}
	seen := make(map[string]struct{}, len(profile.Datasets))
	for _, dataset := range profile.Datasets {
		if dataset.Kind == "" || dataset.SplitBy != "" || destination.SheetIDs[dataset.Kind] == "" ||
			destination.Config.SheetIDEnvMapping[dataset.Kind] == "" {
			return errors.New("mall weather feishu execution: invalid planned dataset")
		}
		if _, duplicate := seen[dataset.Kind]; duplicate {
			return errors.New("mall weather feishu execution: duplicate planned dataset")
		}
		seen[dataset.Kind] = struct{}{}
		columns, err := mallWeatherExportRenderColumns(dataset)
		if err != nil || len(columns) == 0 || len(columns) > maxMallWeatherFeishuColumns {
			return errors.New("mall weather feishu execution: invalid planned columns")
		}
		if err := validateMallWeatherFeishuPlannedUniqueKeys(
			destination.Config.WriteMode,
			columns,
			destination.Config.UniqueKeyFields[dataset.Kind],
		); err != nil {
			return errors.New("mall weather feishu execution: invalid planned unique keys")
		}
	}
	return nil
}
