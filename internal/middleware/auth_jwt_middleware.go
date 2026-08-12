// 授权中间件
package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"gin-biz-web-api/constant"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"
	"gin-biz-web-api/pkg/errcode"
	"gin-biz-web-api/pkg/jwt"
	"gin-biz-web-api/pkg/responses"
)

func AuthJWT() gin.HandlerFunc {
	return func(c *gin.Context) {

		response := responses.New(c)

		// 自动获取 token，并解析 token
		claims, err := jwt.NewJWT().ParseToken(c)

		// jwt 解析失败
		if err != nil {
			response.ToErrorResponse(errcode.BadRequest.WithDetails(err.Error()), err.Error())
			c.Abort() // 终止后续中间件和处理函数的执行
			return
		}

		// jwt 解析成功，设置用户信息
		var user model.User
		result := database.DB.WithContext(c.Request.Context()).First(&user, claims.U)
		if result.Error != nil || !validConsoleSession(&user, claims.V) {
			response.ToSafeErrorResponse(errcode.Unauthorized, "身份凭证无效或已过期")
			c.Abort()
			return
		}

		// 将用户信息存入 gin.context 上下文中，方便后续直接从上下文中拿到用户信息
		c.Set(constant.CurrentUserID, user.GetStringID())
		c.Set(constant.CurrentUserInfo, user)

		c.Next() // 继续执行后续中间件和处理函数
	}
}

// AuthOpenToken authenticates public API requests using the dedicated token
// request header. It intentionally ignores internal JWT credentials and every
// other possible credential source so the public and internal trust boundaries
// cannot be mixed.
func AuthOpenToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		response := responses.New(c)
		token, ok := openAPIToken(c.Request.Header.Values("token"))
		if !ok {
			response.ToSafeErrorResponse(errcode.Unauthorized, "身份凭证缺失或格式错误")
			return
		}

		sum := sha256.Sum256([]byte(token))
		var user model.User
		result := openAPIUserLookup(
			c.Request.Context(),
			database.DB,
			hex.EncodeToString(sum[:]),
		).Take(&user)
		if result.Error != nil || user.ID == 0 {
			response.ToSafeErrorResponse(errcode.Unauthorized, "身份凭证无效或已过期")
			return
		}

		c.Set(constant.CurrentUserID, user.GetStringID())
		c.Set(constant.CurrentUserInfo, user)
		c.Next()
	}
}

func openAPIUserLookup(ctx context.Context, db *gorm.DB, tokenHash string) *gorm.DB {
	return db.WithContext(ctx).
		Table("users AS open_user").
		Select("open_user.*").
		Joins("INNER JOIN open_api_credentials AS credential ON credential.user_id = open_user.id").
		Where(
			"credential.token_hash = ? AND credential.status = ? AND open_user.account_type = ? AND open_user.status = ?",
			tokenHash,
			model.OpenAPICredentialStatusActive,
			model.AccountTypeOpenAPI,
			model.AccountStatusActive,
		)
}

// AuthInternalBearerJWT authenticates internal management requests from the
// Authorization header only. Public API tokens are never accepted here.
func AuthInternalBearerJWT() gin.HandlerFunc {
	return func(c *gin.Context) {
		response := responses.New(c)
		token, ok := bearerToken(c.GetHeader("Authorization"))
		if !ok || strings.HasPrefix(token, "dg_open_") {
			response.ToSafeErrorResponse(errcode.Unauthorized, "身份凭证缺失或格式错误")
			return
		}

		claims, err := jwt.NewJWT().ParseToken(c, token)
		if err != nil {
			response.ToSafeErrorResponse(errcode.Unauthorized, "身份凭证无效或已过期")
			return
		}

		var user model.User
		result := database.DB.WithContext(c.Request.Context()).First(&user, claims.U)
		if result.Error != nil || !validConsoleSession(&user, claims.V) {
			response.ToSafeErrorResponse(errcode.Unauthorized, "身份凭证无效或已过期")
			return
		}

		c.Set(constant.CurrentUserID, user.GetStringID())
		c.Set(constant.CurrentUserInfo, user)
		c.Next()
	}
}

func validConsoleSession(user *model.User, authVersion uint64) bool {
	return user != nil &&
		user.ID != 0 &&
		user.AccountType == model.AccountTypeConsole &&
		user.Status == model.AccountStatusActive &&
		user.AuthVersion == authVersion
}

func openAPIToken(values []string) (string, bool) {
	if len(values) != 1 || values[0] == "" || values[0] != strings.TrimSpace(values[0]) {
		return "", false
	}
	if !strings.HasPrefix(values[0], "dg_open_") || len(values[0]) == len("dg_open_") {
		return "", false
	}
	return values[0], true
}

func bearerToken(authorization string) (string, bool) {
	parts := strings.Fields(authorization)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}
