package routers

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestDataAuthorizationRoutesArePostOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	api := router.Group("/api")
	registerDataAuthorizationRoutes(api)
	want := map[string]bool{
		"/api/v1/data-authorizations/accounts/query":                  false,
		"/api/v1/data-authorizations/accounts/create":                 false,
		"/api/v1/data-authorizations/accounts/:id/permissions/grant":  false,
		"/api/v1/data-authorizations/accounts/:id/permissions/revoke": false,
		"/api/v1/data-authorizations/accounts/:id/token/reissue":      false,
		"/api/v1/data-authorizations/audits/query":                    false,
	}
	for _, route := range router.Routes() {
		if _, exists := want[route.Path]; !exists {
			continue
		}
		if route.Method != http.MethodPost {
			t.Fatalf("route %s method = %s", route.Path, route.Method)
		}
		want[route.Path] = true
	}
	for path, found := range want {
		if !found {
			t.Fatalf("route %s is missing", path)
		}
	}
}
