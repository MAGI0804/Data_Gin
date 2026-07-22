package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	"gin-biz-web-api/pkg/providerhttp"
)

const (
	defaultBaseURL          = "https://open.feishu.cn"
	tenantTokenPath         = "/open-apis/auth/v3/tenant_access_token/internal"
	maxTenantTokenBodyBytes = 64 * 1024
	maxTenantTokenLifetime  = 24 * time.Hour
)

type TenantToken struct {
	Value     string        `json:"-"`
	ExpiresIn time.Duration `json:"-"`
}

func (TenantToken) String() string   { return "feishu.TenantToken{redacted}" }
func (TenantToken) GoString() string { return "feishu.TenantToken{redacted}" }

type TokenError struct {
	Class     providerhttp.ErrorClass
	Retryable bool
	HTTPCode  int
	Code      int
	cause     error
}

func (err *TokenError) Error() string {
	if err == nil {
		return "feishu token: unknown error"
	}
	return fmt.Sprintf("feishu token: request failed (%s)", err.Class)
}

func (err *TokenError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.cause
}

type TokenClient struct {
	endpoint  string
	appID     string
	appSecret string
	http      *http.Client
}

func NewTokenClient(appID, appSecret string, httpClient *http.Client) (*TokenClient, error) {
	return newTokenClient(defaultBaseURL, appID, appSecret, httpClient, false)
}

func newTokenClient(
	baseURL string,
	appID string,
	appSecret string,
	httpClient *http.Client,
	allowLoopbackHTTP bool,
) (*TokenClient, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed == nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" ||
		(parsed.Scheme != "https" && !(allowLoopbackHTTP && parsed.Scheme == "http" && isLoopbackHost(parsed.Hostname()))) ||
		!validTokenCredential(appID) || !validTokenCredential(appSecret) {
		return nil, fmt.Errorf("feishu token: invalid client configuration")
	}
	if httpClient == nil {
		httpClient, err = providerhttp.NewClient(providerhttp.ClientConfig{})
		if err != nil {
			return nil, fmt.Errorf("feishu token: create HTTP client: %w", err)
		}
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + tenantTokenPath
	return &TokenClient{endpoint: parsed.String(), appID: appID, appSecret: appSecret, http: httpClient}, nil
}

func (client *TokenClient) Fetch(ctx context.Context) (TenantToken, error) {
	if client == nil || client.http == nil || ctx == nil || client.endpoint == "" ||
		!validTokenCredential(client.appID) || !validTokenCredential(client.appSecret) {
		return TenantToken{}, fmt.Errorf("feishu token: invalid fetch configuration")
	}
	payload, err := json.Marshal(struct {
		AppID     string `json:"app_id"`
		AppSecret string `json:"app_secret"`
	}{AppID: client.appID, AppSecret: client.appSecret})
	if err != nil {
		return TenantToken{}, fmt.Errorf("feishu token: encode request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.endpoint, bytes.NewReader(payload))
	if err != nil {
		return TenantToken{}, fmt.Errorf("feishu token: create request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	request.Header.Set("Accept", "application/json")
	response, err := client.http.Do(request)
	if err != nil {
		classification := providerhttp.ClassifyRetry(0, err)
		return TenantToken{}, &TokenError{Class: classification.Class, Retryable: classification.Retryable, cause: err}
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxTenantTokenBodyBytes+1))
	if err != nil {
		return TenantToken{}, &TokenError{Class: providerhttp.ErrorClassResponse, Retryable: true, HTTPCode: response.StatusCode, cause: err}
	}
	if len(body) > maxTenantTokenBodyBytes {
		return TenantToken{}, &TokenError{Class: providerhttp.ErrorClassResponse, HTTPCode: response.StatusCode, cause: fmt.Errorf("response body exceeds limit")}
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		classification := providerhttp.ClassifyRetry(response.StatusCode, nil)
		if classification.Class == providerhttp.ErrorClassNone {
			classification.Class = providerhttp.ErrorClassResponse
		}
		return TenantToken{}, &TokenError{
			Class: classification.Class, Retryable: classification.Retryable, HTTPCode: response.StatusCode,
		}
	}
	var decoded struct {
		Code              int    `json:"code"`
		Message           string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
		Expire            int64  `json:"expire"`
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(&decoded); err != nil {
		return TenantToken{}, &TokenError{Class: providerhttp.ErrorClassResponse, HTTPCode: response.StatusCode, cause: err}
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return TenantToken{}, &TokenError{Class: providerhttp.ErrorClassResponse, HTTPCode: response.StatusCode, cause: fmt.Errorf("response contains trailing data")}
	}
	if decoded.Code != 0 {
		return TenantToken{}, &TokenError{
			Class: providerhttp.ErrorClassAuth, HTTPCode: response.StatusCode, Code: decoded.Code,
		}
	}
	if !validTenantToken(decoded.TenantAccessToken) || decoded.Expire < 60 ||
		decoded.Expire > int64(maxTenantTokenLifetime/time.Second) {
		return TenantToken{}, &TokenError{
			Class: providerhttp.ErrorClassResponse, HTTPCode: response.StatusCode, cause: fmt.Errorf("invalid token response"),
		}
	}
	expiresIn := time.Duration(decoded.Expire) * time.Second
	return TenantToken{Value: decoded.TenantAccessToken, ExpiresIn: expiresIn}, nil
}

func validTokenCredential(value string) bool {
	if value == "" || len(value) > 4096 || value != strings.TrimSpace(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validTenantToken(value string) bool {
	return len(value) >= 16 && validTokenCredential(value)
}

func isLoopbackHost(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}
