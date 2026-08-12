package routers

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"gin-biz-web-api/internal/controller/data_ctrl"
)

func TestRegisterReportRoutesUsesExpectedMethodsAndPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	registerReportRoutes(router.Group("/api"), &data_ctrl.ReportController{})
	routes := make(map[string]struct{})
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	for _, expected := range []string{
		http.MethodPost + " /api/v1/reports",
		http.MethodGet + " /api/v1/reports",
		http.MethodGet + " /api/v1/reports/:id",
		http.MethodPut + " /api/v1/reports/:id",
		http.MethodPost + " /api/v1/reports/:id/publish",
		http.MethodPost + " /api/v1/reports/:id/runs",
	} {
		if _, exists := routes[expected]; !exists {
			t.Fatalf("missing route %q: %#v", expected, routes)
		}
	}
}
