package data_ctrl

import (
	"context"
	"net/http"

	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/internal/service/data_svc"
	"gin-biz-web-api/pkg/auth"
	"gin-biz-web-api/pkg/responses"

	"github.com/gin-gonic/gin"
)

type ReportCategoryAccessServiceAPI interface {
	List(context.Context, uint) (*data_svc.ReportCategoryAccessListDTO, error)
	Replace(context.Context, uint, requestbody.ReportCategoryAccessSaveRequest) (*data_svc.ReportCategoryAccessDTO, error)
}

type ReportCategoryAccessController struct {
	service ReportCategoryAccessServiceAPI
}

func NewReportCategoryAccessController() *ReportCategoryAccessController {
	return NewReportCategoryAccessControllerWithService(data_svc.NewReportCategoryAccessService())
}

func NewReportCategoryAccessControllerWithService(service ReportCategoryAccessServiceAPI) *ReportCategoryAccessController {
	if service == nil {
		panic("report category access controller: nil service")
	}
	return &ReportCategoryAccessController{service: service}
}

func (controller *ReportCategoryAccessController) List(c *gin.Context) {
	result, err := controller.service.List(c.Request.Context(), auth.CurrentUserID(c))
	if err != nil {
		writeReportError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}

func (controller *ReportCategoryAccessController) Replace(c *gin.Context) {
	var request requestbody.ReportCategoryAccessSaveRequest
	if err := decodeMallJSON(c, &request); err != nil {
		writeReportError(c, data_svc.ErrReportInvalid)
		return
	}
	result, err := controller.service.Replace(c.Request.Context(), auth.CurrentUserID(c), request)
	if err != nil {
		writeReportError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}
