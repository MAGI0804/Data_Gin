package responses

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

// OpenAPIDateTimeLayout is the canonical date-time format exposed by open APIs.
const OpenAPIDateTimeLayout = "2006-01-02 15:04:05"

type openAPIData struct {
	value interface{}
}

// ForOpenAPI preserves the response shape while normalizing every RFC3339
// date-time value at the public serialization boundary.
func ForOpenAPI(value interface{}) json.Marshaler {
	return openAPIData{value: value}
}

func (data openAPIData) MarshalJSON() ([]byte, error) {
	raw, err := json.Marshal(data.value)
	if err != nil {
		return nil, fmt.Errorf("open api response: marshal data: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value interface{}
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("open api response: decode data: %w", err)
	}
	return json.Marshal(formatOpenAPIDateTimes(value))
}

func formatOpenAPIDateTimes(value interface{}) interface{} {
	return formatOpenAPIField("", value)
}

func formatOpenAPIField(field string, value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		for key, item := range typed {
			typed[key] = formatOpenAPIField(key, item)
		}
		return typed
	case []interface{}:
		for index := range typed {
			typed[index] = formatOpenAPIField(field, typed[index])
		}
		return typed
	case string:
		if openAPIDateFields[field] {
			parsed, err := time.Parse(time.DateOnly, typed)
			if err == nil {
				return parsed.Format(OpenAPIDateTimeLayout)
			}
		}
		if openAPIDateTimeFields[field] {
			parsed, err := time.Parse(time.RFC3339Nano, typed)
			if err == nil {
				return parsed.Format(OpenAPIDateTimeLayout)
			}
		}
	}
	return value
}

var openAPIDateFields = map[string]bool{
	"forecastDateLocal": true,
}

var openAPIDateTimeFields = map[string]bool{
	"snapshotAtUtc":           true,
	"snapshotAtLocal":         true,
	"providerServerTimeUtc":   true,
	"providerServerTimeLocal": true,
	"forecastMinuteUtc":       true,
	"forecastMinuteLocal":     true,
	"forecastTimeUtc":         true,
	"forecastTimeLocal":       true,
	"issuedAtUtc":             true,
	"issuedAtLocal":           true,
	"fetchedAtUtc":            true,
	"fetchedAtLocal":          true,
	"publishedAtUtc":          true,
	"publishedAtLocal":        true,
	"firstSeenAtUtc":          true,
	"firstSeenAtLocal":        true,
	"lastSeenAtUtc":           true,
	"lastSeenAtLocal":         true,
	"endedAtUtc":              true,
	"endedAtLocal":            true,
	"observedStartUtc":        true,
	"observedStartLocal":      true,
	"observedEndUtc":          true,
	"observedEndLocal":        true,
}
