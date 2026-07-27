package caiyun

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestRequestBuilderConstructsSignedV26Request(t *testing.T) {
	builder := fixedRequestBuilder(t)
	request, err := builder.NewWeatherRequest(context.Background(), WeatherRequest{
		Longitude: 121.4551234, Latitude: 31.2285678,
		HourlySteps: 360, DailySteps: 15, Alert: true, Unit: "metric:v2",
	})
	if err != nil {
		path := builder.weatherBaseURL.JoinPath("v2.6", builder.signer.appKey, "121.4551234,31.2285678", "weather").EscapedPath()
		t.Fatalf("NewWeatherRequest() error=%v path=%q validPath=%t validNonce=%t", err, path, validEscapedPath(path), validNonce(testNonce))
	}
	if request.Method != http.MethodGet || request.URL.Scheme != "https" || request.URL.Host != "api.caiyun.invalid" {
		t.Fatalf("request URL=%s method=%s", request.URL, request.Method)
	}
	if request.URL.EscapedPath() != "/v2.6/test_app_key_2026/121.4551234,31.2285678/weather" {
		t.Fatalf("path=%q", request.URL.EscapedPath())
	}
	if request.URL.RawQuery != "alert=true&dailysteps=15&hourlysteps=360&unit=metric%3Av2" {
		t.Fatalf("query=%q", request.URL.RawQuery)
	}
	if request.Header.Get("x-cy-app-key") != "" {
		t.Fatalf("v2.6 app key header=%q", request.Header.Get("x-cy-app-key"))
	}
	assertFixedSigningHeaders(t, request, "jW2s-OXHx_tLmeu4uMRnfWqUeHvw1mbDWOjYFK9Yvbk=")
}

func TestRequestBuilderRejectsInvalidBoundaries(t *testing.T) {
	builder := fixedRequestBuilder(t)
	weatherRequests := []WeatherRequest{
		{Longitude: math.NaN(), Latitude: 31, HourlySteps: 1, DailySteps: 1, Unit: "metric:v2"},
		{Longitude: 181, Latitude: 31, HourlySteps: 1, DailySteps: 1, Unit: "metric:v2"},
		{Longitude: 121, Latitude: 91, HourlySteps: 1, DailySteps: 1, Unit: "metric:v2"},
		{Longitude: 121, Latitude: 31, HourlySteps: 0, DailySteps: 1, Unit: "metric:v2"},
		{Longitude: 121, Latitude: 31, HourlySteps: 361, DailySteps: 1, Unit: "metric:v2"},
		{Longitude: 121, Latitude: 31, HourlySteps: 1, DailySteps: 16, Unit: "metric:v2"},
		{Longitude: 121, Latitude: 31, HourlySteps: 1, DailySteps: 1, Unit: "metric:v2&token=bad"},
	}
	for index, input := range weatherRequests {
		if _, err := builder.NewWeatherRequest(context.Background(), input); err == nil {
			t.Fatalf("weather request %d error=nil", index)
		}
	}
}

func TestRequestBuilderRejectsUnsafeConfigurationAndNonce(t *testing.T) {
	for _, baseURL := range []string{"http://api.caiyun.invalid", "https://user:pass@api.caiyun.invalid", "https://api.caiyun.invalid?token=value", "not-a-url"} {
		if _, err := NewRequestBuilder(baseURL, testAppKey, testAppSecret); err == nil {
			t.Fatalf("NewRequestBuilder(%q) error=nil", baseURL)
		}
	}
	builder, err := newRequestBuilder(
		"https://api.caiyun.invalid", testAppKey, testAppSecret,
		func() time.Time { return time.Unix(testTimestamp, 0) },
		func() (string, error) { return "bad\r\nInjected: true", nil },
	)
	if err != nil {
		t.Fatalf("newRequestBuilder() error=%v", err)
	}
	_, err = builder.NewWeatherRequest(context.Background(), WeatherRequest{
		Longitude: 121, Latitude: 31, HourlySteps: 1, DailySteps: 1, Unit: "metric:v2",
	})
	if err == nil || strings.Contains(err.Error(), "Injected") {
		t.Fatalf("NewWeatherRequest() error=%v", err)
	}
}

func TestRequestBuilderCredentialsCannotBeFormattedOrSerialized(t *testing.T) {
	builder := fixedRequestBuilder(t)
	for _, rendered := range []string{fmt.Sprint(builder), fmt.Sprintf("%+v", builder), fmt.Sprintf("%#v", builder)} {
		if strings.Contains(rendered, testAppKey) || strings.Contains(rendered, testAppSecret) {
			t.Fatalf("formatted builder leaked credentials: %s", rendered)
		}
	}
	encoded, err := json.Marshal(builder)
	if err != nil {
		t.Fatalf("json.Marshal() error=%v", err)
	}
	if strings.Contains(string(encoded), testAppKey) || strings.Contains(string(encoded), testAppSecret) {
		t.Fatalf("serialized builder leaked credentials: %s", encoded)
	}
}

func TestSecureNonceIsUniqueAndWithinContract(t *testing.T) {
	seen := make(map[string]struct{}, 32)
	for index := 0; index < 32; index++ {
		nonce, err := secureNonce()
		if err != nil {
			t.Fatalf("secureNonce() error=%v", err)
		}
		if !validNonce(nonce) {
			t.Fatalf("nonce=%q", nonce)
		}
		if _, exists := seen[nonce]; exists {
			t.Fatalf("duplicate nonce=%q", nonce)
		}
		seen[nonce] = struct{}{}
	}
}

func fixedRequestBuilder(t *testing.T) *RequestBuilder {
	t.Helper()
	builder, err := newRequestBuilder(
		"https://api.caiyun.invalid", testAppKey, testAppSecret,
		func() time.Time { return time.Unix(testTimestamp, 0) },
		func() (string, error) { return testNonce, nil },
	)
	if err != nil {
		t.Fatalf("newRequestBuilder() error=%v", err)
	}
	return builder
}

func assertFixedSigningHeaders(t *testing.T, request *http.Request, expectedSignature string) {
	t.Helper()
	if request.Header.Get("Accept") != "application/json" || request.Header.Get("x-cy-nonce") != testNonce ||
		request.Header.Get("x-cy-timestamp") != "1784688000" || request.Header.Get("x-cy-signature") != expectedSignature {
		t.Fatalf("headers=%#v", request.Header)
	}
}
