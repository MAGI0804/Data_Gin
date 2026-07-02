package destination

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRenderTemplateReplacesFields(t *testing.T) {
	body := RenderTemplate(`{"order_no":"{{ order_no }}","amount":"{{amount}}"}`, map[string]interface{}{
		"order_no": "ORDER-1",
		"amount":   12.34,
	})

	want := `{"order_no":"ORDER-1","amount":"12.34"}`
	if body != want {
		t.Fatalf("body = %s, want %s", body, want)
	}
}

func TestHTTPPublisherPublishSendsRenderedPayload(t *testing.T) {
	var requestBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		requestBody = string(body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	publisher := HTTPPublisher{}
	result, err := publisher.Publish(context.Background(), Config{
		"url":              server.URL,
		"payload_template": `{"order_no":"{{order_no}}"}`,
	}, CleanRecord{
		Content: map[string]interface{}{
			"order_no": "ORDER-1",
		},
	})
	if err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}

	if !result.Success {
		t.Fatal("Publish result Success = false, want true")
	}
	if requestBody != `{"order_no":"ORDER-1"}` {
		t.Fatalf("requestBody = %s, want rendered payload", requestBody)
	}
}
