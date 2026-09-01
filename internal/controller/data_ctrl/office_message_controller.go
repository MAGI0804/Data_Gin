package data_ctrl

import (
	"errors"
	"net/http"
	"strconv"

	"gin-biz-web-api/internal/reportoracle"
	"gin-biz-web-api/internal/service/data_svc"
	"gin-biz-web-api/pkg/auth"
	"gin-biz-web-api/pkg/errcode"
	"gin-biz-web-api/pkg/responses"

	"github.com/gin-gonic/gin"
)

type OfficeMessageController struct {
	service  *data_svc.OfficeMessageService
	metadata *data_svc.OfficeOracleMetadataService
}

func NewOfficeMessageController() *OfficeMessageController {
	return &OfficeMessageController{service: data_svc.NewOfficeMessageService(), metadata: data_svc.NewOfficeOracleMetadataService()}
}

func (controller *OfficeMessageController) ListMessages(c *gin.Context) {
	items, err := controller.service.ListMessages(c.Request.Context())
	if err != nil {
		writeOfficeMessageError(c, err)
		return
	}
	responses.New(c).ToResponse(gin.H{"items": items})
}

func (controller *OfficeMessageController) CreateMessage(c *gin.Context) {
	var input data_svc.OfficeMessageInput
	if err := c.ShouldBindJSON(&input); err != nil {
		writeOfficeMessageError(c, data_svc.ErrOfficeMessageInvalid)
		return
	}
	item, err := controller.service.CreateMessage(c.Request.Context(), auth.CurrentUserID(c), input)
	if err != nil {
		writeOfficeMessageError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusCreated, item)
}

func (controller *OfficeMessageController) UpdateMessage(c *gin.Context) {
	id, err := officeUint(c.Param("id"))
	if err != nil {
		writeOfficeMessageError(c, err)
		return
	}
	var input data_svc.OfficeMessageInput
	if err := c.ShouldBindJSON(&input); err != nil {
		writeOfficeMessageError(c, data_svc.ErrOfficeMessageInvalid)
		return
	}
	item, err := controller.service.UpdateMessage(c.Request.Context(), auth.CurrentUserID(c), id, input)
	if err != nil {
		writeOfficeMessageError(c, err)
		return
	}
	responses.New(c).ToResponse(item)
}

func (controller *OfficeMessageController) DeleteMessage(c *gin.Context) {
	id, err := officeUint(c.Param("id"))
	if err != nil {
		writeOfficeMessageError(c, err)
		return
	}
	lockVersion, err := officeUint64(c.Query("expectedLockVersion"))
	if err != nil {
		writeOfficeMessageError(c, err)
		return
	}
	if err := controller.service.DeleteMessage(c.Request.Context(), id, lockVersion); err != nil {
		writeOfficeMessageError(c, err)
		return
	}
	responses.New(c).ToResponse(gin.H{"id": id})
}

func (controller *OfficeMessageController) ListTargets(c *gin.Context) {
	items, err := controller.service.ListTargets(c.Request.Context())
	if err != nil {
		writeOfficeMessageError(c, err)
		return
	}
	responses.New(c).ToResponse(gin.H{"items": items})
}

func (controller *OfficeMessageController) CreateTarget(c *gin.Context) {
	var input data_svc.OfficePushTargetInput
	if err := c.ShouldBindJSON(&input); err != nil {
		writeOfficeMessageError(c, data_svc.ErrOfficeMessageInvalid)
		return
	}
	item, err := controller.service.CreateTarget(c.Request.Context(), auth.CurrentUserID(c), input)
	if err != nil {
		writeOfficeMessageError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusCreated, item)
}

func (controller *OfficeMessageController) UpdateTarget(c *gin.Context) {
	id, err := officeUint(c.Param("id"))
	if err != nil {
		writeOfficeMessageError(c, err)
		return
	}
	var input data_svc.OfficePushTargetInput
	if err := c.ShouldBindJSON(&input); err != nil {
		writeOfficeMessageError(c, data_svc.ErrOfficeMessageInvalid)
		return
	}
	item, err := controller.service.UpdateTarget(c.Request.Context(), auth.CurrentUserID(c), id, input)
	if err != nil {
		writeOfficeMessageError(c, err)
		return
	}
	responses.New(c).ToResponse(item)
}

func (controller *OfficeMessageController) DeleteTarget(c *gin.Context) {
	id, err := officeUint(c.Param("id"))
	if err != nil {
		writeOfficeMessageError(c, err)
		return
	}
	lockVersion, err := officeUint64(c.Query("expectedLockVersion"))
	if err != nil {
		writeOfficeMessageError(c, err)
		return
	}
	if err := controller.service.DeleteTarget(c.Request.Context(), id, lockVersion); err != nil {
		writeOfficeMessageError(c, err)
		return
	}
	responses.New(c).ToResponse(gin.H{"id": id})
}

func (controller *OfficeMessageController) CreateRun(c *gin.Context) {
	targetID, err := officeUint(c.Param("id"))
	if err != nil {
		writeOfficeMessageError(c, err)
		return
	}
	var input data_svc.OfficePushRunInput
	if err := c.ShouldBindJSON(&input); err != nil {
		writeOfficeMessageError(c, data_svc.ErrOfficeMessageInvalid)
		return
	}
	run, err := controller.service.CreateRun(c.Request.Context(), auth.CurrentUserID(c), targetID, input)
	if err != nil {
		writeOfficeMessageError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusAccepted, run)
}

func (controller *OfficeMessageController) ListRuns(c *gin.Context) {
	limit := 100
	if raw := c.Query("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 500 {
			writeOfficeMessageError(c, data_svc.ErrOfficeMessageInvalid)
			return
		}
		limit = parsed
	}
	items, err := controller.service.ListRuns(c.Request.Context(), limit)
	if err != nil {
		writeOfficeMessageError(c, err)
		return
	}
	responses.New(c).ToResponse(gin.H{"items": items})
}

func (controller *OfficeMessageController) ListProcedures(c *gin.Context) {
	items, err := controller.metadata.ListProcedures(c.Request.Context(), c.Query("owner"), c.Query("search"), officeLimit(c))
	if err != nil {
		writeOfficeMessageError(c, err)
		return
	}
	responses.New(c).ToResponse(gin.H{"items": items})
}

func (controller *OfficeMessageController) ProcedureSignature(c *gin.Context) {
	items, err := controller.metadata.ProcedureSignature(c.Request.Context(), reportoracle.ProcedureRef{
		Owner: c.Query("owner"), Package: c.Query("package"), Name: c.Query("name"), Overload: c.Query("overload"),
	})
	if err != nil {
		writeOfficeMessageError(c, err)
		return
	}
	responses.New(c).ToResponse(gin.H{"items": items})
}

func (controller *OfficeMessageController) ListResultTables(c *gin.Context) {
	items, err := controller.metadata.ListResultTables(c.Request.Context(), c.Query("owner"), c.Query("search"), officeLimit(c))
	if err != nil {
		writeOfficeMessageError(c, err)
		return
	}
	responses.New(c).ToResponse(gin.H{"items": items})
}

func (controller *OfficeMessageController) ResultTableSchema(c *gin.Context) {
	items, err := controller.metadata.ResultTableSchema(c.Request.Context(), reportoracle.ResultTableRef{Owner: c.Query("owner"), Name: c.Query("name")})
	if err != nil {
		writeOfficeMessageError(c, err)
		return
	}
	responses.New(c).ToResponse(gin.H{"items": items})
}

func (controller *OfficeMessageController) TestSelect(c *gin.Context) {
	var input data_svc.OfficeSelectTestInput
	if err := c.ShouldBindJSON(&input); err != nil {
		writeOfficeMessageError(c, data_svc.ErrOfficeMessageInvalid)
		return
	}
	items, err := controller.metadata.TestSelect(c.Request.Context(), input)
	if err != nil {
		writeOfficeMessageError(c, err)
		return
	}
	responses.New(c).ToResponse(gin.H{"columns": items})
}

func officeLimit(c *gin.Context) int {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if err != nil || limit < 1 || limit > 100 {
		return 50
	}
	return limit
}

func officeUint(value string) (uint, error) {
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil || parsed == 0 || uint64(uint(parsed)) != parsed {
		return 0, data_svc.ErrOfficeMessageInvalid
	}
	return uint(parsed), nil
}

func officeUint64(value string) (uint64, error) {
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil || parsed == 0 {
		return 0, data_svc.ErrOfficeMessageInvalid
	}
	return parsed, nil
}

func writeOfficeMessageError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, data_svc.ErrOfficeMessageInvalid), errors.Is(err, reportoracle.ErrInvalidConfiguration):
		responses.New(c).ToSafeErrorResponse(errcode.BadRequest, "办公消息参数无效")
	case errors.Is(err, data_svc.ErrOfficeMessageNotFound):
		responses.New(c).ToSafeErrorResponse(errcode.NotFound, "办公消息或推送目标不存在")
	case errors.Is(err, data_svc.ErrOfficeMessageConflict):
		responses.New(c).ToSafeErrorResponse(errcode.Conflict, "配置已变化或仍被引用，请刷新后重试")
	default:
		responses.New(c).ToSafeErrorResponse(errcode.InternalServerError, "办公消息服务暂时不可用")
	}
}
