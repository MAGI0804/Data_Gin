package weather

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"gin-biz-web-api/connector/caiyun"
	"gin-biz-web-api/model"
)

const (
	ProviderCaiyun         = "caiyun"
	QualityStatusValid     = "valid"
	QualityStatusWarning   = "warning"
	minimumMappingUnixTime = int64(946684800)
	maximumMappingUnixTime = int64(4102444800)
)

var (
	ErrInvalidMappingInput = errors.New("weather mapper: invalid input")
	checksumPattern        = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)
)

type MappingMetadata struct {
	MallID       uint
	FetchRunID   uint
	FetchedAtUTC time.Time
	RawChecksum  string
}

type ForecastMappingInput struct {
	Metadata MappingMetadata
	Weather  *caiyun.WeatherBundle
	Minutely *caiyun.MinutelyBundle
	Hourly   *caiyun.HourlyBundle
}

type ForecastModelBatch struct {
	Realtime          *model.MallWeatherRealtime
	Minutely          []model.MallWeatherMinutely
	Hourly            []model.MallWeatherHourly
	ParseWarningsJSON model.JSONText
	RowCountsJSON     model.JSONText
}

type validatedMappingMetadata struct {
	MallID       uint
	FetchRunID   uint
	FetchedAtUTC time.Time
	RawChecksum  string
}

func MapForecasts(input ForecastMappingInput) (*ForecastModelBatch, error) {
	metadata, err := validateMappingMetadata(input.Metadata)
	if err != nil || input.Weather == nil || input.Weather.Metadata.ServerTimeUTC.IsZero() {
		return nil, ErrInvalidMappingInput
	}
	issuedAtUTC := input.Weather.Metadata.ServerTimeUTC.UTC()
	if !validMappingTime(issuedAtUTC) || !matchingIssuedAt(input.Minutely, input.Hourly, issuedAtUTC) {
		return nil, ErrInvalidMappingInput
	}

	realtimeFlags, realtimeStatus := qualityFields(input.Weather.Warnings)
	realtime := mapRealtime(metadata, issuedAtUTC, input.Weather.Realtime, realtimeStatus, realtimeFlags)
	warnings := append([]caiyun.ParseWarning(nil), input.Weather.Warnings...)
	metadataWarnings := topLevelWeatherWarnings(input.Weather.Warnings)
	minutelyRows := make([]model.MallWeatherMinutely, 0)
	if input.Minutely == nil {
		warnings = append(warnings, caiyun.ParseWarning{Code: "MODULE_NOT_PARSED", Path: "result.minutely"})
	} else {
		minutelyRows = mapMinutely(metadata, input.Minutely, metadataWarnings)
		warnings = append(warnings, input.Minutely.Warnings...)
	}
	hourlyRows := make([]model.MallWeatherHourly, 0)
	if input.Hourly == nil {
		warnings = append(warnings, caiyun.ParseWarning{Code: "MODULE_NOT_PARSED", Path: "result.hourly"})
	} else {
		hourlyRows = mapHourly(metadata, input.Hourly, metadataWarnings)
		warnings = append(warnings, input.Hourly.Warnings...)
	}
	warningsJSON, err := marshalJSONText(warnings)
	if err != nil {
		return nil, ErrInvalidMappingInput
	}
	rowCountsJSON, err := marshalJSONText(map[string]int{
		"realtime": 1,
		"minutely": len(minutelyRows),
		"hourly":   len(hourlyRows),
	})
	if err != nil {
		return nil, ErrInvalidMappingInput
	}
	return &ForecastModelBatch{
		Realtime: realtime, Minutely: minutelyRows, Hourly: hourlyRows,
		ParseWarningsJSON: warningsJSON, RowCountsJSON: rowCountsJSON,
	}, nil
}

func validateMappingMetadata(input MappingMetadata) (validatedMappingMetadata, error) {
	if input.MallID == 0 || input.FetchRunID == 0 || !validMappingTime(input.FetchedAtUTC) || !checksumPattern.MatchString(input.RawChecksum) {
		return validatedMappingMetadata{}, ErrInvalidMappingInput
	}
	input.FetchedAtUTC = input.FetchedAtUTC.UTC()
	input.RawChecksum = strings.ToLower(input.RawChecksum)
	return validatedMappingMetadata{
		MallID: input.MallID, FetchRunID: input.FetchRunID,
		FetchedAtUTC: input.FetchedAtUTC, RawChecksum: input.RawChecksum,
	}, nil
}

func validMappingTime(value time.Time) bool {
	if value.IsZero() {
		return false
	}
	unixTime := value.Unix()
	return unixTime >= minimumMappingUnixTime && unixTime < maximumMappingUnixTime
}

func matchingIssuedAt(minutely *caiyun.MinutelyBundle, hourly *caiyun.HourlyBundle, issuedAtUTC time.Time) bool {
	return (minutely == nil || minutely.IssuedAtUTC.Equal(issuedAtUTC)) &&
		(hourly == nil || hourly.IssuedAtUTC.Equal(issuedAtUTC))
}

func topLevelWeatherWarnings(warnings []caiyun.ParseWarning) []caiyun.ParseWarning {
	result := make([]caiyun.ParseWarning, 0)
	for _, warning := range warnings {
		if !strings.HasPrefix(warning.Path, "result.realtime") {
			result = append(result, warning)
		}
	}
	return result
}

func qualityFields(warnings []caiyun.ParseWarning) (model.JSONText, string) {
	flags, err := marshalJSONText(warnings)
	if err != nil {
		return model.JSONText("[]"), QualityStatusWarning
	}
	if len(warnings) == 0 {
		return flags, QualityStatusValid
	}
	return flags, QualityStatusWarning
}

func marshalJSONText(value interface{}) (model.JSONText, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return model.JSONText(raw), nil
}

func cloneFloat(value *float64) *float64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneInt(value *int) *int {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
