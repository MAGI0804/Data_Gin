package middleware

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"gin-biz-web-api/pkg/database"
	"gin-biz-web-api/pkg/errcode"
	"gin-biz-web-api/pkg/responses"
)

type databaseAvailabilityChecker func(context.Context) bool

// RequireDatabaseAvailability prevents a database outage from making every
// concurrent API request wait for a new connection attempt.
func RequireDatabaseAvailability() gin.HandlerFunc {
	return requireDatabaseAvailability(database.CanServe)
}

func requireDatabaseAvailability(check databaseAvailabilityChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}
		if check != nil && check(c.Request.Context()) {
			c.Next()
			return
		}
		c.Header("Retry-After", "1")
		responses.New(c).ToSafeErrorResponse(errcode.ServiceUnavailable, "数据库服务暂时不可用")
		c.Abort()
	}
}
