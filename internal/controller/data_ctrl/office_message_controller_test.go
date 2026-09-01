package data_ctrl

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gin-biz-web-api/internal/service/data_svc"

	"github.com/gin-gonic/gin"
)

func TestBindOfficeJSONRejectsUnknownAndTrailingFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name string
		body string
	}{
		{name: "unknown field", body: `{"parameters":{},"requestId":"9ac63f51-1e15-40b0-ae0a-2b1c29b9de35","typo":true}`},
		{name: "trailing document", body: `{"parameters":{},"requestId":"9ac63f51-1e15-40b0-ae0a-2b1c29b9de35"}{}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(response)
			context.Request = httptest.NewRequest("POST", "/", strings.NewReader(test.body))
			var input data_svc.OfficePushRunInput
			if err := bindOfficeJSON(context, &input); err == nil {
				t.Fatalf("bindOfficeJSON() accepted %s", test.body)
			}
		})
	}
}

func TestWriteOfficeMessageErrorRecordsPrivateCauseWithoutLeakingResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	writeOfficeMessageError(context, errors.New("query oracle: password=secret-private ORA-00904"))

	if response.Code != http.StatusInternalServerError || len(context.Errors.ByType(gin.ErrorTypePrivate)) != 1 {
		t.Fatalf("response=%d errors=%v", response.Code, context.Errors)
	}
	if strings.Contains(response.Body.String(), "password") || strings.Contains(response.Body.String(), "secret-private") || strings.Contains(response.Body.String(), "ORA-00904") {
		t.Fatalf("response leaked private cause: %s", response.Body)
	}
}
