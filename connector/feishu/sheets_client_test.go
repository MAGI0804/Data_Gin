package feishu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"gin-biz-web-api/pkg/providerhttp"
)

func TestSheetsClientInspectsAndValidatesRequiredSheets(t *testing.T) {
	tokens := &fakeSheetsTokenProvider{token: "tenant-token-value-123"}
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet ||
			request.URL.Path != "/open-apis/sheets/v3/spreadsheets/spreadsheet_abc/sheets/query" ||
			request.Header.Get("Authorization") != "Bearer tenant-token-value-123" {
			t.Fatalf("request method=%s path=%s authorization=%q", request.Method, request.URL.Path, request.Header.Get("Authorization"))
		}
		_, _ = response.Write([]byte(`{
			"code":0,"msg":"success","data":{"sheets":[
				{"sheet_id":"sheet_hourly","title":" Hourly ","index":1,"hidden":false,"resource_type":"sheet","grid_properties":{"row_count":1000,"column_count":20,"frozen_row_count":1,"frozen_column_count":0}},
				{"sheet_id":"sheet_realtime","title":"Realtime","index":0,"hidden":false,"resource_type":"sheet","grid_properties":{"row_count":200,"column_count":10,"frozen_row_count":1,"frozen_column_count":1}}
			]}}
		`))
	}))
	defer server.Close()
	client, err := newSheetsClient(server.URL, tokens, server.Client(), true)
	if err != nil {
		t.Fatalf("newSheetsClient() error=%v", err)
	}
	metadata, err := client.Inspect(t.Context(), "spreadsheet_abc", []string{"sheet_realtime", "sheet_hourly"})
	if err != nil {
		t.Fatalf("Inspect() error=%v", err)
	}
	if metadata.SpreadsheetToken != "spreadsheet_abc" || len(metadata.Sheets) != 2 ||
		metadata.Sheets[0].SheetID != "sheet_realtime" || metadata.Sheets[1].Title != "Hourly" ||
		metadata.Sheets[1].GridProperties.RowCount != 1000 || tokens.tokenCalls != 1 || tokens.refreshCalls != 0 {
		t.Fatalf("metadata=%+v tokens=%+v", metadata, tokens)
	}
	encoded, err := json.Marshal(metadata)
	if err != nil || strings.Contains(string(encoded), "spreadsheet_abc") || strings.Contains(string(encoded), "sheet_realtime") {
		t.Fatalf("encoded metadata=%s error=%v", encoded, err)
	}
}

func TestSheetsClientRefreshesOnceAfterUnauthorized(t *testing.T) {
	tokens := &fakeSheetsTokenProvider{token: "tenant-token-expired-123", refreshed: "tenant-token-fresh-123"}
	var mu sync.Mutex
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		mu.Lock()
		requests++
		current := requests
		mu.Unlock()
		if current == 1 {
			if request.Header.Get("Authorization") != "Bearer tenant-token-expired-123" {
				t.Fatalf("first authorization=%q", request.Header.Get("Authorization"))
			}
			response.WriteHeader(http.StatusUnauthorized)
			return
		}
		if request.Header.Get("Authorization") != "Bearer tenant-token-fresh-123" {
			t.Fatalf("second authorization=%q", request.Header.Get("Authorization"))
		}
		_, _ = response.Write([]byte(validSheetsMetadataResponse("sheet_realtime")))
	}))
	defer server.Close()
	client, err := newSheetsClient(server.URL, tokens, server.Client(), true)
	if err != nil {
		t.Fatalf("newSheetsClient() error=%v", err)
	}
	if _, err := client.Inspect(t.Context(), "spreadsheet_abc", []string{"sheet_realtime"}); err != nil {
		t.Fatalf("Inspect() error=%v", err)
	}
	if tokens.tokenCalls != 1 || tokens.refreshCalls != 1 || requests != 2 {
		t.Fatalf("tokens=%+v requests=%d", tokens, requests)
	}
}

func TestSheetsClientClassifiesSafeHTTPFailures(t *testing.T) {
	secret := "tenant-token-must-not-leak"
	for _, test := range []struct {
		name      string
		status    int
		class     providerhttp.ErrorClass
		retryable bool
	}{
		{name: "forbidden", status: http.StatusForbidden, class: providerhttp.ErrorClassAuth},
		{name: "rate limited", status: http.StatusTooManyRequests, class: providerhttp.ErrorClassRateLimited, retryable: true},
		{name: "provider unavailable", status: http.StatusServiceUnavailable, class: providerhttp.ErrorClassProvider, retryable: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
				response.WriteHeader(test.status)
				_, _ = response.Write([]byte(`{"msg":"` + secret + `"}`))
			}))
			defer server.Close()
			client, err := newSheetsClient(server.URL, &fakeSheetsTokenProvider{token: "tenant-token-value-123"}, server.Client(), true)
			if err != nil {
				t.Fatalf("newSheetsClient() error=%v", err)
			}
			_, err = client.Inspect(t.Context(), "spreadsheet_abc", []string{"sheet_realtime"})
			var sheetsError *SheetsError
			if !errors.As(err, &sheetsError) || sheetsError.Class != test.class || sheetsError.Retryable != test.retryable ||
				strings.Contains(fmt.Sprintf("%v", err), secret) {
				t.Fatalf("Inspect() error=%v sheetsError=%+v", err, sheetsError)
			}
		})
	}
}

func TestSheetsClientRejectsMissingOrMalformedSheets(t *testing.T) {
	responses := []struct {
		name string
		body string
	}{
		{name: "missing required", body: validSheetsMetadataResponse("sheet_other")},
		{name: "duplicate id", body: `{"code":0,"data":{"sheets":[
			{"sheet_id":"sheet_same","title":"A","index":0,"resource_type":"sheet","grid_properties":{"row_count":10,"column_count":10}},
			{"sheet_id":"sheet_same","title":"B","index":1,"resource_type":"sheet","grid_properties":{"row_count":10,"column_count":10}}
		]}}`},
		{name: "invalid grid", body: `{"code":0,"data":{"sheets":[
			{"sheet_id":"sheet_realtime","title":"Realtime","index":0,"resource_type":"sheet","grid_properties":{"row_count":0,"column_count":10}}
		]}}`},
	}
	for _, test := range responses {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
				_, _ = response.Write([]byte(test.body))
			}))
			defer server.Close()
			client, err := newSheetsClient(server.URL, &fakeSheetsTokenProvider{token: "tenant-token-value-123"}, server.Client(), true)
			if err != nil {
				t.Fatalf("newSheetsClient() error=%v", err)
			}
			if _, err := client.Inspect(t.Context(), "spreadsheet_abc", []string{"sheet_realtime"}); err == nil {
				t.Fatal("Inspect() accepted invalid metadata")
			}
		})
	}
}

func TestSheetsClientRejectsUnsafeResourceIdentifiersBeforeNetwork(t *testing.T) {
	tokens := &fakeSheetsTokenProvider{token: "tenant-token-value-123"}
	client, err := newSheetsClient("https://open.feishu.cn", tokens, &http.Client{}, false)
	if err != nil {
		t.Fatalf("newSheetsClient() error=%v", err)
	}
	for _, request := range []struct {
		spreadsheet string
		sheets      []string
	}{
		{spreadsheet: "../secret", sheets: []string{"sheet_realtime"}},
		{spreadsheet: "x", sheets: []string{"sheet_realtime"}},
		{spreadsheet: "spreadsheet_abc", sheets: nil},
		{spreadsheet: "spreadsheet_abc", sheets: []string{"sheet_realtime", "sheet_realtime"}},
		{spreadsheet: "spreadsheet_abc", sheets: []string{"sheet/unsafe"}},
	} {
		if _, err := client.Inspect(t.Context(), request.spreadsheet, request.sheets); err == nil {
			t.Fatalf("Inspect(%q,%v) accepted unsafe input", request.spreadsheet, request.sheets)
		}
	}
	if tokens.tokenCalls != 0 || tokens.refreshCalls != 0 {
		t.Fatalf("token provider was called: %+v", tokens)
	}
}

func validSheetsMetadataResponse(sheetID string) string {
	return `{"code":0,"msg":"success","data":{"sheets":[` +
		`{"sheet_id":"` + sheetID + `","title":"Realtime","index":0,"hidden":false,"resource_type":"sheet",` +
		`"grid_properties":{"row_count":100,"column_count":10,"frozen_row_count":1,"frozen_column_count":0}}]}}`
}

type fakeSheetsTokenProvider struct {
	mu           sync.Mutex
	token        string
	refreshed    string
	tokenErr     error
	refreshErr   error
	tokenCalls   int
	refreshCalls int
}

func (provider *fakeSheetsTokenProvider) Token(context.Context) (string, error) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	provider.tokenCalls++
	return provider.token, provider.tokenErr
}

func (provider *fakeSheetsTokenProvider) Refresh(context.Context) (string, error) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	provider.refreshCalls++
	return provider.refreshed, provider.refreshErr
}
