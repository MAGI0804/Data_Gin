package middleware

import (
	"gin-biz-web-api/global"
	"gin-biz-web-api/pkg/errcode"
	"gin-biz-web-api/pkg/responses"

	"github.com/gin-gonic/gin"
)

const mallWeatherDisabledMessage = "商场天气服务未启用，请联系管理员完成配置后重试"

func RequireMallWeatherEnabled() gin.HandlerFunc {
	return requireMallWeatherEnabled(func() bool {
		return global.MallWeatherEnabledAtStartup
	})
}

func requireMallWeatherEnabled(enabled func() bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if enabled == nil || !enabled() {
			responses.New(c).ToSafeErrorResponse(errcode.ServiceUnavailable, mallWeatherDisabledMessage)
			return
		}
		c.Next()
	}
}
