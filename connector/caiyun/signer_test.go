package caiyun

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

const (
	testAppKey    = "test_app_key_2026"
	testAppSecret = "test_app_secret_2026"
	testNonce     = "550e8400-e29b-41d4-a716-446655440000"
	testTimestamp = int64(1784688000)
)

func TestSignerMatchesFixedVectors(t *testing.T) {
	signer, err := NewSigner(testAppKey, testAppSecret)
	if err != nil {
		t.Fatalf("NewSigner() error=%v", err)
	}
	tests := []struct {
		name     string
		path     string
		query    url.Values
		expected string
	}{
		{
			name: "v2.6 sorted query with encoded colon",
			path: "/v2.6/test_app_key_2026/121.4551234,31.2285678/weather",
			query: url.Values{
				"unit": {"metric:v2"}, "hourlysteps": {"360"}, "alert": {"true"}, "dailysteps": {"15"},
			},
			expected: "jW2s-OXHx_tLmeu4uMRnfWqUeHvw1mbDWOjYFK9Yvbk=",
		},
		{
			name: "v3 negative coordinate",
			path: "/v3/lifeindex",
			query: url.Values{
				"longitude": {"121.4551234"}, "latitude": {"-31.2285678"}, "fields": {"all"}, "days": {"15"},
			},
			expected: "57yLnEA-zpAWn2MYSN6WBcb_U78EgD9OnICjFio6WLk=",
		},
		{
			name:     "no query",
			path:     "/v3/lifeindex",
			query:    nil,
			expected: "pxCnk-A5M6a-D65BxNi521WPFsA39U0URXkWicBLe90=",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			signature, err := signer.Sign(http.MethodGet, test.path, testNonce, testTimestamp, test.query)
			if err != nil {
				t.Fatalf("Sign() error=%v", err)
			}
			if signature != test.expected {
				t.Fatalf("signature=%q want=%q", signature, test.expected)
			}
		})
	}
}

func TestSignerRejectsAmbiguousOrUnsafeInput(t *testing.T) {
	signer, err := NewSigner(testAppKey, testAppSecret)
	if err != nil {
		t.Fatalf("NewSigner() error=%v", err)
	}
	tests := []struct {
		name      string
		method    string
		path      string
		nonce     string
		timestamp int64
	}{
		{name: "non GET", method: http.MethodPost, path: "/v3/lifeindex", nonce: testNonce, timestamp: testTimestamp},
		{name: "query in path", method: http.MethodGet, path: "/v3/lifeindex?token=value", nonce: testNonce, timestamp: testTimestamp},
		{name: "short nonce", method: http.MethodGet, path: "/v3/lifeindex", nonce: "too-short", timestamp: testTimestamp},
		{name: "header injection nonce", method: http.MethodGet, path: "/v3/lifeindex", nonce: "0123456789abcdef\r\n", timestamp: testTimestamp},
		{name: "invalid timestamp", method: http.MethodGet, path: "/v3/lifeindex", nonce: testNonce, timestamp: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := signer.Sign(test.method, test.path, test.nonce, test.timestamp, nil); err == nil {
				t.Fatal("Sign() error=nil")
			}
		})
	}
}

func TestSignerCredentialsCannotBeFormattedOrSerialized(t *testing.T) {
	signer, err := NewSigner(testAppKey, testAppSecret)
	if err != nil {
		t.Fatalf("NewSigner() error=%v", err)
	}
	for _, rendered := range []string{fmt.Sprint(signer), fmt.Sprintf("%+v", signer), fmt.Sprintf("%#v", signer)} {
		if strings.Contains(rendered, testAppKey) || strings.Contains(rendered, testAppSecret) {
			t.Fatalf("formatted signer leaked credentials: %s", rendered)
		}
	}
	encoded, err := json.Marshal(signer)
	if err != nil {
		t.Fatalf("json.Marshal() error=%v", err)
	}
	if strings.Contains(string(encoded), testAppKey) || strings.Contains(string(encoded), testAppSecret) {
		t.Fatalf("serialized signer leaked credentials: %s", encoded)
	}
}

func TestNewSignerRejectsMissingOrPathUnsafeCredentials(t *testing.T) {
	for _, credentials := range [][2]string{{"", testAppSecret}, {testAppKey, "  "}, {"key/segment", testAppSecret}, {"key?query", testAppSecret}} {
		if _, err := NewSigner(credentials[0], credentials[1]); err == nil {
			t.Fatalf("NewSigner(%q, redacted) error=nil", credentials[0])
		}
	}
}
