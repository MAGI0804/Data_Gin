package caiyun

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"
)

type basicLifeIndexSpec struct {
	providerKey string
	indexType   int
}

var basicLifeIndexSpecs = []basicLifeIndexSpec{
	{providerKey: "carWashing", indexType: 6},
	{providerKey: "coldRisk", indexType: 7},
	{providerKey: "comfort", indexType: 8},
	{providerKey: "dressing", indexType: 10},
	{providerKey: "ultraviolet", indexType: 26},
}

func mergeDailyBasicLifeIndices(raw json.RawMessage, issuedAtUTC time.Time, zone *time.Location, rows map[time.Time]*DailyForecast, warnings *[]ParseWarning) {
	if len(raw) == 0 || string(raw) == "null" {
		return
	}
	var groups map[string]json.RawMessage
	if !decodeWeatherObject(raw, "result.daily.life_index", false, &groups, warnings) {
		return
	}
	knownGroups := make(map[string]struct{}, len(basicLifeIndexSpecs))
	for _, spec := range basicLifeIndexSpecs {
		knownGroups[spec.providerKey] = struct{}{}
		series := groups[spec.providerKey]
		mergeDailyItems(series, "result.daily.life_index."+spec.providerKey, false, issuedAtUTC, zone, rows, warnings,
			func(item dailyItem, itemPath string, row *DailyForecast) {
				mergeDailyLifeIndexItem(row, spec, item, itemPath, warnings)
			})
	}
	unknownGroups := make([]string, 0)
	for group := range groups {
		if _, known := knownGroups[group]; !known {
			unknownGroups = append(unknownGroups, group)
		}
	}
	sort.Strings(unknownGroups)
	for _, group := range unknownGroups {
		const unknownPath = "result.daily.life_index.unknown"
		*warnings = append(*warnings, ParseWarning{Code: "UNKNOWN_LIFE_INDEX_GROUP", Path: unknownPath})
		mergeDailyItems(groups[group], unknownPath, false, issuedAtUTC, zone, rows, warnings,
			func(item dailyItem, _ string, row *DailyForecast) {
				if row.basicLifeIndexRaw == nil {
					row.basicLifeIndexRaw = make(map[string]json.RawMessage)
				}
				row.basicLifeIndexRaw[group] = cloneRawMessage(item.ProviderJSON)
			})
	}
}

func mergeDailyLifeIndexItem(row *DailyForecast, spec basicLifeIndexSpec, item dailyItem, itemPath string, warnings *[]ParseWarning) {
	code, name, known := lifeIndexIdentity(spec.indexType)
	level := decodeBasicLifeIndexLevel(item.Index, itemPath+".index", warnings)
	description := decodeWeatherString(item.Description, itemPath+".desc", maximumLifeIndexDescRunes, true, warnings)
	parsed := LifeIndexItem{
		Type: spec.indexType, Code: code, Name: name, Level: level,
		Description: description, UnknownType: !known, ProviderJSON: cloneRawMessage(item.ProviderJSON),
	}
	replaced := false
	for index := range row.BasicLifeIndices {
		if row.BasicLifeIndices[index].Type == spec.indexType {
			row.BasicLifeIndices[index] = parsed
			replaced = true
			break
		}
	}
	if !replaced {
		row.BasicLifeIndices = append(row.BasicLifeIndices, parsed)
	}
	if row.basicLifeIndexRaw == nil {
		row.basicLifeIndexRaw = make(map[string]json.RawMessage)
	}
	row.basicLifeIndexRaw[spec.providerKey] = cloneRawMessage(item.ProviderJSON)
}

func decodeBasicLifeIndexLevel(raw json.RawMessage, path string, warnings *[]ParseWarning) *int {
	if len(raw) == 0 || string(raw) == "null" {
		*warnings = append(*warnings, ParseWarning{Code: "MISSING_LEVEL", Path: path})
		return nil
	}
	var level int
	if err := json.Unmarshal(raw, &level); err != nil {
		var text string
		if json.Unmarshal(raw, &text) != nil {
			*warnings = append(*warnings, ParseWarning{Code: "INVALID_LEVEL", Path: path})
			return nil
		}
		parsed, err := strconv.Atoi(strings.TrimSpace(text))
		if err != nil {
			*warnings = append(*warnings, ParseWarning{Code: "INVALID_LEVEL", Path: path})
			return nil
		}
		level = parsed
	}
	if level < -100 || level > 100 {
		*warnings = append(*warnings, ParseWarning{Code: "INVALID_LEVEL", Path: path})
		return nil
	}
	return &level
}

func finalizeDailyLifeIndices(row *DailyForecast) error {
	if len(row.BasicLifeIndices) > 0 {
		sort.Slice(row.BasicLifeIndices, func(left, right int) bool {
			return row.BasicLifeIndices[left].Type < row.BasicLifeIndices[right].Type
		})
	}
	if len(row.basicLifeIndexRaw) > 0 {
		raw, err := json.Marshal(row.basicLifeIndexRaw)
		if err != nil {
			return err
		}
		row.BasicLifeIndexJSON = raw
	}
	row.basicLifeIndexRaw = nil
	return nil
}
