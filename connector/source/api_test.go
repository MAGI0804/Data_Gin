package source

import (
	"context"
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
