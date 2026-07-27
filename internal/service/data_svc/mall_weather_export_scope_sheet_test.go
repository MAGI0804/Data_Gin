package data_svc

import (
	"path/filepath"
	"testing"
	"time"

	"gin-biz-web-api/internal/requestbody"

	"github.com/xuri/excelize/v2"
)

func TestMallWeatherExportRendererAddsScopeSheetToFixedProfile(t *testing.T) {
	renderer, err := newMallWeatherExportRenderer(&fakeMallWeatherExportDataPager{}, 100, 2)
	if err != nil {
		t.Fatalf("newMallWeatherExportRenderer() error=%v", err)
	}
	generatedAt := time.Date(2026, 7, 27, 12, 35, 56, 0, time.UTC)
	snapshotAt := time.Date(2026, 7, 27, 12, 34, 56, 0, time.UTC)
	output := filepath.Join(t.TempDir(), "fixed-scope.xlsx")
	result, err := renderer.Render(t.Context(), MallWeatherExportRenderRequest{
		ProfileCode: fixedMallWeatherExportProfileCode,
		Config: MallWeatherExportProfileConfig{
			TimeZone: "Asia/Shanghai", UnitSystem: "metric",
			DateFormat: time.DateOnly, DateTimeFormat: "2006-01-02 15:04:05",
			Datasets: []requestbody.MallWeatherExportDataset{{Kind: "hourly", SheetName: "小时"}},
		},
		GeneratedAt: generatedAt, SnapshotAt: snapshotAt, OutputPath: output,
	}, nil)
	if err != nil {
		t.Fatalf("Render() error=%v", err)
	}
	if result.SheetCount != 2 || result.ProcessedRows != 0 {
		t.Fatalf("result=%+v", result)
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
	sheets := workbook.GetSheetList()
	if len(sheets) != 2 || sheets[0] != mallWeatherExportScopeSheetName || sheets[1] != "小时" {
		t.Fatalf("sheets=%v", sheets)
	}
	want := map[string]string{
		"B2":  "2026-07-27 20:35:56",
		"B3":  "2026-07-27 20:34:56",
		"B5":  "商场中心点",
		"B6":  "1 km",
		"B7":  "彩云天气",
		"B13": "1 km 业务半径不代表所有天气字段均为 1 km 精度",
	}
	for cell, wantValue := range want {
		value, err := workbook.GetCellValue(mallWeatherExportScopeSheetName, cell)
		if err != nil || value != wantValue {
			t.Fatalf("cell=%s value=%q want=%q error=%v", cell, value, wantValue, err)
		}
	}
}
