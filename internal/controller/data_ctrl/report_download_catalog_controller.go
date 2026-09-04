package data_ctrl

import (
	"context"
	"net/http"
	"strings"

	"gin-biz-web-api/internal/service/data_svc"
	"gin-biz-web-api/pkg/auth"
	"gin-biz-web-api/pkg/responses"

	"github.com/gin-gonic/gin"
)

type ReportDownloadCatalogServiceAPI interface {
	List(context.Context, uint, uint, int, string, string) (*data_svc.ReportDraftListDTO, error)
}

type ReportDownloadCatalogController struct {
	service ReportDownloadCatalogServiceAPI
}

func NewReportDownloadCatalogController() *ReportDownloadCatalogController {
	return NewReportDownloadCatalogControllerWithService(data_svc.NewReportDownloadCatalogService())
}

func NewReportDownloadCatalogControllerWithService(service ReportDownloadCatalogServiceAPI) *ReportDownloadCatalogController {
	if service == nil {
		panic("report download catalog controller: nil service")
	}
	return &ReportDownloadCatalogController{service: service}
}

func (controller *ReportDownloadCatalogController) List(c *gin.Context) {
	afterID, limit, err := parseReportListQuery(c)
	if err != nil {
		writeReportError(c, err)
		return
	}
	result, err := controller.service.List(
		c.Request.Context(),
		auth.CurrentUserID(c),
		afterID,
		limit,
		strings.TrimSpace(c.Query("category")),
		strings.TrimSpace(c.Query("search")),
	)
	if err != nil {
		writeReportError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}
