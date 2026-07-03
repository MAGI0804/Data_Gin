package source

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"gin-biz-web-api/pkg/config"
)

type APIConnector struct{}

func (APIConnector) Code() string {
	return "api_poll"
}

func (connector APIConnector) Test(ctx context.Context, cfg Config) error {
	_, err := connector.doRequest(ctx, cfg)
	return err
}

func (connector APIConnector) Fetch(ctx context.Context, cfg Config, cursor FetchCursor) (*FetchResult, error) {
	body, err := connector.doRequestWithCursor(ctx, cfg, cursor)
	if err != nil {
		return nil, err
	}

	records, err := extractRecords(body, StringValue(cfg, "records_path"))
	if err != nil {
		return nil, err
	}

	return &FetchResult{
		Records: records,
		Cursor:  cursor,
	}, nil
}

func (APIConnector) doRequest(ctx context.Context, cfg Config) ([]byte, error) {
	return APIConnector{}.doRequestWithCursor(ctx, cfg, FetchCursor{})
}

type requestRuntime struct {
	AuthToken string
	Cursor    FetchCursor
}

func (connector APIConnector) doRequestWithCursor(ctx context.Context, cfg Config, cursor FetchCursor) ([]byte, error) {
	runtime := requestRuntime{Cursor: cursor}
	if err := connector.resolveAuth(ctx, cfg, &runtime); err != nil {
		return nil, err
	}
	return connector.doConfiguredRequest(ctx, cfg, runtime)
}

func (connector APIConnector) resolveAuth(ctx context.Context, cfg Config, runtime *requestRuntime) error {
	auth, ok := cfg["auth"].(map[string]interface{})
	if !ok || StringValue(auth, "type") != "request_token" {
		return nil
	}

	requestCfg, ok := auth["request"].(map[string]interface{})
	if !ok {
		return errorsf("api source auth.request is required")
	}

	body, err := connector.doConfiguredRequest(ctx, Config(requestCfg), *runtime)
	if err != nil {
		return fmt.Errorf("api source auth request failed: %w", err)
	}

	tokenPath := StringValue(auth, "token_path")
	if tokenPath == "" {
		tokenPath = "access_token"
	}

	var payload interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("decode api source auth response: %w", err)
	}

	tokenValue := lookupPath(payload, tokenPath)
	token, ok := tokenValue.(string)
	if !ok || token == "" {
		return fmt.Errorf("api source auth token not found at %q", tokenPath)
	}
	runtime.AuthToken = token
	return nil
}

func (connector APIConnector) doConfiguredRequest(ctx context.Context, cfg Config, runtime requestRuntime) ([]byte, error) {
	requestURL, err := resolveString(cfg["url"], runtime)
	if err != nil {
		return nil, fmt.Errorf("resolve api source url: %w", err)
	}
	if requestURL == "" {
		return nil, errorsf("api source config url is required")
	}

	method := strings.ToUpper(StringValue(cfg, "method"))
	if method == "" {
		method = http.MethodGet
	}

	parsedURL, err := url.Parse(requestURL)
	if err != nil {
		return nil, fmt.Errorf("parse api source url: %w", err)
	}
	if err := applyQueryParams(parsedURL, cfg["query_params"], runtime); err != nil {
		return nil, err
	}
	if err := applyAuthInjection(parsedURL, nil, cfg, runtime); err != nil {
		return nil, err
	}

	body, err := buildRequestBody(cfg, runtime)
	if err != nil {
		return nil, err
	}

	timeout := time.Duration(IntValue(cfg, "timeout_seconds", 30)) * time.Second
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, method, parsedURL.String(), body)
	if err != nil {
		return nil, fmt.Errorf("create api source request: %w", err)
	}

	if err := applyHeaders(req.Header, cfg["headers"], runtime); err != nil {
		return nil, err
	}
	if err := applyAuthInjection(parsedURL, req.Header, cfg, runtime); err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send api source request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read api source response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("api source returned http status %d", resp.StatusCode)
	}

	return respBody, nil
}

func buildRequestBody(cfg Config, runtime requestRuntime) (io.Reader, error) {
	if bodyJSON, ok := cfg["body_json"].(map[string]interface{}); ok {
		resolved, err := resolveObject(bodyJSON, runtime)
		if err != nil {
			return nil, fmt.Errorf("resolve api source body_json: %w", err)
		}
		bodyBytes, err := json.Marshal(resolved)
		if err != nil {
			return nil, fmt.Errorf("encode api source body_json: %w", err)
		}
		return bytes.NewReader(bodyBytes), nil
	}

	if bodyParams, ok := cfg["body_params"].([]interface{}); ok {
		bodyMap := map[string]interface{}{}
		for _, item := range bodyParams {
			param, ok := item.(map[string]interface{})
			if !ok {
				return nil, errorsf("api source body_params item must be object")
			}
			name := StringValue(param, "name")
			if name == "" {
				return nil, errorsf("api source body_params item name is required")
			}
			value, err := resolveValue(paramValueSpec(param), runtime)
			if err != nil {
				return nil, fmt.Errorf("resolve api source body param %q: %w", name, err)
			}
			bodyMap[name] = value
		}
		bodyBytes, err := json.Marshal(bodyMap)
		if err != nil {
			return nil, fmt.Errorf("encode api source body_params: %w", err)
		}
		return bytes.NewReader(bodyBytes), nil
	}

	if rawBody := StringValue(cfg, "body"); rawBody != "" {
		resolved, err := resolveString(rawBody, runtime)
		if err != nil {
			return nil, fmt.Errorf("resolve api source body: %w", err)
		}
		return bytes.NewBufferString(resolved), nil
	}

	return nil, nil
}

func applyHeaders(headers http.Header, raw interface{}, runtime requestRuntime) error {
	switch typed := raw.(type) {
	case map[string]interface{}:
		for key, value := range typed {
			resolved, err := resolveString(value, runtime)
			if err != nil {
				return fmt.Errorf("resolve api source header %q: %w", key, err)
			}
			headers.Set(key, resolved)
		}
	case []interface{}:
		for _, item := range typed {
			param, ok := item.(map[string]interface{})
			if !ok {
				return errorsf("api source headers item must be object")
			}
			name := StringValue(param, "name")
			if name == "" {
				return errorsf("api source headers item name is required")
			}
			value, err := resolveString(paramValueSpec(param), runtime)
			if err != nil {
				return fmt.Errorf("resolve api source header %q: %w", name, err)
			}
			headers.Set(name, value)
		}
	}
	return nil
}

func applyQueryParams(parsedURL *url.URL, raw interface{}, runtime requestRuntime) error {
	params, ok := raw.([]interface{})
	if !ok {
		return nil
	}

	query := parsedURL.Query()
	for _, item := range params {
		param, ok := item.(map[string]interface{})
		if !ok {
			return errorsf("api source query_params item must be object")
		}
		name := StringValue(param, "name")
		if name == "" {
			return errorsf("api source query_params item name is required")
		}
		value, err := resolveString(paramValueSpec(param), runtime)
		if err != nil {
			return fmt.Errorf("resolve api source query param %q: %w", name, err)
		}
		query.Set(name, value)
	}
	parsedURL.RawQuery = query.Encode()
	return nil
}

func applyAuthInjection(parsedURL *url.URL, headers http.Header, cfg Config, runtime requestRuntime) error {
	if runtime.AuthToken == "" {
		return nil
	}
	auth, ok := cfg["auth"].(map[string]interface{})
	if !ok {
		return nil
	}
	inject, ok := auth["inject"].(map[string]interface{})
	if !ok {
		return nil
	}

	name := StringValue(inject, "name")
	if name == "" {
		return errorsf("api source auth.inject.name is required")
	}

	switch StringValue(inject, "in") {
	case "query":
		query := parsedURL.Query()
		query.Set(name, runtime.AuthToken)
		parsedURL.RawQuery = query.Encode()
	case "header":
		if headers != nil {
			headers.Set(name, runtime.AuthToken)
		}
	}
	return nil
}

func paramValueSpec(param map[string]interface{}) interface{} {
	if value, ok := param["value"]; ok {
		return value
	}
	return param
}

func resolveObject(input map[string]interface{}, runtime requestRuntime) (map[string]interface{}, error) {
	output := make(map[string]interface{}, len(input))
	for key, value := range input {
		resolved, err := resolveValue(value, runtime)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		output[key] = resolved
	}
	return output, nil
}

func resolveValue(value interface{}, runtime requestRuntime) (interface{}, error) {
	switch typed := value.(type) {
	case map[string]interface{}:
		source := StringValue(typed, "source")
		if source == "" {
			return resolveObject(typed, runtime)
		}
		return resolveDynamicValue(source, typed, runtime)
	case []interface{}:
		values := make([]interface{}, 0, len(typed))
		for _, item := range typed {
			resolved, err := resolveValue(item, runtime)
			if err != nil {
				return nil, err
			}
			values = append(values, resolved)
		}
		return values, nil
	case string:
		return typed, nil
	default:
		return value, nil
	}
}

func resolveDynamicValue(source string, spec map[string]interface{}, runtime requestRuntime) (interface{}, error) {
	switch source {
	case "static":
		return spec["value"], nil
	case "config":
		path := StringValue(spec, "path")
		if path == "" {
			return nil, errorsf("config path is required")
		}
		return config.Get(path, spec["fallback"]), nil
	case "env":
		name := StringValue(spec, "name")
		if name == "" {
			return nil, errorsf("env name is required")
		}
		if value, ok := os.LookupEnv(name); ok {
			return value, nil
		}
		return spec["fallback"], nil
	case "time":
		now := time.Now().Add(time.Duration(IntValue(spec, "offset_seconds", 0)) * time.Second)
		if boolValue(spec, "unix") {
			return now.Unix(), nil
		}
		format := StringValue(spec, "format")
		if format == "" {
			format = time.RFC3339
		}
		return now.Format(format), nil
	case "auth":
		return runtime.AuthToken, nil
	case "cursor":
		return runtime.Cursor.Value, nil
	default:
		return nil, fmt.Errorf("unsupported value source %q", source)
	}
}

func resolveString(value interface{}, runtime requestRuntime) (string, error) {
	resolved, err := resolveValue(value, runtime)
	if err != nil {
		return "", err
	}
	if resolved == nil {
		return "", nil
	}
	switch typed := resolved.(type) {
	case string:
		return typed, nil
	default:
		return fmt.Sprintf("%v", typed), nil
	}
}

func boolValue(cfg Config, key string) bool {
	value, ok := cfg[key]
	if !ok || value == nil {
		return false
	}
	if typed, ok := value.(bool); ok {
		return typed
	}
	return false
}

func extractRecords(body []byte, recordsPath string) ([]map[string]interface{}, error) {
	var payload interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode api source response: %w", err)
	}

	value := payload
	if recordsPath != "" {
		value = lookupPath(payload, recordsPath)
	}

	switch typed := value.(type) {
	case []interface{}:
		records := make([]map[string]interface{}, 0, len(typed))
		for _, item := range typed {
			record, ok := item.(map[string]interface{})
			if !ok {
				return nil, errorsf("api source record is not an object")
			}
			records = append(records, record)
		}
		return records, nil
	case map[string]interface{}:
		return []map[string]interface{}{typed}, nil
	default:
		return nil, errorsf("api source response records must be object or array")
	}
}

func lookupPath(payload interface{}, path string) interface{} {
	current := payload
	for _, part := range strings.Split(path, ".") {
		object, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		current = object[part]
	}
	return current
}

func errorsf(format string, args ...interface{}) error {
	return fmt.Errorf(format, args...)
}
