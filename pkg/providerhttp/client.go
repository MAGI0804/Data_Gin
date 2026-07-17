// Package providerhttp contains shared, credential-safe HTTP infrastructure
// for external weather-platform providers.
package providerhttp

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	defaultTimeout               = 10 * time.Second
	defaultMaxIdleConns          = 100
	defaultMaxIdleConnsPerHost   = 20
	defaultIdleConnTimeout       = 90 * time.Second
	defaultTLSHandshakeTimeout   = 5 * time.Second
	defaultResponseHeaderTimeout = 10 * time.Second
	maxTraceIDLength             = 128
)

type ClientConfig struct {
	Timeout               time.Duration
	MaxIdleConns          int
	MaxIdleConnsPerHost   int
	IdleConnTimeout       time.Duration
	TLSHandshakeTimeout   time.Duration
	ResponseHeaderTimeout time.Duration
}

func NewClient(cfg ClientConfig) (*http.Client, error) {
	if err := applyClientDefaults(&cfg); err != nil {
		return nil, err
	}
	defaultTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok || defaultTransport == nil {
		return nil, fmt.Errorf("provider http: default transport is unavailable")
	}
	transport := defaultTransport.Clone()
	transport.MaxIdleConns = cfg.MaxIdleConns
	transport.MaxIdleConnsPerHost = cfg.MaxIdleConnsPerHost
	transport.IdleConnTimeout = cfg.IdleConnTimeout
	transport.TLSHandshakeTimeout = cfg.TLSHandshakeTimeout
	transport.ResponseHeaderTimeout = cfg.ResponseHeaderTimeout
	transport.ForceAttemptHTTP2 = true

	return &http.Client{
		Timeout:   cfg.Timeout,
		Transport: traceTransport{base: transport},
	}, nil
}

func applyClientDefaults(cfg *ClientConfig) error {
	if cfg.Timeout < 0 || cfg.MaxIdleConns < 0 || cfg.MaxIdleConnsPerHost < 0 ||
		cfg.IdleConnTimeout < 0 || cfg.TLSHandshakeTimeout < 0 || cfg.ResponseHeaderTimeout < 0 {
		return fmt.Errorf("provider http: client settings must not be negative")
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = defaultTimeout
	}
	if cfg.MaxIdleConns == 0 {
		cfg.MaxIdleConns = defaultMaxIdleConns
	}
	if cfg.MaxIdleConnsPerHost == 0 {
		cfg.MaxIdleConnsPerHost = defaultMaxIdleConnsPerHost
	}
	if cfg.MaxIdleConnsPerHost > cfg.MaxIdleConns {
		return fmt.Errorf("provider http: per-host idle connections exceed total idle connections")
	}
	if cfg.IdleConnTimeout == 0 {
		cfg.IdleConnTimeout = defaultIdleConnTimeout
	}
	if cfg.TLSHandshakeTimeout == 0 {
		cfg.TLSHandshakeTimeout = defaultTLSHandshakeTimeout
	}
	if cfg.ResponseHeaderTimeout == 0 {
		cfg.ResponseHeaderTimeout = defaultResponseHeaderTimeout
	}
	return nil
}

type traceIDContextKey struct{}

func WithTraceID(ctx context.Context, traceID string) (context.Context, error) {
	if ctx == nil {
		return nil, fmt.Errorf("provider http: context is required")
	}
	if !validTraceID(traceID) {
		return nil, fmt.Errorf("provider http: trace id contains invalid characters or length")
	}
	return context.WithValue(ctx, traceIDContextKey{}, traceID), nil
}

func TraceIDFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	traceID, ok := ctx.Value(traceIDContextKey{}).(string)
	return traceID, ok && validTraceID(traceID)
}

func validTraceID(traceID string) bool {
	if traceID == "" || len(traceID) > maxTraceIDLength {
		return false
	}
	for _, character := range traceID {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || strings.ContainsRune("-_.:", character) {
			continue
		}
		return false
	}
	return true
}

type traceTransport struct {
	base http.RoundTripper
}

func (transport traceTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	traceID, ok := TraceIDFromContext(request.Context())
	if !ok || request.Header.Get("X-Request-ID") != "" {
		return transport.base.RoundTrip(request)
	}
	request = request.Clone(request.Context())
	request.Header.Set("X-Request-ID", traceID)
	return transport.base.RoundTrip(request)
}

func (transport traceTransport) CloseIdleConnections() {
	if closer, ok := transport.base.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
}

type ErrorClass string

const (
	ErrorClassNone        ErrorClass = "none"
	ErrorClassCanceled    ErrorClass = "canceled"
	ErrorClassTimeout     ErrorClass = "timeout"
	ErrorClassDNS         ErrorClass = "dns"
	ErrorClassTLS         ErrorClass = "tls"
	ErrorClassTransport   ErrorClass = "transport"
	ErrorClassRateLimited ErrorClass = "rate_limited"
	ErrorClassAuth        ErrorClass = "authentication"
	ErrorClassRequest     ErrorClass = "invalid_request"
	ErrorClassResponse    ErrorClass = "invalid_response"
	ErrorClassProvider    ErrorClass = "provider_unavailable"
)

type RetryClassification struct {
	Class     ErrorClass
	Retryable bool
}

// ClassifyRetry determines whether a worker may safely ask Asynq to retry.
// It does not perform retries, keeping the retry budget at the task layer.
func ClassifyRetry(statusCode int, err error) RetryClassification {
	if err != nil {
		return classifyTransportError(err)
	}
	switch {
	case statusCode == 0:
		return RetryClassification{Class: ErrorClassNone}
	case statusCode == http.StatusTooManyRequests:
		return RetryClassification{Class: ErrorClassRateLimited, Retryable: true}
	case statusCode == http.StatusRequestTimeout:
		return RetryClassification{Class: ErrorClassTimeout, Retryable: true}
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		return RetryClassification{Class: ErrorClassAuth}
	case statusCode >= http.StatusInternalServerError:
		return RetryClassification{Class: ErrorClassProvider, Retryable: true}
	case statusCode >= http.StatusBadRequest:
		return RetryClassification{Class: ErrorClassRequest}
	default:
		return RetryClassification{Class: ErrorClassNone}
	}
}

func classifyTransportError(err error) RetryClassification {
	if errors.Is(err, context.Canceled) {
		return RetryClassification{Class: ErrorClassCanceled}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return RetryClassification{Class: ErrorClassTimeout, Retryable: true}
	}

	var dnsError *net.DNSError
	if errors.As(err, &dnsError) {
		return RetryClassification{Class: ErrorClassDNS, Retryable: dnsError.IsTimeout || dnsError.IsTemporary}
	}
	var certificateInvalidError x509.CertificateInvalidError
	var hostnameError x509.HostnameError
	var unknownAuthorityError x509.UnknownAuthorityError
	if errors.As(err, &certificateInvalidError) || errors.As(err, &hostnameError) || errors.As(err, &unknownAuthorityError) {
		return RetryClassification{Class: ErrorClassTLS}
	}
	var networkError net.Error
	if errors.As(err, &networkError) {
		return RetryClassification{Class: ErrorClassTransport, Retryable: true}
	}
	return RetryClassification{Class: ErrorClassTransport, Retryable: true}
}
