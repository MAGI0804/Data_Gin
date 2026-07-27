package data_svc

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/internal/requestbody"

	"github.com/xuri/excelize/v2"
)

func TestMallWeatherExportRendererWritesSplitWorkbookAndProgress(t *testing.T) {
	minimum, maximum := 10.0, 30.0
	rows := make([]data_dao.MallWeatherExportDataRow, 1001)
	for index := range rows {
		rows[index] = data_dao.MallWeatherExportDataRow{
			CursorID: uint(index + 1), SplitCity: "上海", SplitDate: "2026-07-22T00:00:00Z",
			Values: map[string]interface{}{"mall_code": "M001", "temperature_c": "20.5", "description": "=SUM(A1:A2)"},
		}
	}
	pager := &fakeMallWeatherExportDataPager{rows: rows}
	renderer, err := newMallWeatherExportRenderer(pager, 250, 8)
	if err != nil {
		t.Fatalf("newMallWeatherExportRenderer() error=%v", err)
	}
	output := filepath.Join(t.TempDir(), "weather.xlsx")
	progressCalls := 0
	result, err := renderer.Render(t.Context(), MallWeatherExportRenderRequest{
		ProfileCode: "test_profile",
		Config: MallWeatherExportProfileConfig{
			TimeZone: "Asia/Shanghai", UnitSystem: "metric", DateFormat: "2006-01-02", DateTimeFormat: "2006-01-02 15:04:05",
			Datasets: []requestbody.MallWeatherExportDataset{{
				Kind: "hourly", SheetName: "小时", SplitBy: "city", MaxRows: 600, FreezeHeader: true, AutoFilter: true,
				ConditionalFormats: []requestbody.MallWeatherExportConditionalFormat{{
					Field: "temperature_c", Operator: "between", Value: &minimum, SecondValue: &maximum,
					BackgroundColor: "#ff0000",
				}},
				Columns: []requestbody.MallWeatherExportColumn{
					{Field: "mall_code", Title: "商场", Width: 24, Format: "text"},
					{Field: "temperature_c", Title: "温度", Format: "decimal"},
					{Field: "description", Title: "说明", Format: "text"},
				},
			}},
		},
		SnapshotAt: time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC), EstimatedRows: 10_000, OutputPath: output,
	}, func(progress MallWeatherExportRenderProgress) error {
		progressCalls++
		if progress.ProcessedRows != 1000 || progress.CurrentSheet == "" {
			t.Fatalf("progress=%+v", progress)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Render() error=%v", err)
	}
	if result.ProcessedRows != 1001 || result.SheetCount != 2 || !result.UsedStream || progressCalls != 1 {
		t.Fatalf("result=%+v progressCalls=%d", result, progressCalls)
	}
	workbook, err := excelize.OpenFile(output)
	if err != nil {
		t.Fatalf("OpenFile() error=%v", err)
	}
	t.Cleanup(func() {
		if err := workbook.Close(); err != nil {
			t.Errorf("Close() error=%v", err)
		}
	})
	if len(workbook.GetSheetList()) != 2 {
		t.Fatalf("sheets=%v", workbook.GetSheetList())
	}
	sheets := workbook.GetSheetList()
	formula, err := workbook.GetCellFormula(sheets[0], "C2")
	if err != nil {
		t.Fatalf("GetCellFormula() error=%v", err)
	}
	value, err := workbook.GetCellValue(sheets[0], "C2")
	if err != nil {
		t.Fatalf("GetCellValue() error=%v", err)
	}
	if formula != "" || value != "=SUM(A1:A2)" {
		t.Fatalf("formula=%q value=%q", formula, value)
	}
	secondValue, err := workbook.GetCellValue(sheets[1], "C2")
	if err != nil {
		t.Fatalf("GetCellValue(second sheet) error=%v", err)
	}
	if secondValue != "=SUM(A1:A2)" {
		t.Fatalf("second sheet value=%q", secondValue)
	}
	formats, err := workbook.GetConditionalFormats(sheets[0])
	if err != nil {
		t.Fatalf("GetConditionalFormats() error=%v", err)
	}
	rules := formats["B2:B601"]
	if len(rules) != 1 || rules[0].Criteria != "between" || rules[0].MinValue != "10" || rules[0].MaxValue != "30" {
		t.Fatalf("conditional formats=%+v", formats)
	}
	panes, err := workbook.GetPanes(sheets[0])
	if err != nil || !panes.Freeze || panes.YSplit != 1 || panes.TopLeftCell != "A2" {
		t.Fatalf("panes=%+v error=%v", panes, err)
	}
	width, err := workbook.GetColWidth(sheets[0], "A")
	if err != nil || width != 24 {
		t.Fatalf("column width=%v error=%v", width, err)
	}
}

func TestMallWeatherExportRendererUsesRegularWriterForSmallJobs(t *testing.T) {
	pager := &fakeMallWeatherExportDataPager{rows: []data_dao.MallWeatherExportDataRow{{
		CursorID: 1, Values: map[string]interface{}{"mall_code": "M001"},
	}}}
	renderer, err := newMallWeatherExportRenderer(pager, 100, 4)
	if err != nil {
		t.Fatalf("newMallWeatherExportRenderer() error=%v", err)
	}
	output := filepath.Join(t.TempDir(), "small.xlsx")
	result, err := renderer.Render(context.Background(), MallWeatherExportRenderRequest{
		ProfileCode: "test_profile",
		Config: MallWeatherExportProfileConfig{
			TimeZone: "UTC", UnitSystem: "metric", DateFormat: time.DateOnly, DateTimeFormat: time.RFC3339,
			Datasets: []requestbody.MallWeatherExportDataset{{Kind: "malls", SheetName: "商场", MaxRows: 100,
				Columns: []requestbody.MallWeatherExportColumn{{Field: "mall_code", Title: "编码", Format: "text"}}}},
		},
		SnapshotAt: time.Now().UTC(), EstimatedRows: 1, OutputPath: output,
	}, nil)
	if err != nil || result.UsedStream || result.ProcessedRows != 1 {
		t.Fatalf("result=%+v error=%v", result, err)
	}
}

func TestMallWeatherExportRendererUsesFixedCurrentForecastWindows(t *testing.T) {
	pager := &fakeMallWeatherExportDataPager{}
	renderer, err := newMallWeatherExportRenderer(pager, 100, 8)
	if err != nil {
		t.Fatalf("newMallWeatherExportRenderer() error=%v", err)
	}
	latest := true
	snapshot := time.Date(2026, 7, 27, 12, 34, 56, 0, time.UTC)
	output := filepath.Join(t.TempDir(), "current-weather.xlsx")
	_, err = renderer.Render(t.Context(), MallWeatherExportRenderRequest{
		ProfileCode: fixedMallWeatherExportProfileCode,
		Config: MallWeatherExportProfileConfig{
			TimeZone: "Asia/Shanghai", UnitSystem: "metric",
			DateFormat: time.DateOnly, DateTimeFormat: time.RFC3339,
			Datasets: []requestbody.MallWeatherExportDataset{
				{Kind: "minutely", SheetName: "分钟", Latest: &latest},
				{Kind: "hourly", SheetName: "小时", Latest: &latest},
				{Kind: "daily", SheetName: "逐日", Latest: &latest},
				{Kind: "life_indices", SheetName: "生活指数", Latest: &latest},
			},
		},
		SnapshotAt: snapshot, EstimatedRows: 0, OutputPath: output,
	}, nil)
	if err != nil {
		t.Fatalf("Render() error=%v", err)
	}
	if len(pager.requests) != 4 {
		t.Fatalf("requests=%+v", pager.requests)
	}
	byKind := make(map[string]data_dao.MallWeatherExportDataPageRequest, len(pager.requests))
	for _, request := range pager.requests {
		byKind[request.Kind] = request
	}
	minutely := byKind["minutely"].Filter
	if minutely.StartUTC == nil || minutely.EndUTC == nil ||
		!minutely.StartUTC.Equal(time.Date(2026, 7, 27, 12, 34, 0, 0, time.UTC)) ||
		!minutely.EndUTC.Equal(time.Date(2026, 7, 27, 14, 34, 0, 0, time.UTC)) {
		t.Fatalf("minutely filter=%+v", minutely)
	}
	hourly := byKind["hourly"].Filter
	if hourly.StartUTC == nil || hourly.EndUTC == nil ||
		!hourly.StartUTC.Equal(time.Date(2026, 7, 27, 13, 0, 0, 0, time.UTC)) ||
		!hourly.EndUTC.Equal(time.Date(2026, 8, 11, 13, 0, 0, 0, time.UTC)) {
		t.Fatalf("hourly filter=%+v", hourly)
	}
	for _, kind := range []string{"daily", "life_indices"} {
		filter := byKind[kind].Filter
		if filter.StartDate != "2026-07-27" || filter.EndDate != "2026-08-11" {
			t.Fatalf("kind=%s filter=%+v", kind, filter)
		}
	}
}

func TestMallWeatherExportRendererPreservesExplicitRange(t *testing.T) {
	pager := &fakeMallWeatherExportDataPager{}
	renderer, err := newMallWeatherExportRenderer(pager, 100, 8)
	if err != nil {
		t.Fatalf("newMallWeatherExportRenderer() error=%v", err)
	}
	latest := true
	wantStart := time.Date(2026, 7, 1, 1, 2, 3, 0, time.UTC)
	wantEnd := time.Date(2026, 7, 3, 4, 5, 6, 0, time.UTC)
	output := filepath.Join(t.TempDir(), "explicit-range.xlsx")
	_, err = renderer.Render(t.Context(), MallWeatherExportRenderRequest{
		ProfileCode: fixedMallWeatherExportProfileCode,
		Config: MallWeatherExportProfileConfig{
			TimeZone: "Asia/Shanghai", UnitSystem: "metric",
			DateFormat: time.DateOnly, DateTimeFormat: time.RFC3339,
			Datasets: []requestbody.MallWeatherExportDataset{
				{Kind: "hourly", SheetName: "小时", Latest: &latest},
				{Kind: "daily", SheetName: "逐日", Latest: &latest},
			},
		},
		Filter: data_dao.MallWeatherExportEstimateFilter{
			StartUTC: &wantStart, EndUTC: &wantEnd,
			StartDate: "2026-07-01", EndDate: "2026-07-03",
		},
		SnapshotAt: time.Date(2026, 7, 27, 12, 34, 56, 0, time.UTC),
		OutputPath: output,
	}, nil)
	if err != nil {
		t.Fatalf("Render() error=%v", err)
	}
	if len(pager.requests) != 2 {
		t.Fatalf("requests=%+v", pager.requests)
	}
	for _, request := range pager.requests {
		filter := request.Filter
		if filter.StartUTC == nil || filter.EndUTC == nil ||
			!filter.StartUTC.Equal(wantStart) || !filter.EndUTC.Equal(wantEnd) ||
			filter.StartDate != "2026-07-01" || filter.EndDate != "2026-07-03" {
			t.Fatalf("kind=%s filter=%+v", request.Kind, filter)
		}
	}
}

func TestMallWeatherExportRendererRemovesPartialOutputWhenProgressStops(t *testing.T) {
	rows := make([]data_dao.MallWeatherExportDataRow, mallWeatherExportProgressRows)
	for index := range rows {
		rows[index] = data_dao.MallWeatherExportDataRow{
			CursorID: uint(index + 1),
			Values:   map[string]interface{}{"mall_code": "M001"},
		}
	}
	renderer, err := newMallWeatherExportRenderer(&fakeMallWeatherExportDataPager{rows: rows}, 250, 4)
	if err != nil {
		t.Fatalf("newMallWeatherExportRenderer() error=%v", err)
	}
	output := filepath.Join(t.TempDir(), "cancelled.xlsx")
	stopErr := errors.New("stop rendering")
	_, err = renderer.Render(t.Context(), MallWeatherExportRenderRequest{
		ProfileCode: "test_profile",
		Config: MallWeatherExportProfileConfig{
			TimeZone: "UTC", UnitSystem: "metric", DateFormat: time.DateOnly, DateTimeFormat: time.RFC3339,
			Datasets: []requestbody.MallWeatherExportDataset{{
				Kind: "malls", SheetName: "商场", MaxRows: 2000,
				Columns: []requestbody.MallWeatherExportColumn{{Field: "mall_code", Title: "编码", Format: "text"}},
			}},
		},
		SnapshotAt: time.Now().UTC(), EstimatedRows: mallWeatherExportStreamThreshold, OutputPath: output,
	}, func(MallWeatherExportRenderProgress) error {
		return stopErr
	})
	if !errors.Is(err, stopErr) {
		t.Fatalf("Render() error=%v, want %v", err, stopErr)
	}
	if _, err := os.Stat(output); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial output still exists: %v", err)
	}
}

func TestMallWeatherExportRendererDoesNotOverwriteExistingOutput(t *testing.T) {
	output := filepath.Join(t.TempDir(), "existing.xlsx")
	original := []byte("existing content")
	if err := os.WriteFile(output, original, 0o600); err != nil {
		t.Fatalf("WriteFile() error=%v", err)
	}
	renderer, err := newMallWeatherExportRenderer(&fakeMallWeatherExportDataPager{}, 100, 4)
	if err != nil {
		t.Fatalf("newMallWeatherExportRenderer() error=%v", err)
	}
	_, err = renderer.Render(t.Context(), MallWeatherExportRenderRequest{
		ProfileCode: "test_profile",
		Config: MallWeatherExportProfileConfig{
			TimeZone: "UTC", UnitSystem: "metric", DateFormat: time.DateOnly, DateTimeFormat: time.RFC3339,
			Datasets: []requestbody.MallWeatherExportDataset{{Kind: "malls", SheetName: "商场"}},
		},
		SnapshotAt: time.Now().UTC(), EstimatedRows: 0, OutputPath: output,
	}, nil)
	if err == nil {
		t.Fatal("Render() succeeded with an existing output path")
	}
	content, readErr := os.ReadFile(output)
	if readErr != nil || string(content) != string(original) {
		t.Fatalf("existing output changed: content=%q error=%v", content, readErr)
	}
}

func TestValidateMallWeatherExportPage(t *testing.T) {
	tests := []struct {
		name      string
		page      *data_dao.MallWeatherExportDataPage
		afterID   uint
		wantNext  uint
		wantError bool
	}{
		{name: "nil page", wantError: true},
		{name: "empty final page", page: &data_dao.MallWeatherExportDataPage{}, afterID: 5, wantNext: 5},
		{name: "empty continuation", page: &data_dao.MallWeatherExportDataPage{HasMore: true}, wantError: true},
		{
			name: "non-increasing rows",
			page: &data_dao.MallWeatherExportDataPage{
				Rows: []data_dao.MallWeatherExportDataRow{{CursorID: 6}, {CursorID: 6}}, NextAfterID: 6,
			},
			afterID: 5, wantError: true,
		},
		{
			name: "mismatched next cursor",
			page: &data_dao.MallWeatherExportDataPage{
				Rows: []data_dao.MallWeatherExportDataRow{{CursorID: 6}}, NextAfterID: 7,
			},
			afterID: 5, wantError: true,
		},
		{
			name: "valid continuation",
			page: &data_dao.MallWeatherExportDataPage{
				Rows: []data_dao.MallWeatherExportDataRow{{CursorID: 6}, {CursorID: 9}}, NextAfterID: 9, HasMore: true,
			},
			afterID: 5, wantNext: 9,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next, err := validateMallWeatherExportPage(tt.page, tt.afterID)
			if (err != nil) != tt.wantError || next != tt.wantNext {
				t.Fatalf("validateMallWeatherExportPage() next=%d error=%v", next, err)
			}
		})
	}
}

type fakeMallWeatherExportDataPager struct {
	rows     []data_dao.MallWeatherExportDataRow
	requests []data_dao.MallWeatherExportDataPageRequest
}

func (pager *fakeMallWeatherExportDataPager) Page(
	_ context.Context,
	request data_dao.MallWeatherExportDataPageRequest,
) (*data_dao.MallWeatherExportDataPage, error) {
	pager.requests = append(pager.requests, request)
	start := 0
	for start < len(pager.rows) && pager.rows[start].CursorID <= request.AfterID {
		start++
	}
	end := start + request.Limit
	if end > len(pager.rows) {
		end = len(pager.rows)
	}
	rows := append([]data_dao.MallWeatherExportDataRow(nil), pager.rows[start:end]...)
	page := &data_dao.MallWeatherExportDataPage{Rows: rows, HasMore: end < len(pager.rows)}
	if len(rows) > 0 {
		page.NextAfterID = rows[len(rows)-1].CursorID
	}
	return page, nil
}
