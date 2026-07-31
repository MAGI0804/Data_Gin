package configsecret

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

const Placeholder = "[已隐藏]"

func RedactJSON(config string) (string, bool) {
	if !json.Valid([]byte(config)) {
		return "{}", true
	}
	var value interface{}
	if err := json.Unmarshal([]byte(config), &value); err != nil {
		return "{}", true
	}
	redacted, hasSecret := redactValue(value, "")
	encoded, err := json.Marshal(redacted)
	if err != nil {
		return "{}", true
	}
	return string(encoded), hasSecret
}

func RedactValue(value interface{}, key string) (interface{}, bool) {
	return redactValue(value, key)
}

func NewJSON(value, fallback string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = fallback
	}
	if !json.Valid([]byte(value)) {
		return "", fmt.Errorf("config_json must be valid json")
	}
	if containsPlaceholderJSON(value) {
		return "", fmt.Errorf("config_json must not contain a redacted secret placeholder")
	}
	return value, nil
}

func MergeJSON(existing, submitted string) (string, error) {
	if !json.Valid([]byte(existing)) || !json.Valid([]byte(submitted)) {
		return "", fmt.Errorf("config_json must be valid json")
	}
	var oldValue, newValue interface{}
	if err := json.Unmarshal([]byte(existing), &oldValue); err != nil {
		return "", fmt.Errorf("decode existing config_json: %w", err)
	}
	if err := json.Unmarshal([]byte(submitted), &newValue); err != nil {
		return "", fmt.Errorf("decode submitted config_json: %w", err)
	}
	merged, err := mergeValue(oldValue, newValue, "")
	if err != nil {
		return "", err
	}
	encoded, err := json.Marshal(merged)
	if err != nil {
		return "", fmt.Errorf("encode merged config_json: %w", err)
	}
	return string(encoded), nil
}

func redactValue(value interface{}, key string) (interface{}, bool) {
	if SensitiveKey(key) {
		return Placeholder, true
	}
	if strings.EqualFold(strings.TrimSpace(key), "url") {
		if rawURL, ok := value.(string); ok {
			return redactURL(rawURL)
		}
	}
	switch typed := value.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{}, len(typed))
		hasSecret := false
		pairName, pairHasName := typed["name"].(string)
		pairValueIsSecret := pairHasName && SensitiveKey(pairName)
		for childKey, childValue := range typed {
			if pairValueIsSecret && childKey == "value" {
				result[childKey] = Placeholder
				hasSecret = true
				continue
			}
			clean, hidden := redactValue(childValue, childKey)
			result[childKey] = clean
			hasSecret = hasSecret || hidden
		}
		return result, hasSecret
	case []interface{}:
		result := make([]interface{}, len(typed))
		hasSecret := false
		for index, childValue := range typed {
			clean, hidden := redactValue(childValue, key)
			result[index] = clean
			hasSecret = hasSecret || hidden
		}
		return result, hasSecret
	default:
		return value, false
	}
}

func mergeValue(existing, submitted interface{}, key string) (interface{}, error) {
	if text, ok := submitted.(string); ok && isPlaceholder(text) {
		if existing == nil {
			return nil, fmt.Errorf("redacted secret placeholder has no existing value")
		}
		return existing, nil
	}
	if strings.EqualFold(strings.TrimSpace(key), "url") {
		if oldURL, oldOK := existing.(string); oldOK {
			if newURL, newOK := submitted.(string); newOK {
				return mergeURL(oldURL, newURL)
			}
		}
	}

	switch next := submitted.(type) {
	case map[string]interface{}:
		old, _ := existing.(map[string]interface{})
		result := make(map[string]interface{}, len(next))
		pairName, pairHasName := next["name"].(string)
		pairValueIsSecret := pairHasName && SensitiveKey(pairName)
		for childKey, childValue := range next {
			oldChild, exists := old[childKey]
			if SensitiveKey(childKey) && isEmptySecretValue(childValue) {
				return nil, fmt.Errorf("secret fields must be retained or replaced")
			}
			if pairValueIsSecret && childKey == "value" && isEmptySecretValue(childValue) {
				return nil, fmt.Errorf("secret fields must be retained or replaced")
			}
			if pairValueIsSecret && childKey == "value" && isPlaceholderValue(childValue) {
				if !exists {
					return nil, fmt.Errorf("redacted secret placeholder has no existing value")
				}
				result[childKey] = oldChild
				continue
			}
			merged, err := mergeValue(oldChild, childValue, childKey)
			if err != nil {
				return nil, err
			}
			result[childKey] = merged
		}
		for childKey, oldChild := range old {
			if _, present := next[childKey]; present {
				continue
			}
			if SensitiveKey(childKey) || namedValueSecret(oldChild) {
				return nil, fmt.Errorf("secret fields must be retained or replaced")
			}
		}
		return result, nil
	case []interface{}:
		old, _ := existing.([]interface{})
		result := make([]interface{}, len(next))
		for index, childValue := range next {
			var oldChild interface{}
			if index < len(old) {
				oldChild = old[index]
			}
			merged, err := mergeValue(oldChild, childValue, key)
			if err != nil {
				return nil, err
			}
			result[index] = merged
		}
		if len(next) < len(old) {
			for _, oldChild := range old[len(next):] {
				if namedValueSecret(oldChild) {
					return nil, fmt.Errorf("secret fields must be retained or replaced")
				}
			}
		}
		return result, nil
	default:
		return submitted, nil
	}
}

func SensitiveKey(key string) bool {
	normalized := strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r + ('a' - 'A')
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return -1
	}, key)
	return strings.Contains(normalized, "secret") || strings.Contains(normalized, "token") || strings.Contains(normalized, "password") || strings.Contains(normalized, "authorization") || strings.Contains(normalized, "apikey") || strings.Contains(normalized, "privatekey") || normalized == "dsn"
}

func redactURL(rawURL string) (string, bool) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return Placeholder, true
	}
	hasSecret := false
	if parsed.User != nil {
		parsed.User = url.User(Placeholder)
		hasSecret = true
	}
	query := parsed.Query()
	for key := range query {
		if SensitiveKey(key) {
			query[key] = []string{Placeholder}
			hasSecret = true
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), hasSecret
}

func mergeURL(existing, submitted string) (string, error) {
	oldURL, err := url.Parse(existing)
	if err != nil {
		return "", fmt.Errorf("decode existing url: %w", err)
	}
	newURL, err := url.Parse(submitted)
	if err != nil {
		return "", fmt.Errorf("decode submitted url: %w", err)
	}
	if oldURL.User != nil {
		if newURL.User == nil {
			return "", fmt.Errorf("secret fields must be retained or replaced")
		}
		if isPlaceholder(newURL.User.Username()) {
			newURL.User = oldURL.User
		}
	}
	oldQuery := oldURL.Query()
	newQuery := newURL.Query()
	for key, oldValues := range oldQuery {
		if !SensitiveKey(key) {
			continue
		}
		newValues, present := newQuery[key]
		if !present || len(newValues) == 0 || strings.TrimSpace(newValues[0]) == "" {
			return "", fmt.Errorf("secret fields must be retained or replaced")
		}
		if len(newValues) == 1 && isPlaceholder(newValues[0]) {
			newQuery[key] = oldValues
		}
	}
	newURL.RawQuery = newQuery.Encode()
	return newURL.String(), nil
}

func containsPlaceholderJSON(value string) bool {
	var decoded interface{}
	if json.Unmarshal([]byte(value), &decoded) != nil {
		return true
	}
	return containsPlaceholder(decoded)
}

func containsPlaceholder(value interface{}) bool {
	switch typed := value.(type) {
	case string:
		return isPlaceholder(typed) || strings.Contains(typed, url.QueryEscape(Placeholder))
	case map[string]interface{}:
		for _, child := range typed {
			if containsPlaceholder(child) {
				return true
			}
		}
	case []interface{}:
		for _, child := range typed {
			if containsPlaceholder(child) {
				return true
			}
		}
	}
	return false
}

func isPlaceholder(value string) bool {
	return strings.TrimSpace(value) == Placeholder
}

func isPlaceholderValue(value interface{}) bool {
	text, ok := value.(string)
	return ok && isPlaceholder(text)
}

func isEmptySecretValue(value interface{}) bool {
	text, ok := value.(string)
	return ok && strings.TrimSpace(text) == ""
}

func namedValueSecret(value interface{}) bool {
	object, ok := value.(map[string]interface{})
	if !ok {
		return false
	}
	name, ok := object["name"].(string)
	return ok && SensitiveKey(name)
}
