package data_ctrl

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/internal/service/data_svc"
	"gin-biz-web-api/pkg/auth"
	"gin-biz-web-api/pkg/errcode"
	"gin-biz-web-api/pkg/responses"
)

type ReportInputQueryServiceAPI interface {
	List(context.Context, uint) (*data_svc.ReportInputQueryListDTO, error)
	Options(context.Context, uint, uint, string, string) (*data_svc.ReportInputOptionListDTO, error)
	ListDefinitions(context.Context, uint) (*data_svc.ReportInputQueryDefinitionListDTO, error)
	GetDefinition(context.Context, uint, uint) (*data_svc.ReportInputQueryDefinitionDTO, error)
	CreateDefinition(context.Context, uint, requestbody.ReportInputQueryDefinitionSaveRequest) (*data_svc.ReportInputQueryDefinitionDTO, error)
	UpdateDefinition(context.Context, uint, uint, requestbody.ReportInputQueryDefinitionSaveRequest) (*data_svc.ReportInputQueryDefinitionDTO, error)
	DeleteDefinition(context.Context, uint, uint, uint64) (*data_svc.ReportInputQueryDefinitionDeleteDTO, error)
	TestDefinition(context.Context, uint, uint, requestbody.ReportInputQueryDefinitionTestRequest) (*data_svc.ReportInputQueryTestDTO, error)
	TestDefinitionDraft(context.Context, uint, requestbody.ReportInputQueryDefinitionTestRequest) (*data_svc.ReportInputQueryTestDTO, error)
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
	if !controller.ready(c) {
		return
	}
	result, err := controller.service.List(c.Request.Context(), auth.CurrentUserID(c))
	if err != nil {
		writeReportError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}

func (controller *ReportInputQueryController) Options(c *gin.Context) {
	if !controller.ready(c) {
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

func (controller *ReportInputQueryController) ListDefinitions(c *gin.Context) {
	if !controller.ready(c) {
		return
	}
	result, err := controller.service.ListDefinitions(c.Request.Context(), auth.CurrentUserID(c))
	if err != nil {
		writeReportInputQueryDefinitionError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}

func (controller *ReportInputQueryController) GetDefinition(c *gin.Context) {
	definitionID, ok := controller.definitionID(c)
	if !ok {
		return
	}
	result, err := controller.service.GetDefinition(c.Request.Context(), auth.CurrentUserID(c), definitionID)
	if err != nil {
		writeReportInputQueryDefinitionError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}

func (controller *ReportInputQueryController) CreateDefinition(c *gin.Context) {
	if !controller.ready(c) {
		return
	}
	var request requestbody.ReportInputQueryDefinitionSaveRequest
	if err := decodeMallJSON(c, &request); err != nil {
		writeReportInputQueryDefinitionError(c, fmt.Errorf("%w: invalid JSON body", data_svc.ErrReportInputQueryInvalid))
		return
	}
	result, err := controller.service.CreateDefinition(c.Request.Context(), auth.CurrentUserID(c), request)
	if err != nil {
		writeReportInputQueryDefinitionError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusCreated, result)
}

func (controller *ReportInputQueryController) UpdateDefinition(c *gin.Context) {
	definitionID, ok := controller.definitionID(c)
	if !ok {
		return
	}
	var request requestbody.ReportInputQueryDefinitionSaveRequest
	if err := decodeMallJSON(c, &request); err != nil {
		writeReportInputQueryDefinitionError(c, fmt.Errorf("%w: invalid JSON body", data_svc.ErrReportInputQueryInvalid))
		return
	}
	result, err := controller.service.UpdateDefinition(c.Request.Context(), auth.CurrentUserID(c), definitionID, request)
	if err != nil {
		writeReportInputQueryDefinitionError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}

func (controller *ReportInputQueryController) DeleteDefinition(c *gin.Context) {
	definitionID, ok := controller.definitionID(c)
	if !ok {
		return
	}
	expectedLockVersion, err := strconv.ParseUint(strings.TrimSpace(c.Query("expectedLockVersion")), 10, 64)
	if err != nil || expectedLockVersion == 0 {
		writeReportInputQueryDefinitionError(c, data_svc.ErrReportInputQueryInvalid)
		return
	}
	result, err := controller.service.DeleteDefinition(c.Request.Context(), auth.CurrentUserID(c), definitionID, expectedLockVersion)
	if err != nil {
		writeReportInputQueryDefinitionError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}

func (controller *ReportInputQueryController) TestDefinition(c *gin.Context) {
	definitionID, ok := controller.definitionID(c)
	if !ok {
		return
	}
	var request requestbody.ReportInputQueryDefinitionTestRequest
	if err := decodeMallJSON(c, &request); err != nil {
		writeReportInputQueryDefinitionError(c, fmt.Errorf("%w: invalid JSON body", data_svc.ErrReportInputQueryInvalid))
		return
	}
	result, err := controller.service.TestDefinition(c.Request.Context(), auth.CurrentUserID(c), definitionID, request)
	if err != nil {
		writeReportInputQueryDefinitionError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}

func (controller *ReportInputQueryController) TestDefinitionDraft(c *gin.Context) {
	if !controller.ready(c) {
		return
	}
	var request requestbody.ReportInputQueryDefinitionTestRequest
	if err := decodeMallJSON(c, &request); err != nil {
		writeReportInputQueryDefinitionError(c, fmt.Errorf("%w: invalid JSON body", data_svc.ErrReportInputQueryInvalid))
		return
	}
	result, err := controller.service.TestDefinitionDraft(c.Request.Context(), auth.CurrentUserID(c), request)
	if err != nil {
		writeReportInputQueryDefinitionError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}

func (controller *ReportInputQueryController) definitionID(c *gin.Context) (uint, bool) {
	if !controller.ready(c) {
		return 0, false
	}
	definitionID, err := parseReportUint(c.Param("id"), "input query definition id")
	if err != nil {
		writeReportInputQueryDefinitionError(c, data_svc.ErrReportInputQueryInvalid)
		return 0, false
	}
	return definitionID, true
}

func (controller *ReportInputQueryController) ready(c *gin.Context) bool {
	if controller.initErr == nil && controller.service != nil {
		return true
	}
	writeReportInputQueryDefinitionError(c, fmt.Errorf("%w: configuration is invalid", data_svc.ErrReportInputQueryUnavailable))
	return false
}

func writeReportInputQueryDefinitionError(c *gin.Context, err error) {
	if c != nil && err != nil {
		_ = c.Error(err).SetType(gin.ErrorTypePrivate)
	}
	switch {
	case errors.Is(err, data_svc.ErrReportInputQueryNotFound):
		responses.New(c).ToSafeErrorResponse(errcode.NotFound, "输入选项查询不存在")
	case errors.Is(err, data_svc.ErrReportInputQueryConflict):
		responses.New(c).ToSafeErrorResponse(errcode.Conflict, "查询名称已存在、配置已更新，或仍被报表引用")
	case errors.Is(err, data_svc.ErrReportInputQueryInvalid):
		responses.New(c).ToSafeErrorResponse(errcode.UnprocessableEntity, "输入选项查询参数校验失败")
	case errors.Is(err, data_svc.ErrReportInputQueryUnavailable):
		responses.New(c).ToSafeErrorResponse(errcode.ServiceUnavailable, "默认 Oracle 输入查询服务暂时不可用")
	default:
		responses.New(c).ToSafeErrorResponse(errcode.InternalServerError, "输入选项查询配置服务暂时不可用")
	}
}
