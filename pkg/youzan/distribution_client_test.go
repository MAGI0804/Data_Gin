package youzan

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDistributionClientFetchOrderPage(t *testing.T) {
	tests := []struct {
		name               string
		timeFilter         OrderTimeFilter
		expectedStartField string
		expectedEndField   string
		unexpectedFields   []string
	}{
		{
			name:               "created time",
			timeFilter:         OrderTimeFilterCreated,
			expectedStartField: "start_created",
			expectedEndField:   "end_created",
			unexpectedFields:   []string{"start_success", "end_success"},
		},
		{
			name:               "success time",
			timeFilter:         OrderTimeFilterSuccess,
			expectedStartField: "start_success",
			expectedEndField:   "end_success",
			unexpectedFields:   []string{"start_created", "end_created"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requestBodies := make(chan map[string]any, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got := r.URL.Query().Get("access_token"); got != "test-token" {
					t.Errorf("access_token = %q", got)
				}
				var requestBody map[string]any
				if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
					t.Errorf("decode request body: %v", err)
					http.Error(w, "invalid request", http.StatusBadRequest)
					return
				}
				requestBodies <- requestBody
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"success":true,"code":200,"data":{"full_order_info_list":[{"full_order_info":{"order_info":{"tid":"D100"},"buyer_info":{"fans_nickname":"encrypted"}}}],"paginator":{"has_next":true}}}`)
			}))
			defer server.Close()

			client := NewDistributionClient(server.URL, server.URL, server.Client())
			orders, hasNext, err := client.FetchOrderPage(
				context.Background(), "test-token", test.timeFilter,
				"2026-07-05 00:00:00", "2026-07-05 23:59:59", 1, 100,
			)
			if err != nil {
				t.Fatalf("FetchOrderPage() error = %v", err)
			}
			if len(orders) != 1 || !hasNext {
				t.Fatalf("orders = %#v, hasNext = %v", orders, hasNext)
			}
			if orders[0]["order_info"].(map[string]any)["tid"] != "D100" {
				t.Fatalf("unexpected order payload: %#v", orders[0])
			}

			requestBody := <-requestBodies
			if got := requestBody[test.expectedStartField]; got != "2026-07-05 00:00:00" {
				t.Errorf("%s = %v, want %q", test.expectedStartField, got, "2026-07-05 00:00:00")
			}
			if got := requestBody[test.expectedEndField]; got != "2026-07-05 23:59:59" {
				t.Errorf("%s = %v, want %q", test.expectedEndField, got, "2026-07-05 23:59:59")
			}
			for _, field := range test.unexpectedFields {
				if _, exists := requestBody[field]; exists {
					t.Errorf("request unexpectedly contains %s", field)
				}
			}
		})
	}
}

func TestParseOrderTimeFilter(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected OrderTimeFilter
		wantErr  bool
	}{
		{name: "default", expected: OrderTimeFilterCreated},
		{name: "created", input: "created", expected: OrderTimeFilterCreated},
		{name: "success", input: "success", expected: OrderTimeFilterSuccess},
		{name: "trim whitespace", input: " success ", expected: OrderTimeFilterSuccess},
		{name: "unsupported", input: "pay", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := ParseOrderTimeFilter(test.input)
			if (err != nil) != test.wantErr {
				t.Fatalf("ParseOrderTimeFilter(%q) error = %v, wantErr %v", test.input, err, test.wantErr)
			}
			if actual != test.expected {
				t.Errorf("ParseOrderTimeFilter(%q) = %q, want %q", test.input, actual, test.expected)
			}
		})
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
