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

func TestAuthOpenTokenRejectsOtherCredentialSources(t *testing.T) {
	tests := []struct {
		name          string
		tokenHeader   string
		authorization string
		url           string
	}{
		{name: "missing"},
		{name: "authorization open token is ignored", authorization: "Bearer dg_open_value"},
		{name: "authorization internal jwt is ignored", authorization: "Bearer abc.def.ghi"},
		{name: "query token is ignored", url: "/open?token=dg_open_value"},
		{name: "non open token is rejected", tokenHeader: "abc.def.ghi"},
		{name: "empty open token is rejected", tokenHeader: "dg_open_"},
		{name: "surrounding whitespace is rejected", tokenHeader: " dg_open_value "},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			router := gin.New()
			calls := 0
			router.POST("/open", AuthOpenToken(), func(c *gin.Context) {
				calls++
				c.Status(http.StatusNoContent)
			})
			url := test.url
			if url == "" {
				url = "/open"
			}
			request := httptest.NewRequest(http.MethodPost, url, nil)
			if test.tokenHeader != "" {
				request.Header.Set("token", test.tokenHeader)
			}
			if test.authorization != "" {
				request.Header.Set("Authorization", test.authorization)
			}
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

func TestOpenAPIToken(t *testing.T) {
	token, ok := openAPIToken([]string{"dg_open_abc123"})
	if !ok || token != "dg_open_abc123" {
		t.Fatalf("token=%q ok=%t", token, ok)
	}
	for _, values := range [][]string{
		nil,
		{""},
		{"dg_open_"},
		{"internal-jwt"},
		{"dg_open_one", "dg_open_two"},
	} {
		if _, accepted := openAPIToken(values); accepted {
			t.Fatalf("accepted values %#v", values)
		}
	}
}

func TestAuthInternalBearerJWTRejectsOpenToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	calls := 0
	router.POST("/internal", AuthInternalBearerJWT(), func(c *gin.Context) {
		calls++
		c.Status(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodPost, "/internal", nil)
	request.Header.Set("Authorization", "Bearer dg_open_value")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized || calls != 0 {
		t.Fatalf("status=%d calls=%d body=%s", recorder.Code, calls, recorder.Body.String())
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
