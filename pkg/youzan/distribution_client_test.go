package youzan

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDistributionClientFetchOrderPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("access_token"); got != "test-token" {
			t.Fatalf("access_token = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"success":true,"code":200,"data":{"full_order_info_list":[{"full_order_info":{"order_info":{"tid":"D100"},"buyer_info":{"fans_nickname":"encrypted"}}}],"paginator":{"has_next":true}}}`)
	}))
	defer server.Close()

	client := NewDistributionClient(server.URL, server.URL, server.Client())
	orders, hasNext, err := client.FetchOrderPage(context.Background(), "test-token", "2026-07-05 00:00:00", "2026-07-05 23:59:59", 1, 100)
	if err != nil {
		t.Fatalf("FetchOrderPage() error = %v", err)
	}
	if len(orders) != 1 || !hasNext {
		t.Fatalf("orders = %#v, hasNext = %v", orders, hasNext)
	}
	if orders[0]["order_info"].(map[string]any)["tid"] != "D100" {
		t.Fatalf("unexpected order payload: %#v", orders[0])
	}
}

func TestDistributionClientDecryptBatchAcceptsStringArray(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"success":true,"code":200,"data":["nickname-1","nickname-2"]}`)
	}))
	defer server.Close()

	client := NewDistributionClient(server.URL, server.URL, server.Client())
	values, err := client.DecryptBatch(context.Background(), "test-token", []string{"encrypted-1", "encrypted-2"})
	if err != nil {
		t.Fatalf("DecryptBatch() error = %v", err)
	}
	if fmt.Sprint(values) != "[nickname-1 nickname-2]" {
		t.Fatalf("values = %v", values)
	}
}

func TestDistributionClientDecryptBatchAcceptsSourceValueMap(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"trace_id":"test-trace","code":200,"data":{"encrypted-2":"nickname-2","encrypted-1":"nickname-1"},"success":true,"message":"successful"}`)
	}))
	defer server.Close()

	client := NewDistributionClient(server.URL, server.URL, server.Client())
	values, err := client.DecryptBatch(context.Background(), "test-token", []string{"encrypted-1", "encrypted-2"})
	if err != nil {
		t.Fatalf("DecryptBatch() error = %v", err)
	}
	if fmt.Sprint(values) != "[nickname-1 nickname-2]" {
		t.Fatalf("values = %v", values)
	}
}

func TestDistributionClientRejectsDecryptBatchOverLimit(t *testing.T) {
	client := NewDistributionClient("http://unused", "http://unused", http.DefaultClient)
	_, err := client.DecryptBatch(context.Background(), "test-token", make([]string, 10001))
	if err == nil {
		t.Fatal("DecryptBatch() should reject more than 10000 sources")
	}
}
