package jwt

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

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
