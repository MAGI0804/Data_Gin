package data_svc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/credential"
)

const (
	mallWeatherFeishuDestinationType  = "feishu_sheet"
	defaultMallWeatherFeishuBatchRows = 200
	maxMallWeatherFeishuBatchRows     = 500
	defaultMallWeatherFeishuTimeout   = 20
	maxMallWeatherFeishuTimeout       = 120
	maxMallWeatherFeishuColumns       = 100
)

var mallWeatherFeishuDatasetKinds = map[string]struct{}{
	"realtime":     {},
	"minutely":     {},
	"hourly":       {},
	"daily":        {},
	"alerts":       {},
	"life_indices": {},
}

var mallWeatherFeishuSheetResourceEnvs = map[string]struct{}{
	credential.EnvFeishuWeatherRealtimeSheetID:  {},
	credential.EnvFeishuWeatherMinutelySheetID:  {},
	credential.EnvFeishuWeatherHourlySheetID:    {},
	credential.EnvFeishuWeatherDailySheetID:     {},
	credential.EnvFeishuWeatherAlertSheetID:     {},
	credential.EnvFeishuWeatherLifeIndexSheetID: {},
}

type MallWeatherFeishuDestinationConfig struct {
	SpreadsheetTokenEnv string              `json:"spreadsheetTokenEnv"`
	SheetIDEnvMapping   map[string]string   `json:"sheetIdEnvMapping"`
	WriteMode           string              `json:"write_mode"`
	BatchRows           int                 `json:"batch_rows"`
	ProfileCode         string              `json:"profile_code"`
	UniqueKeyFields     map[string][]string `json:"unique_key_fields,omitempty"`
	TimeoutSeconds      int                 `json:"timeout_seconds"`
	CreateIfMissing     bool                `json:"createIfMissing,omitempty"`
	AllowHeaderRewrite  bool                `json:"allowHeaderRewrite,omitempty"`
}

type MallWeatherFeishuResolvedDestination struct {
	DestinationID    uint
	Code             string
	Config           MallWeatherFeishuDestinationConfig
	SpreadsheetToken string            `json:"-"`
	SheetIDs         map[string]string `json:"-"`
}

type mallWeatherFeishuDestinationSnapshotData struct {
	DestinationID       uint                              `json:"destinationId"`
	Code                string                            `json:"code"`
	SpreadsheetTokenEnv string                            `json:"spreadsheetTokenEnv"`
	Sheets              []mallWeatherFeishuSheetReference `json:"sheets"`
	WriteMode           string                            `json:"writeMode"`
	BatchRows           int                               `json:"batchRows"`
	ProfileCode         string                            `json:"profileCode"`
	UniqueKeyFields     map[string][]string               `json:"uniqueKeyFields,omitempty"`
	TimeoutSeconds      int                               `json:"timeoutSeconds"`
	CreateIfMissing     bool                              `json:"createIfMissing"`
	AllowHeaderRewrite  bool                              `json:"allowHeaderRewrite"`
}

type mallWeatherFeishuSheetReference struct {
	Dataset string `json:"dataset"`
	Env     string `json:"env"`
}

func (MallWeatherFeishuResolvedDestination) String() string {
	return "data_svc.MallWeatherFeishuResolvedDestination{redacted}"
}

func (MallWeatherFeishuResolvedDestination) GoString() string {
	return "data_svc.MallWeatherFeishuResolvedDestination{redacted}"
}

type mallWeatherFeishuResourceResolver interface {
	EnvironmentValue(string) (string, error)
}

func resolveMallWeatherFeishuDestination(
	destination *model.DestinationDefinition,
	resources mallWeatherFeishuResourceResolver,
) (*MallWeatherFeishuResolvedDestination, error) {
	if destination == nil || resources == nil {
		return nil, errors.New("mall weather feishu destination: invalid destination")
	}
	isDestinationValid := destination.ID != 0 && destination.Enabled &&
		strings.TrimSpace(destination.Code) != "" &&
		strings.TrimSpace(destination.DestinationType) == mallWeatherFeishuDestinationType
	if !isDestinationValid {
		return nil, errors.New("mall weather feishu destination: invalid destination")
	}
	config, err := parseMallWeatherFeishuDestinationConfig(destination.ConfigJSON)
	if err != nil {
		return nil, err
	}
	spreadsheetToken, err := resources.EnvironmentValue(config.SpreadsheetTokenEnv)
	if err != nil || strings.TrimSpace(spreadsheetToken) == "" {
		return nil, errors.New("mall weather feishu destination: spreadsheet resource is unavailable")
	}
	sheetIDs := make(map[string]string, len(config.SheetIDEnvMapping))
	seenIDs := make(map[string]struct{}, len(config.SheetIDEnvMapping))
	for dataset, envName := range config.SheetIDEnvMapping {
		sheetID, resolveErr := resources.EnvironmentValue(envName)
		if resolveErr != nil || strings.TrimSpace(sheetID) == "" {
			return nil, errors.New("mall weather feishu destination: sheet resource is unavailable")
		}
		if _, exists := seenIDs[sheetID]; exists {
			return nil, errors.New("mall weather feishu destination: sheet resource is duplicated")
		}
		seenIDs[sheetID] = struct{}{}
		sheetIDs[dataset] = sheetID
	}
	return &MallWeatherFeishuResolvedDestination{
		DestinationID:    destination.ID,
		Code:             strings.TrimSpace(destination.Code),
		Config:           config,
		SpreadsheetToken: spreadsheetToken,
		SheetIDs:         sheetIDs,
	}, nil
}

func parseMallWeatherFeishuDestinationConfig(raw string) (MallWeatherFeishuDestinationConfig, error) {
	var config MallWeatherFeishuDestinationConfig
	if raw == "" || len(raw) > 64*1024 {
		return config, errors.New("mall weather feishu destination: invalid config")
	}
	decoder := json.NewDecoder(bytes.NewReader([]byte(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return config, fmt.Errorf("mall weather feishu destination: decode config: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return config, errors.New("mall weather feishu destination: config must contain one object")
	}
	config.SpreadsheetTokenEnv = strings.TrimSpace(config.SpreadsheetTokenEnv)
	config.WriteMode = strings.ToLower(strings.TrimSpace(config.WriteMode))
	config.ProfileCode = strings.TrimSpace(config.ProfileCode)
	if config.BatchRows == 0 {
		config.BatchRows = defaultMallWeatherFeishuBatchRows
	}
	if config.TimeoutSeconds == 0 {
		config.TimeoutSeconds = defaultMallWeatherFeishuTimeout
	}
	isSpreadsheetReferenceValid := config.SpreadsheetTokenEnv == credential.EnvFeishuWeatherSpreadsheetToken
	isWriteModeValid := config.WriteMode == "append" || config.WriteMode == "upsert" ||
		config.WriteMode == "overwrite_range"
	isBatchSizeValid := config.BatchRows >= 1 && config.BatchRows <= maxMallWeatherFeishuBatchRows
	isTimeoutValid := config.TimeoutSeconds >= 1 && config.TimeoutSeconds <= maxMallWeatherFeishuTimeout
	isMappingSizeValid := len(config.SheetIDEnvMapping) >= 1 &&
		len(config.SheetIDEnvMapping) <= len(mallWeatherFeishuDatasetKinds)
	if !isSpreadsheetReferenceValid || !mallWeatherExportProfileCodePattern.MatchString(config.ProfileCode) ||
		!isWriteModeValid || !isBatchSizeValid || !isTimeoutValid || !isMappingSizeValid {
		return config, errors.New("mall weather feishu destination: invalid config values")
	}
	normalizedMapping := make(map[string]string, len(config.SheetIDEnvMapping))
	seenEnvNames := make(map[string]struct{}, len(config.SheetIDEnvMapping))
	for dataset, envName := range config.SheetIDEnvMapping {
		dataset = strings.ToLower(strings.TrimSpace(dataset))
		envName = strings.TrimSpace(envName)
		_, isDatasetAllowed := mallWeatherFeishuDatasetKinds[dataset]
		_, isResourceAllowed := mallWeatherFeishuSheetResourceEnvs[envName]
		if !isDatasetAllowed || !isResourceAllowed {
			return config, errors.New("mall weather feishu destination: invalid sheet mapping")
		}
		if _, exists := normalizedMapping[dataset]; exists {
			return config, errors.New("mall weather feishu destination: duplicate dataset mapping")
		}
		if _, exists := seenEnvNames[envName]; exists {
			return config, errors.New("mall weather feishu destination: duplicate sheet resource")
		}
		seenEnvNames[envName] = struct{}{}
		normalizedMapping[dataset] = envName
	}
	config.SheetIDEnvMapping = normalizedMapping
	keys, err := normalizeMallWeatherFeishuUniqueKeys(config.WriteMode, config.SheetIDEnvMapping, config.UniqueKeyFields)
	if err != nil {
		return config, err
	}
	config.UniqueKeyFields = keys
	return config, nil
}

func normalizeMallWeatherFeishuUniqueKeys(
	writeMode string,
	mapping map[string]string,
	values map[string][]string,
) (map[string][]string, error) {
	if writeMode != "upsert" {
		if len(values) != 0 {
			return nil, errors.New("mall weather feishu destination: unique keys require upsert mode")
		}
		return nil, nil
	}
	if len(values) != len(mapping) {
		return nil, errors.New("mall weather feishu destination: upsert keys are incomplete")
	}
	result := make(map[string][]string, len(values))
	for rawDataset, rawFields := range values {
		dataset := strings.ToLower(strings.TrimSpace(rawDataset))
		if _, exists := mapping[dataset]; !exists || len(rawFields) == 0 || len(rawFields) > 8 {
			return nil, errors.New("mall weather feishu destination: invalid upsert keys")
		}
		allowed, ok := data_dao.MallWeatherExportDatasetFields(dataset)
		if !ok {
			return nil, errors.New("mall weather feishu destination: unknown upsert dataset")
		}
		fields := make([]string, 0, len(rawFields))
		seen := make(map[string]struct{}, len(rawFields))
		for _, field := range rawFields {
			field = strings.TrimSpace(field)
			if _, exists := allowed[field]; !exists {
				return nil, errors.New("mall weather feishu destination: unknown upsert field")
			}
			if _, exists := seen[field]; exists {
				return nil, errors.New("mall weather feishu destination: duplicate upsert field")
			}
			seen[field] = struct{}{}
			fields = append(fields, field)
		}
		result[dataset] = fields
	}
	return result, nil
}

func mallWeatherFeishuDestinationSnapshot(
	destination *MallWeatherFeishuResolvedDestination,
) ([]byte, error) {
	if destination == nil || destination.DestinationID == 0 || strings.TrimSpace(destination.Code) == "" {
		return nil, errors.New("mall weather feishu destination: invalid snapshot identity")
	}
	datasets := make([]string, 0, len(destination.Config.SheetIDEnvMapping))
	for dataset := range destination.Config.SheetIDEnvMapping {
		datasets = append(datasets, dataset)
	}
	sort.Strings(datasets)
	references := make([]mallWeatherFeishuSheetReference, 0, len(datasets))
	for _, dataset := range datasets {
		references = append(references, mallWeatherFeishuSheetReference{
			Dataset: dataset,
			Env:     destination.Config.SheetIDEnvMapping[dataset],
		})
	}
	return json.Marshal(mallWeatherFeishuDestinationSnapshotData{
		DestinationID:       destination.DestinationID,
		Code:                strings.TrimSpace(destination.Code),
		SpreadsheetTokenEnv: destination.Config.SpreadsheetTokenEnv,
		Sheets:              references,
		WriteMode:           destination.Config.WriteMode,
		BatchRows:           destination.Config.BatchRows,
		ProfileCode:         destination.Config.ProfileCode,
		UniqueKeyFields:     destination.Config.UniqueKeyFields,
		TimeoutSeconds:      destination.Config.TimeoutSeconds,
		CreateIfMissing:     destination.Config.CreateIfMissing,
		AllowHeaderRewrite:  destination.Config.AllowHeaderRewrite,
	})
}
