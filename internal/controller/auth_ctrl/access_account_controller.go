package auth_ctrl

import (
	"errors"
	"net/http"

	"gin-biz-web-api/internal/requests/auth_request"
	"gin-biz-web-api/internal/service/auth_svc"
	"gin-biz-web-api/pkg/auth"
	"gin-biz-web-api/pkg/errcode"
	"gin-biz-web-api/pkg/responses"

	"github.com/gin-gonic/gin"
)

type AccessAccountController struct {
	service *auth_svc.AccessAccountService
}

func NewAccessAccountController() *AccessAccountController {
	return &AccessAccountController{service: auth_svc.NewAccessAccountService()}
}

func (controller *AccessAccountController) Query(c *gin.Context) {
	var request auth_request.AccessAccountQueryRequest
	if decodeDataAuthorizationJSON(c, &request) != nil {
		writeAccessAccountError(c, auth_svc.ErrAccessAccountInvalid)
		return
	}
	result, err := controller.service.Query(c.Request.Context(), auth.CurrentUserID(c), request)
	writeAccessAccountResult(c, result, err, http.StatusOK)
}

func (controller *AccessAccountController) Create(c *gin.Context) {
	var request auth_request.AccessAccountCreateRequest
	if decodeDataAuthorizationJSON(c, &request) != nil {
		writeAccessAccountError(c, auth_svc.ErrAccessAccountInvalid)
		return
	}
	result, err := controller.service.Create(c.Request.Context(), auth.CurrentUserID(c), c.GetHeader("Idempotency-Key"), request)
	writeAccessAccountResult(c, result, err, http.StatusCreated)
}

func (controller *AccessAccountController) Update(c *gin.Context) {
	id, err := parseDataAuthorizationID(c.Param("id"))
	if err != nil {
		writeAccessAccountError(c, auth_svc.ErrAccessAccountInvalid)
		return
	}
	var request auth_request.AccessAccountUpdateRequest
	if decodeDataAuthorizationJSON(c, &request) != nil {
		writeAccessAccountError(c, auth_svc.ErrAccessAccountInvalid)
		return
	}
	result, err := controller.service.Update(c.Request.Context(), auth.CurrentUserID(c), id, c.GetHeader("Idempotency-Key"), request)
	writeAccessAccountResult(c, result, err, http.StatusOK)
}

func (controller *AccessAccountController) SetStatus(c *gin.Context) {
	id, ok := accessAccountID(c)
	if !ok {
		return
	}
	var request auth_request.AccessAccountStatusRequest
	if decodeDataAuthorizationJSON(c, &request) != nil {
		writeAccessAccountError(c, auth_svc.ErrAccessAccountInvalid)
		return
	}
	err := controller.service.SetStatus(c.Request.Context(), auth.CurrentUserID(c), id, c.GetHeader("Idempotency-Key"), request)
	writeAccessAccountResult(c, gin.H{"changed": err == nil}, err, http.StatusOK)
}

func (controller *AccessAccountController) ResetPassword(c *gin.Context) {
	id, ok := accessAccountID(c)
	if !ok {
		return
	}
	var request auth_request.AccessAccountPasswordResetRequest
	if decodeDataAuthorizationJSON(c, &request) != nil {
		writeAccessAccountError(c, auth_svc.ErrAccessAccountInvalid)
		return
	}
	err := controller.service.ResetPassword(c.Request.Context(), auth.CurrentUserID(c), id, c.GetHeader("Idempotency-Key"), request)
	writeAccessAccountResult(c, gin.H{"changed": err == nil}, err, http.StatusOK)
}

func (controller *AccessAccountController) ReplaceRoles(c *gin.Context) {
	id, ok := accessAccountID(c)
	if !ok {
		return
	}
	var request auth_request.AccessAccountRolesRequest
	if decodeDataAuthorizationJSON(c, &request) != nil {
		writeAccessAccountError(c, auth_svc.ErrAccessAccountInvalid)
		return
	}
	err := controller.service.ReplaceRoles(c.Request.Context(), auth.CurrentUserID(c), id, c.GetHeader("Idempotency-Key"), request)
	writeAccessAccountResult(c, gin.H{"changed": err == nil}, err, http.StatusOK)
}

func (controller *AccessAccountController) ReplaceMallScope(c *gin.Context) {
	id, ok := accessAccountID(c)
	if !ok {
		return
	}
	var request auth_request.AccessAccountMallScopeRequest
	if decodeDataAuthorizationJSON(c, &request) != nil {
		writeAccessAccountError(c, auth_svc.ErrAccessAccountInvalid)
		return
	}
	err := controller.service.ReplaceMallScope(c.Request.Context(), auth.CurrentUserID(c), id, c.GetHeader("Idempotency-Key"), request)
	writeAccessAccountResult(c, gin.H{"changed": err == nil}, err, http.StatusOK)
}

func (controller *AccessAccountController) ListReportCategories(c *gin.Context) {
	id, ok := accessAccountID(c)
	if !ok {
		return
	}
	result, err := controller.service.ListReportCategories(c.Request.Context(), auth.CurrentUserID(c), id)
	writeAccessAccountResult(c, result, err, http.StatusOK)
}

func (controller *AccessAccountController) ReplaceReportCategory(c *gin.Context) {
	id, ok := accessAccountID(c)
	if !ok {
		return
	}
	var request auth_request.AccessAccountReportCategoryRequest
	if decodeDataAuthorizationJSON(c, &request) != nil {
		writeAccessAccountError(c, auth_svc.ErrAccessAccountInvalid)
		return
	}
	err := controller.service.ReplaceReportCategory(c.Request.Context(), auth.CurrentUserID(c), id, c.GetHeader("Idempotency-Key"), request)
	writeAccessAccountResult(c, gin.H{"changed": err == nil}, err, http.StatusOK)
}

func accessAccountID(c *gin.Context) (uint, bool) {
	id, err := parseDataAuthorizationID(c.Param("id"))
	if err != nil {
		writeAccessAccountError(c, auth_svc.ErrAccessAccountInvalid)
		return 0, false
	}
	return id, true
}

func writeAccessAccountResult(c *gin.Context, result interface{}, err error, status int) {
	if err != nil {
		writeAccessAccountError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(status, result)
}

func writeAccessAccountError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, auth_svc.ErrAccessAccountForbidden):
		responses.New(c).ToSafeErrorResponse(errcode.Forbidden, "无权管理该账号")
	case errors.Is(err, auth_svc.ErrAccessAccountNotFound):
		responses.New(c).ToSafeErrorResponse(errcode.NotFound, "账号不存在")
	case errors.Is(err, auth_svc.ErrAccessAccountConflict):
		responses.New(c).ToSafeErrorResponse(errcode.Conflict, "账号请求冲突")
	case errors.Is(err, auth_svc.ErrAccessAccountInvalid):
		responses.New(c).ToSafeErrorResponse(errcode.UnprocessableEntity, "账号请求参数校验失败")
	default:
		responses.New(c).ToSafeErrorResponse(errcode.InternalServerError, "账号管理服务暂时不可用")
	}
}
