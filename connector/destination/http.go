package destination

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type HTTPPublisher struct{}

func (HTTPPublisher) Code() string {
	return "http"
}

func (publisher HTTPPublisher) Test(ctx context.Context, cfg Config) error {
	url := StringValue(cfg, "url")
	if url == "" {
		return fmt.Errorf("http destination config url is required")
	}

	method := strings.ToUpper(StringValue(cfg, "test_method"))
	if method == "" {
		method = http.MethodHead
	}

	timeout := time.Duration(IntValue(cfg, "timeout_seconds", 30)) * time.Second
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, method, url, nil)
	if err != nil {
		return fmt.Errorf("create http destination test request: %w", err)
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send http destination test request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("http destination returned status %d", resp.StatusCode)
	}

	return nil
}

func (publisher HTTPPublisher) Publish(ctx context.Context, cfg Config, record CleanRecord) (*PublishResult, error) {
	body := RenderTemplate(StringValue(cfg, "payload_template"), record.Content)
	result := &PublishResult{
		RequestBody: body,
	}

	respBody, status, err := publisher.send(ctx, cfg, body)
	result.HTTPStatus = status
	result.ResponseBody = respBody
	if err != nil {
		result.Success = false
		result.ErrorMessage = err.Error()
		return result, err
	}

	result.Success = status >= http.StatusOK && status < http.StatusMultipleChoices
	if !result.Success {
		result.ErrorMessage = fmt.Sprintf("http status %d", status)
		return result, fmt.Errorf("%s", result.ErrorMessage)
	}

	return result, nil
}

func (HTTPPublisher) send(ctx context.Context, cfg Config, body string) (string, int, error) {
	url := StringValue(cfg, "url")
	if url == "" {
		return "", 0, fmt.Errorf("http destination config url is required")
	}

	method := strings.ToUpper(StringValue(cfg, "method"))
	if method == "" {
		method = http.MethodPost
	}

	timeout := time.Duration(IntValue(cfg, "timeout_seconds", 30)) * time.Second
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, method, url, bytes.NewBufferString(body))
	if err != nil {
		return "", 0, fmt.Errorf("create http destination request: %w", err)
	}

	if headers, ok := cfg["headers"].(map[string]interface{}); ok {
		for key, value := range headers {
			if str, ok := value.(string); ok {
				req.Header.Set(key, str)
			}
		}
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("send http destination request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", resp.StatusCode, fmt.Errorf("read http destination response: %w", err)
	}

	return string(respBody), resp.StatusCode, nil
}

func RenderTemplate(template string, content map[string]interface{}) string {
	if template == "" {
		template = "{}"
	}

	pattern := regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_.-]+)\s*\}\}`)
	return pattern.ReplaceAllStringFunc(template, func(match string) string {
		parts := pattern.FindStringSubmatch(match)
		if len(parts) != 2 {
			return ""
		}
		value := lookup(content, parts[1])
		if value == nil {
			return ""
		}
		return fmt.Sprintf("%v", value)
	})
}

func lookup(content map[string]interface{}, path string) interface{} {
	current := interface{}(content)
	for _, part := range strings.Split(path, ".") {
		object, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		current = object[part]
	}
	return current
}
