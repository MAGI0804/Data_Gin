package data_svc

import (
	"context"
	"errors"
	"testing"
	"time"

	"gin-biz-web-api/connector/feishu"
	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/internal/requestbody"
)

func TestMallWeatherFeishuUpsertScannerBuildsMappingsBeforeMarker(t *testing.T) {
	store := &fakeMallWeatherFeishuUpsertMappingStore{}
	sheets := &fakeMallWeatherFeishuUpsertScanSheets{}
	scanner := newTestMallWeatherFeishuUpsertScanner(t, sheets, store)

	result, err := scanner.Ensure(t.Context(), testMallWeatherFeishuUpsertScanRequest())
	if err != nil {
		t.Fatalf("Ensure() error=%v", err)
	}
	if !result.Initialized || result.LastDataRow != 3 || result.MappedRows != 2 || store.resetCalls != 1 ||
		store.createCalls != 1 || store.markCalls != 1 || len(store.mappings) != 2 ||
		store.mappings[0].RowNumber != 2 || store.mappings[1].RowNumber != 3 ||
		store.mappings[0].BusinessKey == store.mappings[1].BusinessKey {
		t.Fatalf("result=%+v store=%+v", result, store)
	}
	if store.events[0] != "reset" || store.events[1] != "create" || store.events[2] != "mark" {
		t.Fatalf("events=%v", store.events)
	}
}

func TestMallWeatherFeishuUpsertScannerReusesCompleteSchema(t *testing.T) {
	store := &fakeMallWeatherFeishuUpsertMappingStore{initialized: true}
	scanner := newTestMallWeatherFeishuUpsertScanner(t, &fakeMallWeatherFeishuUpsertScanSheets{}, store)

	result, err := scanner.Ensure(t.Context(), testMallWeatherFeishuUpsertScanRequest())
	if err != nil {
		t.Fatalf("Ensure() error=%v", err)
	}
	if !result.Initialized || result.LastDataRow != 3 || result.MappedRows != 0 ||
		store.resetCalls != 0 || store.createCalls != 0 || store.markCalls != 0 {
		t.Fatalf("result=%+v store=%+v", result, store)
	}
}

func TestMallWeatherFeishuUpsertScannerNeverMarksPartialScan(t *testing.T) {
	tests := []struct {
		name      string
		sheets    *fakeMallWeatherFeishuUpsertScanSheets
		createErr error
	}{
		{
			name:      "mapping persistence fails",
			sheets:    &fakeMallWeatherFeishuUpsertScanSheets{},
			createErr: errors.New("database unavailable"),
		},
		{
			name:   "duplicate remote key",
			sheets: &fakeMallWeatherFeishuUpsertScanSheets{duplicateKey: true},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &fakeMallWeatherFeishuUpsertMappingStore{createErr: test.createErr}
			scanner := newTestMallWeatherFeishuUpsertScanner(t, test.sheets, store)
			if _, err := scanner.Ensure(t.Context(), testMallWeatherFeishuUpsertScanRequest()); err == nil {
				t.Fatal("Ensure() returned nil error")
			}
			if store.resetCalls != 1 || store.markCalls != 0 {
				t.Fatalf("reset=%d mark=%d events=%v", store.resetCalls, store.markCalls, store.events)
			}
		})
	}
}

func TestNormalizeMallWeatherFeishuScannedRowsRejectsHiddenDataAfterBlankKey(t *testing.T) {
	values := &feishu.SheetValues{Rows: [][]feishu.SheetCell{{
		{Type: feishu.SheetCellBlank},
		{Type: feishu.SheetCellString, Text: "hidden"},
	}}}
	if _, _, err := normalizeMallWeatherFeishuScannedRows(values, 2, 2, 2); err == nil {
		t.Fatal("normalizeMallWeatherFeishuScannedRows() accepted hidden data after blank key")
	}
}

func newTestMallWeatherFeishuUpsertScanner(
	t *testing.T,
	sheets mallWeatherFeishuRangeReader,
	store mallWeatherFeishuUpsertMappingStore,
) *mallWeatherFeishuUpsertScanner {
	t.Helper()
	scanner, err := newMallWeatherFeishuUpsertScanner(
		sheets,
		store,
		func() time.Time { return time.Date(2026, 7, 23, 13, 0, 0, 0, time.UTC) },
	)
	if err != nil {
		t.Fatalf("newMallWeatherFeishuUpsertScanner() error=%v", err)
	}
	return scanner
}

func testMallWeatherFeishuUpsertScanRequest() mallWeatherFeishuUpsertScanRequest {
	return mallWeatherFeishuUpsertScanRequest{
		Destination: testMallWeatherFeishuUpsertDestination(),
		Dataset: requestbody.MallWeatherExportDataset{
			Kind: "hourly", SheetName: "Hourly", Columns: testMallWeatherFeishuUpsertColumns(),
		},
		GridRows: 4,
	}
}

type fakeMallWeatherFeishuUpsertMappingStore struct {
	initialized bool
	createErr   error
	resetCalls  int
	createCalls int
	markCalls   int
	mappings    []data_dao.MallWeatherSheetRowMapping
	events      []string
}

func (store *fakeMallWeatherFeishuUpsertMappingStore) IsInitialized(
	context.Context,
	uint,
	string,
	string,
	string,
) (bool, error) {
	return store.initialized, nil
}

func (store *fakeMallWeatherFeishuUpsertMappingStore) ResetMappings(context.Context, uint, string) error {
	store.resetCalls++
	store.events = append(store.events, "reset")
	return nil
}

func (store *fakeMallWeatherFeishuUpsertMappingStore) CreateScannedMappings(
	_ context.Context,
	_ uint,
	_ string,
	_ string,
	mappings []data_dao.MallWeatherSheetRowMapping,
	_ time.Time,
) error {
	store.createCalls++
	store.events = append(store.events, "create")
	store.mappings = append(store.mappings, mappings...)
	return store.createErr
}

func (store *fakeMallWeatherFeishuUpsertMappingStore) MarkInitialized(
	context.Context,
	uint,
	string,
	string,
	string,
	time.Time,
) error {
	store.markCalls++
	store.events = append(store.events, "mark")
	return nil
}

type fakeMallWeatherFeishuUpsertScanSheets struct {
	duplicateKey bool
}

func (sheets *fakeMallWeatherFeishuUpsertScanSheets) ReadRange(
	_ context.Context,
	_ string,
	readRange feishu.SheetRange,
) (*feishu.SheetValues, error) {
	secondMall := "M002"
	if sheets.duplicateKey {
		secondMall = "M001"
	}
	if readRange.StartColumn == 1 && readRange.EndColumn == 1 {
		return &feishu.SheetValues{Rows: [][]feishu.SheetCell{
			{{Type: feishu.SheetCellString, Text: "M001"}},
			{{Type: feishu.SheetCellString, Text: secondMall}},
			{{Type: feishu.SheetCellBlank}},
		}}, nil
	}
	return &feishu.SheetValues{Rows: [][]feishu.SheetCell{
		testMallWeatherFeishuUpsertCells("M001", "2026-07-23T10:00:00Z", "sunny"),
		testMallWeatherFeishuUpsertCells(secondMall, "2026-07-23T10:00:00Z", "cloudy"),
	}}, nil
}
