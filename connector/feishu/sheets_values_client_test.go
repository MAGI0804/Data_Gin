package feishu

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"gin-biz-web-api/pkg/providerhttp"
)

func TestSheetsClientReadsValidatedScalarRange(t *testing.T) {
	tokens := &fakeSheetsTokenProvider{token: "tenant-token-value-123"}
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet ||
			request.URL.Path != "/open-apis/sheets/v2/spreadsheets/spreadsheet_abc/values/sheet_hourly!A1:D2" ||
			request.Header.Get("Authorization") != "Bearer tenant-token-value-123" {
			t.Fatalf(
				"request method=%s path=%s authorization=%q",
				request.Method,
				request.URL.Path,
				request.Header.Get("Authorization"),
			)
		}
		_, _ = response.Write([]byte(`{
			"code":0,"msg":"success","data":{"revision":9,"valueRange":{
				"range":"sheet_hourly!A1:D2","revision":9,
				"values":[["name",12.5,true,null],["city"]]
			}}}
		`))
	}))
	defer server.Close()

	client, err := newSheetsClient(server.URL, tokens, server.Client(), true)
	if err != nil {
		t.Fatalf("newSheetsClient() error=%v", err)
	}
	values, err := client.ReadRange(t.Context(), "spreadsheet_abc", SheetRange{
		SheetID: "sheet_hourly", StartRow: 1, EndRow: 2, StartColumn: 1, EndColumn: 4,
	})
	if err != nil {
		t.Fatalf("ReadRange() error=%v", err)
	}
	if values.Revision != 9 || len(values.Rows) != 2 || len(values.Rows[0]) != 4 ||
		values.Rows[0][0].Type != SheetCellString || values.Rows[0][0].Text != "name" ||
		values.Rows[0][1].Type != SheetCellNumber || values.Rows[0][1].Number.String() != "12.5" ||
		values.Rows[0][2].Type != SheetCellBoolean || !values.Rows[0][2].Boolean ||
		values.Rows[0][3].Type != SheetCellBlank || tokens.tokenCalls != 1 || tokens.refreshCalls != 0 {
		t.Fatalf("values=%+v tokens=%+v", values, tokens)
	}
	encoded, err := json.Marshal(values)
	if err != nil || strings.Contains(string(encoded), "spreadsheet_abc") ||
		strings.Contains(string(encoded), "sheet_hourly") || strings.Contains(string(encoded), "name") {
		t.Fatalf("encoded values=%s error=%v", encoded, err)
	}
	if diagnostic := fmt.Sprintf("%+v", values); strings.Contains(diagnostic, "name") {
		t.Fatalf("diagnostic formatting leaks remote cell values")
	}
}

func TestSheetsClientReadRangeRefreshesOnceAfterUnauthorized(t *testing.T) {
	tokens := &fakeSheetsTokenProvider{token: "tenant-token-expired-123", refreshed: "tenant-token-fresh-123"}
	var mu sync.Mutex
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		mu.Lock()
		requests++
		current := requests
		mu.Unlock()
		if current == 1 {
			response.WriteHeader(http.StatusUnauthorized)
			return
		}
		if request.Header.Get("Authorization") != "Bearer tenant-token-fresh-123" {
			t.Fatalf("refreshed authorization=%q", request.Header.Get("Authorization"))
		}
		_, _ = response.Write([]byte(validSheetValuesResponse("sheet_hourly!A1:A1", `[["header"]]`)))
	}))
	defer server.Close()

	client, err := newSheetsClient(server.URL, tokens, server.Client(), true)
	if err != nil {
		t.Fatalf("newSheetsClient() error=%v", err)
	}
	if _, err := client.ReadRange(t.Context(), "spreadsheet_abc", SheetRange{
		SheetID: "sheet_hourly", StartRow: 1, EndRow: 1, StartColumn: 1, EndColumn: 1,
	}); err != nil {
		t.Fatalf("ReadRange() error=%v", err)
	}
	if tokens.tokenCalls != 1 || tokens.refreshCalls != 1 || requests != 2 {
		t.Fatalf("tokens=%+v requests=%d", tokens, requests)
	}
}

func TestSheetsClientReadRangeRejectsMalformedResponses(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "mismatched range",
			body: validSheetValuesResponse("sheet_other!A1:D2", `[["header"]]`),
		},
		{
			name: "composite cell",
			body: validSheetValuesResponse("sheet_hourly!A1:D2", `[[{"text":"unsafe"}]]`),
		},
		{
			name: "too many columns",
			body: validSheetValuesResponse("sheet_hourly!A1:D2", `[[1,2,3,4,5]]`),
		},
		{
			name: "negative revision",
			body: `{"code":0,"data":{"revision":-1,"valueRange":{"range":"sheet_hourly!A1:D2","values":[]}}}`,
		},
		{
			name: "trailing data",
			body: validSheetValuesResponse("sheet_hourly!A1:D2", `[]`) + `{}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
				_, _ = response.Write([]byte(test.body))
			}))
			defer server.Close()

			client, err := newSheetsClient(
				server.URL,
				&fakeSheetsTokenProvider{token: "tenant-token-value-123"},
				server.Client(),
				true,
			)
			if err != nil {
				t.Fatalf("newSheetsClient() error=%v", err)
			}
			_, err = client.ReadRange(t.Context(), "spreadsheet_abc", SheetRange{
				SheetID: "sheet_hourly", StartRow: 1, EndRow: 2, StartColumn: 1, EndColumn: 4,
			})
			var sheetsError *SheetsError
			if !errors.As(err, &sheetsError) || sheetsError.Class != providerhttp.ErrorClassResponse {
				t.Fatalf("ReadRange() error=%v", err)
			}
		})
	}
}

func TestSheetsClientReadRangeRejectsUnsafeBoundsBeforeNetwork(t *testing.T) {
	tokens := &fakeSheetsTokenProvider{token: "tenant-token-value-123"}
	client, err := newSheetsClient("https://open.feishu.cn", tokens, &http.Client{}, false)
	if err != nil {
		t.Fatalf("newSheetsClient() error=%v", err)
	}
	tests := []struct {
		name        string
		spreadsheet string
		readRange   SheetRange
	}{
		{
			name: "unsafe spreadsheet", spreadsheet: "../secret",
			readRange: SheetRange{SheetID: "sheet_hourly", StartRow: 1, EndRow: 1, StartColumn: 1, EndColumn: 1},
		},
		{
			name: "unsafe sheet", spreadsheet: "spreadsheet_abc",
			readRange: SheetRange{SheetID: "sheet/unsafe", StartRow: 1, EndRow: 1, StartColumn: 1, EndColumn: 1},
		},
		{
			name: "zero row", spreadsheet: "spreadsheet_abc",
			readRange: SheetRange{SheetID: "sheet_hourly", StartRow: 0, EndRow: 1, StartColumn: 1, EndColumn: 1},
		},
		{
			name: "reversed columns", spreadsheet: "spreadsheet_abc",
			readRange: SheetRange{SheetID: "sheet_hourly", StartRow: 1, EndRow: 1, StartColumn: 2, EndColumn: 1},
		},
		{
			name: "row overflow", spreadsheet: "spreadsheet_abc",
			readRange: SheetRange{SheetID: "sheet_hourly", StartRow: 1, EndRow: math.MaxInt64, StartColumn: 1, EndColumn: 1},
		},
		{
			name: "too many cells", spreadsheet: "spreadsheet_abc",
			readRange: SheetRange{
				SheetID: "sheet_hourly", StartRow: 1, EndRow: maxSheetReadRows,
				StartColumn: 1, EndColumn: maxSheetReadColumns,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := client.ReadRange(t.Context(), test.spreadsheet, test.readRange); err == nil {
				t.Fatalf("ReadRange() accepted spreadsheet=%q range=%+v", test.spreadsheet, test.readRange)
			}
		})
	}
	if tokens.tokenCalls != 0 || tokens.refreshCalls != 0 {
		t.Fatalf("token provider was called: %+v", tokens)
	}
}

func TestSheetsClientReadRangeClassifiesFailuresWithoutLeaks(t *testing.T) {
	const sensitiveValue = "tenant-token-and-response-must-not-leak"
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusForbidden)
		_, _ = response.Write([]byte(`{"msg":"` + sensitiveValue + `"}`))
	}))
	defer server.Close()

	client, err := newSheetsClient(
		server.URL,
		&fakeSheetsTokenProvider{token: sensitiveValue},
		server.Client(),
		true,
	)
	if err != nil {
		t.Fatalf("newSheetsClient() error=%v", err)
	}
	_, err = client.ReadRange(t.Context(), "spreadsheet_abc", SheetRange{
		SheetID: "sheet_hourly", StartRow: 1, EndRow: 1, StartColumn: 1, EndColumn: 1,
	})
	var sheetsError *SheetsError
	if !errors.As(err, &sheetsError) || sheetsError.Class != providerhttp.ErrorClassAuth || sheetsError.Retryable ||
		strings.Contains(fmt.Sprintf("%v", err), sensitiveValue) {
		t.Fatalf("ReadRange() error=%v sheetsError=%+v", err, sheetsError)
	}
}

func validSheetValuesResponse(a1Range string, values string) string {
	return `{"code":0,"msg":"success","data":{"revision":7,"valueRange":{"range":"` + a1Range +
		`","revision":7,"values":` + values + `}}}`
}
