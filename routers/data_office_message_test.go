package routers

import (
	"net/http"
	"testing"

	"gin-biz-web-api/internal/controller/data_ctrl"

	"github.com/gin-gonic/gin"
)

func TestRegisterOfficeMessageRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	registerOfficeMessageRoutes(router.Group("/api"), &data_ctrl.OfficeMessageController{})
	routes := make(map[string]struct{})
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	for _, expected := range []string{
		http.MethodGet + " /api/v1/office-messages",
		http.MethodPost + " /api/v1/office-messages",
		http.MethodPut + " /api/v1/office-messages/:id",
		http.MethodDelete + " /api/v1/office-messages/:id",
		http.MethodGet + " /api/v1/office-feishu-bots",
		http.MethodGet + " /api/v1/office-push-targets",
		http.MethodPost + " /api/v1/office-push-targets",
		http.MethodPost + " /api/v1/office-push-targets/:id/runs",
		http.MethodGet + " /api/v1/office-push-schedules",
		http.MethodPost + " /api/v1/office-push-schedules",
		http.MethodPut + " /api/v1/office-push-schedules/:id",
		http.MethodDelete + " /api/v1/office-push-schedules/:id",
		http.MethodGet + " /api/v1/office-push-runs",
		http.MethodGet + " /api/v1/office-oracle/procedures",
		http.MethodGet + " /api/v1/office-oracle/result-tables",
		http.MethodPost + " /api/v1/office-oracle/select-tests",
	} {
		if _, exists := routes[expected]; !exists {
			t.Fatalf("missing route %q", expected)
		}
	}
}
