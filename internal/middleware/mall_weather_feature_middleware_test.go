package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"gin-biz-web-api/pkg/errcode"
	"gin-biz-web-api/pkg/responses"

	"github.com/gin-gonic/gin"
)

func TestRequireMallWeatherEnabled(t *testing.T) {
	tests := []struct {
		name       string
		enabled    func() bool
		wantStatus int
		wantCalls  int
	}{
		{name: "enabled", enabled: func() bool { return true }, wantStatus: http.StatusNoContent, wantCalls: 1},
		{name: "disabled", enabled: func() bool { return false }, wantStatus: http.StatusServiceUnavailable},
		{name: "missing feature reader fails closed", wantStatus: http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			router := gin.New()
			calls := 0
			router.POST("/weather-write", requireMallWeatherEnabled(tt.enabled), func(c *gin.Context) {
				calls++
				c.Status(http.StatusNoContent)
			})

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/weather-write", nil)
			router.ServeHTTP(recorder, request)

			if recorder.Code != tt.wantStatus || calls != tt.wantCalls {
				t.Fatalf("response = (%d, calls=%d), want (%d, calls=%d)", recorder.Code, calls, tt.wantStatus, tt.wantCalls)
			}
			if tt.wantStatus == http.StatusNoContent {
				return
			}
			var body responses.ResponseData
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Code != errcode.ServiceUnavailable.Code() || body.Msg != mallWeatherDisabledMessage {
				t.Fatalf("response body = %+v", body)
			}
		})
	}
}
