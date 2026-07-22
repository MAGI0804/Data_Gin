package data_ctrl

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/internal/service/data_svc"
	"gin-biz-web-api/pkg/auth"
	"gin-biz-web-api/pkg/errcode"
	"gin-biz-web-api/pkg/responses"

	"github.com/gin-gonic/gin"
)

type MallWeatherRefreshServiceAPI interface {
	Refresh(context.Context, uint, uint, string, requestbody.MallWeatherRefreshRequest) (*data_svc.MallWeatherRefreshResult, bool, error)
}

type MallWeatherRefreshController struct {
	service MallWeatherRefreshServiceAPI
}

func NewMallWeatherRefreshController() *MallWeatherRefreshController {
	return NewMallWeatherRefreshControllerWithService(data_svc.NewMallWeatherRefreshService())
}

func NewMallWeatherRefreshControllerWithService(service MallWeatherRefreshServiceAPI) *MallWeatherRefreshController {
	if service == nil {
		panic("mall weather refresh controller: nil service")
	}
	return &MallWeatherRefreshController{service: service}
}

func (controller *MallWeatherRefreshController) Refresh(c *gin.Context) {
	mallID, err := parseMallUint(c.Param("id"), "mall id")
	if err != nil {
		writeMallWeatherRefreshError(c, err)
		return
	}
	var request requestbody.MallWeatherRefreshRequest
	if err := decodeMallJSON(c, &request); err != nil {
		writeMallWeatherRefreshError(c, fmt.Errorf("%w: invalid JSON body", data_svc.ErrMallInvalidInput))
		return
	}
	result, replayed, err := controller.service.Refresh(
		c.Request.Context(), auth.CurrentUserID(c), mallID, c.GetHeader("Idempotency-Key"), request,
	)
	if err != nil {
		writeMallWeatherRefreshError(c, err)
		return
	}
	if replayed {
		c.Header("Idempotency-Replayed", "true")
	}
	responses.New(c).ToResponseWithStatus(http.StatusAccepted, result)
}

func writeMallWeatherRefreshError(c *gin.Context, err error) {
	code, message := classifyMallWeatherRefreshError(err)
	responses.New(c).ToSafeErrorResponse(code, message)
}

func classifyMallWeatherRefreshError(err error) (*errcode.Error, string) {
	switch {
	case errors.Is(err, data_svc.ErrMallForbidden):
		return errcode.Forbidden, "无权触发天气刷新"
	case errors.Is(err, data_dao.ErrMallNotFound):
		return errcode.NotFound, "商场不存在"
	case errors.Is(err, data_svc.ErrMallIdempotencyConflict), errors.Is(err, data_svc.ErrMallIdempotencyPending):
		return errcode.Conflict, "天气刷新请求冲突，请稍后重试"
	case errors.Is(err, data_svc.ErrMallInvalidInput):
		return errcode.UnprocessableEntity, "天气刷新参数校验失败"
	default:
		return errcode.InternalServerError, "天气刷新服务暂时不可用"
	}
}
