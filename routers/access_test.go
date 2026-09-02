package routers

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAccessRoutesRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	registerAccessRoutes(router.Group("/api"))
	want := map[string]struct{}{
		http.MethodGet + " /api/v1/access/malls":                   {},
		http.MethodPost + " /api/v1/access/accounts/query":         {},
		http.MethodPost + " /api/v1/access/accounts":               {},
		http.MethodPut + " /api/v1/access/accounts/:id/status":     {},
		http.MethodPut + " /api/v1/access/accounts/:id/password":   {},
		http.MethodPut + " /api/v1/access/accounts/:id/roles":      {},
		http.MethodPut + " /api/v1/access/accounts/:id/mall-scope": {},
		http.MethodGet + " /api/v1/access/permissions":             {},
		http.MethodGet + " /api/v1/access/roles":                   {},
		http.MethodPost + " /api/v1/access/roles":                  {},
		http.MethodGet + " /api/v1/access/audits":                  {},
	}
	for _, route := range router.Routes() {
		delete(want, route.Method+" "+route.Path)
	}
	for route := range want {
		t.Errorf("route %s is missing", route)
	}
}
