package source

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPIConnectorFetchExtractsRecordsFromPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want %s", r.Method, http.MethodPost)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"items":[{"id":1,"name":"first"},{"id":2,"name":"second"}]}}`))
	}))
	defer server.Close()

	connector := APIConnector{}
	result, err := connector.Fetch(context.Background(), Config{
		"url":          server.URL,
		"method":       http.MethodPost,
		"records_path": "data.items",
	}, FetchCursor{})
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}

	if len(result.Records) != 2 {
		t.Fatalf("len(Records) = %d, want 2", len(result.Records))
	}
	if result.Records[0]["name"] != "first" {
		t.Fatalf("first record name = %v, want first", result.Records[0]["name"])
	}
}

func TestAPIConnectorFetchRejectsNonSuccessStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"failed"}`))
	}))
	defer server.Close()

	connector := APIConnector{}
	_, err := connector.Fetch(context.Background(), Config{
		"url": server.URL,
	}, FetchCursor{})
	if err == nil {
		t.Fatal("Fetch returned nil error, want http status error")
	}
}

func TestAPIConnectorFetchBuildsDynamicQueryHeaderAndBody(t *testing.T) {
	t.Setenv("API_SOURCE_TEST_HEADER", "header-from-env")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("shop") != "S001" {
			t.Fatalf("shop query = %q, want S001", r.URL.Query().Get("shop"))
		}
		if r.Header.Get("X-Test") != "header-from-env" {
			t.Fatalf("X-Test header = %q, want header-from-env", r.Header.Get("X-Test"))
		}

		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["page_size"] != float64(100) {
			t.Fatalf("page_size = %v, want 100", body["page_size"])
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"items":[{"id":1}]}}`))
	}))
	defer server.Close()

	connector := APIConnector{}
	result, err := connector.Fetch(context.Background(), Config{
		"url":    server.URL,
		"method": http.MethodPost,
		"query_params": []interface{}{
			map[string]interface{}{"name": "shop", "value": map[string]interface{}{"source": "static", "value": "S001"}},
		},
		"headers": []interface{}{
			map[string]interface{}{"name": "X-Test", "value": map[string]interface{}{"source": "env", "name": "API_SOURCE_TEST_HEADER"}},
			map[string]interface{}{"name": "Content-Type", "value": "application/json"},
		},
		"body_json": map[string]interface{}{
			"page_size": map[string]interface{}{"source": "static", "value": 100},
		},
		"records_path": "data.items",
	}, FetchCursor{})
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if len(result.Records) != 1 {
		t.Fatalf("len(Records) = %d, want 1", len(result.Records))
	}
}

func TestAPIConnectorFetchSupportsTokenRequestInjection(t *testing.T) {
	var tokenRequested bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			tokenRequested = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success":true,"code":200,"data":{"access_token":"token-1"}}`))
		case "/orders":
			if !tokenRequested {
				t.Fatal("orders endpoint called before token endpoint")
			}
			if got := r.URL.Query().Get("access_token"); got != "token-1" {
				t.Fatalf("access_token = %q, want token-1", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"items":[{"tid":"T1"}]}}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	connector := APIConnector{}
	result, err := connector.Fetch(context.Background(), Config{
		"url":    server.URL + "/orders",
		"method": http.MethodPost,
		"auth": map[string]interface{}{
			"type": "request_token",
			"request": map[string]interface{}{
				"url":    server.URL + "/token",
				"method": http.MethodPost,
				"headers": []interface{}{
					map[string]interface{}{"name": "Content-Type", "value": "application/json"},
				},
				"body_json": map[string]interface{}{
					"authorize_type": "silent",
					"client_id":      map[string]interface{}{"source": "static", "value": "client-1"},
				},
			},
			"token_path": "data.access_token",
			"inject": map[string]interface{}{
				"in":   "query",
				"name": "access_token",
			},
		},
		"body_json": map[string]interface{}{
			"page_size":     100,
			"start_success": map[string]interface{}{"source": "time", "format": "2006-01-02 15:04:05", "offset_seconds": -300},
			"end_success":   map[string]interface{}{"source": "time", "format": "2006-01-02 15:04:05"},
		},
		"records_path": "data.items",
	}, FetchCursor{})
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if len(result.Records) != 1 || result.Records[0]["tid"] != "T1" {
		t.Fatalf("records = %#v, want tid T1", result.Records)
	}
}
