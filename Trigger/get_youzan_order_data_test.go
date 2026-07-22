package Trigger

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

type capturedYouzanOrderRequest struct {
	params map[string]interface{}
	query  url.Values
	err    error
}

func TestYouzanOrderClient_GetOrders_UsesCreatedTimeRange(t *testing.T) {
	t.Parallel()

	captured := make(chan capturedYouzanOrderRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := map[string]interface{}{}
		err := json.NewDecoder(r.Body).Decode(&params)
		captured <- capturedYouzanOrderRequest{
			params: params,
			query:  r.URL.Query(),
			err:    err,
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"code":    200,
			"message": "",
			"data": map[string]interface{}{
				"full_order_info_list": []interface{}{},
				"paginator": map[string]interface{}{
					"has_next":    false,
					"next_cursor": "",
				},
			},
		})
	}))
	defer server.Close()

	client := youzanOrderClient{
		httpClient: server.Client(),
		ordersURL:  server.URL,
	}

	orders, err := client.getOrders("test-token", "2026-07-22 10:00:00", "2026-07-22 10:05:00")
	if err != nil {
		t.Fatalf("getOrders() error = %v", err)
	}
	if len(orders) != 0 {
		t.Fatalf("getOrders() returned %d orders, want 0", len(orders))
	}

	request := <-captured
	if request.err != nil {
		t.Fatalf("decode request body: %v", request.err)
	}
	if got := request.query.Get("access_token"); got != "test-token" {
		t.Errorf("access_token = %q, want %q", got, "test-token")
	}
	if got := request.params["start_created"]; got != "2026-07-22 10:00:00" {
		t.Errorf("start_created = %v, want %q", got, "2026-07-22 10:00:00")
	}
	if got := request.params["end_created"]; got != "2026-07-22 10:05:00" {
		t.Errorf("end_created = %v, want %q", got, "2026-07-22 10:05:00")
	}
	if _, exists := request.params["start_success"]; exists {
		t.Error("request unexpectedly contains start_success")
	}
	if _, exists := request.params["end_success"]; exists {
		t.Error("request unexpectedly contains end_success")
	}
}
