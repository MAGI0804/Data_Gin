package feishu

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"gin-biz-web-api/pkg/providerhttp"
)

func TestSheetsClientAppendsValidatedScalarBatch(t *testing.T) {
	tokens := &fakeSheetsTokenProvider{token: "tenant-token-value-123"}
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost ||
			request.URL.Path != "/open-apis/sheets/v2/spreadsheets/spreadsheet_abc/values_append" ||
			request.Header.Get("Authorization") != "Bearer tenant-token-value-123" ||
			request.Header.Get("Content-Type") != "application/json" ||
			request.Header.Get("Accept") != "application/json" {
			t.Fatalf("request method=%s path=%s headers=%v", request.Method, request.URL.Path, request.Header)
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		var decoded struct {
			ValueRange struct {
				Range  string  `json:"range"`
				Values [][]any `json:"values"`
			} `json:"valueRange"`
		}
		decoder := json.NewDecoder(strings.NewReader(string(body)))
		decoder.UseNumber()
		if err := decoder.Decode(&decoded); err != nil || decoded.ValueRange.Range != "sheet_hourly!B2:E2" ||
			len(decoded.ValueRange.Values) != 1 || len(decoded.ValueRange.Values[0]) != 4 ||
			decoded.ValueRange.Values[0][0] != "mall-a" || decoded.ValueRange.Values[0][1] != json.Number("12.5") ||
			decoded.ValueRange.Values[0][2] != true || decoded.ValueRange.Values[0][3] != nil {
			t.Fatalf("request body=%s error=%v", body, err)
		}
		_, _ = response.Write([]byte(`{"code":0,"data":{"updates":{
			"revision":11,"updatedRange":"sheet_hourly!B7:E7",
			"updatedRows":1,"updatedColumns":4,"updatedCells":4
		}}}`))
	}))
	defer server.Close()

	client, err := newSheetsClient(server.URL, tokens, server.Client(), true)
	if err != nil {
		t.Fatalf("newSheetsClient() error=%v", err)
	}
	result, err := client.AppendValues(t.Context(), "spreadsheet_abc", SheetWriteRange{
		Range: SheetRange{SheetID: "sheet_hourly", StartRow: 2, EndRow: 2, StartColumn: 2, EndColumn: 5},
		Rows: [][]SheetCell{{
			{Type: SheetCellString, Text: "mall-a"},
			{Type: SheetCellNumber, Number: "12.5"},
			{Type: SheetCellBoolean, Boolean: true},
			{Type: SheetCellBlank},
		}},
	})
	if err != nil || result == nil || result.Revision != 11 || result.UpdatedRowStart != 7 ||
		result.UpdatedRowEnd != 7 || result.UpdatedRows != 1 || result.UpdatedColumns != 4 ||
		result.UpdatedCells != 4 || tokens.tokenCalls != 1 || tokens.refreshCalls != 0 {
		t.Fatalf("AppendValues() result=%+v error=%v tokens=%+v", result, err, tokens)
	}
}

func TestSheetsClientBatchUpdatesMultipleFixedRanges(t *testing.T) {
	tokens := &fakeSheetsTokenProvider{token: "tenant-token-value-123"}
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost ||
			request.URL.Path != "/open-apis/sheets/v2/spreadsheets/spreadsheet_abc/values_batch_update" {
			t.Fatalf("request method=%s path=%s", request.Method, request.URL.Path)
		}
		var decoded struct {
			ValueRanges []struct {
				Range  string  `json:"range"`
				Values [][]any `json:"values"`
			} `json:"valueRanges"`
		}
		if err := json.NewDecoder(request.Body).Decode(&decoded); err != nil || len(decoded.ValueRanges) != 2 ||
			decoded.ValueRanges[0].Range != "sheet_hourly!A2:B2" ||
			decoded.ValueRanges[1].Range != "sheet_hourly!A8:A9" ||
			len(decoded.ValueRanges[1].Values) != 2 {
			t.Fatalf("batch request=%+v error=%v", decoded, err)
		}
		_, _ = response.Write([]byte(`{"code":0,"data":{"revision":15,"responses":[{"revision":14},{"revision":15}]}}`))
	}))
	defer server.Close()

	client, err := newSheetsClient(server.URL, tokens, server.Client(), true)
	if err != nil {
		t.Fatalf("newSheetsClient() error=%v", err)
	}
	result, err := client.BatchUpdateValues(t.Context(), "spreadsheet_abc", []SheetWriteRange{
		{
			Range: SheetRange{SheetID: "sheet_hourly", StartRow: 2, EndRow: 2, StartColumn: 1, EndColumn: 2},
			Rows:  [][]SheetCell{{{Type: SheetCellString, Text: "a"}, {Type: SheetCellNumber, Number: "1"}}},
		},
		{
			Range: SheetRange{SheetID: "sheet_hourly", StartRow: 8, EndRow: 9, StartColumn: 1, EndColumn: 1},
			Rows: [][]SheetCell{
				{{Type: SheetCellString, Text: "b"}},
				{{Type: SheetCellString, Text: "c"}},
			},
		},
	})
	if err != nil || result == nil || result.Revision != 15 || tokens.tokenCalls != 1 {
		t.Fatalf("BatchUpdateValues() result=%+v error=%v tokens=%+v", result, err, tokens)
	}
}

func TestSheetsClientWriteRefreshesOnlyAfterUnauthorized(t *testing.T) {
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
		_, _ = response.Write([]byte(`{"code":0,"data":{"revision":3,"updates":{
			"updatedRange":"sheet_hourly!A9:A9","updatedRows":1,"updatedColumns":1,"updatedCells":1
		}}}`))
	}))
	defer server.Close()

	client, err := newSheetsClient(server.URL, tokens, server.Client(), true)
	if err != nil {
		t.Fatalf("newSheetsClient() error=%v", err)
	}
	if _, err := client.AppendValues(t.Context(), "spreadsheet_abc", singleStringWrite("sheet_hourly", "value")); err != nil {
		t.Fatalf("AppendValues() error=%v", err)
	}
	if tokens.tokenCalls != 1 || tokens.refreshCalls != 1 || requests != 2 {
		t.Fatalf("tokens=%+v requests=%d", tokens, requests)
	}
}

func TestSheetsClientWriteDoesNotRetryTaskLevelFailures(t *testing.T) {
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
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
				requests++
				response.WriteHeader(test.status)
				_, _ = response.Write([]byte(`{"msg":"remote-secret"}`))
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
			_, err = client.AppendValues(t.Context(), "spreadsheet_abc", singleStringWrite("sheet_hourly", "cell-secret"))
			var sheetsError *SheetsError
			if !errors.As(err, &sheetsError) || sheetsError.Class != test.class ||
				sheetsError.Retryable != test.retryable || requests != 1 {
				t.Fatalf("AppendValues() error=%v sheetsError=%+v requests=%d", err, sheetsError, requests)
			}
		})
	}
}

func TestSheetsClientWriteRejectsUnsafeInputBeforeNetwork(t *testing.T) {
	tokens := &fakeSheetsTokenProvider{token: "tenant-token-value-123"}
	client, err := newSheetsClient("https://open.feishu.cn", tokens, &http.Client{}, false)
	if err != nil {
		t.Fatalf("newSheetsClient() error=%v", err)
	}
	valid := singleStringWrite("sheet_hourly", "value")
	tests := []struct {
		name        string
		spreadsheet string
		write       SheetWriteRange
	}{
		{name: "unsafe spreadsheet", spreadsheet: "../secret", write: valid},
		{name: "unsafe sheet", spreadsheet: "spreadsheet_abc", write: singleStringWrite("sheet/unsafe", "value")},
		{
			name: "zero row", spreadsheet: "spreadsheet_abc",
			write: SheetWriteRange{
				Range: SheetRange{SheetID: "sheet_hourly", StartRow: 0, EndRow: 1, StartColumn: 1, EndColumn: 1},
				Rows:  [][]SheetCell{{{Type: SheetCellString, Text: "value"}}},
			},
		},
		{
			name: "too many rows", spreadsheet: "spreadsheet_abc",
			write: SheetWriteRange{
				Range: SheetRange{SheetID: "sheet_hourly", StartRow: 1, EndRow: maxSheetWriteRows + 1, StartColumn: 1, EndColumn: 1},
				Rows:  [][]SheetCell{{{Type: SheetCellString, Text: "value"}}},
			},
		},
		{
			name: "too many columns", spreadsheet: "spreadsheet_abc",
			write: SheetWriteRange{
				Range: SheetRange{SheetID: "sheet_hourly", StartRow: 1, EndRow: 1, StartColumn: 1, EndColumn: maxSheetWriteColumns + 1},
				Rows:  [][]SheetCell{{{Type: SheetCellString, Text: "value"}}},
			},
		},
		{
			name: "ragged values", spreadsheet: "spreadsheet_abc",
			write: SheetWriteRange{
				Range: SheetRange{SheetID: "sheet_hourly", StartRow: 1, EndRow: 1, StartColumn: 1, EndColumn: 2},
				Rows:  [][]SheetCell{{{Type: SheetCellString, Text: "value"}}},
			},
		},
		{name: "invalid cell type", spreadsheet: "spreadsheet_abc", write: SheetWriteRange{
			Range: valid.Range, Rows: [][]SheetCell{{{Type: SheetCellType("formula"), Text: "=1+1"}}},
		}},
		{name: "invalid number", spreadsheet: "spreadsheet_abc", write: SheetWriteRange{
			Range: valid.Range, Rows: [][]SheetCell{{{Type: SheetCellNumber, Number: "NaN"}}},
		}},
		{name: "mixed cell fields", spreadsheet: "spreadsheet_abc", write: SheetWriteRange{
			Range: valid.Range, Rows: [][]SheetCell{{{Type: SheetCellString, Text: "value", Number: "1"}}},
		}},
		{name: "request body limit", spreadsheet: "spreadsheet_abc", write: singleStringWrite(
			"sheet_hourly", strings.Repeat("x", maxSheetsWriteRequestBodyBytes),
		)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := client.AppendValues(t.Context(), test.spreadsheet, test.write); err == nil {
				t.Fatalf("AppendValues() accepted spreadsheet=%q write=%+v", test.spreadsheet, test.write)
			}
		})
	}
	if _, err := client.BatchUpdateValues(t.Context(), "spreadsheet_abc", nil); err == nil {
		t.Fatal("BatchUpdateValues() accepted empty ranges")
	}
	if _, err := client.BatchUpdateValues(
		t.Context(), "spreadsheet_abc", make([]SheetWriteRange, maxSheetWriteRanges+1),
	); err == nil {
		t.Fatal("BatchUpdateValues() accepted too many ranges")
	}
	if tokens.tokenCalls != 0 || tokens.refreshCalls != 0 {
		t.Fatalf("token provider was called: %+v", tokens)
	}
}

func TestSheetsClientBatchUpdateRejectsAggregateCellOverflow(t *testing.T) {
	tokens := &fakeSheetsTokenProvider{token: "tenant-token-value-123"}
	client, err := newSheetsClient("https://open.feishu.cn", tokens, &http.Client{}, false)
	if err != nil {
		t.Fatalf("newSheetsClient() error=%v", err)
	}
	rows := makeScalarRows(500, 128)
	writes := []SheetWriteRange{
		{
			Range: SheetRange{SheetID: "sheet_hourly", StartRow: 1, EndRow: 500, StartColumn: 1, EndColumn: 128},
			Rows:  rows,
		},
		{
			Range: SheetRange{SheetID: "sheet_hourly", StartRow: 501, EndRow: 1000, StartColumn: 1, EndColumn: 128},
			Rows:  rows,
		},
	}
	if _, err := client.BatchUpdateValues(t.Context(), "spreadsheet_abc", writes); err == nil {
		t.Fatal("BatchUpdateValues() accepted aggregate cell overflow")
	}
	if tokens.tokenCalls != 0 {
		t.Fatalf("token provider was called: %+v", tokens)
	}
}

func TestSheetsClientWriteRejectsMalformedResponses(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "malformed", body: `{"code":`},
		{name: "trailing data", body: `{"code":0,"data":{}}{}`},
		{name: "missing data", body: `{"code":0}`},
		{name: "negative revision", body: `{"code":0,"data":{"revision":-1}}`},
		{name: "missing append acknowledgement", body: `{"code":0,"data":{"revision":1}}`},
		{name: "unsafe updated range", body: `{"code":0,"data":{"updates":{"updatedRange":"../sheet!A1:A1","updatedRows":1,"updatedColumns":1,"updatedCells":1}}}`},
		{name: "wrong append dimensions", body: `{"code":0,"data":{"updates":{"updatedRange":"sheet_hourly!A1:A2","updatedRows":2,"updatedColumns":1,"updatedCells":2}}}`},
		{name: "oversized", body: strings.Repeat("x", maxSheetsResponseBodyBytes+1)},
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
			_, err = client.AppendValues(t.Context(), "spreadsheet_abc", singleStringWrite("sheet_hourly", "value"))
			var sheetsError *SheetsError
			if !errors.As(err, &sheetsError) || sheetsError.Class != providerhttp.ErrorClassResponse {
				t.Fatalf("AppendValues() error=%v sheetsError=%+v", err, sheetsError)
			}
		})
	}
}

func TestSheetsClientWriteDiagnosticsDoNotLeakSecrets(t *testing.T) {
	const (
		spreadsheetSecret = "spreadsheet_secret_123"
		sheetSecret       = "sheet_secret_123"
		cellSecret        = "cell-secret-must-not-leak"
		responseSecret    = "response-secret-must-not-leak"
	)
	write := singleStringWrite(sheetSecret, cellSecret)
	encoded, err := json.Marshal(write)
	if err != nil || strings.Contains(string(encoded), sheetSecret) || strings.Contains(string(encoded), cellSecret) ||
		strings.Contains(fmt.Sprintf("%+v", write), sheetSecret) || strings.Contains(fmt.Sprintf("%+v", write), cellSecret) {
		t.Fatalf("write diagnostics=%s formatted=%+v error=%v", encoded, write, err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusForbidden)
		_, _ = response.Write([]byte(`{"msg":"` + responseSecret + `"}`))
	}))
	defer server.Close()
	client, err := newSheetsClient(
		server.URL,
		&fakeSheetsTokenProvider{token: "tenant-token-must-not-leak"},
		server.Client(),
		true,
	)
	if err != nil {
		t.Fatalf("newSheetsClient() error=%v", err)
	}
	_, err = client.AppendValues(t.Context(), spreadsheetSecret, write)
	diagnostic := fmt.Sprintf("%+v", err)
	for _, secret := range []string{spreadsheetSecret, sheetSecret, cellSecret, responseSecret, "tenant-token-must-not-leak"} {
		if strings.Contains(diagnostic, secret) {
			t.Fatalf("error diagnostic leaks %q: %s", secret, diagnostic)
		}
	}
}

func TestSheetsClientWriteSanitizesTransportCause(t *testing.T) {
	const (
		spreadsheetSecret = "spreadsheet_transport_secret"
		cellSecret        = "cell-transport-secret"
		tokenSecret       = "tenant-token-transport-secret"
		transportSecret   = "transport-internal-secret"
	)
	httpClient := &http.Client{Transport: sheetWriteRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf(
			"%s url=%s authorization=%s",
			transportSecret,
			request.URL.String(),
			request.Header.Get("Authorization"),
		)
	})}
	client, err := newSheetsClient(
		"https://open.feishu.cn",
		&fakeSheetsTokenProvider{token: tokenSecret},
		httpClient,
		false,
	)
	if err != nil {
		t.Fatalf("newSheetsClient() error=%v", err)
	}
	_, err = client.AppendValues(
		t.Context(),
		spreadsheetSecret,
		singleStringWrite("sheet_hourly", cellSecret),
	)
	var sheetsError *SheetsError
	if !errors.As(err, &sheetsError) || sheetsError.Class != providerhttp.ErrorClassTransport || !sheetsError.Retryable {
		t.Fatalf("AppendValues() error=%v sheetsError=%+v", err, sheetsError)
	}
	diagnostic := fmt.Sprintf("error=%+v cause=%+v", err, errors.Unwrap(err))
	for _, secret := range []string{spreadsheetSecret, cellSecret, tokenSecret, transportSecret} {
		if strings.Contains(diagnostic, secret) {
			t.Fatalf("transport diagnostic leaks %q: %s", secret, diagnostic)
		}
	}
}

func singleStringWrite(sheetID string, value string) SheetWriteRange {
	return SheetWriteRange{
		Range: SheetRange{SheetID: sheetID, StartRow: 1, EndRow: 1, StartColumn: 1, EndColumn: 1},
		Rows:  [][]SheetCell{{{Type: SheetCellString, Text: value}}},
	}
}

func makeScalarRows(rows int, columns int) [][]SheetCell {
	result := make([][]SheetCell, rows)
	for row := range result {
		result[row] = make([]SheetCell, columns)
		for column := range result[row] {
			result[row][column] = SheetCell{Type: SheetCellBlank}
		}
	}
	return result
}

type sheetWriteRoundTripperFunc func(*http.Request) (*http.Response, error)

func (roundTrip sheetWriteRoundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}
