package middleware

import (
	"context"
	"errors"
	"strconv"

	"gin-biz-web-api/internal/service/auth_svc"
	"gin-biz-web-api/pkg/auth"
	"gin-biz-web-api/pkg/errcode"
	"gin-biz-web-api/pkg/responses"

	"github.com/gin-gonic/gin"
)

type mallScopeChecker interface {
	Require(ctx context.Context, userID, mallID uint) error
}

// RequireMallScope protects routes whose mall identifier is a path parameter.
func RequireMallScope(param string) gin.HandlerFunc {
	return requireMallScope(auth_svc.NewMallScopeService(), param)
}

func requireMallScope(checker mallScopeChecker, param string) gin.HandlerFunc {
	return func(c *gin.Context) {
		mallID, err := strconv.ParseUint(c.Param(param), 10, 64)
		if err != nil || mallID == 0 {
			responses.New(c).ToSafeErrorResponse(errcode.UnprocessableEntity, "商场参数不正确")
			return
		}
		err = checker.Require(c.Request.Context(), auth.CurrentUserID(c), uint(mallID))
		if errors.Is(err, auth_svc.ErrMallScopeForbidden) {
			responses.New(c).ToSafeErrorResponse(errcode.Forbidden, "无权访问该商场")
			return
		}
		if err != nil {
			responses.New(c).ToSafeErrorResponse(errcode.ServiceUnavailable, "商场范围校验暂时不可用")
			return
		}
		c.Next()
	}
}
