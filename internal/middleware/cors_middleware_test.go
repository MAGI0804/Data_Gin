package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCorsAllowsPatchPreflight(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(Cors())
	router.PATCH("/resource", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	request := httptest.NewRequest(http.MethodOptions, "/resource", nil)
	request.Header.Set("Origin", "https://console.example.com")
	request.Header.Set("Access-Control-Request-Method", http.MethodPatch)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d", recorder.Code)
	}
	methods := recorder.Header().Get("Access-Control-Allow-Methods")
	if !strings.Contains(","+methods+",", ",PATCH,") {
		t.Fatalf("allowed methods = %q", methods)
	}
	if recorder.Header().Get("Access-Control-Allow-Origin") != "https://console.example.com" {
		t.Fatalf("allowed origin = %q", recorder.Header().Get("Access-Control-Allow-Origin"))
	}
}
