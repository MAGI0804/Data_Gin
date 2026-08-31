package shanghaimall

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestPostKerrySalesAcceptsSuccessfulResponse(t *testing.T) {
	result, err := postKerrySales(context.Background(), kerryConfig{
		SalesURL: "https://kerry.test/sales",
		Client:   kerryTestClient(`{"error":false,"errorCode":0}`),
	}, map[string]interface{}{"docKey": "ORDER-0"})

	if err != nil {
		t.Fatalf("postKerrySales() error = %v", err)
	}
	if result == nil || !result.Success {
		t.Fatalf("postKerrySales() result = %+v, want successful result", result)
	}
}

func TestPostKerrySalesTreatsExistingSalesDocAsSuccess(t *testing.T) {
	result, err := postKerrySales(context.Background(), kerryConfig{
		SalesURL: "https://kerry.test/sales",
		Client:   kerryTestClient(`{"error":true,"errorCode":551510,"errorMessage":"SalesDoc already exists."}`),
	}, map[string]interface{}{"docKey": "ORDER-1"})

	if err != nil {
		t.Fatalf("postKerrySales() error = %v", err)
	}
	if result == nil || !result.Success {
		t.Fatalf("postKerrySales() result = %+v, want successful idempotent result", result)
	}
}

func TestPostKerrySalesKeepsOtherErrorsFailed(t *testing.T) {
	result, err := postKerrySales(context.Background(), kerryConfig{
		SalesURL: "https://kerry.test/sales",
		Client:   kerryTestClient(`{"error":true,"errorCode":551511,"errorMessage":"Another error."}`),
	}, map[string]interface{}{"docKey": "ORDER-2"})

	if err == nil {
		t.Fatal("postKerrySales() error = nil, want Kerry push failure")
	}
	if result == nil || result.Success {
		t.Fatalf("postKerrySales() result = %+v, want failed result", result)
	}
}

type kerryRoundTripFunc func(*http.Request) (*http.Response, error)

func (roundTrip kerryRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

func kerryTestClient(responseBody string) *http.Client {
	return &http.Client{Transport: kerryRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(responseBody)),
			Request:    request,
		}, nil
	})}
}
