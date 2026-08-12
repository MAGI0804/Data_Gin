package sms

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

type providerFunc func(context.Context, providerRequest) (providerResponse, error)

func (f providerFunc) Send(ctx context.Context, request providerRequest) (providerResponse, error) {
	return f(ctx, request)
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
