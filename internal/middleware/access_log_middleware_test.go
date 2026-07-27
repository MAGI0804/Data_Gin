package middleware

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAccessLogWriterDoesNotBufferExcelResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Writer.Header().Set(
		"Content-Type",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	)
	writer := &AccessLogWriter{
		ResponseWriter: context.Writer,
		body:           bytes.NewBuffer(nil),
	}

	payload := []byte("PK\x03\x04xlsx")
	if _, err := writer.Write(payload); err != nil {
		t.Fatalf("Write() error=%v", err)
	}
	if writer.body.Len() != 0 {
		t.Fatalf("access log buffered %d Excel bytes", writer.body.Len())
	}
	if got := recorder.Body.Bytes(); !bytes.Equal(got, payload) {
		t.Fatalf("response=%q, want %q", got, payload)
	}
}

func TestAccessLogWriterBoundsTextualResponsePreview(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer := &AccessLogWriter{
		ResponseWriter: context.Writer,
		body:           bytes.NewBuffer(nil),
	}

	payload := bytes.Repeat([]byte("x"), maxAccessLogResponseBodyBytes+1)
	if _, err := writer.Write(payload); err != nil {
		t.Fatalf("Write() error=%v", err)
	}
	if writer.body.Len() != maxAccessLogResponseBodyBytes || !writer.truncated {
		t.Fatalf("preview bytes=%d truncated=%t", writer.body.Len(), writer.truncated)
	}
	if got := recorder.Body.Len(); got != len(payload) {
		t.Fatalf("response bytes=%d, want %d", got, len(payload))
	}
}
