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
	registerReportDatasourceRoutes(router.Group("/api"), &data_ctrl.ReportDatasourceController{})
	registerReportRunRoutes(router.Group("/api"), &data_ctrl.ReportRunController{})
	registerReportExportRoutes(router.Group("/api"), &data_ctrl.ReportExportController{})
	registerReportAuditRoutes(router.Group("/api"), &data_ctrl.ReportAuditController{})
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
		http.MethodGet + " /api/v1/reports/:id/run-contract",
		http.MethodPost + " /api/v1/reports/:id/runs",
		http.MethodGet + " /api/v1/report-datasources",
		http.MethodGet + " /api/v1/report-datasources/:id",
		http.MethodPost + " /api/v1/report-datasources",
		http.MethodPut + " /api/v1/report-datasources/:id",
		http.MethodPost + " /api/v1/report-datasources/:id/test",
		http.MethodGet + " /api/v1/report-runs/:id",
		http.MethodGet + " /api/v1/report-runs/:id/results",
		http.MethodPost + " /api/v1/report-runs/:id/cancel",
		http.MethodPost + " /api/v1/report-runs/:id/export",
		http.MethodGet + " /api/v1/report-exports/:id",
		http.MethodGet + " /api/v1/report-exports/:id/download",
		http.MethodGet + " /api/v1/report-audits",
	} {
		if _, exists := routes[expected]; !exists {
			t.Fatalf("missing route %q: %#v", expected, routes)
		}
	}
}
