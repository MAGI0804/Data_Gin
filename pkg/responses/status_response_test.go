package responses

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gin-biz-web-api/pkg/errcode"

	"github.com/gin-gonic/gin"
)

func TestToResponseWithStatusUsesRealHTTPStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)

	New(context).ToResponseWithStatus(http.StatusCreated, gin.H{"id": 7})

	if recorder.Code != http.StatusCreated {
		t.Fatalf("HTTP status = %d, want %d", recorder.Code, http.StatusCreated)
	}
	var body ResponseData
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != errcode.Success.Code() {
		t.Fatalf("application code = %d, want %d", body.Code, errcode.Success.Code())
	}
}

func TestToSafeErrorResponseMapsStatusWithoutLeakingDetails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)

	New(context).ToSafeErrorResponse(errcode.Conflict, "资源版本冲突")

	if recorder.Code != http.StatusConflict {
		t.Fatalf("HTTP status = %d, want %d", recorder.Code, http.StatusConflict)
	}
	if body := recorder.Body.String(); body == "" || strings.Contains(body, "database") {
		t.Fatalf("unsafe response body = %q", body)
	}
}

func TestForbiddenUsesForbiddenHTTPStatus(t *testing.T) {
	if got := errcode.Forbidden.HttpStatusCode(); got != http.StatusForbidden {
		t.Fatalf("Forbidden HTTP status = %d, want %d", got, http.StatusForbidden)
	}
}
