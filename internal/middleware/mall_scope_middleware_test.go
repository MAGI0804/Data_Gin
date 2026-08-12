package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"gin-biz-web-api/constant"
	"gin-biz-web-api/internal/service/auth_svc"

	"github.com/gin-gonic/gin"
)

type fakeMallScopeChecker struct{ err error }

func (checker fakeMallScopeChecker) Require(context.Context, uint, uint) error { return checker.err }

func TestRequireMallScopeFailsClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name   string
		err    error
		status int
	}{
		{name: "allowed", status: http.StatusNoContent},
		{name: "forbidden", err: auth_svc.ErrMallScopeForbidden, status: http.StatusForbidden},
		{name: "store unavailable", err: errors.New("db unavailable"), status: http.StatusServiceUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/malls/:id", func(c *gin.Context) {
				c.Set(constant.CurrentUserID, "7")
			}, requireMallScope(fakeMallScopeChecker{err: test.err}, "id"), func(c *gin.Context) { c.Status(http.StatusNoContent) })
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/malls/9", nil))
			if recorder.Code != test.status {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}
