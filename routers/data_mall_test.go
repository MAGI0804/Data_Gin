package routers

import (
	"net/http"
	"testing"

	"gin-biz-web-api/internal/controller/data_ctrl"

	"github.com/gin-gonic/gin"
)

func TestAPIDataRegistersMallCRUDRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	registerMallRoutes(router.Group("/api"), &data_ctrl.MallController{})

	routes := make(map[string]struct{})
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	for _, expected := range []string{
		http.MethodPost + " /api/v1/malls",
		http.MethodGet + " /api/v1/malls",
		http.MethodGet + " /api/v1/malls/:id",
		http.MethodPatch + " /api/v1/malls/:id",
		http.MethodDelete + " /api/v1/malls/:id",
	} {
		if _, ok := routes[expected]; !ok {
			t.Errorf("route %q is not registered", expected)
		}
	}
}
