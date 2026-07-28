// 授权中间件
package middleware

import (
	"strings"

	"gin-biz-web-api/constant"

	"github.com/gin-gonic/gin"

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
		database.DB.First(&user, claims.U)
		if user.ID == 0 {
			response.ToErrorResponse(errcode.Unauthorized, "找不到对应用户")
			c.Abort()
			return
		}

		// 将用户信息存入 gin.context 上下文中，方便后续直接从上下文中拿到用户信息
		c.Set(constant.CurrentUserID, user.GetStringID())
		c.Set(constant.CurrentUserInfo, user)

		c.Next() // 继续执行后续中间件和处理函数
	}
}

// AuthOpenJWT authenticates open API requests from the standard Authorization
// header only. Unlike the legacy middleware, it does not accept credentials in
// query parameters, form fields, or custom headers and always returns a stable
// HTTP 401 response for invalid credentials.
func AuthOpenJWT() gin.HandlerFunc {
	return func(c *gin.Context) {
		response := responses.New(c)
		token, ok := bearerToken(c.GetHeader("Authorization"))
		if !ok {
			response.ToSafeErrorResponse(errcode.Unauthorized, "身份凭证缺失或格式错误")
			return
		}

		claims, err := jwt.NewJWT().ParseToken(c, token)
		if err != nil {
			response.ToSafeErrorResponse(errcode.Unauthorized, "身份凭证无效或已过期")
			return
		}

		var user model.User
		result := database.DB.First(&user, claims.U)
		if result.Error != nil || user.ID == 0 {
			response.ToSafeErrorResponse(errcode.Unauthorized, "身份凭证无效或已过期")
			return
		}

		c.Set(constant.CurrentUserID, user.GetStringID())
		c.Set(constant.CurrentUserInfo, user)
		c.Next()
	}
}

func bearerToken(authorization string) (string, bool) {
	parts := strings.Fields(authorization)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}
