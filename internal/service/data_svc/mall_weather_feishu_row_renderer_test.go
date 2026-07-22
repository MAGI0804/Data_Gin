package data_svc

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"
	"time"

	"gin-biz-web-api/connector/feishu"
	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/internal/requestbody"
)

func TestRenderMallWeatherFeishuBatchReusesExportFormatting(t *testing.T) {
	t.Parallel()
	profile := testMallWeatherFeishuRenderProfile()
	dataset := requestbody.MallWeatherExportDataset{Kind: "hourly", Columns: []requestbody.MallWeatherExportColumn{
		{Field: "forecast_time", Format: "datetime"},
		{Field: "temperature_c", Format: "decimal"},
		{Field: "humidity_pct", Format: "percent"},
		{Field: "quality_status", Format: "text"},
	}}
	rows := []data_dao.MallWeatherExportDataRow{{
		CursorID: 7,
		Values: map[string]interface{}{
			"forecast_time":  time.Date(2026, 7, 23, 1, 2, 3, 0, time.UTC),
			"temperature_c":  0.0,
			"humidity_pct":   50.0,
			"quality_status": "valid",
		},
	}}
	result, err := renderMallWeatherFeishuBatch(profile, dataset, rows)
	if err != nil {
		t.Fatalf("renderMallWeatherFeishuBatch() error=%v", err)
	}
	if len(result.Rows) != 1 || len(result.Rows[0]) != 4 || result.FirstCursor != 7 || result.LastCursor != 7 ||
		len(result.Checksum) != 64 || result.Rows[0][0].Type != feishu.SheetCellString ||
		result.Rows[0][0].Text != "2026-07-23 09:02:03" || result.Rows[0][1].Number.String() != "32" ||
		result.Rows[0][2].Number.String() != "0.5" || result.Rows[0][3].Text != "valid" {
		t.Fatalf("result=%+v rows=%+v", result, result.Rows)
	}
	encoded, err := json.Marshal(result)
	if err != nil || string(encoded) != "{}" || fmt.Sprintf("%+v", result) != "data_svc.mallWeatherFeishuRenderedBatch{redacted}" {
		t.Fatalf("diagnostics=%s formatted=%+v error=%v", encoded, result, err)
	}
}

func TestRenderMallWeatherFeishuBatchChecksumIsDeterministic(t *testing.T) {
	t.Parallel()
	profile := testMallWeatherFeishuRenderProfile()
	dataset := requestbody.MallWeatherExportDataset{Kind: "hourly", Columns: []requestbody.MallWeatherExportColumn{{
		Field: "quality_status", Format: "text",
	}}}
	rows := []data_dao.MallWeatherExportDataRow{{CursorID: 1, Values: map[string]interface{}{"quality_status": "valid"}}}
	first, err := renderMallWeatherFeishuBatch(profile, dataset, rows)
	if err != nil {
		t.Fatalf("renderMallWeatherFeishuBatch() error=%v", err)
	}
	second, err := renderMallWeatherFeishuBatch(profile, dataset, rows)
	if err != nil || first.Checksum != second.Checksum {
		t.Fatalf("checksums first=%q second=%q error=%v", first.Checksum, second.Checksum, err)
	}
	rows[0].Values["quality_status"] = "stale"
	changed, err := renderMallWeatherFeishuBatch(profile, dataset, rows)
	if err != nil || changed.Checksum == first.Checksum {
		t.Fatalf("changed checksum=%q original=%q error=%v", changed.Checksum, first.Checksum, err)
	}
}

func TestRenderMallWeatherFeishuBatchRejectsInvalidRows(t *testing.T) {
	t.Parallel()
	profile := testMallWeatherFeishuRenderProfile()
	dataset := requestbody.MallWeatherExportDataset{Kind: "hourly", Columns: []requestbody.MallWeatherExportColumn{{
		Field: "temperature_c", Format: "decimal",
	}}}
	for _, rows := range [][]data_dao.MallWeatherExportDataRow{
		nil,
		{{CursorID: 0, Values: map[string]interface{}{"temperature_c": 1.0}}},
		{{CursorID: 1, Values: map[string]interface{}{}}},
		{{CursorID: 1, Values: map[string]interface{}{"temperature_c": math.NaN()}}},
		{
			{CursorID: 2, Values: map[string]interface{}{"temperature_c": 1.0}},
			{CursorID: 1, Values: map[string]interface{}{"temperature_c": 2.0}},
		},
	} {
		if _, err := renderMallWeatherFeishuBatch(profile, dataset, rows); err == nil {
			t.Fatalf("renderMallWeatherFeishuBatch() accepted rows=%+v", rows)
		}
	}
}

func testMallWeatherFeishuRenderProfile() MallWeatherExportProfileDTO {
	return MallWeatherExportProfileDTO{
		ID: 9, Version: 3, Enabled: true, TimeZone: "Asia/Shanghai", UnitSystem: "imperial",
		DateFormat: "2006-01-02", DateTimeFormat: "2006-01-02 15:04:05",
	}
}
