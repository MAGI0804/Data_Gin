package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestContextTimeoutSupportsStreamingRequestSkipper(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(ContextTimeout(time.Minute, func(c *gin.Context) bool {
		return c.Request.URL.Path == "/download"
	}))
	router.GET("/:resource", func(c *gin.Context) {
		_, hasDeadline := c.Request.Context().Deadline()
		c.JSON(http.StatusOK, gin.H{"hasDeadline": hasDeadline})
	})

	tests := []struct {
		name         string
		path         string
		wantDeadline bool
	}{
		{name: "regular API", path: "/jobs", wantDeadline: true},
		{name: "streaming download", path: "/download", wantDeadline: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			router.ServeHTTP(recorder, request)
			wantBody := `{"hasDeadline":true}`
			if !test.wantDeadline {
				wantBody = `{"hasDeadline":false}`
			}
			if recorder.Code != http.StatusOK || recorder.Body.String() != wantBody {
				t.Fatalf("status=%d body=%s, want %s", recorder.Code, recorder.Body.String(), wantBody)
			}
		})
	}
}
