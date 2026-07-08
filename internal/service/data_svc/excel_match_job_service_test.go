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

type fakeExcelImportUpdater struct {
	existing map[string]struct{}
	updated  map[string]string
	keys     []string
	writes   int
}

func (f *fakeExcelImportUpdater) FindKeys(ctx context.Context, matchField string, keys []string) (map[string]struct{}, error) {
	f.keys = append(f.keys, keys...)
	result := map[string]struct{}{}
	for _, key := range keys {
		if _, ok := f.existing[key]; ok {
			result[key] = struct{}{}
		}
	}
	return result, nil
}

func (f *fakeExcelImportUpdater) UpdateByKey(ctx context.Context, matchField, key, writeField, value string) (int64, error) {
	if f.updated == nil {
		f.updated = map[string]string{}
	}
	f.updated[key] = value
	f.writes++
	return 1, nil
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

func TestNormalizeExcelImportConfigRejectsUnknownWriteField(t *testing.T) {
	_, err := normalizeExcelMatchConfig(ExcelMatchConfig{
		Operation:        excelOperationImportUpdate,
		TableName:        "bojun_retail_orders",
		DBMatchField:     "docno",
		MatchExcelColumn: "订单号",
		DBWriteField:     "tot_amt_actual",
		WriteExcelColumn: "匹配单号",
	})
	if err == nil {
		t.Fatal("normalizeExcelMatchConfig returned nil error, want invalid import write field error")
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
		SheetName: "Sheet1",
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

func TestProcessExcelMatchFileUsesConfiguredSheet(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "source.xlsx")
	outputPath := filepath.Join(dir, "result.xlsx")

	f := excelize.NewFile()
	defaultSheet := f.GetSheetName(0)
	if err := f.SetSheetRow(defaultSheet, "A1", &[]interface{}{"错误列"}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.NewSheet("Data"); err != nil {
		t.Fatal(err)
	}
	rows := [][]interface{}{
		{"店铺名称", "订单号", "金额"},
		{"杭州恒隆", "B001", "10"},
	}
	for i, row := range rows {
		cell, _ := excelize.CoordinatesToCellName(1, i+1)
		if err := f.SetSheetRow("Data", cell, &row); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.SaveAs(inputPath); err != nil {
		t.Fatal(err)
	}

	cfg, err := normalizeExcelMatchConfig(ExcelMatchConfig{
		SheetName:        "Data",
		Filters:          []ExcelMatchFilter{{Column: "店铺名称", Op: "eq", Value: "杭州恒隆"}},
		MatchExcelColumn: "订单号",
		DBTemplate:       "bojun_retail_order",
		DBMatchField:     "docno",
		DBValueField:     "tot_amt_actual",
		OutputColumnName: "伯俊金额",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = processExcelMatchFile(context.Background(), inputPath, outputPath, cfg, &fakeExcelMatchLookup{value: map[string]string{"B001": "10"}})
	if err != nil {
		t.Fatalf("processExcelMatchFile returned error: %v", err)
	}
}

func TestProcessExcelImportUpdateFileUpdatesMatchedRowsOnly(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "source.xlsx")

	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	rows := [][]interface{}{
		{"订单号", "匹配单号"},
		{"B001", "M001"},
		{"B002", "M002"},
		{"", "M003"},
	}
	for i, row := range rows {
		cell, _ := excelize.CoordinatesToCellName(1, i+1)
		if err := f.SetSheetRow(sheet, cell, &row); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.SaveAs(inputPath); err != nil {
		t.Fatal(err)
	}

	cfg, err := normalizeExcelMatchConfig(ExcelMatchConfig{
		Operation:        excelOperationImportUpdate,
		SheetName:        "Sheet1",
		TableName:        "bojun_retail_orders",
		DBMatchField:     "docno",
		MatchExcelColumn: "订单号",
		DBWriteField:     "matched_docno",
		WriteExcelColumn: "匹配单号",
		ConfirmWrite:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	updater := &fakeExcelImportUpdater{existing: map[string]struct{}{"B001": {}}}
	stats, err := processExcelImportUpdateFileWithProgress(context.Background(), inputPath, cfg, updater, nil)
	if err != nil {
		t.Fatalf("processExcelImportUpdateFileWithProgress returned error: %v", err)
	}
	if stats.TotalRows != 3 || stats.ProcessedRows != 3 || stats.MatchedRows != 1 || stats.UnmatchedRows != 2 {
		t.Fatalf("stats = %+v, want total=3 processed=3 matched=1 unmatched=2", stats)
	}
	if !reflect.DeepEqual(updater.updated, map[string]string{"B001": "M001"}) {
		t.Fatalf("updated = %#v", updater.updated)
	}
}

func TestProcessExcelImportUpdateFileDryRunDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "source.xlsx")

	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	rows := [][]interface{}{
		{"订单号", "匹配单号"},
		{"B001", "M001"},
	}
	for i, row := range rows {
		cell, _ := excelize.CoordinatesToCellName(1, i+1)
		if err := f.SetSheetRow(sheet, cell, &row); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.SaveAs(inputPath); err != nil {
		t.Fatal(err)
	}

	cfg, err := normalizeExcelMatchConfig(ExcelMatchConfig{
		Operation:        excelOperationImportUpdate,
		SheetName:        "Sheet1",
		TableName:        "bojun_retail_orders",
		DBMatchField:     "docno",
		MatchExcelColumn: "订单号",
		DBWriteField:     "matched_docno",
		WriteExcelColumn: "匹配单号",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.DryRun {
		t.Fatal("DryRun = false, want true when confirmWrite is not set")
	}
	updater := &fakeExcelImportUpdater{existing: map[string]struct{}{"B001": {}}}
	stats, err := processExcelImportUpdateFileWithProgress(context.Background(), inputPath, cfg, updater, nil)
	if err != nil {
		t.Fatalf("processExcelImportUpdateFileWithProgress returned error: %v", err)
	}
	if stats.MatchedRows != 1 || stats.UnmatchedRows != 0 {
		t.Fatalf("stats = %+v, want one matched row", stats)
	}
	if updater.writes != 0 || len(updater.updated) != 0 {
		t.Fatalf("dry run wrote rows: writes=%d updated=%#v", updater.writes, updater.updated)
	}
}

func TestProcessExcelImportUpdateFileClearsMatchedDocNo(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "source.xlsx")

	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	rows := [][]interface{}{
		{"外部订单编号", "订单号"},
		{"B001", "M001"},
		{"B002", "M002"},
	}
	for i, row := range rows {
		cell, _ := excelize.CoordinatesToCellName(1, i+1)
		if err := f.SetSheetRow(sheet, cell, &row); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.SaveAs(inputPath); err != nil {
		t.Fatal(err)
	}

	cfg, err := normalizeExcelMatchConfig(ExcelMatchConfig{
		Operation:        excelOperationClearMatched,
		SheetName:        "Sheet1",
		TableName:        "bojun_retail_orders",
		DBMatchField:     "docno",
		MatchExcelColumn: "外部订单编号",
		DBWriteField:     "matched_docno",
		ConfirmWrite:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	updater := &fakeExcelImportUpdater{existing: map[string]struct{}{"B001": {}, "B002": {}}}
	stats, err := processExcelImportUpdateFileWithProgress(context.Background(), inputPath, cfg, updater, nil)
	if err != nil {
		t.Fatalf("processExcelImportUpdateFileWithProgress returned error: %v", err)
	}
	if stats.MatchedRows != 2 || stats.UnmatchedRows != 0 {
		t.Fatalf("stats = %+v, want two matched rows", stats)
	}
	if !reflect.DeepEqual(updater.updated, map[string]string{"B001": "", "B002": ""}) {
		t.Fatalf("updated = %#v, want matched_docno cleared", updater.updated)
	}
}
