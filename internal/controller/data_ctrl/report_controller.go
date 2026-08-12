package data_ctrl

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"gin-biz-web-api/internal/reportrepo"
	"gin-biz-web-api/internal/reportsecret"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/internal/service/data_svc"
	"gin-biz-web-api/pkg/auth"
	"gin-biz-web-api/pkg/errcode"
	"gin-biz-web-api/pkg/responses"
)

type ReportDraftServiceAPI interface {
	Create(context.Context, uint, requestbody.ReportDraftSaveRequest) (*data_svc.ReportDraftDTO, error)
	Get(context.Context, uint, uint) (*data_svc.ReportDraftDTO, error)
	List(context.Context, uint, uint, int, string, string) (*data_svc.ReportDraftListDTO, error)
	Update(context.Context, uint, uint, requestbody.ReportDraftSaveRequest) (*data_svc.ReportDraftDTO, error)
}

type ReportController struct {
	service        ReportDraftServiceAPI
	publishService ReportPublishServiceAPI
}

type ReportPublishServiceAPI interface {
	Publish(context.Context, uint, uint, uint64) (*data_svc.ReportPublicationDTO, error)
}

func NewReportController() *ReportController {
	repository := reportrepo.New()
	return NewReportControllerWithServices(data_svc.NewReportDraftServiceWithStore(repository), data_svc.NewReportPublishService(repository, reportsecret.EnvironmentKeyring{}, data_svc.OpenReportOracle))
}

func NewReportControllerWithService(service ReportDraftServiceAPI) *ReportController {
	return NewReportControllerWithServices(service, nil)
}

func NewReportControllerWithServices(service ReportDraftServiceAPI, publishService ReportPublishServiceAPI) *ReportController {
	if service == nil {
		panic("report controller: nil service")
	}
	return &ReportController{service: service, publishService: publishService}
}

func (controller *ReportController) Create(c *gin.Context) {
	var request requestbody.ReportDraftSaveRequest
	if err := decodeMallJSON(c, &request); err != nil {
		writeReportError(c, fmt.Errorf("%w: invalid JSON body", data_svc.ErrReportInvalid))
		return
	}
	result, err := controller.service.Create(c.Request.Context(), auth.CurrentUserID(c), request)
	if err != nil {
		writeReportError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusCreated, result)
}

func (controller *ReportController) Get(c *gin.Context) {
	reportID, err := parseReportUint(c.Param("id"), "report id")
	if err != nil {
		writeReportError(c, err)
		return
	}
	result, err := controller.service.Get(c.Request.Context(), auth.CurrentUserID(c), reportID)
	if err != nil {
		writeReportError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}

func (controller *ReportController) List(c *gin.Context) {
	afterID, limit, err := parseReportListQuery(c)
	if err != nil {
		writeReportError(c, err)
		return
	}
	result, err := controller.service.List(
		c.Request.Context(), auth.CurrentUserID(c), afterID, limit,
		strings.TrimSpace(c.Query("category")), strings.TrimSpace(c.Query("search")),
	)
	if err != nil {
		writeReportError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}

func (controller *ReportController) Update(c *gin.Context) {
	reportID, err := parseReportUint(c.Param("id"), "report id")
	if err != nil {
		writeReportError(c, err)
		return
	}
	var request requestbody.ReportDraftSaveRequest
	if err := decodeMallJSON(c, &request); err != nil {
		writeReportError(c, fmt.Errorf("%w: invalid JSON body", data_svc.ErrReportInvalid))
		return
	}
	result, err := controller.service.Update(c.Request.Context(), auth.CurrentUserID(c), reportID, request)
	if err != nil {
		writeReportError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}

func (controller *ReportController) Publish(c *gin.Context) {
	if controller.publishService == nil {
		writeReportError(c, errors.New("report publication service is unavailable"))
		return
	}
	reportID, err := parseReportUint(c.Param("id"), "report id")
	if err != nil {
		writeReportError(c, err)
		return
	}
	var request requestbody.ReportPublishRequest
	if err := decodeMallJSON(c, &request); err != nil || request.ExpectedLockVersion == 0 {
		writeReportError(c, fmt.Errorf("%w: invalid publication request", data_svc.ErrReportInvalid))
		return
	}
	result, err := controller.publishService.Publish(c.Request.Context(), auth.CurrentUserID(c), reportID, request.ExpectedLockVersion)
	if err != nil {
		writeReportError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}

func parseReportListQuery(c *gin.Context) (uint, int, error) {
	var afterID uint
	var limit int
	var err error
	if value := strings.TrimSpace(c.Query("afterId")); value != "" {
		afterID, err = parseReportUint(value, "afterId")
		if err != nil {
			return 0, 0, err
		}
	}
	if value := strings.TrimSpace(c.Query("limit")); value != "" {
		parsed, parseErr := strconv.ParseUint(value, 10, 16)
		if parseErr != nil || parsed == 0 {
			return 0, 0, fmt.Errorf("%w: invalid limit", data_svc.ErrReportInvalid)
		}
		limit = int(parsed)
	}
	return afterID, limit, nil
}

func parseReportUint(value, field string) (uint, error) {
	parsed, err := strconv.ParseUint(strings.TrimSpace(value), 10, strconv.IntSize)
	if err != nil || parsed == 0 {
		return 0, fmt.Errorf("%w: invalid %s", data_svc.ErrReportInvalid, field)
	}
	return uint(parsed), nil
}

func writeReportError(c *gin.Context, err error) {
	code, message := classifyReportError(err)
	responses.New(c).ToSafeErrorResponse(code, message)
}

func classifyReportError(err error) (*errcode.Error, string) {
	switch {
	case errors.Is(err, data_svc.ErrReportNotFound):
		return errcode.NotFound, "报表草稿不存在"
	case errors.Is(err, data_svc.ErrReportConflict):
		return errcode.Conflict, "报表草稿已被修改或编码已存在，请刷新后重试"
	case errors.Is(err, data_svc.ErrReportInvalid):
		return errcode.UnprocessableEntity, "报表草稿参数校验失败"
	case errors.Is(err, data_svc.ErrReportPublicationInvalid):
		return errcode.UnprocessableEntity, "Oracle过程或结果表与报表配置不一致"
	default:
		return errcode.InternalServerError, "报表配置服务暂时不可用"
	}
}
