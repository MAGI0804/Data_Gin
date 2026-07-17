package geocoder

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gin-biz-web-api/pkg/providerhttp"
)

const testAmapKey = "amap-test-credential"

func TestAmapClientGeocodeEncodesRequestAndMapsCandidate(t *testing.T) {
	var called bool
	handler := http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		called = true
		if request.URL.Path != "/v3/geocode/geo" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		query := request.URL.Query()
		if query.Get("key") != testAmapKey || query.Get("address") != "上海市黄浦区 A&B 商场" || query.Get("city") != "310000" || query.Get("output") != "JSON" {
			t.Fatalf("query = %#v", query)
		}
		if request.Header.Get("X-Request-ID") != "trace-amap-1" {
			t.Fatalf("X-Request-ID = %q", request.Header.Get("X-Request-ID"))
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{
			"status":"1","count":"1","info":"OK","infocode":"10000",
			"echo":%q,
			"geocodes":[{
				"formatted_address":"上海市黄浦区示例路1号","country":"中国","province":"上海市",
				"city":[],"citycode":"021","district":"黄浦区","township":[],
				"street":"示例路","number":"1号","adcode":"310101",
				"location":"121.473701,31.230416","level":"兴趣点"
			}]
		}`, testAmapKey)
	})

	client := newTestAmapClient(t, handler)
	ctx, err := providerhttp.WithTraceID(context.Background(), "trace-amap-1")
	if err != nil {
		t.Fatalf("WithTraceID() error = %v", err)
	}
	response, err := client.Geocode(ctx, Request{Address: " 上海市黄浦区 A&B 商场 ", City: "310000"})
	if err != nil {
		t.Fatalf("Geocode() error = %v", err)
	}
	if !called || response.Count != 1 || len(response.Candidates) != 1 {
		t.Fatalf("response = %+v", response)
	}
	candidate := response.Candidates[0]
	if candidate.Longitude != 121.473701 || candidate.Latitude != 31.230416 {
		t.Fatalf("coordinates = %f,%f", candidate.Longitude, candidate.Latitude)
	}
	if candidate.CoordinateSystem != "GCJ02" || candidate.City != "" || candidate.Street != "示例路" {
		t.Fatalf("candidate = %+v", candidate)
	}
	if strings.Contains(string(response.RawJSON), testAmapKey) || !json.Valid(response.RawJSON) {
		t.Fatalf("raw response was not safely retained: %s", response.RawJSON)
	}
}

func TestAmapClientReturnsSafeBusinessError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"status":"0","count":"0","info":"INVALID_USER_KEY","infocode":"10001","geocodes":[]}`)
	})
	client := newTestAmapClient(t, handler)

	response, err := client.Geocode(context.Background(), Request{Address: "上海市测试商场"})
	var providerError *ProviderError
	if !errors.As(err, &providerError) {
		t.Fatalf("Geocode() error = %v, want ProviderError", err)
	}
	if providerError.Class != providerhttp.ErrorClassAuth || providerError.Retryable || providerError.Code != "10001" {
		t.Fatalf("ProviderError = %+v", providerError)
	}
	if response == nil || response.Info != "INVALID_USER_KEY" {
		t.Fatalf("response = %+v", response)
	}
	if strings.Contains(err.Error(), "INVALID_USER_KEY") || strings.Contains(err.Error(), testAmapKey) {
		t.Fatalf("error exposed unsafe details: %v", err)
	}
}

func TestAmapClientClassifiesHTTPFailureAndRedactsRawBody(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintf(w, `{"request_url":%q}`, request.URL.String())
	})
	client := newTestAmapClient(t, handler)

	response, err := client.Geocode(context.Background(), Request{Address: "上海市测试商场"})
	var providerError *ProviderError
	if !errors.As(err, &providerError) {
		t.Fatalf("Geocode() error = %v, want ProviderError", err)
	}
	if providerError.Class != providerhttp.ErrorClassProvider || !providerError.Retryable || providerError.HTTPStatus != http.StatusBadGateway {
		t.Fatalf("ProviderError = %+v", providerError)
	}
	if response == nil || strings.Contains(string(response.RawJSON), testAmapKey) {
		t.Fatalf("raw response exposed credential: %s", response.RawJSON)
	}
}

func TestAmapClientRejectsInvalidProviderResponse(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"malformed JSON", `not-json`},
		{"missing status", `{"count":"0","infocode":"10000","geocodes":[]}`},
		{"missing count", `{"status":"1","info":"OK","infocode":"10000","geocodes":[]}`},
		{"unsafe infocode", "{\"status\":\"0\",\"count\":\"0\",\"info\":\"bad\",\"infocode\":\"10001\\nInjected\",\"geocodes\":[]}"},
		{"count mismatch", `{"status":"1","count":"2","info":"OK","infocode":"10000","geocodes":[]}`},
		{"invalid longitude", `{"status":"1","count":"1","info":"OK","infocode":"10000","geocodes":[{"location":"181,31"}]}`},
		{"unexpected field type", `{"status":"1","count":"1","info":"OK","infocode":"10000","geocodes":[{"location":["121,31"]}]}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				fmt.Fprint(w, tt.body)
			})
			client := newTestAmapClient(t, handler)

			_, err := client.Geocode(context.Background(), Request{Address: "上海市测试商场"})
			var providerError *ProviderError
			if !errors.As(err, &providerError) || providerError.Class != providerhttp.ErrorClassResponse || providerError.Retryable {
				t.Fatalf("Geocode() error = %+v", err)
			}
		})
	}
}

func TestAmapClientDoesNotReturnPartialCandidates(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{
			"status":"1","count":"2","info":"OK","infocode":"10000",
			"geocodes":[
				{"location":"121.1,31.1"},
				{"location":"invalid"}
			]
		}`)
	})
	client := newTestAmapClient(t, handler)

	response, err := client.Geocode(context.Background(), Request{Address: "上海市测试商场"})
	var providerError *ProviderError
	if !errors.As(err, &providerError) || providerError.Class != providerhttp.ErrorClassResponse {
		t.Fatalf("Geocode() error = %v", err)
	}
	if response == nil || len(response.Candidates) != 0 {
		t.Fatalf("partial candidates escaped validation: %+v", response)
	}
}

func TestAmapClientTransportErrorDoesNotExposeRequestCredential(t *testing.T) {
	client, err := NewAmapClient("https://restapi.amap.com", testAmapKey, leakingCanceledDoer{})
	if err != nil {
		t.Fatalf("NewAmapClient() error = %v", err)
	}
	_, err = client.Geocode(context.Background(), Request{Address: "上海市测试商场"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Geocode() error = %v, want context.Canceled", err)
	}
	if strings.Contains(err.Error(), testAmapKey) || strings.Contains(fmt.Sprintf("%+v", err), testAmapKey) {
		t.Fatalf("error exposed credential: %v", err)
	}
}

func TestAmapClientRejectsOversizedResponse(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, strings.Repeat("x", 32))
	})
	client := newTestAmapClient(t, handler)
	client.maxResponseBytes = 16

	_, err := client.Geocode(context.Background(), Request{Address: "上海市测试商场"})
	var providerError *ProviderError
	if !errors.As(err, &providerError) || providerError.Class != providerhttp.ErrorClassResponse {
		t.Fatalf("Geocode() error = %v", err)
	}
}

func TestNewAmapClientValidatesConfigurationAndRedactsFormatting(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		apiKey  string
	}{
		{"remote HTTP", "http://restapi.amap.com", testAmapKey},
		{"URL credentials", "https://user:password@restapi.amap.com", testAmapKey},
		{"URL query", "https://restapi.amap.com?key=value", testAmapKey},
		{"missing key", "https://restapi.amap.com", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewAmapClient(tt.baseURL, tt.apiKey, http.DefaultClient); err == nil {
				t.Fatal("NewAmapClient() error = nil")
			}
		})
	}

	client, err := NewAmapClient("https://restapi.amap.com", testAmapKey, http.DefaultClient)
	if err != nil {
		t.Fatalf("NewAmapClient() error = %v", err)
	}
	for _, rendered := range []string{fmt.Sprint(client), fmt.Sprintf("%+v", client), fmt.Sprintf("%#v", client)} {
		if strings.Contains(rendered, testAmapKey) {
			t.Fatalf("formatted client exposed credential: %s", rendered)
		}
	}
	encoded, err := json.Marshal(client)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if strings.Contains(string(encoded), testAmapKey) {
		t.Fatalf("JSON exposed credential: %s", encoded)
	}
}

func TestClassifyAmapInfocode(t *testing.T) {
	tests := []struct {
		code      string
		class     providerhttp.ErrorClass
		retryable bool
	}{
		{"10001", providerhttp.ErrorClassAuth, false},
		{"10014", providerhttp.ErrorClassRateLimited, true},
		{"10016", providerhttp.ErrorClassProvider, true},
		{"20001", providerhttp.ErrorClassRequest, false},
		{"30000", providerhttp.ErrorClassProvider, true},
		{"unknown", providerhttp.ErrorClassProvider, false},
	}
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			got := classifyAmapInfocode(tt.code)
			if got.Class != tt.class || got.Retryable != tt.retryable {
				t.Fatalf("classifyAmapInfocode() = %+v", got)
			}
		})
	}
}

type leakingCanceledDoer struct{}

func (leakingCanceledDoer) Do(request *http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("send %s: %w", request.URL.String(), context.Canceled)
}

type handlerDoer struct {
	handler http.Handler
}

func (doer handlerDoer) Do(request *http.Request) (*http.Response, error) {
	recorder := httptest.NewRecorder()
	doer.handler.ServeHTTP(recorder, request)
	return recorder.Result(), nil
}

func newTestAmapClient(t *testing.T, handler http.Handler) *AmapClient {
	t.Helper()
	client, err := NewAmapClient("https://127.0.0.1", testAmapKey, handlerDoer{handler: handler})
	if err != nil {
		t.Fatalf("NewAmapClient() error = %v", err)
	}
	return client
}
