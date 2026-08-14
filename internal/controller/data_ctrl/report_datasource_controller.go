package data_ctrl

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"gin-biz-web-api/internal/reportoracle"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/internal/service/data_svc"
	"gin-biz-web-api/pkg/auth"
	"gin-biz-web-api/pkg/errcode"
	"gin-biz-web-api/pkg/responses"
)

type ReportDatasourceServiceAPI interface {
	List(context.Context, uint) ([]data_svc.ReportDatasourceDTO, error)
	Get(context.Context, uint, uint) (*data_svc.ReportDatasourceDTO, error)
	Create(context.Context, uint, requestbody.ReportDatasourceSaveRequest) (*data_svc.ReportDatasourceDTO, error)
	Update(context.Context, uint, uint, requestbody.ReportDatasourceSaveRequest) (*data_svc.ReportDatasourceDTO, error)
	Test(context.Context, uint, uint) (*data_svc.ReportDatasourceTestDTO, error)
	TestConnection(context.Context, uint, requestbody.ReportDatasourceConnectionTestRequest) (*data_svc.ReportDatasourceTestDTO, error)
	ListProcedures(context.Context, uint, uint, data_svc.ReportProcedureCatalogQuery) (*data_svc.ReportProcedurePageDTO, error)
	GetProcedureSignature(context.Context, uint, uint, reportoracle.ProcedureRef) (*data_svc.ReportProcedureSignatureDTO, error)
}

type ReportDatasourceController struct{ service ReportDatasourceServiceAPI }

func NewReportDatasourceController() *ReportDatasourceController {
	return NewReportDatasourceControllerWithService(data_svc.NewReportDatasourceService())
}

func NewReportDatasourceControllerWithService(service ReportDatasourceServiceAPI) *ReportDatasourceController {
	if service == nil {
		panic("report datasource controller: nil service")
	}
	return &ReportDatasourceController{service: service}
}

func (controller *ReportDatasourceController) List(c *gin.Context) {
	result, err := controller.service.List(c.Request.Context(), auth.CurrentUserID(c))
	if err != nil {
		writeReportDatasourceError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, map[string]interface{}{"items": result})
}

func (controller *ReportDatasourceController) Get(c *gin.Context) {
	datasourceID, err := parseReportUint(c.Param("id"), "datasource id")
	if err != nil {
		writeReportDatasourceError(c, err)
		return
	}
	result, err := controller.service.Get(c.Request.Context(), auth.CurrentUserID(c), datasourceID)
	if err != nil {
		writeReportDatasourceError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}

func (controller *ReportDatasourceController) Create(c *gin.Context) {
	var request requestbody.ReportDatasourceSaveRequest
	if err := decodeMallJSON(c, &request); err != nil {
		writeReportDatasourceError(c, fmt.Errorf("%w: invalid JSON body", data_svc.ErrReportDatasourceInvalid))
		return
	}
	result, err := controller.service.Create(c.Request.Context(), auth.CurrentUserID(c), request)
	if err != nil {
		writeReportDatasourceError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusCreated, result)
}

func (controller *ReportDatasourceController) Update(c *gin.Context) {
	datasourceID, err := parseReportUint(c.Param("id"), "datasource id")
	if err != nil {
		writeReportDatasourceError(c, err)
		return
	}
	var request requestbody.ReportDatasourceSaveRequest
	if err := decodeMallJSON(c, &request); err != nil {
		writeReportDatasourceError(c, fmt.Errorf("%w: invalid JSON body", data_svc.ErrReportDatasourceInvalid))
		return
	}
	result, err := controller.service.Update(c.Request.Context(), auth.CurrentUserID(c), datasourceID, request)
	if err != nil {
		writeReportDatasourceError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}

func (controller *ReportDatasourceController) Test(c *gin.Context) {
	datasourceID, err := parseReportUint(c.Param("id"), "datasource id")
	if err != nil {
		writeReportDatasourceError(c, err)
		return
	}
	result, err := controller.service.Test(c.Request.Context(), auth.CurrentUserID(c), datasourceID)
	if err != nil {
		writeReportDatasourceError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}

func (controller *ReportDatasourceController) TestConnection(c *gin.Context) {
	var request requestbody.ReportDatasourceConnectionTestRequest
	if err := decodeMallJSON(c, &request); err != nil {
		writeReportDatasourceError(c, fmt.Errorf("%w: invalid JSON body", data_svc.ErrReportDatasourceInvalid))
		return
	}
	result, err := controller.service.TestConnection(c.Request.Context(), auth.CurrentUserID(c), request)
	if err != nil {
		writeReportDatasourceError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}

func (controller *ReportDatasourceController) ListProcedures(c *gin.Context) {
	datasourceID, err := parseReportUint(c.Param("id"), "datasource id")
	if err != nil {
		writeReportDatasourceError(c, err)
		return
	}
	limit := 0
	if rawLimit := c.Query("limit"); rawLimit != "" {
		limit, err = strconv.Atoi(rawLimit)
		if err != nil {
			writeReportDatasourceError(c, data_svc.ErrReportDatasourceInvalid)
			return
		}
	}
	result, err := controller.service.ListProcedures(c.Request.Context(), auth.CurrentUserID(c), datasourceID, data_svc.ReportProcedureCatalogQuery{
		Owner: c.Query("owner"), Search: c.Query("search"), After: c.Query("after"), Limit: limit,
	})
	if err != nil {
		writeReportDatasourceError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}

func (controller *ReportDatasourceController) GetProcedureSignature(c *gin.Context) {
	datasourceID, err := parseReportUint(c.Param("id"), "datasource id")
	if err != nil {
		writeReportDatasourceError(c, err)
		return
	}
	result, err := controller.service.GetProcedureSignature(c.Request.Context(), auth.CurrentUserID(c), datasourceID, reportoracle.ProcedureRef{
		Owner: c.Query("owner"), Package: c.Query("package"), Name: c.Query("name"), Overload: c.Query("overload"),
	})
	if err != nil {
		writeReportDatasourceError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}

func writeReportDatasourceError(c *gin.Context, err error) {
	if c != nil && err != nil {
		_ = c.Error(err).SetType(gin.ErrorTypePrivate)
	}
	code, message := classifyReportDatasourceError(err)
	responses.New(c).ToSafeErrorResponse(code, message)
}

func classifyReportDatasourceError(err error) (*errcode.Error, string) {
	switch {
	case errors.Is(err, data_svc.ErrReportDatasourceNotFound):
		return errcode.NotFound, "报表数据源不存在"
	case errors.Is(err, data_svc.ErrReportDatasourceConflict):
		return errcode.Conflict, "报表数据源编码已存在，或连接仍被未清理的运行使用"
	case errors.Is(err, data_svc.ErrReportDatasourceInvalid):
		return errcode.UnprocessableEntity, "报表数据源参数校验失败"
	case errors.Is(err, data_svc.ErrReportDatasourceCredentialUnavailable):
		return errcode.ServiceUnavailable, "报表凭据加密配置不可用，请联系管理员"
	case errors.Is(err, data_svc.ErrReportDatasourceOracleUnavailable):
		return errcode.ServiceUnavailable, "Oracle 元数据服务暂时不可用，请确认数据源已启用且连接正常"
	default:
		return errcode.InternalServerError, "报表数据源服务暂时不可用"
	}
}
