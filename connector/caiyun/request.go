package caiyun

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	maximumHourlySteps = 360
	maximumDailySteps  = 15
	coordinateScale    = 7
)

var unitPattern = regexp.MustCompile(`^[A-Za-z0-9_:-]{1,32}$`)

type WeatherRequest struct {
	Longitude   float64
	Latitude    float64
	HourlySteps int
	DailySteps  int
	Alert       bool
	Unit        string
}

type LifeIndexRequest struct {
	Longitude float64
	Latitude  float64
	Days      int
	Fields    string
}

type RequestBuilder struct {
	weatherBaseURL   *url.URL
	lifeIndexBaseURL *url.URL
	signer           *Signer
	now              func() time.Time
	nonce            func() (string, error)
}

func NewRequestBuilder(weatherBaseURL, lifeIndexBaseURL, appKey, appSecret string) (*RequestBuilder, error) {
	return newRequestBuilder(weatherBaseURL, lifeIndexBaseURL, appKey, appSecret, time.Now, secureNonce)
}

func newRequestBuilder(weatherBaseURL, lifeIndexBaseURL, appKey, appSecret string, now func() time.Time, nonce func() (string, error)) (*RequestBuilder, error) {
	weatherBase, err := parseBaseURL(weatherBaseURL)
	if err != nil {
		return nil, err
	}
	lifeIndexBase, err := parseBaseURL(lifeIndexBaseURL)
	if err != nil {
		return nil, err
	}
	signer, err := NewSigner(appKey, appSecret)
	if err != nil {
		return nil, err
	}
	if now == nil || nonce == nil {
		return nil, fmt.Errorf("caiyun request builder: clock and nonce source are required")
	}
	return &RequestBuilder{
		weatherBaseURL: weatherBase, lifeIndexBaseURL: lifeIndexBase,
		signer: signer, now: now, nonce: nonce,
	}, nil
}

func (RequestBuilder) String() string   { return "caiyun.RequestBuilder{redacted}" }
func (RequestBuilder) GoString() string { return "caiyun.RequestBuilder{redacted}" }
func (RequestBuilder) MarshalJSON() ([]byte, error) {
	return []byte("{}"), nil
}

func (builder *RequestBuilder) NewWeatherRequest(ctx context.Context, input WeatherRequest) (*http.Request, error) {
	if builder == nil || builder.weatherBaseURL == nil || builder.signer == nil {
		return nil, fmt.Errorf("caiyun request builder: not configured")
	}
	if ctx == nil || !validCoordinates(input.Longitude, input.Latitude) || input.HourlySteps < 1 || input.HourlySteps > maximumHourlySteps ||
		input.DailySteps < 1 || input.DailySteps > maximumDailySteps || !unitPattern.MatchString(input.Unit) {
		return nil, fmt.Errorf("caiyun request builder: invalid weather request")
	}

	location := formatCoordinate(input.Longitude) + "," + formatCoordinate(input.Latitude)
	requestURL := builder.weatherBaseURL.JoinPath("v2.6", builder.signer.appKey, location, "weather")
	query := make(url.Values, 4)
	query.Set("alert", strconv.FormatBool(input.Alert))
	query.Set("dailysteps", strconv.Itoa(input.DailySteps))
	query.Set("hourlysteps", strconv.Itoa(input.HourlySteps))
	query.Set("unit", input.Unit)
	return builder.newSignedRequest(ctx, requestURL, query, false)
}

func (builder *RequestBuilder) NewLifeIndexRequest(ctx context.Context, input LifeIndexRequest) (*http.Request, error) {
	if builder == nil || builder.lifeIndexBaseURL == nil || builder.signer == nil {
		return nil, fmt.Errorf("caiyun request builder: not configured")
	}
	input.Fields = strings.TrimSpace(input.Fields)
	if ctx == nil || !validCoordinates(input.Longitude, input.Latitude) || input.Days < 1 || input.Days > maximumDailySteps || input.Fields != "all" {
		return nil, fmt.Errorf("caiyun request builder: invalid life index request")
	}

	requestURL := builder.lifeIndexBaseURL.JoinPath("v3", "lifeindex")
	query := make(url.Values, 4)
	query.Set("days", strconv.Itoa(input.Days))
	query.Set("fields", input.Fields)
	query.Set("latitude", formatCoordinate(input.Latitude))
	query.Set("longitude", formatCoordinate(input.Longitude))
	return builder.newSignedRequest(ctx, requestURL, query, true)
}

func (builder *RequestBuilder) newSignedRequest(ctx context.Context, requestURL *url.URL, query url.Values, includeAppKeyHeader bool) (*http.Request, error) {
	nonce, err := builder.nonce()
	if err != nil || !validNonce(nonce) {
		return nil, fmt.Errorf("caiyun request builder: generate nonce")
	}
	timestamp := builder.now().Unix()
	requestURL.RawQuery = cloneValues(query).Encode()
	signature, err := builder.signer.Sign(http.MethodGet, requestURL.EscapedPath(), nonce, timestamp, query)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("caiyun request builder: create request")
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("x-cy-nonce", nonce)
	request.Header.Set("x-cy-timestamp", strconv.FormatInt(timestamp, 10))
	request.Header.Set("x-cy-signature", signature)
	if includeAppKeyHeader {
		request.Header.Set("x-cy-app-key", builder.signer.appKey)
	}
	return request, nil
}

func parseBaseURL(rawURL string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("caiyun request builder: invalid base URL")
	}
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	return parsed, nil
}

func secureNonce() (string, error) {
	nonce, err := uuid.NewRandom()
	if err != nil {
		return "", fmt.Errorf("caiyun request builder: secure random source unavailable")
	}
	return nonce.String(), nil
}

func validCoordinates(longitude, latitude float64) bool {
	return !math.IsNaN(longitude) && !math.IsInf(longitude, 0) && longitude >= -180 && longitude <= 180 &&
		!math.IsNaN(latitude) && !math.IsInf(latitude, 0) && latitude >= -90 && latitude <= 90
}

func formatCoordinate(value float64) string {
	return strconv.FormatFloat(value, 'f', coordinateScale, 64)
}
