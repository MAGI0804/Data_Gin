package routers

import (
	"net/http"
	"os"
	"strings"
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
		http.MethodDelete + " /api/v1/reports/:id",
		http.MethodPost + " /api/v1/reports/:id/publish",
		http.MethodGet + " /api/v1/reports/:id/versions",
		http.MethodGet + " /api/v1/reports/:id/version-diff",
		http.MethodGet + " /api/v1/reports/:id/run-contract",
		http.MethodPost + " /api/v1/reports/:id/runs",
		http.MethodGet + " /api/v1/report-datasources",
		http.MethodGet + " /api/v1/report-datasources/:id",
		http.MethodPost + " /api/v1/report-datasources",
		http.MethodPut + " /api/v1/report-datasources/:id",
		http.MethodPost + " /api/v1/report-datasources/:id/test",
		http.MethodGet + " /api/v1/report-datasources/:id/procedures",
		http.MethodGet + " /api/v1/report-datasources/:id/procedure-signature",
		http.MethodGet + " /api/v1/report-datasources/:id/result-tables",
		http.MethodGet + " /api/v1/report-datasources/:id/result-table-schema",
		http.MethodPost + " /api/v1/report-datasource-connection-tests",
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

func TestRegisterReportCenterRoutesSkipsDisabledModule(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	registerReportCenterRoutes(router.Group("/api"), false)
	if routes := router.Routes(); len(routes) != 0 {
		t.Fatalf("disabled report center registered routes: %#v", routes)
	}
}

func TestReportDatasourceDraftConnectionTestUsesDedicatedRateLimit(t *testing.T) {
	source, err := os.ReadFile("data.go")
	if err != nil {
		t.Fatalf("read data routes: %v", err)
	}
	line := `api.POST("/v1/report-datasource-connection-tests", middleware.AuthJWT(), middleware.RequirePermission(model.PermissionReportManage), middleware.LimitRoute("30-M"), controller.TestConnection)`
	if !strings.Contains(string(source), line) {
		t.Fatal("draft Oracle connection test must keep its dedicated route rate limit")
	}
}
