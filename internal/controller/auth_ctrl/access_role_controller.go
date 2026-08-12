package auth_ctrl

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math/bits"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"gin-biz-web-api/internal/requests/auth_request"
	"gin-biz-web-api/internal/service/auth_svc"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/auth"
	"gin-biz-web-api/pkg/errcode"
	"gin-biz-web-api/pkg/responses"
)

const maxAccessRoleBodyBytes int64 = 64 << 10

type AccessRoleService interface {
	PermissionCatalog(context.Context, uint) ([]model.Permission, error)
	ListRoles(context.Context, uint) ([]auth_svc.AccessRoleDTO, error)
	CreateRole(context.Context, uint, string, auth_request.AccessRoleCreateRequest) (*auth_svc.AccessRoleMutationResult, error)
	UpdateRole(context.Context, uint, uint, string, auth_request.AccessRoleUpdateRequest) (*auth_svc.AccessRoleMutationResult, error)
	SetRoleStatus(context.Context, uint, uint, string, auth_request.AccessRoleStatusRequest) (*auth_svc.AccessRoleMutationResult, error)
	ReplacePermissions(context.Context, uint, uint, string, auth_request.AccessRolePermissionsRequest) (*auth_svc.AccessRoleMutationResult, error)
	DeleteRole(context.Context, uint, uint, string, auth_request.AccessRoleDeleteRequest) (*auth_svc.AccessRoleMutationResult, error)
	QueryAudits(context.Context, uint, auth_request.AccessAuditQueryRequest) (*auth_svc.AccessAuditQueryResult, error)
}

type AccessRoleController struct{ service AccessRoleService }

func NewAccessRoleController() *AccessRoleController {
	return NewAccessRoleControllerWithService(auth_svc.NewAccessRoleService())
}

func NewAccessRoleControllerWithService(service AccessRoleService) *AccessRoleController {
	if service == nil {
		panic("access role controller: nil service")
	}
	return &AccessRoleController{service: service}
}

func (c *AccessRoleController) PermissionCatalog(ctx *gin.Context) {
	result, err := c.service.PermissionCatalog(ctx.Request.Context(), auth.CurrentUserID(ctx))
	writeAccessRoleResult(ctx, result, err, http.StatusOK)
}
func (c *AccessRoleController) ListRoles(ctx *gin.Context) {
	result, err := c.service.ListRoles(ctx.Request.Context(), auth.CurrentUserID(ctx))
	writeAccessRoleResult(ctx, result, err, http.StatusOK)
}
func (c *AccessRoleController) CreateRole(ctx *gin.Context) {
	var request auth_request.AccessRoleCreateRequest
	if decodeAccessRoleJSON(ctx, &request) != nil {
		writeAccessRoleError(ctx, auth_svc.ErrAccessRoleInvalidInput)
		return
	}
	result, err := c.service.CreateRole(ctx.Request.Context(), auth.CurrentUserID(ctx), ctx.GetHeader("Idempotency-Key"), request)
	writeAccessRoleMutation(ctx, result, err, http.StatusCreated)
}
func (c *AccessRoleController) UpdateRole(ctx *gin.Context) {
	id, ok := accessRoleID(ctx)
	if !ok {
		return
	}
	var request auth_request.AccessRoleUpdateRequest
	if decodeAccessRoleJSON(ctx, &request) != nil {
		writeAccessRoleError(ctx, auth_svc.ErrAccessRoleInvalidInput)
		return
	}
	result, err := c.service.UpdateRole(ctx.Request.Context(), auth.CurrentUserID(ctx), id, ctx.GetHeader("Idempotency-Key"), request)
	writeAccessRoleMutation(ctx, result, err, http.StatusOK)
}
func (c *AccessRoleController) SetRoleStatus(ctx *gin.Context) {
	id, ok := accessRoleID(ctx)
	if !ok {
		return
	}
	var request auth_request.AccessRoleStatusRequest
	if decodeAccessRoleJSON(ctx, &request) != nil {
		writeAccessRoleError(ctx, auth_svc.ErrAccessRoleInvalidInput)
		return
	}
	result, err := c.service.SetRoleStatus(ctx.Request.Context(), auth.CurrentUserID(ctx), id, ctx.GetHeader("Idempotency-Key"), request)
	writeAccessRoleMutation(ctx, result, err, http.StatusOK)
}
func (c *AccessRoleController) ReplacePermissions(ctx *gin.Context) {
	id, ok := accessRoleID(ctx)
	if !ok {
		return
	}
	var request auth_request.AccessRolePermissionsRequest
	if decodeAccessRoleJSON(ctx, &request) != nil {
		writeAccessRoleError(ctx, auth_svc.ErrAccessRoleInvalidInput)
		return
	}
	result, err := c.service.ReplacePermissions(ctx.Request.Context(), auth.CurrentUserID(ctx), id, ctx.GetHeader("Idempotency-Key"), request)
	writeAccessRoleMutation(ctx, result, err, http.StatusOK)
}
func (c *AccessRoleController) DeleteRole(ctx *gin.Context) {
	id, ok := accessRoleID(ctx)
	if !ok {
		return
	}
	var request auth_request.AccessRoleDeleteRequest
	if decodeAccessRoleJSON(ctx, &request) != nil {
		writeAccessRoleError(ctx, auth_svc.ErrAccessRoleInvalidInput)
		return
	}
	result, err := c.service.DeleteRole(ctx.Request.Context(), auth.CurrentUserID(ctx), id, ctx.GetHeader("Idempotency-Key"), request)
	writeAccessRoleMutation(ctx, result, err, http.StatusOK)
}
func (c *AccessRoleController) QueryAudits(ctx *gin.Context) {
	var request auth_request.AccessAuditQueryRequest
	if err := ctx.ShouldBindQuery(&request); err != nil {
		writeAccessRoleError(ctx, auth_svc.ErrAccessRoleInvalidInput)
		return
	}
	result, err := c.service.QueryAudits(ctx.Request.Context(), auth.CurrentUserID(ctx), request)
	writeAccessRoleResult(ctx, result, err, http.StatusOK)
}

func accessRoleID(c *gin.Context) (uint, bool) {
	parsed, err := strconv.ParseUint(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || parsed == 0 || (bits.UintSize == 32 && parsed > uint64(^uint(0))) {
		writeAccessRoleError(c, auth_svc.ErrAccessRoleInvalidInput)
		return 0, false
	}
	return uint(parsed), true
}
func decodeAccessRoleJSON(c *gin.Context, destination interface{}) error {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxAccessRoleBodyBytes)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}
func writeAccessRoleMutation(c *gin.Context, result *auth_svc.AccessRoleMutationResult, err error, status int) {
	if result != nil && result.Replayed {
		c.Header("Idempotency-Replayed", "true")
	}
	writeAccessRoleResult(c, result, err, status)
}
func writeAccessRoleResult(c *gin.Context, result interface{}, err error, status int) {
	if err != nil {
		writeAccessRoleError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(status, result)
}
func writeAccessRoleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, auth_svc.ErrAccessRoleForbidden):
		responses.New(c).ToSafeErrorResponse(errcode.Forbidden, "无权管理角色或授予该权限")
	case errors.Is(err, auth_svc.ErrAccessRoleNotFound):
		responses.New(c).ToSafeErrorResponse(errcode.NotFound, "角色不存在")
	case errors.Is(err, auth_svc.ErrAccessRoleConflict):
		responses.New(c).ToSafeErrorResponse(errcode.Conflict, "角色请求冲突，请刷新后重试")
	case errors.Is(err, auth_svc.ErrAccessRoleInvalidInput):
		responses.New(c).ToSafeErrorResponse(errcode.UnprocessableEntity, "角色请求参数校验失败")
	default:
		responses.New(c).ToSafeErrorResponse(errcode.ServiceUnavailable, "角色权限服务暂时不可用")
	}
}
func accessRoleTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, 5*time.Second)
}
