package providerhttp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

const redactedValue = "***"

var (
	sensitiveQueryKeys = map[string]struct{}{
		"access_key": {}, "access_key_id": {}, "access_key_secret": {}, "access_token": {},
		"api_key": {}, "apikey": {}, "app_id": {}, "app_key": {}, "app_secret": {},
		"client_secret": {}, "key": {}, "password": {}, "secret": {}, "signature": {},
		"tenant_access_token": {}, "token": {}, "x-cy-signature": {},
	}
	sensitiveHeaderNames = map[string]struct{}{
		"authorization": {}, "cookie": {}, "proxy-authorization": {}, "set-cookie": {},
		"tenant-access-token": {}, "x-access-token": {}, "x-api-key": {}, "x-cy-app-key": {}, "x-cy-nonce": {},
		"x-cy-signature": {}, "x-cy-timestamp": {},
	}
	sensitiveQueryPattern      = regexp.MustCompile(`(?i)([?&](?:access_key|access_key_id|access_key_secret|access_token|api_key|apikey|app_id|app_key|app_secret|client_secret|key|password|secret|signature|tenant_access_token|token|x-cy-signature)=)[^&#\s\"]*`)
	sensitiveAssignmentPattern = regexp.MustCompile(`(?i)(^|[\s,;])((?:[a-z0-9]+[_-])*(?:password|passwd|secret|token|signature|key(?:[_-](?:id|secret))?)=)[^&#\s\",;]*`)
	authorizationPattern       = regexp.MustCompile(`(?i)(authorization|proxy-authorization)\s*[:=]\s*[^\r\n,;]+`)
	sensitiveHeaderPattern     = regexp.MustCompile(`(?i)(tenant-access-token|x-access-token|x-api-key|x-cy-app-key|x-cy-nonce|x-cy-signature|x-cy-timestamp)\s*[:=]\s*[^\s,;]+`)
)

func RedactURL(rawURL string, sensitiveValues ...string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return redactedValue
	}
	if parsed.User != nil {
		parsed.User = url.User(redactedValue)
	}
	query := parsed.Query()
	for key := range query {
		if _, sensitive := sensitiveQueryKeys[strings.ToLower(key)]; sensitive {
			query[key] = []string{redactedValue}
		}
	}
	parsed.RawQuery = query.Encode()
	parsed.Fragment = ""

	redacted := parsed.String()
	redacted = strings.ReplaceAll(redacted, "%2A%2A%2A", redactedValue)
	return redactExactValues(redacted, sensitiveValues...)
}

func RedactHeaders(headers http.Header) http.Header {
	redacted := make(http.Header, len(headers))
	for name, values := range headers {
		name = http.CanonicalHeaderKey(name)
		if _, sensitive := sensitiveHeaderNames[strings.ToLower(name)]; sensitive {
			redacted[name] = []string{redactedValue}
			continue
		}
		redacted[name] = append([]string(nil), values...)
	}
	return redacted
}

func RedactText(message string, sensitiveValues ...string) string {
	redacted := sensitiveQueryPattern.ReplaceAllString(message, "${1}"+redactedValue)
	redacted = sensitiveAssignmentPattern.ReplaceAllString(redacted, "${1}${2}"+redactedValue)
	redacted = authorizationPattern.ReplaceAllString(redacted, "${1}: "+redactedValue)
	redacted = sensitiveHeaderPattern.ReplaceAllString(redacted, "${1}: "+redactedValue)
	return redactExactValues(redacted, sensitiveValues...)
}

// RedactJSON preserves the original bytes when no sensitive value is present.
// When redaction is needed, it parses and re-encodes the document so unusual
// credential characters cannot corrupt JSON syntax.
func RedactJSON(data []byte, sensitiveValues ...string) ([]byte, error) {
	redactedText := RedactText(string(data), sensitiveValues...)
	if redactedText == string(data) {
		if !json.Valid(data) {
			return nil, fmt.Errorf("provider http: invalid json")
		}
		return append([]byte(nil), data...), nil
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value interface{}
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("provider http: invalid json")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("provider http: invalid json")
	}
	value = redactJSONValue(value, sensitiveValues...)
	redacted, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("provider http: encode redacted json")
	}
	return redacted, nil
}

func redactJSONValue(value interface{}, sensitiveValues ...string) interface{} {
	switch typed := value.(type) {
	case string:
		return RedactText(typed, sensitiveValues...)
	case []interface{}:
		for index := range typed {
			typed[index] = redactJSONValue(typed[index], sensitiveValues...)
		}
		return typed
	case map[string]interface{}:
		redacted := make(map[string]interface{}, len(typed))
		for key, child := range typed {
			redacted[RedactText(key, sensitiveValues...)] = redactJSONValue(child, sensitiveValues...)
		}
		return redacted
	default:
		return value
	}
}

func redactExactValues(message string, sensitiveValues ...string) string {
	redacted := message
	for _, value := range sensitiveValues {
		if value == "" {
			continue
		}
		for _, encoded := range []string{value, url.QueryEscape(value), url.PathEscape(value)} {
			if encoded != "" {
				redacted = strings.ReplaceAll(redacted, encoded, redactedValue)
			}
		}
	}
	return redacted
}
