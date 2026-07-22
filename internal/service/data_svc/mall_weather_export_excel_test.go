package data_svc

import (
	"math"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
)

func TestMallWeatherExportExcelValuesArePlainTextAndConverted(t *testing.T) {
	file := excelize.NewFile()
	t.Cleanup(func() { _ = file.Close() })
	styles, err := newMallWeatherExportExcelStyles(file)
	if err != nil {
		t.Fatalf("newMallWeatherExportExcelStyles() error=%v", err)
	}
	location, _ := time.LoadLocation("Asia/Shanghai")
	formula, err := mallWeatherExportExcelValue(
		"description", "text", "metric", location, "2006-01-02", "2006-01-02 15:04:05",
		"=HYPERLINK(\"https://example.invalid\")", styles,
	)
	if err != nil {
		t.Fatalf("mallWeatherExportExcelValue(formula) error=%v", err)
	}
	converted, err := mallWeatherExportExcelValue(
		"temperature_c", "decimal", "imperial", location, "2006-01-02", "2006-01-02 15:04:05", "20", styles,
	)
	if err != nil {
		t.Fatalf("mallWeatherExportExcelValue(temperature) error=%v", err)
	}
	if err := setMallWeatherExportRegularRow(file, "Sheet1", 1, []mallWeatherExportExcelCell{formula, converted}); err != nil {
		t.Fatalf("setMallWeatherExportRegularRow() error=%v", err)
	}
	formulaValue, err := file.GetCellValue("Sheet1", "A1")
	if err != nil {
		t.Fatalf("GetCellValue() error=%v", err)
	}
	formulaExpression, err := file.GetCellFormula("Sheet1", "A1")
	if err != nil {
		t.Fatalf("GetCellFormula() error=%v", err)
	}
	temperature, err := file.GetCellValue("Sheet1", "B1", excelize.Options{RawCellValue: true})
	if err != nil {
		t.Fatalf("GetCellValue(temperature) error=%v", err)
	}
	if formulaExpression != "" || !strings.HasPrefix(formulaValue, "=HYPERLINK") || temperature != "68" {
		t.Fatalf("formula=%q expression=%q temperature=%q", formulaValue, formulaExpression, temperature)
	}
}

func TestMallWeatherExportExcelPercentAndDateFormatting(t *testing.T) {
	file := excelize.NewFile()
	t.Cleanup(func() { _ = file.Close() })
	styles, err := newMallWeatherExportExcelStyles(file)
	if err != nil {
		t.Fatalf("newMallWeatherExportExcelStyles() error=%v", err)
	}
	location, _ := time.LoadLocation("Asia/Shanghai")
	percent, err := mallWeatherExportExcelValue(
		"humidity_pct", "percent", "metric", location, "2006/01/02", "2006/01/02 15:04",
		float64(75), styles,
	)
	if err != nil {
		t.Fatalf("percent error=%v", err)
	}
	date, err := mallWeatherExportExcelValue(
		"forecast_time", "datetime", "metric", location, "2006/01/02", "2006/01/02 15:04",
		time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC), styles,
	)
	if err != nil {
		t.Fatalf("date error=%v", err)
	}
	if err := setMallWeatherExportRegularRow(file, "Sheet1", 1, []mallWeatherExportExcelCell{percent, date}); err != nil {
		t.Fatalf("setMallWeatherExportRegularRow() error=%v", err)
	}
	percentRaw, _ := file.GetCellValue("Sheet1", "A1", excelize.Options{RawCellValue: true})
	dateValue, _ := file.GetCellValue("Sheet1", "B1")
	if percentRaw != "0.75" || dateValue != "2026/07/22 08:00" {
		t.Fatalf("percent=%q date=%q", percentRaw, dateValue)
	}
	if _, _, err := mallWeatherExportNumericValue("temperature_c", "decimal", "metric", math.NaN()); err == nil {
		t.Fatal("mallWeatherExportNumericValue() accepted NaN")
	}
}

func TestMallWeatherExportStreamRowPreservesPlainText(t *testing.T) {
	file := excelize.NewFile()
	t.Cleanup(func() { _ = file.Close() })
	styles, err := newMallWeatherExportExcelStyles(file)
	if err != nil {
		t.Fatalf("newMallWeatherExportExcelStyles() error=%v", err)
	}
	location := time.UTC
	formula, err := mallWeatherExportExcelValue(
		"description", "general", "metric", location, "2006-01-02", "2006-01-02 15:04:05",
		"+cmd|' /C calc'!A0", styles,
	)
	if err != nil {
		t.Fatalf("mallWeatherExportExcelValue() error=%v", err)
	}
	number, err := mallWeatherExportExcelValue(
		"temperature_c", "general", "metric", location, "2006-01-02", "2006-01-02 15:04:05",
		"20.5", styles,
	)
	if err != nil {
		t.Fatalf("mallWeatherExportExcelValue(number) error=%v", err)
	}
	writer, err := file.NewStreamWriter("Sheet1")
	if err != nil {
		t.Fatalf("NewStreamWriter() error=%v", err)
	}
	if err := writer.SetRow("A1", mallWeatherExportStreamRow([]mallWeatherExportExcelCell{formula, number})); err != nil {
		t.Fatalf("SetRow() error=%v", err)
	}
	if err := writer.Flush(); err != nil {
		t.Fatalf("Flush() error=%v", err)
	}
	expression, _ := file.GetCellFormula("Sheet1", "A1")
	value, _ := file.GetCellValue("Sheet1", "A1")
	numericValue, _ := file.GetCellValue("Sheet1", "B1", excelize.Options{RawCellValue: true})
	if expression != "" || value != "+cmd|' /C calc'!A0" || numericValue != "20.5" {
		t.Fatalf("expression=%q value=%q numeric=%q", expression, value, numericValue)
	}
}

func TestMallWeatherExportSheetNamerIsSafeDeterministicAndBounded(t *testing.T) {
	namer, err := newMallWeatherExportSheetNamer(3)
	if err != nil {
		t.Fatalf("newMallWeatherExportSheetNamer() error=%v", err)
	}
	first, err := namer.Name("未来360小时", "上海/浦东:[测试]", 1)
	if err != nil {
		t.Fatalf("Name() error=%v", err)
	}
	repeated, err := namer.Name("未来360小时", "上海/浦东:[测试]", 1)
	if err != nil || repeated != first {
		t.Fatalf("repeated=%q first=%q error=%v", repeated, first, err)
	}
	collision, err := namer.Name("未来360小时", "上海_浦东__测试", 1)
	if err != nil {
		t.Fatalf("collision Name() error=%v", err)
	}
	if first == collision || utf8.RuneCountInString(first) > 31 || utf8.RuneCountInString(collision) > 31 ||
		strings.ContainsAny(first+collision, `[]:*?/\\`) {
		t.Fatalf("first=%q collision=%q", first, collision)
	}
	if _, err := namer.Name("第三个", "", 1); err != nil {
		t.Fatalf("third Name() error=%v", err)
	}
	if _, err := namer.Name("第四个", "", 1); err == nil {
		t.Fatal("Name() exceeded sheet limit")
	}
}

func TestRenderMallWeatherExportFileNameUsesProfileTimeZone(t *testing.T) {
	location, _ := time.LoadLocation("Asia/Shanghai")
	name, err := renderMallWeatherExportFileName(
		"商场天气_{{date:20060102_150405}}.xlsx",
		time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC),
		location,
	)
	if err != nil || name != "商场天气_20260722_080000.xlsx" {
		t.Fatalf("name=%q error=%v", name, err)
	}
	for _, invalid := range []string{"../secret.xlsx", "weather_{{unknown}}.xlsx", "weather_{{date:2006/01/02}}.xlsx"} {
		if _, err := renderMallWeatherExportFileName(invalid, time.Now(), location); err == nil {
			t.Fatalf("renderMallWeatherExportFileName(%q) accepted invalid template", invalid)
		}
	}
}
