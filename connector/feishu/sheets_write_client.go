package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"gin-biz-web-api/pkg/providerhttp"
)

const maxSheetsWriteRequestBodyBytes = 2 * 1024 * 1024

const (
	maxSheetWriteRanges  = 100
	maxSheetWriteRows    = int64(5_000)
	maxSheetWriteColumns = int64(128)
	maxSheetWriteCells   = int64(100_000)
)

// SheetWriteRange is a fixed rectangular write. Its diagnostic and JSON
// surfaces are redacted because Rows can contain customer data.
type SheetWriteRange struct {
	Range SheetRange    `json:"-"`
	Rows  [][]SheetCell `json:"-"`
}

func (SheetWriteRange) String() string   { return "feishu.SheetWriteRange{redacted}" }
func (SheetWriteRange) GoString() string { return "feishu.SheetWriteRange{redacted}" }

// SheetWriteResult contains only non-sensitive acknowledgement metadata.
type SheetWriteResult struct {
	Revision int64
}

type sheetWriteSpec struct {
	a1Range string
	values  [][]any
	cells   int64
}

type sheetValueRangeRequest struct {
	Range  string  `json:"range"`
	Values [][]any `json:"values"`
}

type sheetAppendRequest struct {
	ValueRange sheetValueRangeRequest `json:"valueRange"`
}

type sheetBatchUpdateRequest struct {
	ValueRanges []sheetValueRangeRequest `json:"valueRanges"`
}

type sheetWriteResponse struct {
	Code int `json:"code"`
	Data *struct {
		Revision int64 `json:"revision"`
		Updates  *struct {
			Revision int64 `json:"revision"`
		} `json:"updates"`
		Responses []struct {
			Revision int64 `json:"revision"`
		} `json:"responses"`
	} `json:"data"`
}

// AppendValues appends one validated rectangular batch. Apart from a single
// credential refresh after HTTP 401, it never retries because append is not
// idempotent.
func (client *SheetsClient) AppendValues(
	ctx context.Context,
	spreadsheetToken string,
	writeRange SheetWriteRange,
) (*SheetWriteResult, error) {
	if !validSheetsWriteClient(client, ctx, spreadsheetToken) {
		return nil, errors.New("feishu sheets: invalid append request")
	}
	spec, err := normalizeSheetWriteRange(writeRange)
	if err != nil {
		return nil, err
	}
	body, err := marshalSheetWriteBody(sheetAppendRequest{ValueRange: sheetValueRangeRequest{
		Range: spec.a1Range, Values: spec.values,
	}})
	if err != nil {
		return nil, err
	}
	return client.postSheetWrite(ctx, spreadsheetToken, "values_append", body)
}

// BatchUpdateValues overwrites validated fixed ranges in one request. Callers
// retain the ranges and checksums needed to recover idempotently at task level.
func (client *SheetsClient) BatchUpdateValues(
	ctx context.Context,
	spreadsheetToken string,
	writeRanges []SheetWriteRange,
) (*SheetWriteResult, error) {
	if !validSheetsWriteClient(client, ctx, spreadsheetToken) ||
		len(writeRanges) == 0 || len(writeRanges) > maxSheetWriteRanges {
		return nil, errors.New("feishu sheets: invalid batch update request")
	}
	requestRanges := make([]sheetValueRangeRequest, len(writeRanges))
	var totalCells int64
	for index, writeRange := range writeRanges {
		spec, err := normalizeSheetWriteRange(writeRange)
		if err != nil || totalCells > maxSheetWriteCells-spec.cells {
			return nil, errors.New("feishu sheets: invalid batch update request")
		}
		totalCells += spec.cells
		requestRanges[index] = sheetValueRangeRequest{Range: spec.a1Range, Values: spec.values}
	}
	body, err := marshalSheetWriteBody(sheetBatchUpdateRequest{ValueRanges: requestRanges})
	if err != nil {
		return nil, err
	}
	return client.postSheetWrite(ctx, spreadsheetToken, "values_batch_update", body)
}

func validSheetsWriteClient(client *SheetsClient, ctx context.Context, spreadsheetToken string) bool {
	return client != nil && client.baseURL != nil && client.tokens != nil && client.http != nil && ctx != nil &&
		validFeishuSpreadsheetToken(spreadsheetToken)
}

func normalizeSheetWriteRange(writeRange SheetWriteRange) (sheetWriteSpec, error) {
	a1Range, err := buildSheetWriteA1Range(writeRange.Range)
	if err != nil {
		return sheetWriteSpec{}, err
	}
	rows := writeRange.Range.EndRow - writeRange.Range.StartRow + 1
	columns := writeRange.Range.EndColumn - writeRange.Range.StartColumn + 1
	if int64(len(writeRange.Rows)) != rows {
		return sheetWriteSpec{}, errors.New("feishu sheets: invalid write values")
	}
	values := make([][]any, len(writeRange.Rows))
	for rowIndex, row := range writeRange.Rows {
		if int64(len(row)) != columns {
			return sheetWriteSpec{}, errors.New("feishu sheets: invalid write values")
		}
		values[rowIndex] = make([]any, len(row))
		for columnIndex, cell := range row {
			value, valid := sheetCellWriteValue(cell)
			if !valid {
				return sheetWriteSpec{}, errors.New("feishu sheets: invalid write values")
			}
			values[rowIndex][columnIndex] = value
		}
	}
	return sheetWriteSpec{
		a1Range: a1Range,
		values:  values,
		cells:   rows * columns,
	}, nil
}

func buildSheetWriteA1Range(writeRange SheetRange) (string, error) {
	invalidRows := writeRange.StartRow < 1 || writeRange.EndRow < writeRange.StartRow ||
		writeRange.EndRow > maxSheetRowNumber
	invalidColumns := writeRange.StartColumn < 1 || writeRange.EndColumn < writeRange.StartColumn ||
		writeRange.EndColumn > maxSheetWriteColumns
	if !validFeishuSheetID(writeRange.SheetID) || invalidRows || invalidColumns {
		return "", errors.New("feishu sheets: invalid write range")
	}
	rows := writeRange.EndRow - writeRange.StartRow + 1
	columns := writeRange.EndColumn - writeRange.StartColumn + 1
	if rows > maxSheetWriteRows || columns > maxSheetWriteColumns || rows > maxSheetWriteCells/columns {
		return "", errors.New("feishu sheets: invalid write range")
	}
	return writeRange.SheetID + "!" + sheetColumnName(writeRange.StartColumn) + strconv.FormatInt(writeRange.StartRow, 10) +
		":" + sheetColumnName(writeRange.EndColumn) + strconv.FormatInt(writeRange.EndRow, 10), nil
}

func sheetCellWriteValue(cell SheetCell) (any, bool) {
	switch cell.Type {
	case SheetCellBlank:
		return nil, cell.Text == "" && cell.Number == "" && !cell.Boolean
	case SheetCellString:
		return cell.Text, utf8.ValidString(cell.Text) && cell.Number == "" && !cell.Boolean
	case SheetCellNumber:
		if cell.Text != "" || cell.Number == "" || cell.Boolean {
			return nil, false
		}
		value, err := strconv.ParseFloat(cell.Number.String(), 64)
		return cell.Number, err == nil && !math.IsInf(value, 0) && !math.IsNaN(value)
	case SheetCellBoolean:
		return cell.Boolean, cell.Text == "" && cell.Number == ""
	default:
		return nil, false
	}
}

func marshalSheetWriteBody(value any) ([]byte, error) {
	body, err := json.Marshal(value)
	if err != nil || len(body) == 0 || len(body) > maxSheetsWriteRequestBodyBytes {
		return nil, errors.New("feishu sheets: write request body is invalid")
	}
	return body, nil
}

func (client *SheetsClient) postSheetWrite(
	ctx context.Context,
	spreadsheetToken string,
	operation string,
	body []byte,
) (*SheetWriteResult, error) {
	token, err := client.tokens.Token(ctx)
	if err != nil {
		return nil, err
	}
	for attempt := 0; attempt < 2; attempt++ {
		result, statusCode, err := client.postSheetWriteOnce(ctx, spreadsheetToken, operation, token, body)
		if statusCode == http.StatusUnauthorized && attempt == 0 {
			token, err = client.tokens.Refresh(ctx)
			if err != nil {
				return nil, err
			}
			continue
		}
		return result, err
	}
	return nil, &SheetsError{Class: providerhttp.ErrorClassAuth, HTTPCode: http.StatusUnauthorized}
}

func (client *SheetsClient) postSheetWriteOnce(
	ctx context.Context,
	spreadsheetToken string,
	operation string,
	token string,
	body []byte,
) (*SheetWriteResult, int, error) {
	if !validTenantToken(token) {
		return nil, 0, errors.New("feishu sheets: token provider returned invalid token")
	}
	endpoint := *client.baseURL
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/open-apis/sheets/v2/spreadsheets/" +
		url.PathEscape(spreadsheetToken) + "/" + operation
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return nil, 0, errors.New("feishu sheets: create write request failed")
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	response, err := client.http.Do(request)
	if err != nil {
		classification := providerhttp.ClassifyRetry(0, err)
		return nil, 0, &SheetsError{
			Class: classification.Class, Retryable: classification.Retryable, cause: safeSheetsWriteCause(err),
		}
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		classification := providerhttp.ClassifyRetry(response.StatusCode, nil)
		if classification.Class == providerhttp.ErrorClassNone {
			classification.Class = providerhttp.ErrorClassResponse
		}
		return nil, response.StatusCode, &SheetsError{
			Class: classification.Class, Retryable: classification.Retryable, HTTPCode: response.StatusCode,
		}
	}
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxSheetsResponseBodyBytes+1))
	if err != nil {
		return nil, response.StatusCode, &SheetsError{
			Class: providerhttp.ErrorClassResponse, Retryable: true, HTTPCode: response.StatusCode, cause: err,
		}
	}
	if len(responseBody) > maxSheetsResponseBodyBytes {
		return nil, response.StatusCode, &SheetsError{
			Class: providerhttp.ErrorClassResponse, HTTPCode: response.StatusCode,
		}
	}
	decoded, err := decodeSheetWriteResponse(responseBody)
	if err != nil {
		return nil, response.StatusCode, &SheetsError{
			Class: providerhttp.ErrorClassResponse, HTTPCode: response.StatusCode,
		}
	}
	if decoded.Code != 0 {
		return nil, response.StatusCode, &SheetsError{
			Class: providerhttp.ErrorClassRequest, HTTPCode: response.StatusCode, Code: decoded.Code,
		}
	}
	return &SheetWriteResult{Revision: sheetWriteRevision(decoded)}, response.StatusCode, nil
}

func safeSheetsWriteCause(err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return context.Canceled
	case errors.Is(err, context.DeadlineExceeded):
		return context.DeadlineExceeded
	default:
		return errors.New("feishu sheets: write transport failed")
	}
}

func decodeSheetWriteResponse(body []byte) (sheetWriteResponse, error) {
	var decoded sheetWriteResponse
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(&decoded); err != nil {
		return decoded, errors.New("decode write response failed")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return decoded, errors.New("write response contains trailing data")
	}
	if decoded.Code == 0 && decoded.Data == nil {
		return decoded, errors.New("write response data is missing")
	}
	if decoded.Data != nil {
		if decoded.Data.Revision < 0 || (decoded.Data.Updates != nil && decoded.Data.Updates.Revision < 0) {
			return decoded, errors.New("write response revision is invalid")
		}
		for _, response := range decoded.Data.Responses {
			if response.Revision < 0 {
				return decoded, errors.New("write response revision is invalid")
			}
		}
	}
	return decoded, nil
}

func sheetWriteRevision(response sheetWriteResponse) int64 {
	if response.Data == nil {
		return 0
	}
	revision := response.Data.Revision
	if response.Data.Updates != nil && response.Data.Updates.Revision > revision {
		revision = response.Data.Updates.Revision
	}
	for _, item := range response.Data.Responses {
		if item.Revision > revision {
			revision = item.Revision
		}
	}
	return revision
}
