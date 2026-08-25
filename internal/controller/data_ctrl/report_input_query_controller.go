package data_ctrl

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"gin-biz-web-api/internal/service/data_svc"
	"gin-biz-web-api/pkg/auth"
	"gin-biz-web-api/pkg/responses"
)

type ReportInputQueryServiceAPI interface {
	List() *data_svc.ReportInputQueryListDTO
	Options(context.Context, uint, uint, string, string) (*data_svc.ReportInputOptionListDTO, error)
}

type ReportInputQueryController struct {
	service ReportInputQueryServiceAPI
	initErr error
}

func NewReportInputQueryController() *ReportInputQueryController {
	service, err := data_svc.NewReportInputQueryService()
	return &ReportInputQueryController{service: service, initErr: err}
}

func NewReportInputQueryControllerWithService(service ReportInputQueryServiceAPI) *ReportInputQueryController {
	if service == nil {
		panic("report input query controller: nil service")
	}
	return &ReportInputQueryController{service: service}
}

func (controller *ReportInputQueryController) List(c *gin.Context) {
	if controller.initErr != nil || controller.service == nil {
		writeReportError(c, fmt.Errorf("%w: configuration is invalid", data_svc.ErrReportInputQueryUnavailable))
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, controller.service.List())
}

func (controller *ReportInputQueryController) Options(c *gin.Context) {
	if controller.initErr != nil || controller.service == nil {
		writeReportError(c, fmt.Errorf("%w: configuration is invalid", data_svc.ErrReportInputQueryUnavailable))
		return
	}
	reportID, err := parseReportUint(c.Param("id"), "report id")
	if err != nil {
		writeReportError(c, err)
		return
	}
	result, err := controller.service.Options(
		c.Request.Context(), auth.CurrentUserID(c), reportID,
		strings.TrimSpace(c.Param("condition_code")), strings.TrimSpace(c.Query("name")),
	)
	if err != nil {
		writeReportError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}
