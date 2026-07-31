package destination

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestHTTPPublisherTestOnlyUsesSafeMethods(t *testing.T) {
	tests := []struct {
		name        string
		testMethod  string
		wantMethod  string
		wantRequest bool
	}{
		{name: "defaults to HEAD", wantMethod: http.MethodHead, wantRequest: true},
		{name: "allows GET", testMethod: http.MethodGet, wantMethod: http.MethodGet, wantRequest: true},
		{name: "rejects POST", testMethod: http.MethodPost},
		{name: "rejects PUT", testMethod: http.MethodPut},
		{name: "rejects PATCH", testMethod: http.MethodPatch},
		{name: "rejects DELETE", testMethod: http.MethodDelete},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				requests++
				if request.Method != testCase.wantMethod {
					t.Errorf("request method = %s, want %s", request.Method, testCase.wantMethod)
				}
				body, err := io.ReadAll(request.Body)
				if err != nil {
					t.Fatalf("read test request body: %v", err)
				}
				if len(body) != 0 {
					t.Fatalf("test request body = %q, want empty", body)
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()

			cfg := Config{"url": server.URL}
			if testCase.testMethod != "" {
				cfg["test_method"] = testCase.testMethod
			}
			err := (HTTPPublisher{}).Test(context.Background(), cfg)
			if testCase.wantRequest {
				if err != nil {
					t.Fatalf("Test returned error: %v", err)
				}
				if requests != 1 {
					t.Fatalf("requests = %d, want 1", requests)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), "must be HEAD or GET") {
				t.Fatalf("Test error = %v, want rejected unsafe method", err)
			}
			if requests != 0 {
				t.Fatalf("requests = %d, want 0 for unsafe test method", requests)
			}
		})
	}
}
