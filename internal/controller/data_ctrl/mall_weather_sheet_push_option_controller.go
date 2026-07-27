package data_ctrl

import (
	"context"
	"errors"
	"net/http"

	"gin-biz-web-api/internal/service/data_svc"
	"gin-biz-web-api/pkg/auth"
	"gin-biz-web-api/pkg/errcode"
	"gin-biz-web-api/pkg/responses"

	"github.com/gin-gonic/gin"
)

type MallWeatherSheetPushOptionServiceAPI interface {
	List(context.Context, uint) (*data_svc.MallWeatherSheetPushOptionResult, error)
}

type MallWeatherSheetPushOptionController struct {
	service MallWeatherSheetPushOptionServiceAPI
}

func NewMallWeatherSheetPushOptionController() *MallWeatherSheetPushOptionController {
	return NewMallWeatherSheetPushOptionControllerWithService(
		data_svc.NewMallWeatherSheetPushOptionService(),
	)
}

func NewMallWeatherSheetPushOptionControllerWithService(
	service MallWeatherSheetPushOptionServiceAPI,
) *MallWeatherSheetPushOptionController {
	if service == nil {
		panic("mall weather sheet push option controller: nil service")
	}
	return &MallWeatherSheetPushOptionController{service: service}
}

func (controller *MallWeatherSheetPushOptionController) List(c *gin.Context) {
	result, err := controller.service.List(c.Request.Context(), auth.CurrentUserID(c))
	if err != nil {
		writeMallWeatherSheetPushOptionError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}

func writeMallWeatherSheetPushOptionError(c *gin.Context, err error) {
	if errors.Is(err, data_svc.ErrMallForbidden) {
		responses.New(c).ToSafeErrorResponse(errcode.Forbidden, "无权查看飞书天气推送目标")
		return
	}
	responses.New(c).ToSafeErrorResponse(errcode.InternalServerError, "飞书天气推送目标服务暂时不可用")
}
