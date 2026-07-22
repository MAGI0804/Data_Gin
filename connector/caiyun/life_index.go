package caiyun

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	maximumLifeIndexDays        = 15
	maximumLifeIndicesPerDay    = 128
	maximumLifeIndexDescRunes   = 1000
	maximumLifeIndexDetailRunes = 16000
)

type ParseWarning struct {
	Code string `json:"code"`
	Path string `json:"path"`
}

type LifeIndexItem struct {
	Type         int
	Code         string
	Name         string
	Level        *int
	Description  string
	Detail       string
	UnknownType  bool
	ProviderJSON json.RawMessage
}

type LifeIndexDay struct {
	Date  time.Time
	Items []LifeIndexItem
}

type LifeIndexBundle struct {
	Days     []LifeIndexDay
	Warnings []ParseWarning
}

type ParseError struct {
	EndpointKind string
}

func (err *ParseError) Error() string {
	if err != nil {
		switch err.EndpointKind {
		case EndpointLifeIndexV3, EndpointWeatherV26:
			return "caiyun parser: invalid " + err.EndpointKind + " response"
		}
	}
	return "caiyun parser: invalid provider response"
}

func ParseLifeIndexV3(raw []byte) (*LifeIndexBundle, error) {
	var envelope struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || len(envelope.Data) == 0 || len(envelope.Data) > maximumLifeIndexDays {
		return nil, &ParseError{EndpointKind: EndpointLifeIndexV3}
	}

	daysByDate := make(map[string]*LifeIndexDay, len(envelope.Data))
	itemsByDate := make(map[string]map[int]LifeIndexItem, len(envelope.Data))
	warnings := make([]ParseWarning, 0)
	for dayIndex, rawDay := range envelope.Data {
		var dayPayload struct {
			Date      string            `json:"date"`
			LifeIndex []json.RawMessage `json:"lifeindex"`
		}
		if err := json.Unmarshal(rawDay, &dayPayload); err != nil || len(dayPayload.LifeIndex) > maximumLifeIndicesPerDay {
			return nil, &ParseError{EndpointKind: EndpointLifeIndexV3}
		}
		date, err := time.Parse("2006-01-02", dayPayload.Date)
		if err != nil || date.Format("2006-01-02") != dayPayload.Date {
			warnings = append(warnings, ParseWarning{Code: "INVALID_DATE", Path: fmt.Sprintf("data[%d].date", dayIndex)})
			continue
		}
		dayKey := dayPayload.Date
		if _, exists := daysByDate[dayKey]; exists {
			warnings = append(warnings, ParseWarning{Code: "DUPLICATE_DATE", Path: fmt.Sprintf("data[%d].date", dayIndex)})
		} else {
			daysByDate[dayKey] = &LifeIndexDay{Date: date, Items: make([]LifeIndexItem, 0, len(dayPayload.LifeIndex))}
			itemsByDate[dayKey] = make(map[int]LifeIndexItem, len(dayPayload.LifeIndex))
		}

		for itemIndex, rawItem := range dayPayload.LifeIndex {
			path := fmt.Sprintf("data[%d].lifeindex[%d]", dayIndex, itemIndex)
			item, itemWarnings, err := parseLifeIndexItem(rawItem, path)
			if err != nil {
				warnings = append(warnings, ParseWarning{Code: "INVALID_ITEM", Path: path})
				continue
			}
			warnings = append(warnings, itemWarnings...)
			if _, exists := itemsByDate[dayKey][item.Type]; exists {
				warnings = append(warnings, ParseWarning{Code: "DUPLICATE_TYPE", Path: path + ".type"})
			}
			itemsByDate[dayKey][item.Type] = item
		}
		if len(itemsByDate[dayKey]) == 0 {
			warnings = append(warnings, ParseWarning{Code: "EMPTY_DAY", Path: fmt.Sprintf("data[%d].lifeindex", dayIndex)})
		}
	}

	dayKeys := make([]string, 0, len(daysByDate))
	for key := range daysByDate {
		dayKeys = append(dayKeys, key)
	}
	sort.Strings(dayKeys)
	days := make([]LifeIndexDay, 0, len(dayKeys))
	for _, key := range dayKeys {
		day := daysByDate[key]
		types := make([]int, 0, len(itemsByDate[key]))
		for indexType := range itemsByDate[key] {
			types = append(types, indexType)
		}
		sort.Ints(types)
		for _, indexType := range types {
			day.Items = append(day.Items, itemsByDate[key][indexType])
		}
		days = append(days, *day)
	}
	if len(days) == 0 {
		return nil, &ParseError{EndpointKind: EndpointLifeIndexV3}
	}
	return &LifeIndexBundle{Days: days, Warnings: warnings}, nil
}

func parseLifeIndexItem(raw json.RawMessage, path string) (LifeIndexItem, []ParseWarning, error) {
	var payload struct {
		Type   *int    `json:"type"`
		Level  *int    `json:"level"`
		Desc   *string `json:"desc"`
		Detail *string `json:"detail"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil || payload.Type == nil || *payload.Type < 0 || payload.Desc == nil || payload.Detail == nil {
		return LifeIndexItem{}, nil, &ParseError{EndpointKind: EndpointLifeIndexV3}
	}
	code, name, known := lifeIndexIdentity(*payload.Type)
	warnings := make([]ParseWarning, 0, 3)
	if !known {
		warnings = append(warnings, ParseWarning{Code: "UNKNOWN_TYPE", Path: path + ".type"})
	}
	level := payload.Level
	if level == nil {
		warnings = append(warnings, ParseWarning{Code: "MISSING_LEVEL", Path: path + ".level"})
	} else if *level < 0 || *level > 100 {
		level = nil
		warnings = append(warnings, ParseWarning{Code: "INVALID_LEVEL", Path: path + ".level"})
	}
	description, truncated := truncateRunes(strings.TrimSpace(*payload.Desc), maximumLifeIndexDescRunes)
	if truncated {
		warnings = append(warnings, ParseWarning{Code: "TEXT_TRUNCATED", Path: path + ".desc"})
	}
	detail, truncated := truncateRunes(strings.TrimSpace(*payload.Detail), maximumLifeIndexDetailRunes)
	if truncated {
		warnings = append(warnings, ParseWarning{Code: "TEXT_TRUNCATED", Path: path + ".detail"})
	}
	return LifeIndexItem{
		Type: *payload.Type, Code: code, Name: name, Level: level,
		Description: description, Detail: detail, UnknownType: !known,
		ProviderJSON: append(json.RawMessage(nil), raw...),
	}, warnings, nil
}

func lifeIndexIdentity(indexType int) (string, string, bool) {
	identity, ok := lifeIndexIdentities[indexType]
	if !ok {
		return fmt.Sprintf("UNKNOWN_%d", indexType), "未知指数", false
	}
	if indexType == 0 {
		return identity.code, identity.name, false
	}
	return identity.code, identity.name, true
}

var lifeIndexIdentities = map[int]struct {
	code string
	name string
}{
	0: {"UNKNOWN_LIFEINDEX", "未知指数"},
	1: {"AIR_CONDITIONER", "空调"}, 2: {"ALLERGY", "过敏"}, 3: {"ANGLING", "钓鱼"},
	4: {"AIR_POLLUTION_DIFFUSION", "空气污染扩散条件"}, 5: {"BOATING", "划船"},
	6: {"CAR_WASHING", "洗车"}, 7: {"COLD_RISK", "感冒"}, 8: {"COMFORT", "舒适度"},
	9: {"DATING", "约会"}, 10: {"DRESSING", "穿衣"}, 11: {"DRINKING", "啤酒"},
	12: {"DRYING", "晾晒"}, 13: {"HAIRDRESSING", "美发"}, 14: {"HEATSTROKE", "中暑"},
	15: {"KITE", "放风筝"}, 16: {"MAKEUP", "化妆"}, 17: {"MOOD", "心情"},
	18: {"MORNING_EXERCISE", "晨练"}, 19: {"NIGHT_LIFE", "夜生活"}, 20: {"RAIN_GEAR", "雨具"},
	21: {"ROAD_CONDITION", "路况"}, 22: {"SHOPPING", "逛街"}, 23: {"SPORT", "运动"},
	24: {"TRAFFIC", "交通"}, 25: {"TRAVEL", "旅游"}, 26: {"ULTRAVIOLET", "紫外线/防晒"},
	27: {"WASH_CLOTHES", "洗衣"}, 28: {"WIND_COLD", "风寒"}, 30: {"STREET_STALL", "摆摊"},
	31: {"TAKEOUT", "送外卖"}, 32: {"CYCLING", "骑行"}, 33: {"HOT_POT", "火锅"},
	35: {"MOLD", "霉变"}, 36: {"STARGAZING", "观星"},
}

func truncateRunes(value string, maximum int) (string, bool) {
	if utf8.RuneCountInString(value) <= maximum {
		return value, false
	}
	runes := []rune(value)
	return string(runes[:maximum]), true
}
