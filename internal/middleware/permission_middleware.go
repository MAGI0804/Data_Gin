package middleware

import (
	"context"

	"github.com/gin-gonic/gin"

	"gin-biz-web-api/constant"
	"gin-biz-web-api/internal/service/auth_svc"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"
	"gin-biz-web-api/pkg/errcode"
	"gin-biz-web-api/pkg/responses"
)

type PermissionChecker interface {
	HasPermission(ctx context.Context, user model.User, code string) (bool, error)
}

func RequirePermission(code string) gin.HandlerFunc {
	return RequirePermissionWithAuthorizer(code, auth_svc.NewAuthorizer(database.DB))
}

func RequirePermissionWithAuthorizer(code string, checker PermissionChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		current, exists := c.Get(constant.CurrentUserInfo)
		user, valid := current.(model.User)
		if !exists || !valid || checker == nil {
			responses.New(c).ToSafeErrorResponse(errcode.Forbidden, "无权执行此操作")
			return
		}
		allowed, err := checker.HasPermission(c.Request.Context(), user, code)
		if err != nil {
			responses.New(c).ToSafeErrorResponse(errcode.ServiceUnavailable, "权限服务暂时不可用")
			return
		}
		if !allowed {
			responses.New(c).ToSafeErrorResponse(errcode.Forbidden, "无权执行此操作")
			return
		}
		c.Next()
	}
}
