package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"gin-biz-web-api/pkg/providerhttp"
)

const maxSheetsMetadataBodyBytes = 1024 * 1024

var (
	feishuSpreadsheetTokenPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{8,128}$`)
	feishuSheetIDPattern          = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
)

type sheetsTokenProvider interface {
	Token(context.Context) (string, error)
	Refresh(context.Context) (string, error)
}

type SheetGridProperties struct {
	RowCount          int64
	ColumnCount       int64
	FrozenRowCount    int64
	FrozenColumnCount int64
}

type SheetMetadata struct {
	SheetID        string `json:"-"`
	Title          string
	Index          int
	Hidden         bool
	ResourceType   string
	GridProperties SheetGridProperties
}

type SpreadsheetMetadata struct {
	SpreadsheetToken string `json:"-"`
	Sheets           []SheetMetadata
}

type SheetsError struct {
	Class     providerhttp.ErrorClass
	Retryable bool
	HTTPCode  int
	Code      int
	cause     error
}

func (err *SheetsError) Error() string {
	if err == nil {
		return "feishu sheets: unknown error"
	}
	return fmt.Sprintf("feishu sheets: metadata request failed (%s)", err.Class)
}

func (err *SheetsError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.cause
}

type SheetsClient struct {
	baseURL *url.URL
	tokens  sheetsTokenProvider
	http    *http.Client
}

func NewSheetsClient(tokens *TenantTokenProvider, httpClient *http.Client) (*SheetsClient, error) {
	return newSheetsClient(defaultBaseURL, tokens, httpClient, false)
}

func newSheetsClient(
	baseURL string,
	tokens sheetsTokenProvider,
	httpClient *http.Client,
	allowLoopbackHTTP bool,
) (*SheetsClient, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed == nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" ||
		(parsed.Scheme != "https" && !(allowLoopbackHTTP && parsed.Scheme == "http" && isLoopbackHost(parsed.Hostname()))) ||
		tokens == nil {
		return nil, fmt.Errorf("feishu sheets: invalid client configuration")
	}
	if httpClient == nil {
		httpClient, err = providerhttp.NewClient(providerhttp.ClientConfig{})
		if err != nil {
			return nil, fmt.Errorf("feishu sheets: create HTTP client: %w", err)
		}
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return &SheetsClient{baseURL: parsed, tokens: tokens, http: httpClient}, nil
}

func (client *SheetsClient) Inspect(
	ctx context.Context,
	spreadsheetToken string,
	requiredSheetIDs []string,
) (*SpreadsheetMetadata, error) {
	if client == nil || client.baseURL == nil || client.tokens == nil || client.http == nil || ctx == nil ||
		!validFeishuSpreadsheetToken(spreadsheetToken) {
		return nil, fmt.Errorf("feishu sheets: invalid metadata request")
	}
	required, err := normalizeRequiredSheetIDs(requiredSheetIDs)
	if err != nil {
		return nil, err
	}
	token, err := client.tokens.Token(ctx)
	if err != nil {
		return nil, err
	}
	for attempt := 0; attempt < 2; attempt++ {
		metadata, statusCode, err := client.inspectOnce(ctx, spreadsheetToken, token)
		if statusCode == http.StatusUnauthorized && attempt == 0 {
			token, err = client.tokens.Refresh(ctx)
			if err != nil {
				return nil, err
			}
			continue
		}
		if err != nil {
			return nil, err
		}
		if err := validateRequiredSheets(metadata, required); err != nil {
			return nil, err
		}
		return metadata, nil
	}
	return nil, &SheetsError{Class: providerhttp.ErrorClassAuth, HTTPCode: http.StatusUnauthorized}
}

func (client *SheetsClient) inspectOnce(
	ctx context.Context,
	spreadsheetToken string,
	token string,
) (*SpreadsheetMetadata, int, error) {
	if !validTenantToken(token) {
		return nil, 0, fmt.Errorf("feishu sheets: token provider returned invalid token")
	}
	endpoint := *client.baseURL
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/open-apis/sheets/v3/spreadsheets/" +
		url.PathEscape(spreadsheetToken) + "/sheets/query"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, 0, fmt.Errorf("feishu sheets: create metadata request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")
	response, err := client.http.Do(request)
	if err != nil {
		classification := providerhttp.ClassifyRetry(0, err)
		return nil, 0, &SheetsError{Class: classification.Class, Retryable: classification.Retryable, cause: err}
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxSheetsMetadataBodyBytes+1))
	if err != nil {
		return nil, response.StatusCode, &SheetsError{
			Class: providerhttp.ErrorClassResponse, Retryable: true, HTTPCode: response.StatusCode, cause: err,
		}
	}
	if len(body) > maxSheetsMetadataBodyBytes {
		return nil, response.StatusCode, &SheetsError{
			Class: providerhttp.ErrorClassResponse, HTTPCode: response.StatusCode, cause: fmt.Errorf("response body exceeds limit"),
		}
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		classification := providerhttp.ClassifyRetry(response.StatusCode, nil)
		if classification.Class == providerhttp.ErrorClassNone {
			classification.Class = providerhttp.ErrorClassResponse
		}
		return nil, response.StatusCode, &SheetsError{
			Class: classification.Class, Retryable: classification.Retryable, HTTPCode: response.StatusCode,
		}
	}
	decoded, err := decodeSheetsMetadataResponse(body)
	if err != nil {
		return nil, response.StatusCode, &SheetsError{
			Class: providerhttp.ErrorClassResponse, HTTPCode: response.StatusCode, cause: err,
		}
	}
	if decoded.Code != 0 {
		return nil, response.StatusCode, &SheetsError{
			Class: providerhttp.ErrorClassRequest, HTTPCode: response.StatusCode, Code: decoded.Code,
		}
	}
	metadata, err := normalizeSheetsMetadata(spreadsheetToken, decoded.Data.Sheets)
	if err != nil {
		return nil, response.StatusCode, &SheetsError{
			Class: providerhttp.ErrorClassResponse, HTTPCode: response.StatusCode, cause: err,
		}
	}
	return metadata, response.StatusCode, nil
}

type sheetsMetadataResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Sheets []sheetMetadataResponseRow `json:"sheets"`
	} `json:"data"`
}

type sheetMetadataResponseRow struct {
	SheetID        string `json:"sheet_id"`
	Title          string `json:"title"`
	Index          int    `json:"index"`
	Hidden         bool   `json:"hidden"`
	ResourceType   string `json:"resource_type"`
	GridProperties struct {
		RowCount          int64 `json:"row_count"`
		ColumnCount       int64 `json:"column_count"`
		FrozenRowCount    int64 `json:"frozen_row_count"`
		FrozenColumnCount int64 `json:"frozen_column_count"`
	} `json:"grid_properties"`
}

func decodeSheetsMetadataResponse(body []byte) (sheetsMetadataResponse, error) {
	var decoded sheetsMetadataResponse
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(&decoded); err != nil {
		return decoded, fmt.Errorf("decode metadata response: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return decoded, fmt.Errorf("metadata response contains trailing data")
	}
	return decoded, nil
}

func normalizeSheetsMetadata(
	spreadsheetToken string,
	rows []sheetMetadataResponseRow,
) (*SpreadsheetMetadata, error) {
	if len(rows) == 0 || len(rows) > 1000 {
		return nil, fmt.Errorf("metadata response has invalid sheet count")
	}
	sheets := make([]SheetMetadata, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	seenIndexes := make(map[int]struct{}, len(rows))
	for _, row := range rows {
		title := strings.TrimSpace(row.Title)
		resourceType := strings.TrimSpace(row.ResourceType)
		if !validFeishuSheetID(row.SheetID) || !validSheetTitle(title) || resourceType != "sheet" || row.Index < 0 ||
			row.GridProperties.RowCount < 1 || row.GridProperties.ColumnCount < 1 ||
			row.GridProperties.FrozenRowCount < 0 || row.GridProperties.FrozenColumnCount < 0 ||
			row.GridProperties.FrozenRowCount > row.GridProperties.RowCount ||
			row.GridProperties.FrozenColumnCount > row.GridProperties.ColumnCount {
			return nil, fmt.Errorf("metadata response contains invalid sheet")
		}
		if _, exists := seen[row.SheetID]; exists {
			return nil, fmt.Errorf("metadata response contains duplicate sheet id")
		}
		if _, exists := seenIndexes[row.Index]; exists {
			return nil, fmt.Errorf("metadata response contains duplicate sheet index")
		}
		seen[row.SheetID] = struct{}{}
		seenIndexes[row.Index] = struct{}{}
		sheets = append(sheets, SheetMetadata{
			SheetID: row.SheetID, Title: title, Index: row.Index, Hidden: row.Hidden,
			ResourceType: resourceType,
			GridProperties: SheetGridProperties{
				RowCount: row.GridProperties.RowCount, ColumnCount: row.GridProperties.ColumnCount,
				FrozenRowCount:    row.GridProperties.FrozenRowCount,
				FrozenColumnCount: row.GridProperties.FrozenColumnCount,
			},
		})
	}
	sort.Slice(sheets, func(left, right int) bool { return sheets[left].Index < sheets[right].Index })
	return &SpreadsheetMetadata{SpreadsheetToken: spreadsheetToken, Sheets: sheets}, nil
}

func normalizeRequiredSheetIDs(values []string) (map[string]struct{}, error) {
	if len(values) == 0 || len(values) > 100 {
		return nil, fmt.Errorf("feishu sheets: required sheet ids are invalid")
	}
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validFeishuSheetID(value) {
			return nil, fmt.Errorf("feishu sheets: required sheet id is invalid")
		}
		if _, exists := result[value]; exists {
			return nil, fmt.Errorf("feishu sheets: required sheet id is duplicated")
		}
		result[value] = struct{}{}
	}
	return result, nil
}

func validateRequiredSheets(metadata *SpreadsheetMetadata, required map[string]struct{}) error {
	if metadata == nil || len(metadata.Sheets) == 0 {
		return fmt.Errorf("feishu sheets: metadata is empty")
	}
	found := make(map[string]struct{}, len(metadata.Sheets))
	for _, sheet := range metadata.Sheets {
		found[sheet.SheetID] = struct{}{}
	}
	for sheetID := range required {
		if _, exists := found[sheetID]; !exists {
			return &SheetsError{Class: providerhttp.ErrorClassRequest, cause: fmt.Errorf("required sheet is missing")}
		}
	}
	return nil
}

func validFeishuSpreadsheetToken(value string) bool {
	return feishuSpreadsheetTokenPattern.MatchString(value)
}

func validFeishuSheetID(value string) bool {
	return feishuSheetIDPattern.MatchString(value)
}

func validSheetTitle(value string) bool {
	if value == "" || len(value) > 255 {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}
