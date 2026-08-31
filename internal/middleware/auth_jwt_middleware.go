// 授权中间件
package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	"gin-biz-web-api/constant"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"
	"gin-biz-web-api/pkg/errcode"
	"gin-biz-web-api/pkg/jwt"
	"gin-biz-web-api/pkg/responses"
)

const (
	authMethodConsoleJWT        = "console_jwt"
	authMethodOpenToken         = "open_token"
	authMethodInternalBearerJWT = "internal_bearer_jwt"
)

var errAuthDatabaseUnavailable = errors.New("authentication database unavailable")

type authUserLookup func(context.Context, string) (model.User, error)
type authClaimsParser func(*gin.Context, ...string) (*jwt.JWTCustomClaims, error)

func AuthJWT() gin.HandlerFunc {
	return authJWTWithUserLookup(lookupConsoleUser)
}

func authJWTWithUserLookup(lookup authUserLookup) gin.HandlerFunc {
	return authJWTWithDependencies(lookup, parseAuthClaims)
}

func authJWTWithDependencies(lookup authUserLookup, parseClaims authClaimsParser) gin.HandlerFunc {
	return func(c *gin.Context) {
		response := responses.New(c)

		// 自动获取 token，并解析 token
		claims, err := parseClaims(c)

		// jwt 解析失败
		if err != nil {
			response.ToSafeErrorResponse(errcode.Unauthorized, "身份凭证无效或已过期")
			return
		}

		// jwt 解析成功，设置用户信息
		user, ok := authenticatedUser(c, response, authMethodConsoleJWT, claims.U, lookup)
		if !ok {
			return
		}
		if !validConsoleSession(&user, claims.V) {
			response.ToSafeErrorResponse(errcode.Unauthorized, "身份凭证无效或已过期")
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
	return authOpenTokenWithUserLookup(lookupOpenAPIUser)
}

func authOpenTokenWithUserLookup(lookup authUserLookup) gin.HandlerFunc {
	return func(c *gin.Context) {
		response := responses.New(c)
		token, ok := openAPIToken(c.Request.Header.Values("token"))
		if !ok {
			response.ToSafeErrorResponse(errcode.Unauthorized, "身份凭证缺失或格式错误")
			return
		}

		sum := sha256.Sum256([]byte(token))
		user, ok := authenticatedUser(
			c,
			response,
			authMethodOpenToken,
			hex.EncodeToString(sum[:]),
			lookup,
		)
		if !ok {
			return
		}
		if user.BaseModel == nil || user.ID == 0 {
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
	return authInternalBearerJWTWithUserLookup(lookupConsoleUser)
}

func authInternalBearerJWTWithUserLookup(lookup authUserLookup) gin.HandlerFunc {
	return authInternalBearerJWTWithDependencies(lookup, parseAuthClaims)
}

func authInternalBearerJWTWithDependencies(lookup authUserLookup, parseClaims authClaimsParser) gin.HandlerFunc {
	return func(c *gin.Context) {
		response := responses.New(c)
		token, ok := bearerToken(c.GetHeader("Authorization"))
		if !ok || strings.HasPrefix(token, "dg_open_") {
			response.ToSafeErrorResponse(errcode.Unauthorized, "身份凭证缺失或格式错误")
			return
		}

		claims, err := parseClaims(c, token)
		if err != nil {
			response.ToSafeErrorResponse(errcode.Unauthorized, "身份凭证无效或已过期")
			return
		}

		user, ok := authenticatedUser(c, response, authMethodInternalBearerJWT, claims.U, lookup)
		if !ok {
			return
		}
		if !validConsoleSession(&user, claims.V) {
			response.ToSafeErrorResponse(errcode.Unauthorized, "身份凭证无效或已过期")
			return
		}

		c.Set(constant.CurrentUserID, user.GetStringID())
		c.Set(constant.CurrentUserInfo, user)
		c.Next()
	}
}

func parseAuthClaims(c *gin.Context, token ...string) (*jwt.JWTCustomClaims, error) {
	return jwt.NewJWT().ParseToken(c, token...)
}

func authenticatedUser(
	c *gin.Context,
	response *responses.Response,
	authMethod string,
	lookupKey string,
	lookup authUserLookup,
) (model.User, bool) {
	if lookup == nil {
		writeAuthServiceUnavailable(c, response, authMethod, errAuthDatabaseUnavailable)
		return model.User{}, false
	}

	user, err := lookup(c.Request.Context(), lookupKey)
	if err == nil {
		return user, true
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		response.ToSafeErrorResponse(errcode.Unauthorized, "身份凭证无效或已过期")
		return model.User{}, false
	}

	writeAuthServiceUnavailable(c, response, authMethod, err)
	return model.User{}, false
}

func writeAuthServiceUnavailable(
	c *gin.Context,
	response *responses.Response,
	authMethod string,
	err error,
) {
	zap.L().Error(
		"authentication user lookup failed",
		zap.String("auth_method", authMethod),
		zap.String("request_method", c.Request.Method),
		zap.String("request_path", c.Request.URL.Path),
		zap.Error(err),
	)
	response.ToSafeErrorResponse(errcode.ServiceUnavailable, "认证服务暂时不可用")
}

func lookupConsoleUser(ctx context.Context, userID string) (model.User, error) {
	var user model.User
	if database.DB == nil {
		return user, errAuthDatabaseUnavailable
	}
	return user, database.DB.WithContext(ctx).First(&user, userID).Error
}

func lookupOpenAPIUser(ctx context.Context, tokenHash string) (model.User, error) {
	var user model.User
	if database.DB == nil {
		return user, errAuthDatabaseUnavailable
	}
	return user, openAPIUserLookup(ctx, database.DB, tokenHash).Take(&user).Error
}

func validConsoleSession(user *model.User, authVersion uint64) bool {
	return user != nil &&
		user.BaseModel != nil &&
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
