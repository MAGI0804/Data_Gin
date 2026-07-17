package geocoder

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"gin-biz-web-api/pkg/providerhttp"
)

const (
	defaultAmapTimeout          = 5 * time.Second
	defaultAmapMaxResponseBytes = int64(2 << 20)
	maxAmapAddressRunes         = 1000
	maxAmapCityRunes            = 128
)

type HTTPDoer interface {
	Do(request *http.Request) (*http.Response, error)
}

type AmapClient struct {
	endpoint         *url.URL
	apiKey           string
	httpClient       HTTPDoer
	timeout          time.Duration
	maxResponseBytes int64
}

var _ Geocoder = (*AmapClient)(nil)

func NewAmapClient(baseURL, apiKey string, httpClient HTTPDoer) (*AmapClient, error) {
	endpoint, err := amapEndpoint(baseURL)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("amap client: api key is required")
	}
	if httpClient == nil {
		httpClient, err = providerhttp.NewClient(providerhttp.ClientConfig{
			Timeout:               defaultAmapTimeout,
			ResponseHeaderTimeout: defaultAmapTimeout,
		})
		if err != nil {
			return nil, fmt.Errorf("amap client: create HTTP client: %w", err)
		}
	}
	return &AmapClient{
		endpoint:         endpoint,
		apiKey:           apiKey,
		httpClient:       httpClient,
		timeout:          defaultAmapTimeout,
		maxResponseBytes: defaultAmapMaxResponseBytes,
	}, nil
}

func (AmapClient) String() string   { return "geocoder.AmapClient{redacted}" }
func (AmapClient) GoString() string { return "geocoder.AmapClient{redacted}" }
func (AmapClient) MarshalJSON() ([]byte, error) {
	return []byte("{}"), nil
}

func (client *AmapClient) Geocode(ctx context.Context, request Request) (*Response, error) {
	if client == nil || client.endpoint == nil || client.httpClient == nil {
		return nil, &ProviderError{Class: providerhttp.ErrorClassRequest}
	}
	request.Address = strings.TrimSpace(request.Address)
	request.City = strings.TrimSpace(request.City)
	if request.Address == "" || utf8.RuneCountInString(request.Address) > maxAmapAddressRunes || utf8.RuneCountInString(request.City) > maxAmapCityRunes {
		return nil, &ProviderError{Class: providerhttp.ErrorClassRequest}
	}

	requestURL := *client.endpoint
	query := requestURL.Query()
	query.Set("key", client.apiKey)
	query.Set("address", request.Address)
	if request.City != "" {
		query.Set("city", request.City)
	}
	query.Set("output", "JSON")
	requestURL.RawQuery = query.Encode()

	requestCtx, cancel := context.WithTimeout(ctx, client.timeout)
	defer cancel()
	httpRequest, err := http.NewRequestWithContext(requestCtx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return nil, &ProviderError{Class: providerhttp.ErrorClassRequest}
	}
	httpRequest.Header.Set("Accept", "application/json")
	if traceID, ok := providerhttp.TraceIDFromContext(requestCtx); ok {
		httpRequest.Header.Set("X-Request-ID", traceID)
	}

	httpResponse, err := client.httpClient.Do(httpRequest)
	if err != nil {
		classification := providerhttp.ClassifyRetry(0, err)
		return nil, &ProviderError{Class: classification.Class, Retryable: classification.Retryable, cause: safeProviderCause(err)}
	}
	body, readErr := io.ReadAll(io.LimitReader(httpResponse.Body, client.maxResponseBytes+1))
	closeErr := httpResponse.Body.Close()
	if readErr != nil {
		classification := providerhttp.ClassifyRetry(0, readErr)
		return nil, &ProviderError{Class: classification.Class, Retryable: classification.Retryable, cause: safeProviderCause(readErr)}
	}
	if closeErr != nil {
		classification := providerhttp.ClassifyRetry(0, closeErr)
		return nil, &ProviderError{Class: classification.Class, Retryable: classification.Retryable, cause: safeProviderCause(closeErr)}
	}
	if int64(len(body)) > client.maxResponseBytes {
		return nil, &ProviderError{Class: providerhttp.ErrorClassResponse}
	}

	// A compromised or misbehaving provider must not be able to reflect the
	// request credential into the raw snapshot.
	safeBody, jsonErr := providerhttp.RedactJSON(body, client.apiKey)
	if jsonErr != nil {
		safeBody = []byte(providerhttp.RedactText(string(body), client.apiKey))
	}
	response := &Response{RawJSON: append(json.RawMessage(nil), safeBody...)}
	if httpResponse.StatusCode < http.StatusOK || httpResponse.StatusCode >= http.StatusMultipleChoices {
		classification := providerhttp.ClassifyRetry(httpResponse.StatusCode, nil)
		return response, &ProviderError{
			Class:      classification.Class,
			HTTPStatus: httpResponse.StatusCode,
			Retryable:  classification.Retryable,
		}
	}
	if jsonErr != nil {
		return response, &ProviderError{Class: providerhttp.ErrorClassResponse}
	}

	var payload amapResponse
	if err := json.Unmarshal(safeBody, &payload); err != nil {
		return response, &ProviderError{Class: providerhttp.ErrorClassResponse}
	}
	response.ProviderStatus = string(payload.Status)
	response.Info = string(payload.Info)
	response.Infocode = string(payload.Infocode)
	if response.ProviderStatus == "" || payload.Count == "" || response.Info == "" || len(response.Info) > 255 || !validAmapInfocode(response.Infocode) {
		return response, &ProviderError{Class: providerhttp.ErrorClassResponse}
	}
	response.Count = len(payload.Geocodes)
	count, err := strconv.Atoi(string(payload.Count))
	if err != nil || count < 0 {
		return response, &ProviderError{Class: providerhttp.ErrorClassResponse}
	}
	response.Count = count

	if response.ProviderStatus != "1" {
		classification := classifyAmapInfocode(response.Infocode)
		return response, &ProviderError{
			Class:     classification.Class,
			Code:      response.Infocode,
			Retryable: classification.Retryable,
		}
	}
	if response.Infocode != "10000" {
		return response, &ProviderError{Class: providerhttp.ErrorClassResponse, Code: response.Infocode}
	}
	if response.Count != len(payload.Geocodes) {
		return response, &ProviderError{Class: providerhttp.ErrorClassResponse}
	}

	candidates := make([]Candidate, 0, len(payload.Geocodes))
	for _, geocode := range payload.Geocodes {
		longitude, latitude, err := parseAmapLocation(string(geocode.Location))
		if err != nil {
			return response, &ProviderError{Class: providerhttp.ErrorClassResponse}
		}
		candidates = append(candidates, Candidate{
			FormattedAddress: string(geocode.FormattedAddress),
			Country:          string(geocode.Country),
			Province:         string(geocode.Province),
			City:             string(geocode.City),
			Citycode:         string(geocode.Citycode),
			District:         string(geocode.District),
			Township:         string(geocode.Township),
			Street:           string(geocode.Street),
			StreetNumber:     string(geocode.Number),
			Adcode:           string(geocode.Adcode),
			Longitude:        longitude,
			Latitude:         latitude,
			CoordinateSystem: "GCJ02",
			Level:            string(geocode.Level),
		})
	}
	response.Candidates = candidates
	return response, nil
}

func safeProviderCause(err error) error {
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	return nil
}

func amapEndpoint(rawBaseURL string) (*url.URL, error) {
	baseURL, err := url.Parse(strings.TrimSpace(rawBaseURL))
	if err != nil || baseURL.Host == "" || baseURL.User != nil || baseURL.RawQuery != "" || baseURL.Fragment != "" {
		return nil, fmt.Errorf("amap client: invalid base URL")
	}
	if baseURL.Scheme != "https" {
		return nil, fmt.Errorf("amap client: base URL must use HTTPS")
	}
	return baseURL.JoinPath("v3", "geocode", "geo"), nil
}

func validAmapInfocode(infocode string) bool {
	if len(infocode) == 0 || len(infocode) > 16 {
		return false
	}
	for _, character := range infocode {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func parseAmapLocation(location string) (float64, float64, error) {
	parts := strings.Split(location, ",")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("amap client: invalid location")
	}
	longitude, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil || longitude < -180 || longitude > 180 {
		return 0, 0, fmt.Errorf("amap client: invalid longitude")
	}
	latitude, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil || latitude < -90 || latitude > 90 {
		return 0, 0, fmt.Errorf("amap client: invalid latitude")
	}
	return longitude, latitude, nil
}

func classifyAmapInfocode(infocode string) providerhttp.RetryClassification {
	switch infocode {
	case "10001", "10005", "10006", "10007", "10008", "10009", "10012", "10013":
		return providerhttp.RetryClassification{Class: providerhttp.ErrorClassAuth}
	case "10003", "10004", "10010", "10014", "10019", "10020", "10021", "10029":
		return providerhttp.RetryClassification{Class: providerhttp.ErrorClassRateLimited, Retryable: true}
	case "10002", "10015", "10016", "10017", "10026":
		return providerhttp.RetryClassification{Class: providerhttp.ErrorClassProvider, Retryable: true}
	case "20000", "20001", "20002", "20003", "20800":
		return providerhttp.RetryClassification{Class: providerhttp.ErrorClassRequest}
	default:
		if strings.HasPrefix(infocode, "3") {
			return providerhttp.RetryClassification{Class: providerhttp.ErrorClassProvider, Retryable: true}
		}
		return providerhttp.RetryClassification{Class: providerhttp.ErrorClassProvider}
	}
}

type flexibleString string

func (value *flexibleString) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		*value = flexibleString(text)
		return nil
	}
	var empty []json.RawMessage
	if err := json.Unmarshal(data, &empty); err == nil && len(empty) == 0 {
		*value = ""
		return nil
	}
	if string(data) == "null" {
		*value = ""
		return nil
	}
	return fmt.Errorf("amap client: expected string or empty array")
}

type amapResponse struct {
	Status   flexibleString `json:"status"`
	Count    flexibleString `json:"count"`
	Info     flexibleString `json:"info"`
	Infocode flexibleString `json:"infocode"`
	Geocodes []amapGeocode  `json:"geocodes"`
}

type amapGeocode struct {
	FormattedAddress flexibleString `json:"formatted_address"`
	Country          flexibleString `json:"country"`
	Province         flexibleString `json:"province"`
	Citycode         flexibleString `json:"citycode"`
	City             flexibleString `json:"city"`
	District         flexibleString `json:"district"`
	Township         flexibleString `json:"township"`
	Street           flexibleString `json:"street"`
	Number           flexibleString `json:"number"`
	Adcode           flexibleString `json:"adcode"`
	Location         flexibleString `json:"location"`
	Level            flexibleString `json:"level"`
}
