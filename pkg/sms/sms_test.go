package sms

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	dysmsapi20170525 "github.com/alibabacloud-go/dysmsapi-20170525/v5/client"
	"github.com/alibabacloud-go/tea/dara"
)

type providerFunc func(context.Context, providerRequest) (providerResponse, error)

func (f providerFunc) Send(ctx context.Context, request providerRequest) (providerResponse, error) {
	return f(ctx, request)
}

type fakeAliyunClient struct {
	request *dysmsapi20170525.SendSmsRequest
	runtime *dara.RuntimeOptions
	result  *dysmsapi20170525.SendSmsResponse
	err     error
}

func (f *fakeAliyunClient) SendSmsWithContext(_ context.Context, request *dysmsapi20170525.SendSmsRequest, runtime *dara.RuntimeOptions) (*dysmsapi20170525.SendSmsResponse, error) {
	f.request, f.runtime = request, runtime
	return f.result, f.err
}

func TestLoadConfig(t *testing.T) {
	for _, name := range []string{EnvAccessKeyID, EnvAccessKeySecret, EnvSignName, EnvTemplateCode, EnvEndpoint} {
		t.Setenv(name, "")
	}

	if _, err := LoadConfig(); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("LoadConfig() error = %v, want ErrNotConfigured", err)
	}

	t.Setenv(EnvAccessKeyID, "key-id")
	t.Setenv(EnvAccessKeySecret, "secret-value")
	t.Setenv(EnvSignName, "sign")
	t.Setenv(EnvTemplateCode, "template")
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if config.endpoint != defaultEndpoint {
		t.Fatalf("endpoint = %q, want %q", config.endpoint, defaultEndpoint)
	}
	for _, formatted := range []string{fmt.Sprintf("%v", config), fmt.Sprintf("%+v", config), fmt.Sprintf("%#v", config)} {
		if strings.Contains(formatted, "secret-value") {
			t.Fatalf("formatted config leaked secret: %s", formatted)
		}
	}
	encoded, err := json.Marshal(config)
	if err != nil || string(encoded) != "{}" {
		t.Fatalf("json.Marshal(config) = %s, %v", encoded, err)
	}
}

func TestClientSendCode(t *testing.T) {
	tests := []struct {
		name        string
		response    providerResponse
		providerErr error
		wantErr     error
	}{
		{name: "accepted", response: providerResponse{code: "OK"}},
		{name: "provider rejected", response: providerResponse{code: "isv.BUSINESS_LIMIT_CONTROL"}, wantErr: ErrProviderRejected},
		{name: "transport failed", providerErr: errors.New("network unavailable")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newClient(Config{signName: "sign", templateCode: "template", timeout: time.Second}, providerFunc(func(ctx context.Context, request providerRequest) (providerResponse, error) {
				if request.phoneNumber != "13800138000" || request.code != "123456" {
					t.Fatalf("unexpected request: %#v", request)
				}
				return tt.response, tt.providerErr
			}))
			err := client.SendCode(context.Background(), "13800138000", "123456")
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("SendCode() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && tt.providerErr == nil && err != nil {
				t.Fatalf("SendCode() error = %v", err)
			}
			if tt.providerErr != nil && !errors.Is(err, tt.providerErr) {
				t.Fatalf("SendCode() error = %v, want wrapped provider error", err)
			}
		})
	}
}

func TestClientSendCodePreservesProviderDiagnostics(t *testing.T) {
	client := newClient(Config{signName: "sign", templateCode: "template", timeout: time.Second}, providerFunc(func(context.Context, providerRequest) (providerResponse, error) {
		return providerResponse{code: "isv.SMS_SIGNATURE_ILLEGAL", message: "signature is invalid", requestID: "request-123"}, nil
	}))

	err := client.SendCode(context.Background(), "13800138000", "123456")
	var providerErr *ProviderError
	if !errors.As(err, &providerErr) {
		t.Fatalf("SendCode() error = %v, want ProviderError", err)
	}
	if providerErr.Code != "isv.SMS_SIGNATURE_ILLEGAL" || providerErr.Message != "signature is invalid" || providerErr.RequestID != "request-123" {
		t.Fatalf("provider error = %#v", providerErr)
	}
	if strings.Contains(err.Error(), "13800138000") || strings.Contains(err.Error(), "123456") {
		t.Fatalf("provider error leaked request data: %v", err)
	}
}

func TestAliyunProviderUsesTypedSendSmsAPI(t *testing.T) {
	fake := &fakeAliyunClient{result: &dysmsapi20170525.SendSmsResponse{Body: &dysmsapi20170525.SendSmsResponseBody{
		Code: dara.String("OK"), Message: dara.String("OK"), RequestId: dara.String("request-123"),
	}}}
	provider := &aliyunProvider{client: fake}

	response, err := provider.Send(context.Background(), providerRequest{
		phoneNumber: "13800138000", signName: "sign", templateCode: "SMS_123", code: "654321",
	})
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if response.code != "OK" || response.requestID != "request-123" {
		t.Fatalf("response = %#v", response)
	}
	if fake.request == nil || dara.StringValue(fake.request.PhoneNumbers) != "13800138000" || dara.StringValue(fake.request.SignName) != "sign" || dara.StringValue(fake.request.TemplateCode) != "SMS_123" || dara.StringValue(fake.request.TemplateParam) != `{"code":"654321"}` {
		t.Fatalf("request = %#v", fake.request)
	}
	if fake.runtime == nil || dara.IntValue(fake.runtime.ConnectTimeout) <= 0 || dara.IntValue(fake.runtime.ReadTimeout) <= 0 {
		t.Fatalf("runtime = %#v", fake.runtime)
	}
}

func TestAliyunProviderRejectsEmptyResponse(t *testing.T) {
	provider := &aliyunProvider{client: &fakeAliyunClient{result: &dysmsapi20170525.SendSmsResponse{}}}
	if _, err := provider.Send(context.Background(), providerRequest{}); err == nil {
		t.Fatal("Send() accepted an empty provider response")
	}
}

func TestClientSendCodeAppliesTimeout(t *testing.T) {
	client := newClient(Config{signName: "sign", templateCode: "template", timeout: time.Millisecond}, providerFunc(func(ctx context.Context, _ providerRequest) (providerResponse, error) {
		<-ctx.Done()
		return providerResponse{}, ctx.Err()
	}))

	err := client.SendCode(context.Background(), "13800138000", "123456")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("SendCode() error = %v, want deadline exceeded", err)
	}
}
