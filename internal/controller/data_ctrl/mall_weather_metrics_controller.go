package data_ctrl

import (
	"context"
	"net/http"

	"gin-biz-web-api/internal/service/data_svc"
	"gin-biz-web-api/pkg/auth"
	"gin-biz-web-api/pkg/responses"

	"github.com/gin-gonic/gin"
)

type MallWeatherMetricsReader interface {
	Snapshot(context.Context, uint) (*data_svc.MallWeatherMetricsResult, error)
}

type MallWeatherMetricsController struct {
	service MallWeatherMetricsReader
}

func NewMallWeatherMetricsController() *MallWeatherMetricsController {
	return NewMallWeatherMetricsControllerWithService(data_svc.NewMallWeatherMetricsService())
}

func NewMallWeatherMetricsControllerWithService(service MallWeatherMetricsReader) *MallWeatherMetricsController {
	if service == nil {
		panic("mall weather metrics controller: nil service")
	}
	return &MallWeatherMetricsController{service: service}
}

func (controller *MallWeatherMetricsController) Snapshot(c *gin.Context) {
	result, err := controller.service.Snapshot(c.Request.Context(), auth.CurrentUserID(c))
	if err != nil {
		writeMallWeatherError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}
