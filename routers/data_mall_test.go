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
	registerMallWeatherRoutes(router.Group("/api"), &data_ctrl.MallWeatherController{})
	registerMallWeatherRefreshRoutes(router.Group("/api"), &data_ctrl.MallWeatherRefreshController{})
	registerMallWeatherExportProfileRoutes(router.Group("/api"), &data_ctrl.MallWeatherExportProfileController{})
	registerMallWeatherExportJobRoutes(router.Group("/api"), &data_ctrl.MallWeatherExportJobController{})
	registerMallWeatherFeishuPushRoutes(router.Group("/api"), &data_ctrl.MallWeatherFeishuPushController{})
	registerMallWeatherSheetPushOptionRoutes(router.Group("/api"), &data_ctrl.MallWeatherSheetPushOptionController{})

	routes := make(map[string]struct{})
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	for _, expected := range []string{
		http.MethodPost + " /api/v1/malls",
		http.MethodPost + " /api/v1/malls/import",
		http.MethodGet + " /api/v1/malls",
		http.MethodGet + " /api/v1/malls/:id",
		http.MethodPatch + " /api/v1/malls/:id",
		http.MethodDelete + " /api/v1/malls/:id",
		http.MethodGet + " /api/v1/malls/:id/weather/overview",
		http.MethodGet + " /api/v1/malls/:id/weather/realtime",
		http.MethodGet + " /api/v1/malls/:id/weather/minutely",
		http.MethodGet + " /api/v1/malls/:id/weather/hourly",
		http.MethodGet + " /api/v1/malls/:id/weather/daily",
		http.MethodGet + " /api/v1/malls/:id/weather/alerts",
		http.MethodGet + " /api/v1/malls/:id/weather/life-indices",
		http.MethodGet + " /api/v1/malls/:id/weather/fetch-runs",
		http.MethodPost + " /api/v1/malls/:id/weather-refresh",
		http.MethodPost + " /api/v1/weather-export-profiles",
		http.MethodGet + " /api/v1/weather-export-profiles",
		http.MethodPost + " /api/v1/weather-exports",
		http.MethodGet + " /api/v1/weather-exports/:job_id",
		http.MethodGet + " /api/v1/weather-exports/:job_id/download",
		http.MethodPost + " /api/v1/weather-sheet-pushes",
		http.MethodGet + " /api/v1/weather-sheet-pushes/:run_id",
		http.MethodPost + " /api/v1/weather-sheet-pushes/dry-run",
		http.MethodGet + " /api/v1/weather-sheet-push-options",
	} {
		if _, ok := routes[expected]; !ok {
			t.Errorf("route %q is not registered", expected)
		}
	}
}
