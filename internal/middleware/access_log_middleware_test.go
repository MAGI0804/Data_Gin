package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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

func TestAccessLogSanitizesCredentialsWithoutMutatingRequest(t *testing.T) {
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/weather-exports/job/content?page=2&token=query-secret",
		strings.NewReader("token=form-secret&reason=manual"),
	)
	request.Host = "example.test"
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Token", "header-secret")
	request.Header.Set("Idempotency-Key", "weather-export-key")

	requestURL, requestURI, requestQuery := sanitizedAccessLogURL(request)
	headers := sanitizedAccessLogHeaders(request.Header)
	body := sanitizedAccessLogRequestBody(
		"application/x-www-form-urlencoded",
		[]byte("token=form-secret&reason=manual"),
		nil,
	)

	for name, value := range map[string]string{
		"request URL":   requestURL,
		"request URI":   requestURI,
		"request query": requestQuery,
		"header":        headers.Get("Token"),
		"body":          body,
	} {
		if strings.Contains(value, "secret") {
			t.Fatalf("%s leaked a credential: %q", name, value)
		}
	}
	if requestQuery != "page=2&token=%5BREDACTED%5D" ||
		headers.Get("Token") != accessLogRedactedValue ||
		headers.Get("Idempotency-Key") != "weather-export-key" ||
		body != "reason=manual&token=%5BREDACTED%5D" {
		t.Fatalf(
			"requestURL=%q requestURI=%q requestQuery=%q headers=%v body=%q",
			requestURL,
			requestURI,
			requestQuery,
			headers,
			body,
		)
	}
	if request.Header.Get("Token") != "header-secret" || request.URL.RawQuery != "page=2&token=query-secret" {
		t.Fatal("sanitization mutated the original request")
	}
}

func TestAccessLogSanitizesNestedJSONAndMultipartFields(t *testing.T) {
	jsonBody := []byte(`{"reason":"manual","credentials":{"accessToken":"token-value","password":"password-value"},"items":[{"x-cy-signature":"signature-value","name":"weather"}]}`)
	sanitizedJSON := sanitizedAccessLogRequestBody("application/json", jsonBody, nil)
	for _, secret := range []string{"token-value", "password-value", "signature-value"} {
		if strings.Contains(sanitizedJSON, secret) {
			t.Fatalf("sanitized JSON leaked %q: %s", secret, sanitizedJSON)
		}
	}
	for _, retained := range []string{`"reason":"manual"`, `"name":"weather"`} {
		if !strings.Contains(sanitizedJSON, retained) {
			t.Fatalf("sanitized JSON dropped non-sensitive field %q: %s", retained, sanitizedJSON)
		}
	}

	multipart := url.Values{
		"token":  {"multipart-secret"},
		"reason": {"manual"},
	}
	sanitizedMultipart := sanitizedAccessLogRequestBody("multipart/form-data", nil, multipart)
	if strings.Contains(sanitizedMultipart, "multipart-secret") ||
		sanitizedMultipart != "reason=manual&token=%5BREDACTED%5D" {
		t.Fatalf("sanitized multipart body=%q", sanitizedMultipart)
	}
}

func TestAccessLogOmitsUnparseableOrUnstructuredBodies(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        []byte
		want        string
	}{
		{name: "invalid JSON", contentType: "application/json", body: []byte(`{"token":`), want: "[unparseable JSON body omitted]"},
		{name: "invalid form", contentType: "application/x-www-form-urlencoded", body: []byte("token=%zz"), want: "[unparseable form body omitted]"},
		{name: "plain text", contentType: "text/plain", body: []byte("possibly sensitive"), want: "[unstructured request body omitted]"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := sanitizedAccessLogRequestBody(test.contentType, test.body, nil); got != test.want {
				t.Fatalf("sanitizedAccessLogRequestBody()=%q, want %q", got, test.want)
			}
		})
	}
}

func TestAccessLogSanitizesStructuredResponseBodies(t *testing.T) {
	body := []byte(`{"token":"response-secret","data":{"name":"weather"}}`)
	got := sanitizedAccessLogBody("application/json; charset=utf-8", body, nil)
	if strings.Contains(got, "response-secret") {
		t.Fatalf("sanitized response leaked a credential: %s", got)
	}
	if !strings.Contains(got, `"name":"weather"`) ||
		!strings.Contains(got, `"token":"[REDACTED]"`) {
		t.Fatalf("sanitized response=%q", got)
	}
}
