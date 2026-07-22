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

type MallWeatherFeishuPushServiceAPI interface {
	DryRun(
		context.Context,
		uint,
		requestbody.MallWeatherFeishuPushRequest,
	) (*data_svc.MallWeatherFeishuDryRunResult, error)
}

type MallWeatherFeishuPushController struct {
	service MallWeatherFeishuPushServiceAPI
}

func NewMallWeatherFeishuPushController() *MallWeatherFeishuPushController {
	return NewMallWeatherFeishuPushControllerWithService(data_svc.NewMallWeatherFeishuPushService())
}

func NewMallWeatherFeishuPushControllerWithService(
	service MallWeatherFeishuPushServiceAPI,
) *MallWeatherFeishuPushController {
	if service == nil {
		panic("mall weather feishu push controller: nil service")
	}
	return &MallWeatherFeishuPushController{service: service}
}

func (controller *MallWeatherFeishuPushController) DryRun(c *gin.Context) {
	var request requestbody.MallWeatherFeishuPushRequest
	if err := decodeMallJSON(c, &request); err != nil {
		writeMallWeatherFeishuPushError(c, fmt.Errorf("%w: invalid JSON body", data_svc.ErrMallWeatherFeishuInvalid))
		return
	}
	result, err := controller.service.DryRun(c.Request.Context(), auth.CurrentUserID(c), request)
	if err != nil {
		writeMallWeatherFeishuPushError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}

func writeMallWeatherFeishuPushError(c *gin.Context, err error) {
	code, message := classifyMallWeatherFeishuPushError(err)
	responses.New(c).ToSafeErrorResponse(code, message)
}

func classifyMallWeatherFeishuPushError(err error) (*errcode.Error, string) {
	switch {
	case errors.Is(err, data_svc.ErrMallForbidden):
		return errcode.Forbidden, "无权推送天气数据到飞书"
	case errors.Is(err, data_svc.ErrMallWeatherFeishuDestinationNotFound):
		return errcode.NotFound, "飞书推送目标不存在"
	case errors.Is(err, data_dao.ErrMallWeatherExportProfileNotFound):
		return errcode.NotFound, "天气导出配置不存在"
	case errors.Is(err, data_svc.ErrMallWeatherExportProfileConflict):
		return errcode.Conflict, "天气导出配置版本冲突"
	case errors.Is(err, data_svc.ErrMallWeatherFeishuInvalid),
		errors.Is(err, data_svc.ErrMallWeatherExportTooLarge):
		return errcode.UnprocessableEntity, "飞书推送参数校验失败"
	default:
		return errcode.InternalServerError, "飞书推送服务暂时不可用"
	}
}
