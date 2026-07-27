package errcode

import (
	"net/http"
	"testing"
)

func TestServiceUnavailableMapsToHTTP503(t *testing.T) {
	if got := ServiceUnavailable.HttpStatusCode(); got != http.StatusServiceUnavailable {
		t.Fatalf("ServiceUnavailable.HttpStatusCode() = %d, want %d", got, http.StatusServiceUnavailable)
	}
}
