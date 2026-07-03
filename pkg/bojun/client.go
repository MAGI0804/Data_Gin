package bojun

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"gin-biz-web-api/pkg/config"

	"github.com/google/uuid"
)

const (
	defaultBaseURL = "http://47.116.189.190:9100/bos/standard"
	defaultFormat  = "json"
	defaultTimeout = 10 * time.Second
)

// Config holds the split fields required by the Bojun signed request method.
type Config struct {
	BaseURL string
	AppKey  string
	Secret  string
	Format  string
	Timeout time.Duration
	Client  *http.Client
}

// DefaultConfig reads the runtime configuration used by SendSignedRequest.
func DefaultConfig() Config {
	timeoutSeconds := envInt("BOJUN_TIMEOUT_SECONDS", config.GetInt("Bojun.TimeoutSeconds", 10))
	if timeoutSeconds <= 0 {
		timeoutSeconds = 10
	}
	return Config{
		BaseURL: strings.TrimRight(envString("BOJUN_BASE_URL", config.GetString("Bojun.BaseURL", defaultBaseURL)), "/"),
		AppKey:  envString("BOJUN_APP_KEY", config.GetString("Bojun.AppKey", "")),
		Secret:  envString("BOJUN_SECRET", config.GetString("Bojun.Secret", "")),
		Format:  envString("BOJUN_FORMAT", config.GetString("Bojun.Format", defaultFormat)),
		Timeout: time.Duration(timeoutSeconds) * time.Second,
	}
}

// SendSignedRequest is the system method equivalent of py_file/bojun/demo.py.
func SendSignedRequest(ctx context.Context, method string, body map[string]interface{}) (map[string]interface{}, error) {
	return SendSignedRequestWithConfig(ctx, DefaultConfig(), method, body)
}

// SendSignedRequestResult keeps the Python method's single map return shape.
func SendSignedRequestResult(ctx context.Context, method string, body map[string]interface{}) map[string]interface{} {
	result, err := SendSignedRequest(ctx, method, body)
	if err != nil {
		return map[string]interface{}{"error": "请求异常: " + err.Error()}
	}
	return result
}

// SendSignedRequestWithConfig sends a POST JSON request signed with Bojun headers.
func SendSignedRequestWithConfig(ctx context.Context, cfg Config, method string, body map[string]interface{}) (map[string]interface{}, error) {
	cfg = normalizeConfig(cfg)
	method = strings.TrimSpace(method)
	if method == "" {
		return nil, fmt.Errorf("bojun method is required")
	}
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("bojun base url is required")
	}
	if cfg.AppKey == "" {
		return nil, fmt.Errorf("bojun appkey is required")
	}
	if cfg.Secret == "" {
		return nil, fmt.Errorf("bojun secret is required")
	}

	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	uniqstr := uuid.NewString()
	sign := BuildSign(cfg.AppKey, cfg.Format, method, timestamp, uniqstr, cfg.Secret)

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	reqCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, cfg.BaseURL+method, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("appkey", cfg.AppKey)
	req.Header.Set("method", method)
	req.Header.Set("timestamp", timestamp)
	req.Header.Set("uniqstr", uniqstr)
	req.Header.Set("sign", sign)
	req.Header.Set("format", cfg.Format)
	req.Header.Set("Content-Type", "application/json")

	resp, err := cfg.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("bojun request returned http status %d: %s", resp.StatusCode, string(respBytes))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// BuildSign implements secret + sorted key=value params + secret, MD5 uppercase.
func BuildSign(appKey, format, method, timestamp, uniqstr, secret string) string {
	params := map[string]string{
		"appkey":    appKey,
		"format":    format,
		"method":    method,
		"timestamp": timestamp,
		"uniqstr":   uniqstr,
	}
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+params[key])
	}
	sum := md5.Sum([]byte(secret + strings.Join(parts, "&") + secret))
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}

func normalizeConfig(cfg Config) Config {
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if cfg.Format == "" {
		cfg.Format = defaultFormat
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultTimeout
	}
	if cfg.Client == nil {
		cfg.Client = &http.Client{Timeout: cfg.Timeout}
	}
	return cfg
}

func envString(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return fallback
}
