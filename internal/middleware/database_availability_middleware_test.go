package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestDatabaseAvailabilityMiddleware(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		available  bool
		wantStatus int
		wantCalls  int
	}{
		{name: "available", method: http.MethodGet, available: true, wantStatus: http.StatusNoContent, wantCalls: 1},
		{name: "unavailable", method: http.MethodGet, wantStatus: http.StatusServiceUnavailable},
		{name: "preflight bypasses database gate", method: http.MethodOptions, wantStatus: http.StatusNoContent, wantCalls: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			router := gin.New()
			calls := 0
			router.Use(requireDatabaseAvailability(func(context.Context) bool { return test.available }))
			router.Handle(test.method, "/resource", func(c *gin.Context) {
				calls++
				c.Status(http.StatusNoContent)
			})

			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(test.method, "/resource", nil))

			if response.Code != test.wantStatus || calls != test.wantCalls {
				t.Fatalf("status=%d calls=%d, want status=%d calls=%d", response.Code, calls, test.wantStatus, test.wantCalls)
			}
			if !test.available && test.method != http.MethodOptions && response.Header().Get("Retry-After") != "1" {
				t.Fatalf("Retry-After=%q, want 1", response.Header().Get("Retry-After"))
			}
		})
	}
}
