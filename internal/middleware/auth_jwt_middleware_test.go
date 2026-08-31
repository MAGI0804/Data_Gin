package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"gorm.io/gorm"

	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/errcode"
	"gin-biz-web-api/pkg/jwt"
	"gin-biz-web-api/pkg/responses"
)

type authMiddlewareTestCase struct {
	name       string
	credential string
	build      func(authUserLookup) gin.HandlerFunc
	request    func(string) *http.Request
	validUser  model.User
}

func TestAuthMiddlewareReturnsServiceUnavailableOnUserLookupFailure(t *testing.T) {
	for _, test := range authMiddlewareTestCases() {
		t.Run(test.name, func(t *testing.T) {
			previousLogger := zap.L()
			core, logs := observer.New(zap.ErrorLevel)
			zap.ReplaceGlobals(zap.New(core))
			t.Cleanup(func() { zap.ReplaceGlobals(previousLogger) })

			const privateDatabaseError = "dial tcp database.internal:3306: connection refused"
			recorder, calls := serveAuthRequest(
				t,
				test.build(func(context.Context, string) (model.User, error) {
					return model.User{}, errors.New(privateDatabaseError)
				}),
				test.request(test.credential),
			)

			assertAuthErrorResponse(t, recorder, http.StatusServiceUnavailable, errcode.ServiceUnavailable.Code())
			if calls != 0 {
				t.Fatalf("downstream calls = %d, want 0", calls)
			}
			if strings.Contains(recorder.Body.String(), privateDatabaseError) ||
				strings.Contains(recorder.Body.String(), test.credential) {
				t.Fatalf("response leaked private authentication detail: %s", recorder.Body.String())
			}
			if logs.Len() != 1 {
				t.Fatalf("error log count = %d, want 1", logs.Len())
			}
			logEntry := logs.All()[0]
			if strings.Contains(logEntry.Message, test.credential) ||
				strings.Contains(logEntry.ContextMap()["error"].(string), test.credential) {
				t.Fatal("authentication failure log contained the raw credential")
			}
			if got := logEntry.ContextMap()["auth_method"]; got != test.name {
				t.Fatalf("auth_method = %v, want %q", got, test.name)
			}
		})
	}
}

func TestAuthMiddlewareReturnsUnauthorizedForMissingUser(t *testing.T) {
	for _, test := range authMiddlewareTestCases() {
		t.Run(test.name, func(t *testing.T) {
			previousLogger := zap.L()
			core, logs := observer.New(zap.ErrorLevel)
			zap.ReplaceGlobals(zap.New(core))
			t.Cleanup(func() { zap.ReplaceGlobals(previousLogger) })

			recorder, calls := serveAuthRequest(
				t,
				test.build(func(context.Context, string) (model.User, error) {
					return model.User{}, gorm.ErrRecordNotFound
				}),
				test.request(test.credential),
			)

			assertAuthErrorResponse(t, recorder, http.StatusUnauthorized, errcode.Unauthorized.Code())
			if calls != 0 {
				t.Fatalf("downstream calls = %d, want 0", calls)
			}
			if logs.Len() != 0 {
				t.Fatalf("error log count = %d, want 0", logs.Len())
			}
		})
	}
}

func TestAuthMiddlewareReturnsUnauthorizedForInvalidUser(t *testing.T) {
	for _, test := range authMiddlewareTestCases() {
		t.Run(test.name, func(t *testing.T) {
			recorder, calls := serveAuthRequest(
				t,
				test.build(func(context.Context, string) (model.User, error) {
					return model.User{}, nil
				}),
				test.request(test.credential),
			)

			assertAuthErrorResponse(t, recorder, http.StatusUnauthorized, errcode.Unauthorized.Code())
			if calls != 0 {
				t.Fatalf("downstream calls = %d, want 0", calls)
			}
		})
	}
}

func TestAuthMiddlewareAllowsValidUser(t *testing.T) {
	for _, test := range authMiddlewareTestCases() {
		t.Run(test.name, func(t *testing.T) {
			recorder, calls := serveAuthRequest(
				t,
				test.build(func(context.Context, string) (model.User, error) {
					return test.validUser, nil
				}),
				test.request(test.credential),
			)

			if recorder.Code != http.StatusNoContent || calls != 1 {
				t.Fatalf("status = %d, downstream calls = %d, body = %s", recorder.Code, calls, recorder.Body.String())
			}
		})
	}
}

func TestAuthJWTReturnsUnauthorizedForMalformedToken(t *testing.T) {
	recorder, calls := serveAuthRequest(
		t,
		authJWTWithDependencies(
			func(context.Context, string) (model.User, error) {
				t.Fatal("user lookup should not run for a malformed token")
				return model.User{}, nil
			},
			func(*gin.Context, ...string) (*jwt.JWTCustomClaims, error) {
				return nil, jwt.ErrTokenMalformed
			},
		),
		httptest.NewRequest(http.MethodGet, "/protected", nil),
	)

	assertAuthErrorResponse(t, recorder, http.StatusUnauthorized, errcode.Unauthorized.Code())
	if calls != 0 {
		t.Fatalf("downstream calls = %d, want 0", calls)
	}
}

func authMiddlewareTestCases() []authMiddlewareTestCase {
	testJWT := &jwt.JWT{Key: []byte("middleware-test-signing-key")}
	consoleToken := testJWT.GenerateVersionedToken("7", "permanent", 3)
	consoleUser := model.User{
		BaseModel:   &model.BaseModel{ID: 7},
		AccountType: model.AccountTypeConsole,
		Status:      model.AccountStatusActive,
		AuthVersion: 3,
	}
	return []authMiddlewareTestCase{
		{
			name:       authMethodConsoleJWT,
			credential: consoleToken,
			build: func(lookup authUserLookup) gin.HandlerFunc {
				return authJWTWithDependencies(lookup, testJWT.ParseToken)
			},
			request: func(token string) *http.Request {
				request := httptest.NewRequest(http.MethodGet, "/protected", nil)
				request.Header.Set("token", token)
				return request
			},
			validUser: consoleUser,
		},
		{
			name:       authMethodOpenToken,
			credential: "dg_open_sensitive-credential",
			build:      authOpenTokenWithUserLookup,
			request: func(token string) *http.Request {
				request := httptest.NewRequest(http.MethodGet, "/protected", nil)
				request.Header.Set("token", token)
				return request
			},
			validUser: model.User{BaseModel: &model.BaseModel{ID: 8}},
		},
		{
			name:       authMethodInternalBearerJWT,
			credential: consoleToken,
			build: func(lookup authUserLookup) gin.HandlerFunc {
				return authInternalBearerJWTWithDependencies(lookup, testJWT.ParseToken)
			},
			request: func(token string) *http.Request {
				request := httptest.NewRequest(http.MethodGet, "/protected", nil)
				request.Header.Set("Authorization", "Bearer "+token)
				return request
			},
			validUser: consoleUser,
		},
	}
}

func serveAuthRequest(t *testing.T, auth gin.HandlerFunc, request *http.Request) (*httptest.ResponseRecorder, int) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	calls := 0
	router.Use(auth)
	router.GET("/protected", func(c *gin.Context) {
		calls++
		c.Status(http.StatusNoContent)
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder, calls
}

func assertAuthErrorResponse(t *testing.T, recorder *httptest.ResponseRecorder, status, code int) {
	t.Helper()
	if recorder.Code != status {
		t.Fatalf("status = %d, want %d, body = %s", recorder.Code, status, recorder.Body.String())
	}
	var body responses.ResponseData
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != code {
		t.Fatalf("response code = %d, want %d", body.Code, code)
	}
}
