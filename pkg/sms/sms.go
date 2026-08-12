package sms

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	openapiutil "github.com/alibabacloud-go/darabonba-openapi/v2/utils"
	dysmsapi20170525 "github.com/alibabacloud-go/dysmsapi-20170525/v5/client"
	"github.com/alibabacloud-go/tea/dara"
)

const (
	EnvAccessKeyID     = "ALIYUN_SMS_ACCESS_KEY_ID"
	EnvAccessKeySecret = "ALIYUN_SMS_ACCESS_KEY_SECRET"
	EnvSignName        = "ALIYUN_SMS_SIGN_NAME"
	EnvTemplateCode    = "ALIYUN_SMS_TEMPLATE_CODE"
	EnvEndpoint        = "ALIYUN_SMS_ENDPOINT"

	defaultEndpoint = "dysmsapi.aliyuncs.com"
	defaultTimeout  = 5 * time.Second
)

var (
	ErrNotConfigured    = errors.New("sms: provider is not configured")
	ErrProviderRejected = errors.New("sms: provider rejected request")
)

// Config keeps SMS credentials private to prevent accidental serialization.
// LoadConfig reads all values directly from the process environment.
type Config struct {
	accessKeyID     string
	accessKeySecret string
	signName        string
	templateCode    string
	endpoint        string
	timeout         time.Duration
}

func (Config) String() string               { return "sms.Config{redacted}" }
func (Config) GoString() string             { return "sms.Config{redacted}" }
func (Config) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

func LoadConfig() (Config, error) {
	config := Config{
		accessKeyID:     os.Getenv(EnvAccessKeyID),
		accessKeySecret: os.Getenv(EnvAccessKeySecret),
		signName:        os.Getenv(EnvSignName),
		templateCode:    os.Getenv(EnvTemplateCode),
		endpoint:        os.Getenv(EnvEndpoint),
		timeout:         defaultTimeout,
	}
	if strings.TrimSpace(config.endpoint) == "" {
		config.endpoint = defaultEndpoint
	}

	missing := make([]string, 0, 4)
	for name, value := range map[string]string{
		EnvAccessKeyID:     config.accessKeyID,
		EnvAccessKeySecret: config.accessKeySecret,
		EnvSignName:        config.signName,
		EnvTemplateCode:    config.templateCode,
	} {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return Config{}, fmt.Errorf("%w: missing %s", ErrNotConfigured, strings.Join(missing, ", "))
	}
	return config, nil
}

type providerRequest struct {
	phoneNumber  string
	signName     string
	templateCode string
	code         string
}

type providerResponse struct {
	code string
}

type provider interface {
	Send(ctx context.Context, request providerRequest) (providerResponse, error)
}

// Client sends verification codes through Alibaba Cloud SMS.
type Client struct {
	provider provider
	config   Config
}

func NewFromEnvironment() (*Client, error) {
	config, err := LoadConfig()
	if err != nil {
		return nil, err
	}

	apiConfig := &openapi.Config{
		AccessKeyId:     dara.String(config.accessKeyID),
		AccessKeySecret: dara.String(config.accessKeySecret),
		Endpoint:        dara.String(config.endpoint),
		ConnectTimeout:  dara.Int(int(config.timeout.Milliseconds())),
		ReadTimeout:     dara.Int(int(config.timeout.Milliseconds())),
	}
	api, err := dysmsapi20170525.NewClient(apiConfig)
	if err != nil {
		return nil, fmt.Errorf("sms: create aliyun client: %w", err)
	}
	return newClient(config, &aliyunProvider{client: api}), nil
}

func newClient(config Config, api provider) *Client {
	return &Client{provider: api, config: config}
}

func (c *Client) SendCode(ctx context.Context, phoneNumber, code string) error {
	if c == nil || c.provider == nil || ctx == nil {
		return fmt.Errorf("sms: invalid client")
	}
	if strings.TrimSpace(phoneNumber) == "" || strings.TrimSpace(code) == "" {
		return fmt.Errorf("sms: phone number and code are required")
	}

	timeout := c.config.timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	response, err := c.provider.Send(requestCtx, providerRequest{
		phoneNumber:  phoneNumber,
		signName:     c.config.signName,
		templateCode: c.config.templateCode,
		code:         code,
	})
	if err != nil {
		return fmt.Errorf("sms: send verification code: %w", err)
	}
	if response.code != "OK" {
		return fmt.Errorf("%w: code=%s", ErrProviderRejected, response.code)
	}
	return nil
}

type aliyunProvider struct {
	client *dysmsapi20170525.Client
}

func (p *aliyunProvider) Send(ctx context.Context, request providerRequest) (providerResponse, error) {
	query := map[string]interface{}{
		"PhoneNumbers": dara.String(request.phoneNumber),
		"SignName":     dara.String(request.signName),
		"TemplateCode": dara.String(request.templateCode),
		"TemplateParam": dara.String(fmt.Sprintf(
			`{"code":%q}`,
			request.code,
		)),
	}
	openAPIRequest := &openapiutil.OpenApiRequest{Query: openapiutil.Query(query)}
	params := &openapiutil.Params{
		Action:      dara.String("SendSms"),
		Version:     dara.String("2017-05-25"),
		Protocol:    dara.String("HTTPS"),
		Pathname:    dara.String("/"),
		Method:      dara.String("POST"),
		AuthType:    dara.String("AK"),
		Style:       dara.String("RPC"),
		ReqBodyType: dara.String("formData"),
		BodyType:    dara.String("json"),
	}

	timeoutMillis := defaultTimeout.Milliseconds()
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return providerResponse{}, ctx.Err()
		}
		timeoutMillis = remaining.Milliseconds()
		if timeoutMillis < 1 {
			timeoutMillis = 1
		}
	}
	body, err := p.client.Client.CallApiWithCtx(ctx, params, openAPIRequest, &dara.RuntimeOptions{
		ConnectTimeout: dara.Int(int(timeoutMillis)),
		ReadTimeout:    dara.Int(int(timeoutMillis)),
	})
	if err != nil {
		return providerResponse{}, err
	}
	response := new(dysmsapi20170525.SendSmsResponse)
	if err := dara.Convert(body, response); err != nil {
		return providerResponse{}, fmt.Errorf("decode provider response: %w", err)
	}
	if response.Body == nil || response.Body.Code == nil {
		return providerResponse{}, fmt.Errorf("provider returned an empty response")
	}
	return providerResponse{code: dara.StringValue(response.Body.Code)}, nil
}
