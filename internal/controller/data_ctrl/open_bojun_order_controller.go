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

type OpenBojunOrderQueryService interface {
	Query(context.Context, uint, requestbody.OpenBojunOrderQueryRequest) (*data_svc.OpenBojunOrderQueryResult, error)
}

type OpenBojunOrderController struct {
	service OpenBojunOrderQueryService
}

func NewOpenBojunOrderController() *OpenBojunOrderController {
	return NewOpenBojunOrderControllerWithService(data_svc.NewOpenBojunOrderQueryService())
}

func NewOpenBojunOrderControllerWithService(service OpenBojunOrderQueryService) *OpenBojunOrderController {
	if service == nil {
		panic("open bojun order controller: nil service")
	}
	return &OpenBojunOrderController{service: service}
}

func (controller *OpenBojunOrderController) Query(c *gin.Context) {
	var request requestbody.OpenBojunOrderQueryRequest
	if err := decodeMallJSON(c, &request); err != nil {
		writeOpenBojunOrderError(c, data_svc.ErrOpenBojunOrderInvalidQuery)
		return
	}
	result, err := controller.service.Query(c.Request.Context(), auth.CurrentUserID(c), request)
	if err != nil {
		writeOpenBojunOrderError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}

func writeOpenBojunOrderError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, data_svc.ErrOpenBojunOrderForbidden):
		responses.New(c).ToSafeErrorResponse(errcode.Forbidden, "无权查询伯俊订单数据")
	case errors.Is(err, data_svc.ErrOpenBojunOrderInvalidQuery):
		responses.New(c).ToSafeErrorResponse(errcode.UnprocessableEntity, "伯俊订单查询参数校验失败")
	default:
		responses.New(c).ToSafeErrorResponse(errcode.InternalServerError, "伯俊订单查询服务暂时不可用")
	}
}
