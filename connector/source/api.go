package source

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
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
	body, err := connector.doRequest(ctx, cfg)
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
	url := StringValue(cfg, "url")
	if url == "" {
		return nil, errorsf("api source config url is required")
	}

	method := strings.ToUpper(StringValue(cfg, "method"))
	if method == "" {
		method = http.MethodGet
	}

	var body io.Reader
	if rawBody := StringValue(cfg, "body"); rawBody != "" {
		body = bytes.NewBufferString(rawBody)
	}

	timeout := time.Duration(IntValue(cfg, "timeout_seconds", 30)) * time.Second
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("create api source request: %w", err)
	}

	if headers, ok := cfg["headers"].(map[string]interface{}); ok {
		for key, value := range headers {
			if str, ok := value.(string); ok {
				req.Header.Set(key, str)
			}
		}
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
