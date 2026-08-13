package data_ctrl

import (
	"context"
	"errors"
	"math/bits"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"gin-biz-web-api/internal/service/data_svc"
	"gin-biz-web-api/pkg/errcode"
	"gin-biz-web-api/pkg/responses"
)

type ReportAuditServiceAPI interface {
	List(context.Context, data_svc.ReportAuditQuery) (*data_svc.ReportAuditListDTO, error)
}

type ReportAuditController struct {
	service ReportAuditServiceAPI
}

func NewReportAuditController() *ReportAuditController {
	return NewReportAuditControllerWithService(data_svc.NewReportAuditService())
}

func NewReportAuditControllerWithService(service ReportAuditServiceAPI) *ReportAuditController {
	if service == nil {
		panic("report audit controller: service is required")
	}
	return &ReportAuditController{service: service}
}

func (controller *ReportAuditController) List(c *gin.Context) {
	query := data_svc.ReportAuditQuery{Limit: 50, Action: c.Query("action"), TargetType: c.Query("targetType")}
	var err error
	if query.AfterID, err = optionalReportAuditUint(c.Query("afterId")); err != nil {
		writeReportAuditError(c, data_svc.ErrReportAuditQueryInvalid)
		return
	}
	if query.TargetID, err = optionalReportAuditUint(c.Query("targetId")); err != nil {
		writeReportAuditError(c, data_svc.ErrReportAuditQueryInvalid)
		return
	}
	if value := strings.TrimSpace(c.Query("limit")); value != "" {
		parsed, parseErr := strconv.ParseUint(value, 10, 16)
		if parseErr != nil || parsed == 0 || parsed > 100 {
			writeReportAuditError(c, data_svc.ErrReportAuditQueryInvalid)
			return
		}
		query.Limit = int(parsed)
	}
	result, err := controller.service.List(c.Request.Context(), query)
	if err != nil {
		writeReportAuditError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}

func optionalReportAuditUint(value string) (uint, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil || parsed == 0 || bits.UintSize == 32 && parsed > uint64(^uint(0)) {
		return 0, data_svc.ErrReportAuditQueryInvalid
	}
	return uint(parsed), nil
}

func writeReportAuditError(c *gin.Context, err error) {
	if errors.Is(err, data_svc.ErrReportAuditQueryInvalid) {
		responses.New(c).ToSafeErrorResponse(errcode.UnprocessableEntity, "报表审计查询参数校验失败")
		return
	}
	responses.New(c).ToSafeErrorResponse(errcode.ServiceUnavailable, "报表审计服务暂时不可用")
}
