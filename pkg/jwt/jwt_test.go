package jwt

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestGenerateVersionedTokenPreservesAuthVersion(t *testing.T) {
	j := &JWT{Key: []byte("test-signing-key"), MaxRefresh: time.Hour, ExpireTime: 10}
	token := j.GenerateVersionedToken("42", "refreshable", 17)
	if token == "" {
		t.Fatal("GenerateVersionedToken() returned empty token")
	}
	claims, err := j.ParseToken(nil, token)
	if err != nil {
		t.Fatalf("ParseToken() error = %v", err)
	}
	if claims.U != "42" || claims.T != "r" || claims.V != 17 {
		t.Fatalf("claims = %#v", claims)
	}
}

func TestRefreshTokenPreservesAuthVersion(t *testing.T) {
	j := &JWT{Key: []byte("test-signing-key"), MaxRefresh: time.Hour, ExpireTime: 10}
	claims := &JWTCustomClaims{U: "42", T: "r", E: time.Now().Add(-time.Second).Unix(), I: time.Now().Add(-time.Minute).Unix(), V: 23}
	expired, err := j.createToken(claims)
	if err != nil {
		t.Fatal(err)
	}
	gctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	gctx.Request = httptest.NewRequest(http.MethodPost, "/", nil).WithContext(context.Background())
	gctx.Request.Header.Set("token", expired)
	refreshed, err := j.RefreshToken(gctx)
	if err != nil {
		t.Fatalf("RefreshToken() error = %v", err)
	}
	parsed, err := j.ParseToken(nil, refreshed)
	if err != nil || parsed.V != 23 {
		t.Fatalf("refreshed claims = %#v, %v", parsed, err)
	}
}

func TestGetTokenReadsFormCredentialWithoutRequiringURLQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/weather-exports/00000000-0000-4000-8000-000000000017/content",
		strings.NewReader("token=form-token"),
	)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	context.Request = request

	token, err := (&JWT{}).GetToken(context)
	if err != nil || token != "form-token" {
		t.Fatalf("GetToken() token=%q error=%v", token, err)
	}
	if context.Request.URL.RawQuery != "" {
		t.Fatalf("token unexpectedly appeared in URL query: %q", context.Request.URL.RawQuery)
	}
}
