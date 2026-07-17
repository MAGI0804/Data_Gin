// Package geocoder defines the provider-neutral geocoding boundary used by
// mall services.
package geocoder

import (
	"context"
	"encoding/json"
	"fmt"

	"gin-biz-web-api/pkg/providerhttp"
)

type Request struct {
	Address string
	City    string
}

type Candidate struct {
	FormattedAddress string
	Country          string
	Province         string
	City             string
	Citycode         string
	District         string
	Township         string
	Street           string
	StreetNumber     string
	Adcode           string
	Longitude        float64
	Latitude         float64
	CoordinateSystem string
	Level            string
}

type Response struct {
	ProviderStatus string
	Info           string
	Infocode       string
	Count          int
	Candidates     []Candidate
	RawJSON        json.RawMessage
}

type Geocoder interface {
	Geocode(ctx context.Context, request Request) (*Response, error)
}

// ProviderError exposes only stable, non-sensitive failure metadata. The
// upstream provider message and request URL are intentionally omitted.
type ProviderError struct {
	Class      providerhttp.ErrorClass
	Code       string
	HTTPStatus int
	Retryable  bool
	cause      error
}

func (err *ProviderError) Error() string {
	if err == nil {
		return "geocoder provider error"
	}
	message := fmt.Sprintf("geocoder provider error: class=%s", err.Class)
	if err.Code != "" {
		message += " code=" + err.Code
	}
	if err.HTTPStatus != 0 {
		message += fmt.Sprintf(" http_status=%d", err.HTTPStatus)
	}
	return message
}

func (err *ProviderError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.cause
}
