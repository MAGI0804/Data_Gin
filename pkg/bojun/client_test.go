package bojun

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/google/uuid"
)

func TestBuildSignMatchesPythonAlgorithm(t *testing.T) {
	sign := BuildSign(
		"test_appkey",
		"json",
		"/store/store.query",
		"1234567890",
		"test-uniqstr",
		"test_secret",
	)
	if sign != "14C997F4A4C4A7B1475B083039715E91" {
		t.Fatalf("sign = %s", sign)
	}
}

func TestSendSignedRequestWithConfigPostsSignedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bos/standard/retail/retail.query" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		expectedHeaders := map[string]string{
			"appkey":       "test_appkey",
			"method":       "/retail/retail.query",
			"format":       "json",
			"Content-Type": "application/json",
		}
		for key, want := range expectedHeaders {
			if got := r.Header.Get(key); got != want {
				t.Fatalf("header %s = %s, want %s", key, got, want)
			}
		}
		timestamp := r.Header.Get("timestamp")
		if _, err := strconv.ParseInt(timestamp, 10, 64); err != nil {
			t.Fatalf("timestamp = %q: %v", timestamp, err)
		}
		uniqstr := r.Header.Get("uniqstr")
		if _, err := uuid.Parse(uniqstr); err != nil {
			t.Fatalf("uniqstr = %q: %v", uniqstr, err)
		}
		wantSign := BuildSign("test_appkey", "json", "/retail/retail.query", timestamp, uniqstr, "test_secret")
		if got := r.Header.Get("sign"); got != wantSign {
			t.Fatalf("sign = %s, want %s", got, wantSign)
		}
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["docno"] != "ABCN001A001P12607022058280027" {
			t.Fatalf("docno = %v", body["docno"])
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code": 0,
			"data": map[string]interface{}{"docno": body["docno"]},
		})
	}))
	defer server.Close()

	got, err := SendSignedRequestWithConfig(context.Background(), Config{
		BaseURL: server.URL + "/bos/standard",
		AppKey:  "test_appkey",
		Secret:  "test_secret",
		Format:  "json",
	}, "/retail/retail.query", map[string]interface{}{
		"docno": "ABCN001A001P12607022058280027",
	})
	if err != nil {
		t.Fatalf("SendSignedRequestWithConfig returned error: %v", err)
	}
	if got["code"].(float64) != 0 {
		t.Fatalf("code = %v", got["code"])
	}
}

func TestSendSignedRequestWithConfigRequiresSecret(t *testing.T) {
	_, err := SendSignedRequestWithConfig(context.Background(), Config{
		BaseURL: "http://example.com",
		AppKey:  "test_appkey",
	}, "/store/store.query", map[string]interface{}{})
	if err == nil {
		t.Fatal("expected missing secret error")
	}
}
