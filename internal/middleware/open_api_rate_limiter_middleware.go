package middleware

import (
	"strconv"
	"strings"
	"time"

	"gin-biz-web-api/constant"
	"gin-biz-web-api/pkg/errcode"
	appLimiter "gin-biz-web-api/pkg/limiter"
	"gin-biz-web-api/pkg/logger"
	"gin-biz-web-api/pkg/responses"

	"github.com/gin-gonic/gin"
	limiterLib "github.com/ulule/limiter/v3"
)

type openAPIRateChecker func(*gin.Context, string, string) (limiterLib.Context, error)

// LimitOpenAPIIP protects authentication and database lookups before a
// caller identity is available. Its scoped key is independent from the global
// /api limiter, so the two policies cannot share counters.
func LimitOpenAPIIP(module, limit string) gin.HandlerFunc {
	return limitOpenAPIIP(module, limit, appLimiter.CheckRate)
}

// LimitOpenAPIUserRoute applies the primary open API quota to an
// authenticated user and route. The credential itself is never used in the
// Redis key.
func LimitOpenAPIUserRoute(module, limit string) gin.HandlerFunc {
	return limitOpenAPIUserRoute(module, limit, appLimiter.CheckRate)
}

func limitOpenAPIIP(module, limit string, checker openAPIRateChecker) gin.HandlerFunc {
	scope := openAPIRateLimitScope(module)
	return func(c *gin.Context) {
		key := strings.Join([]string{
			scope,
			"pre-auth-ip",
			c.ClientIP(),
		}, "|")
		if !checkOpenAPIRate(c, key, limit, checker) {
			return
		}
		c.Next()
	}
}

func limitOpenAPIUserRoute(module, limit string, checker openAPIRateChecker) gin.HandlerFunc {
	scope := openAPIRateLimitScope(module)
	return func(c *gin.Context) {
		userID := c.GetString(constant.CurrentUserID)
		if userID == "" {
			responses.New(c).ToSafeErrorResponse(
				errcode.Unauthorized,
				"身份凭证无效或已过期",
			)
			return
		}
		key := strings.Join([]string{
			scope,
			"user",
			userID,
			"route",
			routeRateLimitKey(c.FullPath()),
		}, "|")
		if !checkOpenAPIRate(c, key, limit, checker) {
			return
		}
		c.Next()
	}
}

func openAPIRateLimitScope(module string) string {
	module = strings.ToLower(strings.TrimSpace(module))
	module = strings.NewReplacer("|", "-", "/", "-", ":", "-").Replace(module)
	if module == "" {
		module = "unknown"
	}
	return "open-" + module
}

func checkOpenAPIRate(c *gin.Context, key, limit string, checker openAPIRateChecker) bool {
	// The legacy limiter avoids duplicate counting within one request. Open API
	// policies use separate scoped keys and must each increment independently.
	c.Set("rate-limiter-once", false)
	rate, err := checker(c, key, limit)
	if err != nil {
		if logger.Logger != nil {
			logger.LogErrorIf(err)
		}
		c.Header("Retry-After", "1")
		responses.New(c).ToSafeErrorResponse(
			errcode.ServiceUnavailable,
			"限流服务暂时不可用",
		)
		return false
	}

	c.Header("X-RateLimit-Limit", strconv.FormatInt(rate.Limit, 10))
	c.Header("X-RateLimit-Remaining", strconv.FormatInt(rate.Remaining, 10))
	c.Header("X-RateLimit-Reset", strconv.FormatInt(rate.Reset, 10))
	if !rate.Reached {
		return true
	}

	retryAfter := rate.Reset - time.Now().Unix()
	if retryAfter < 1 {
		retryAfter = 1
	}
	c.Header("Retry-After", strconv.FormatInt(retryAfter, 10))
	responses.New(c).ToSafeErrorResponse(errcode.TooManyRequests, "开放天气接口请求过于频繁")
	return false
}

func routeRateLimitKey(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "unknown"
	}
	return strings.NewReplacer("/", "-", ":", "-").Replace(path)
}
