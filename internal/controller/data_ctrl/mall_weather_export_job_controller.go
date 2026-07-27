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

type MallWeatherExportJobServiceAPI interface {
	Create(
		context.Context,
		uint,
		string,
		requestbody.MallWeatherExportCreateRequest,
	) (*data_svc.MallWeatherExportCreateResult, bool, error)
	Get(context.Context, uint, string) (*data_svc.MallWeatherExportJobDTO, error)
	Download(context.Context, uint, string) (*data_svc.MallWeatherExportDownloadResult, error)
}

type MallWeatherExportJobController struct {
	service MallWeatherExportJobServiceAPI
}

func NewMallWeatherExportJobController() *MallWeatherExportJobController {
	return NewMallWeatherExportJobControllerWithService(data_svc.NewMallWeatherExportJobService())
}

func NewMallWeatherExportJobControllerWithService(
	service MallWeatherExportJobServiceAPI,
) *MallWeatherExportJobController {
	if service == nil {
		panic("mall weather export job controller: nil service")
	}
	return &MallWeatherExportJobController{service: service}
}

func (controller *MallWeatherExportJobController) Create(c *gin.Context) {
	var request requestbody.MallWeatherExportCreateRequest
	if err := decodeMallJSON(c, &request); err != nil {
		writeMallWeatherExportJobError(
			c,
			fmt.Errorf("%w: invalid JSON body", data_svc.ErrMallWeatherExportInvalid),
		)
		return
	}
	result, replayed, err := controller.service.Create(
		c.Request.Context(),
		auth.CurrentUserID(c),
		c.GetHeader("Idempotency-Key"),
		request,
	)
	if err != nil {
		writeMallWeatherExportJobError(c, err)
		return
	}
	if replayed {
		c.Header("Idempotency-Replayed", "true")
	}
	responses.New(c).ToResponseWithStatus(http.StatusAccepted, result)
}

func (controller *MallWeatherExportJobController) Get(c *gin.Context) {
	disableMallWeatherExportCaching(c)
	result, err := controller.service.Get(
		c.Request.Context(),
		auth.CurrentUserID(c),
		c.Param("job_id"),
	)
	if err != nil {
		writeMallWeatherExportJobError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}

func (controller *MallWeatherExportJobController) Download(c *gin.Context) {
	disableMallWeatherExportCaching(c)
	result, err := controller.service.Download(
		c.Request.Context(),
		auth.CurrentUserID(c),
		c.Param("job_id"),
	)
	if err != nil {
		writeMallWeatherExportJobError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}

func disableMallWeatherExportCaching(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
}

func writeMallWeatherExportJobError(c *gin.Context, err error) {
	code, message := classifyMallWeatherExportJobError(err)
	responses.New(c).ToSafeErrorResponse(code, message)
}

func classifyMallWeatherExportJobError(err error) (*errcode.Error, string) {
	switch {
	case errors.Is(err, data_svc.ErrMallForbidden):
		return errcode.Forbidden, "无权导出天气数据"
	case errors.Is(err, data_dao.ErrMallWeatherExportProfileNotFound):
		return errcode.NotFound, "天气导出配置不存在"
	case errors.Is(err, data_dao.ErrMallWeatherExportJobNotFound):
		return errcode.NotFound, "天气导出任务不存在"
	case errors.Is(err, data_svc.ErrMallIdempotencyConflict),
		errors.Is(err, data_svc.ErrMallIdempotencyPending),
		errors.Is(err, data_svc.ErrMallWeatherExportProfileConflict):
		return errcode.Conflict, "天气导出请求冲突，请稍后重试"
	case errors.Is(err, data_svc.ErrMallWeatherExportNotReady):
		return errcode.Conflict, "天气导出文件尚未生成"
	case errors.Is(err, data_svc.ErrMallWeatherExportExpired):
		return errcode.Conflict, "天气导出文件已过期"
	case errors.Is(err, data_svc.ErrMallWeatherExportInvalid),
		errors.Is(err, data_svc.ErrMallWeatherExportTooLarge):
		return errcode.UnprocessableEntity, "天气导出参数校验失败"
	default:
		return errcode.InternalServerError, "天气导出服务暂时不可用"
	}
}
