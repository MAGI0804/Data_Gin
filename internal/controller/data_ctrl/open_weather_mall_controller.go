package data_ctrl

import (
	"context"
	"errors"
	"net/http"

	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/internal/service/data_svc"
	"gin-biz-web-api/pkg/auth"
	"gin-biz-web-api/pkg/errcode"
	"gin-biz-web-api/pkg/responses"

	"github.com/gin-gonic/gin"
)

type OpenWeatherMallQueryService interface {
	Query(context.Context, uint, requestbody.OpenWeatherMallQueryRequest) (*data_svc.OpenWeatherMallQueryResult, error)
}

type OpenWeatherMallController struct {
	service OpenWeatherMallQueryService
}

func NewOpenWeatherMallController() *OpenWeatherMallController {
	return NewOpenWeatherMallControllerWithService(data_svc.NewOpenWeatherMallQueryService())
}

func NewOpenWeatherMallControllerWithService(service OpenWeatherMallQueryService) *OpenWeatherMallController {
	if service == nil {
		panic("open weather mall controller: nil service")
	}
	return &OpenWeatherMallController{service: service}
}

func (controller *OpenWeatherMallController) Query(c *gin.Context) {
	var request requestbody.OpenWeatherMallQueryRequest
	if err := decodeMallJSON(c, &request); err != nil {
		writeOpenWeatherMallError(c, data_svc.ErrOpenWeatherMallInvalidQuery)
		return
	}
	result, err := controller.service.Query(c.Request.Context(), auth.CurrentUserID(c), request)
	if err != nil {
		writeOpenWeatherMallError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}

func writeOpenWeatherMallError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, data_svc.ErrOpenWeatherMallForbidden):
		responses.New(c).ToSafeErrorResponse(errcode.Forbidden, "无权查询天气商场信息")
	case errors.Is(err, data_svc.ErrOpenWeatherMallInvalidQuery):
		responses.New(c).ToSafeErrorResponse(errcode.UnprocessableEntity, "天气商场查询参数校验失败")
	default:
		responses.New(c).ToSafeErrorResponse(errcode.InternalServerError, "天气商场查询服务暂时不可用")
	}
}
