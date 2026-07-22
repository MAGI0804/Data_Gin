package feishu

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gin-biz-web-api/pkg/providerhttp"
)

func TestTokenClientFetchesValidatedTenantToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != tenantTokenPath ||
			!strings.HasPrefix(request.Header.Get("Content-Type"), "application/json") {
			t.Fatalf("request method=%s path=%s headers=%v", request.Method, request.URL.Path, request.Header)
		}
		var payload map[string]string
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil || payload["app_id"] != "cli_app" ||
			payload["app_secret"] != "private-secret" || len(payload) != 2 {
			t.Fatalf("payload=%v error=%v", payload, err)
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"code":0,"msg":"ok","tenant_access_token":"tenant-token-value-123","expire":7200}`))
	}))
	defer server.Close()
	client, err := newTokenClient(server.URL, "cli_app", "private-secret", server.Client(), true)
	if err != nil {
		t.Fatalf("newTokenClient() error=%v", err)
	}
	token, err := client.Fetch(t.Context())
	if err != nil {
		t.Fatalf("Fetch() error=%v", err)
	}
	if token.Value != "tenant-token-value-123" || token.ExpiresIn != 2*time.Hour ||
		strings.Contains(fmt.Sprintf("%v %#v", token, token), token.Value) {
		t.Fatalf("token=%v expires=%v formatted=%v %#v", token.Value, token.ExpiresIn, token, token)
	}
	encoded, err := json.Marshal(token)
	if err != nil || strings.Contains(string(encoded), token.Value) {
		t.Fatalf("encoded token=%s error=%v", encoded, err)
	}
}

func TestTokenClientClassifiesSafeFailuresWithoutLeakingResponse(t *testing.T) {
	secretToken := "tenant-token-must-not-leak"
	tests := []struct {
		name      string
		status    int
		body      string
		class     providerhttp.ErrorClass
		retryable bool
	}{
		{name: "provider failure", status: http.StatusServiceUnavailable, body: `{"token":"` + secretToken + `"}`, class: providerhttp.ErrorClassProvider, retryable: true},
		{name: "business auth failure", status: http.StatusOK, body: `{"code":10003,"msg":"bad ` + secretToken + `"}`, class: providerhttp.ErrorClassAuth},
		{name: "invalid success", status: http.StatusOK, body: `{"code":0,"tenant_access_token":"short","expire":7200}`, class: providerhttp.ErrorClassResponse},
		{name: "trailing data", status: http.StatusOK, body: `{"code":0,"tenant_access_token":"tenant-token-value-123","expire":7200}{}`, class: providerhttp.ErrorClassResponse},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
				response.WriteHeader(test.status)
				_, _ = response.Write([]byte(test.body))
			}))
			defer server.Close()
			client, err := newTokenClient(server.URL, "cli_app", "private-secret", server.Client(), true)
			if err != nil {
				t.Fatalf("newTokenClient() error=%v", err)
			}
			_, err = client.Fetch(t.Context())
			var tokenError *TokenError
			if !errors.As(err, &tokenError) || tokenError.Class != test.class || tokenError.Retryable != test.retryable ||
				strings.Contains(fmt.Sprintf("%v", err), secretToken) {
				t.Fatalf("Fetch() error=%v tokenError=%+v", err, tokenError)
			}
		})
	}
}

func TestTokenClientRejectsOversizedResponseAndUnsafeConfiguration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte(strings.Repeat("x", maxTenantTokenBodyBytes+1)))
	}))
	defer server.Close()
	client, err := newTokenClient(server.URL, "cli_app", "private-secret", server.Client(), true)
	if err != nil {
		t.Fatalf("newTokenClient() error=%v", err)
	}
	if _, err := client.Fetch(t.Context()); err == nil {
		t.Fatal("Fetch() accepted oversized response")
	}
	if _, err := newTokenClient("http://example.com", "cli_app", "private-secret", nil, true); err == nil {
		t.Fatal("newTokenClient() accepted insecure remote endpoint")
	}
	if _, err := NewTokenClient("cli_app", " secret", nil); err == nil {
		t.Fatal("NewTokenClient() accepted unsafe credential")
	}
}
