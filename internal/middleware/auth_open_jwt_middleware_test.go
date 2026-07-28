package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"gin-biz-web-api/pkg/errcode"
	"gin-biz-web-api/pkg/responses"

	"github.com/gin-gonic/gin"
)

func TestAuthOpenJWTRequiresBearerAuthorization(t *testing.T) {
	tests := []struct {
		name   string
		header string
	}{
		{name: "missing"},
		{name: "legacy token header is not accepted", header: "Token value"},
		{name: "empty bearer", header: "Bearer"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			router := gin.New()
			calls := 0
			router.POST("/open", AuthOpenJWT(), func(c *gin.Context) {
				calls++
				c.Status(http.StatusNoContent)
			})
			request := httptest.NewRequest(http.MethodPost, "/open", nil)
			request.Header.Set("Authorization", test.header)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusUnauthorized || calls != 0 {
				t.Fatalf("status=%d calls=%d body=%s", recorder.Code, calls, recorder.Body.String())
			}
			var body responses.ResponseData
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Code != errcode.Unauthorized.Code() {
				t.Fatalf("response=%+v", body)
			}
		})
	}
}

func TestBearerToken(t *testing.T) {
	token, ok := bearerToken("bearer abc.def")
	if !ok || token != "abc.def" {
		t.Fatalf("token=%q ok=%t", token, ok)
	}
	if _, ok := bearerToken("Bearer one two"); ok {
		t.Fatal("accepted malformed authorization header")
	}
}
