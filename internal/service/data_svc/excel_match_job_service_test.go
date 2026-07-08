package data_svc

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/xuri/excelize/v2"
)

type fakeExcelMatchLookup struct {
	keys  []string
	value map[string]string
}

func (f *fakeExcelMatchLookup) Lookup(ctx context.Context, keys []string, valueField string) (map[string]string, error) {
	f.keys = append(f.keys, keys...)
	return f.value, nil
}

func TestNormalizeExcelMatchConfigRejectsUnknownBojunField(t *testing.T) {
	_, err := normalizeExcelMatchConfig(ExcelMatchConfig{
		Filters: []ExcelMatchFilter{
			{Column: "店铺名称", Op: "eq", Value: "杭州恒隆"},
		},
		MatchExcelColumn: "订单号",
		DBTemplate:       "bojun_retail_order",
		DBMatchField:     "docno",
		DBValueField:     "not_allowed",
		OutputColumnName: "匹配结果",
	})
	if err == nil {
		t.Fatal("normalizeExcelMatchConfig returned nil error, want invalid field error")
	}
}

func TestProcessExcelMatchFileFiltersBeforeLookupAndKeepsAllRows(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "source.xlsx")
	outputPath := filepath.Join(dir, "result.xlsx")

	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	rows := [][]interface{}{
		{"店铺名称", "订单号", "金额"},
		{"杭州恒隆", "B001", "10"},
		{"上海前滩", "B002", "20"},
		{"杭州恒隆", "B003", "30"},
	}
	for i, row := range rows {
		cell, err := excelize.CoordinatesToCellName(1, i+1)
		if err != nil {
			t.Fatalf("CoordinatesToCellName failed: %v", err)
		}
		if err := f.SetSheetRow(sheet, cell, &row); err != nil {
			t.Fatalf("SetSheetRow failed: %v", err)
		}
	}
	if err := f.SaveAs(inputPath); err != nil {
		t.Fatalf("SaveAs failed: %v", err)
	}

	lookup := &fakeExcelMatchLookup{
		value: map[string]string{"B001": "100.50"},
	}
	cfg, err := normalizeExcelMatchConfig(ExcelMatchConfig{
		Filters: []ExcelMatchFilter{
			{Column: "店铺名称", Op: "eq", Value: "杭州恒隆"},
		},
		MatchExcelColumn: "订单号",
		DBTemplate:       "bojun_retail_order",
		DBMatchField:     "docno",
		DBValueField:     "tot_amt_actual",
		OutputColumnName: "伯俊金额",
	})
	if err != nil {
		t.Fatalf("normalizeExcelMatchConfig failed: %v", err)
	}

	stats, err := processExcelMatchFile(context.Background(), inputPath, outputPath, cfg, lookup)
	if err != nil {
		t.Fatalf("processExcelMatchFile failed: %v", err)
	}

	if !reflect.DeepEqual(lookup.keys, []string{"B001", "B003"}) {
		t.Fatalf("lookup keys = %#v, want B001 and B003 only", lookup.keys)
	}
	if stats.TotalRows != 3 || stats.FilteredRows != 2 || stats.MatchedRows != 1 || stats.UnmatchedRows != 1 {
		t.Fatalf("stats = %+v, want total=3 filtered=2 matched=1 unmatched=1", stats)
	}

	out, err := excelize.OpenFile(outputPath)
	if err != nil {
		t.Fatalf("OpenFile result failed: %v", err)
	}
	defer func() { _ = out.Close() }()

	got, err := out.GetRows("Result_1")
	if err != nil {
		t.Fatalf("GetRows result failed: %v", err)
	}
	want := [][]string{
		{"店铺名称", "订单号", "金额", "伯俊金额"},
		{"杭州恒隆", "B001", "10", "100.50"},
		{"上海前滩", "B002", "20"},
		{"杭州恒隆", "B003", "30"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("result rows = %#v, want %#v", got, want)
	}
}
