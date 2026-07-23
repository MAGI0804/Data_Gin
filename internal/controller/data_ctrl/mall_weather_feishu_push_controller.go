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
	Create(
		context.Context,
		uint,
		string,
		requestbody.MallWeatherFeishuPushRequest,
	) (*data_svc.MallWeatherFeishuPushCreateResult, bool, error)
	Get(context.Context, uint, uint) (*data_svc.MallWeatherFeishuPushRunDTO, error)
	DryRun(
		context.Context,
		uint,
		requestbody.MallWeatherFeishuPushRequest,
	) (*data_svc.MallWeatherFeishuDryRunResult, error)
}

func (controller *MallWeatherFeishuPushController) Create(c *gin.Context) {
	var request requestbody.MallWeatherFeishuPushRequest
	if err := decodeMallJSON(c, &request); err != nil {
		writeMallWeatherFeishuPushError(c, fmt.Errorf("%w: invalid JSON body", data_svc.ErrMallWeatherFeishuInvalid))
		return
	}
	result, replayed, err := controller.service.Create(
		c.Request.Context(),
		auth.CurrentUserID(c),
		c.GetHeader("Idempotency-Key"),
		request,
	)
	if err != nil {
		writeMallWeatherFeishuPushError(c, err)
		return
	}
	if replayed {
		c.Header("Idempotency-Replayed", "true")
	}
	responses.New(c).ToResponseWithStatus(http.StatusAccepted, result)
}

func (controller *MallWeatherFeishuPushController) Get(c *gin.Context) {
	runID, err := parseMallUint(c.Param("run_id"), "run id")
	if err != nil {
		writeMallWeatherFeishuPushError(c, errMallWeatherFeishuInvalidPath)
		return
	}
	result, err := controller.service.Get(c.Request.Context(), auth.CurrentUserID(c), runID)
	if err != nil {
		writeMallWeatherFeishuPushError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
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
	case errors.Is(err, data_svc.ErrMallWeatherFeishuDisabled):
		return errcode.Forbidden, "飞书天气推送功能未开启"
	case errors.Is(err, data_svc.ErrMallWeatherFeishuDestinationNotFound):
		return errcode.NotFound, "飞书推送目标不存在"
	case errors.Is(err, data_dao.ErrMallWeatherFeishuRunNotFound):
		return errcode.NotFound, "飞书推送运行记录不存在"
	case errors.Is(err, data_dao.ErrMallWeatherExportProfileNotFound):
		return errcode.NotFound, "天气导出配置不存在"
	case errors.Is(err, data_svc.ErrMallWeatherExportProfileConflict),
		errors.Is(err, data_svc.ErrMallWeatherFeishuDestinationConflict),
		errors.Is(err, data_svc.ErrMallIdempotencyConflict),
		errors.Is(err, data_svc.ErrMallIdempotencyPending):
		return errcode.Conflict, "飞书推送请求冲突，请稍后重试"
	case errors.Is(err, errMallWeatherFeishuInvalidPath):
		return errcode.UnprocessableEntity, "飞书推送运行编号无效"
	case errors.Is(err, data_svc.ErrMallWeatherFeishuInvalid),
		errors.Is(err, data_svc.ErrMallWeatherExportTooLarge):
		return errcode.UnprocessableEntity, "飞书推送参数校验失败"
	default:
		return errcode.InternalServerError, "飞书推送服务暂时不可用"
	}
}

var errMallWeatherFeishuInvalidPath = errors.New("mall weather feishu push controller: invalid path")
