package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"gin-biz-web-api/constant"
	"gin-biz-web-api/model"
)

type permissionCheckerFunc func(context.Context, model.User, string) (bool, error)

func (f permissionCheckerFunc) HasPermission(ctx context.Context, user model.User, code string) (bool, error) {
	return f(ctx, user, code)
}

func TestRequirePermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	providerError := errors.New("database password must stay private")
	tests := []struct {
		name       string
		user       interface{}
		checker    PermissionChecker
		wantStatus int
		wantNext   bool
	}{
		{name: "allowed", user: model.User{}, checker: permissionCheckerFunc(func(context.Context, model.User, string) (bool, error) { return true, nil }), wantStatus: http.StatusNoContent, wantNext: true},
		{name: "denied", user: model.User{}, checker: permissionCheckerFunc(func(context.Context, model.User, string) (bool, error) { return false, nil }), wantStatus: http.StatusForbidden},
		{name: "service unavailable", user: model.User{}, checker: permissionCheckerFunc(func(context.Context, model.User, string) (bool, error) { return false, providerError }), wantStatus: http.StatusServiceUnavailable},
		{name: "missing user", checker: permissionCheckerFunc(func(context.Context, model.User, string) (bool, error) { return true, nil }), wantStatus: http.StatusForbidden},
		{name: "wrong user type", user: &model.User{}, checker: permissionCheckerFunc(func(context.Context, model.User, string) (bool, error) { return true, nil }), wantStatus: http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next := false
			router := gin.New()
			router.Use(func(c *gin.Context) {
				if tt.user != nil {
					c.Set(constant.CurrentUserInfo, tt.user)
				}
			})
			router.GET("/", RequirePermissionWithAuthorizer("data.read", tt.checker), func(c *gin.Context) {
				next = true
				c.Status(http.StatusNoContent)
			})
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
			if recorder.Code != tt.wantStatus || next != tt.wantNext {
				t.Fatalf("status=%d next=%t", recorder.Code, next)
			}
			if strings.Contains(recorder.Body.String(), providerError.Error()) {
				t.Fatalf("response leaked provider error: %s", recorder.Body.String())
			}
		})
	}
}
