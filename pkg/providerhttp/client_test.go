package providerhttp

import (
	"context"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestNewClientConfiguresPoolAndTraceTransport(t *testing.T) {
	client, err := NewClient(ClientConfig{
		Timeout:               2 * time.Second,
		MaxIdleConns:          40,
		MaxIdleConnsPerHost:   8,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   3 * time.Second,
		ResponseHeaderTimeout: 4 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if client.Timeout != 2*time.Second {
		t.Fatalf("Timeout = %v, want 2s", client.Timeout)
	}
	trace, ok := client.Transport.(traceTransport)
	if !ok {
		t.Fatalf("Transport = %T, want traceTransport", client.Transport)
	}
	transport, ok := trace.base.(*http.Transport)
	if !ok {
		t.Fatalf("base transport = %T, want *http.Transport", trace.base)
	}
	if transport.MaxIdleConns != 40 || transport.MaxIdleConnsPerHost != 8 || transport.IdleConnTimeout != 30*time.Second {
		t.Fatalf("unexpected pool config: %+v", transport)
	}
}

func TestNewClientRejectsInvalidPoolConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  ClientConfig
	}{
		{"negative timeout", ClientConfig{Timeout: -time.Second}},
		{"per-host exceeds total", ClientConfig{MaxIdleConns: 2, MaxIdleConnsPerHost: 3}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewClient(tt.cfg); err == nil {
				t.Fatal("NewClient() error = nil")
			}
		})
	}
}

func TestClientPropagatesTraceID(t *testing.T) {
	var received string
	base := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		received = request.Header.Get("X-Request-ID")
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    request,
		}, nil
	})
	client := &http.Client{Transport: traceTransport{base: base}}
	ctx, err := WithTraceID(context.Background(), "trace-20260717")
	if err != nil {
		t.Fatalf("WithTraceID() error = %v", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://provider.invalid/resource", nil)
	if err != nil {
		t.Fatalf("NewRequestWithContext() error = %v", err)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	if err := response.Body.Close(); err != nil {
		t.Fatalf("close response body: %v", err)
	}
	if received != "trace-20260717" {
		t.Fatalf("X-Request-ID = %q", received)
	}
}

type roundTripFunc func(request *http.Request) (*http.Response, error)

func (roundTrip roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

func TestWithTraceIDRejectsHeaderInjection(t *testing.T) {
	for _, traceID := range []string{"", "bad\r\nInjected: true", strings.Repeat("a", maxTraceIDLength+1)} {
		if _, err := WithTraceID(context.Background(), traceID); err == nil {
			t.Fatalf("WithTraceID(%q) error = nil", traceID)
		}
	}
}

func TestTraceTransportClosesUnderlyingIdleConnections(t *testing.T) {
	base := &closeTrackingTransport{}
	traceTransport{base: base}.CloseIdleConnections()
	if !base.closed {
		t.Fatal("CloseIdleConnections() did not reach the base transport")
	}
}

type closeTrackingTransport struct {
	closed bool
}

func (*closeTrackingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("not implemented")
}

func (transport *closeTrackingTransport) CloseIdleConnections() {
	transport.closed = true
}

func TestClassifyRetry(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		err       error
		class     ErrorClass
		retryable bool
	}{
		{"success", http.StatusOK, nil, ErrorClassNone, false},
		{"rate limited", http.StatusTooManyRequests, nil, ErrorClassRateLimited, true},
		{"server error", http.StatusBadGateway, nil, ErrorClassProvider, true},
		{"authentication", http.StatusUnauthorized, nil, ErrorClassAuth, false},
		{"bad request", http.StatusBadRequest, nil, ErrorClassRequest, false},
		{"canceled", 0, context.Canceled, ErrorClassCanceled, false},
		{"deadline", 0, context.DeadlineExceeded, ErrorClassTimeout, true},
		{"DNS not found", 0, &net.DNSError{Err: "no such host", Name: "provider.invalid", IsNotFound: true}, ErrorClassDNS, false},
		{"TLS authority", 0, x509.UnknownAuthorityError{}, ErrorClassTLS, false},
		{"transport", 0, errors.New("connection reset"), ErrorClassTransport, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyRetry(tt.status, tt.err)
			if got.Class != tt.class || got.Retryable != tt.retryable {
				t.Fatalf("ClassifyRetry() = %+v, want class=%s retryable=%v", got, tt.class, tt.retryable)
			}
		})
	}
}
