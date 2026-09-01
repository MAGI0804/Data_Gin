package data_ctrl

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"gin-biz-web-api/internal/service/data_svc"
	"gin-biz-web-api/pkg/auth"
	"gin-biz-web-api/pkg/errcode"
	"gin-biz-web-api/pkg/responses"

	"github.com/gin-gonic/gin"
)

type BusinessOverviewQueryService interface {
	QueryPayments(context.Context, uint, string, string) (*data_svc.BusinessOverviewPaymentResult, error)
}

type BusinessOverviewController struct {
	service BusinessOverviewQueryService
	initErr error
}

func NewBusinessOverviewController() *BusinessOverviewController {
	service, err := data_svc.NewBusinessOverviewService()
	return &BusinessOverviewController{service: service, initErr: err}
}

func NewBusinessOverviewControllerWithService(service BusinessOverviewQueryService) *BusinessOverviewController {
	if service == nil {
		panic("business overview controller: nil service")
	}
	return &BusinessOverviewController{service: service}
}

func (controller *BusinessOverviewController) QueryPayments(c *gin.Context) {
	query := c.Request.URL.Query()
	if len(query) != 2 || len(query["date"]) != 1 || len(query["mallCode"]) != 1 {
		writeBusinessOverviewError(c, data_svc.ErrBusinessOverviewInvalid)
		return
	}
	if controller.initErr != nil || controller.service == nil {
		writeBusinessOverviewError(c, fmt.Errorf("%w: initialization failed", data_svc.ErrBusinessOverviewUnavailable))
		return
	}
	result, err := controller.service.QueryPayments(
		c.Request.Context(), auth.CurrentUserID(c), strings.TrimSpace(query.Get("date")), strings.TrimSpace(query.Get("mallCode")),
	)
	if err != nil {
		writeBusinessOverviewError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}

func writeBusinessOverviewError(c *gin.Context, err error) {
	if c != nil && err != nil {
		_ = c.Error(err).SetType(gin.ErrorTypePrivate)
	}
	switch {
	case errors.Is(err, data_svc.ErrBusinessOverviewForbidden):
		responses.New(c).ToSafeErrorResponse(errcode.Forbidden, "无权查询该商场营业数据")
	case errors.Is(err, data_svc.ErrBusinessOverviewInvalid):
		responses.New(c).ToSafeErrorResponse(errcode.UnprocessableEntity, "营业数据查询参数校验失败")
	default:
		responses.New(c).ToSafeErrorResponse(errcode.InternalServerError, "营业数据查询服务暂时不可用")
	}
}
