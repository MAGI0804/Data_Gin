package data_svc

import (
	"context"
	"errors"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/storage"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

type fakeExcelMatchLookup struct {
	tables      []string
	matchFields []string
	keys        []string
	value       map[string]string
	stepValues  map[string]map[string]string
}

func (f *fakeExcelMatchLookup) Lookup(ctx context.Context, step ExcelMatchStep, keys []string) (map[string]string, error) {
	f.tables = append(f.tables, step.TableName)
	f.matchFields = append(f.matchFields, step.DBMatchField)
	f.keys = append(f.keys, keys...)
	if value, ok := f.stepValues[step.OutputColumnName]; ok {
		return value, nil
	}
	return f.value, nil
}

type fakeExcelImportUpdater struct {
	existing map[string]struct{}
	updated  map[string]string
	keys     []string
	writes   int
}

type fakeExcelMatchSchemaValidator struct {
	tables map[string]map[string]struct{}
}

func (f fakeExcelMatchSchemaValidator) ValidateTableColumns(_ context.Context, tableName string, columns []string) error {
	table, ok := f.tables[tableName]
	if !ok {
		return errors.New("table does not exist")
	}
	for _, column := range columns {
		if _, ok := table[column]; !ok {
			return errors.New("column does not exist")
		}
	}
	return nil
}

func TestNormalizeExcelMatchConfigConvertsLegacyExportToOneStep(t *testing.T) {
	cfg, err := normalizeExcelMatchConfig(ExcelMatchConfig{
		Filters:          []ExcelMatchFilter{{Column: "店铺", Op: "eq", Value: "幼岚-有赞"}},
		MatchExcelColumn: "原始线上订单号",
		DBTemplate:       "bojun_retail_order",
		DBMatchField:     "matched_docno",
		DBValueField:     "c_store_name",
		OutputColumnName: "线下店名称",
	})
	if err != nil {
		t.Fatalf("normalizeExcelMatchConfig returned error: %v", err)
	}
	if len(cfg.Steps) != 1 {
		t.Fatalf("steps length = %d, want 1", len(cfg.Steps))
	}
	step := cfg.Steps[0]
	if step.TableName != "bojun_retail_orders" || step.MatchExcelColumn != "原始线上订单号" || step.DBMatchField != "matched_docno" || step.DBValueField != "c_store_name" || step.OutputColumnName != "线下店名称" {
		t.Fatalf("legacy step = %#v", step)
	}
	if len(cfg.Filters) != 0 {
		t.Fatalf("top-level filters = %#v, want migrated filters", cfg.Filters)
	}
	wantFilters := []ExcelMatchFilter{{Column: "店铺", Op: "eq", Value: "幼岚-有赞"}}
	if !reflect.DeepEqual(step.Filters, wantFilters) {
		t.Fatalf("step filters = %#v, want %#v", step.Filters, wantFilters)
	}
}

func TestNormalizeExcelMatchConfigAcceptsOrderedCustomSteps(t *testing.T) {
	cfg, err := normalizeExcelMatchConfig(ExcelMatchConfig{
		Operation: "export_match",
		Steps: []ExcelMatchStep{
			{TableName: "bojun_retail_orders", MatchExcelColumn: "原始线上订单号", DBMatchField: "matched_docno", DBValueField: "c_store_name", OutputColumnName: "线下店名称"},
			{TableName: "store_mappings", MatchExcelColumn: "线下店名称", DBMatchField: "source_name", DBValueField: "target_code", OutputColumnName: "目标编码"},
		},
	})
	if err != nil {
		t.Fatalf("normalizeExcelMatchConfig returned error: %v", err)
	}
	if len(cfg.Steps) != 2 || cfg.Steps[0].Name != "步骤 1" || cfg.Steps[1].Name != "步骤 2" {
		t.Fatalf("normalized steps = %#v", cfg.Steps)
	}
}

func TestNormalizeExcelMatchConfigAcceptsOrderItemSKUStep(t *testing.T) {
	cfg, err := normalizeExcelMatchConfig(ExcelMatchConfig{
		Operation: excelOperationExportMatch,
		Steps: []ExcelMatchStep{{
			Name:             "补全SKU",
			MatchMode:        excelMatchModeOrderItemSKU,
			TableName:        bojunRetailOrdersTable,
			MatchExcelColumn: "订单号",
			SpecExcelColumn:  "规格编码",
			PriceExcelColumn: "销售单价",
			QtyExcelColumn:   "销售数量",
			DBMatchField:     "docno",
			DBValueField:     "items_json",
			OutputColumnName: "完整SKU",
		}},
	})
	if err != nil {
		t.Fatalf("normalizeExcelMatchConfig returned error: %v", err)
	}
	if got := cfg.Steps[0]; got.MatchMode != excelMatchModeOrderItemSKU || got.SpecExcelColumn != "规格编码" || got.PriceExcelColumn != "销售单价" || got.QtyExcelColumn != "销售数量" {
		t.Fatalf("normalized order item step = %#v", got)
	}
}

func TestNormalizeExcelMatchConfigRejectsIncompleteOrderItemSKUStep(t *testing.T) {
	_, err := normalizeExcelMatchConfig(ExcelMatchConfig{
		Operation: excelOperationExportMatch,
		Steps: []ExcelMatchStep{{
			MatchMode:        excelMatchModeOrderItemSKU,
			TableName:        bojunRetailOrdersTable,
			MatchExcelColumn: "订单号",
			SpecExcelColumn:  "规格编码",
			DBMatchField:     "docno",
			DBValueField:     "items_json",
			OutputColumnName: "完整SKU",
		}},
	})
	if err == nil {
		t.Fatal("normalizeExcelMatchConfig returned nil error, want missing price and quantity columns error")
	}
}

func TestNormalizeExcelMatchConfigRejectsDuplicateStepOutput(t *testing.T) {
	_, err := normalizeExcelMatchConfig(ExcelMatchConfig{
		Operation: "export_match",
		Steps: []ExcelMatchStep{
			{TableName: "bojun_retail_orders", MatchExcelColumn: "订单号", DBMatchField: "docno", DBValueField: "c_store_name", OutputColumnName: "匹配结果"},
			{TableName: "store_mappings", MatchExcelColumn: "匹配结果", DBMatchField: "source_name", DBValueField: "target_code", OutputColumnName: "匹配结果"},
		},
	})
	if err == nil {
		t.Fatal("normalizeExcelMatchConfig returned nil error, want duplicate output error")
	}
}

func TestExcelRowMatchesFiltersSupportsCustomOperators(t *testing.T) {
	row := []string{"杭州恒隆店", "", "已完成"}
	columns := map[string]int{"门店": 0, "备注": 1, "状态": 2}
	tests := []struct {
		name   string
		filter ExcelMatchFilter
		want   bool
	}{
		{name: "equals", filter: ExcelMatchFilter{Column: "状态", Op: "eq", Value: "已完成"}, want: true},
		{name: "not equals", filter: ExcelMatchFilter{Column: "状态", Op: "neq", Value: "待处理"}, want: true},
		{name: "contains", filter: ExcelMatchFilter{Column: "门店", Op: "contains", Value: "恒隆"}, want: true},
		{name: "not contains", filter: ExcelMatchFilter{Column: "门店", Op: "not_contains", Value: "前滩"}, want: true},
		{name: "starts with", filter: ExcelMatchFilter{Column: "门店", Op: "starts_with", Value: "杭州"}, want: true},
		{name: "ends with", filter: ExcelMatchFilter{Column: "门店", Op: "ends_with", Value: "店"}, want: true},
		{name: "empty", filter: ExcelMatchFilter{Column: "备注", Op: "empty"}, want: true},
		{name: "not empty", filter: ExcelMatchFilter{Column: "状态", Op: "not_empty"}, want: true},
		{name: "failed condition", filter: ExcelMatchFilter{Column: "状态", Op: "eq", Value: "待处理"}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := excelRowMatchesFilters(row, columns, []ExcelMatchFilter{tt.filter}); got != tt.want {
				t.Fatalf("excelRowMatchesFilters() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizeExcelMatchConfigRejectsUnknownStepFilterOperator(t *testing.T) {
	_, err := normalizeExcelMatchConfig(ExcelMatchConfig{
		Operation: "export_match",
		Steps: []ExcelMatchStep{{
			Filters:          []ExcelMatchFilter{{Column: "状态", Op: "regex", Value: ".*"}},
			TableName:        "bojun_retail_orders",
			MatchExcelColumn: "订单号",
			DBMatchField:     "docno",
			DBValueField:     "matched_docno",
			OutputColumnName: "匹配结果",
		}},
	})
	if err == nil {
		t.Fatal("normalizeExcelMatchConfig returned nil error, want unsupported filter operator error")
	}
}

func TestValidateExcelExportStepsRejectsMissingColumn(t *testing.T) {
	config := ExcelMatchConfig{Steps: []ExcelMatchStep{{
		TableName:        "bojun_retail_orders",
		DBMatchField:     "docno",
		DBValueField:     "missing_field",
		OutputColumnName: "匹配结果",
	}}}
	validator := fakeExcelMatchSchemaValidator{tables: map[string]map[string]struct{}{
		"bojun_retail_orders": {"docno": {}},
	}}
	if err := validateExcelExportSteps(context.Background(), config, validator); err == nil {
		t.Fatal("validateExcelExportSteps returned nil error, want missing column error")
	}
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

func TestProcessExcelMatchFileUsesPreviousStepOutputAsNextStepInput(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "source.xlsx")
	outputPath := filepath.Join(dir, "result.xlsx")

	f := excelize.NewFile()
	if err := f.SetSheetRow("Sheet1", "A1", &[]interface{}{"订单号"}); err != nil {
		t.Fatal(err)
	}
	if err := f.SetSheetRow("Sheet1", "A2", &[]interface{}{"B001"}); err != nil {
		t.Fatal(err)
	}
	if err := f.SaveAs(inputPath); err != nil {
		t.Fatal(err)
	}

	cfg, err := normalizeExcelMatchConfig(ExcelMatchConfig{
		Operation: "export_match",
		Steps: []ExcelMatchStep{
			{Name: "匹配门店", TableName: "bojun_retail_orders", MatchExcelColumn: "订单号", DBMatchField: "docno", DBValueField: "c_store_name", OutputColumnName: "门店名称"},
			{Name: "匹配区域", Filters: []ExcelMatchFilter{{Column: "门店名称", Op: "contains", Value: "恒隆"}}, TableName: "store_mappings", MatchExcelColumn: "门店名称", DBMatchField: "store_name", DBValueField: "region_name", OutputColumnName: "区域"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	lookup := &fakeExcelMatchLookup{stepValues: map[string]map[string]string{
		"门店名称": {"B001": "杭州恒隆"},
		"区域":   {"杭州恒隆": "华东"},
	}}
	stats, err := processExcelMatchFile(context.Background(), inputPath, outputPath, cfg, lookup)
	if err != nil {
		t.Fatalf("processExcelMatchFile returned error: %v", err)
	}
	if !reflect.DeepEqual(lookup.tables, []string{"bojun_retail_orders", "store_mappings"}) {
		t.Fatalf("lookup tables = %#v", lookup.tables)
	}
	if !reflect.DeepEqual(lookup.keys, []string{"B001", "杭州恒隆"}) {
		t.Fatalf("lookup keys = %#v, want chained keys", lookup.keys)
	}
	if stats.MatchedRows != 1 || stats.UnmatchedRows != 0 {
		t.Fatalf("stats = %+v, want one final matched row", stats)
	}

	out, err := excelize.OpenFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = out.Close() }()
	rows, err := out.GetRows("Result_1")
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"订单号", "门店名称", "区域"}, {"B001", "杭州恒隆", "华东"}}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("result rows = %#v, want %#v", rows, want)
	}
}

func TestProcessExcelMatchFileMatchesOrderItemsWithoutReusingSKUAcrossBatches(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "source.xlsx")
	outputPath := filepath.Join(dir, "result.xlsx")

	f := excelize.NewFile()
	if err := f.SetSheetRow("Sheet1", "A1", &[]interface{}{"订单号", "规格编码", "销售单价", "销售数量"}); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 501; index++ {
		cell, err := excelize.CoordinatesToCellName(1, index+2)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.SetSheetRow("Sheet1", cell, &[]interface{}{"ORDER-1", "C09H1073", "319.20", "1"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.SaveAs(inputPath); err != nil {
		t.Fatal(err)
	}

	cfg, err := normalizeExcelMatchConfig(ExcelMatchConfig{
		Operation: excelOperationExportMatch,
		BatchSize: 500,
		Steps: []ExcelMatchStep{{
			Name:             "补全SKU",
			MatchMode:        excelMatchModeOrderItemSKU,
			TableName:        bojunRetailOrdersTable,
			MatchExcelColumn: "订单号",
			SpecExcelColumn:  "规格编码",
			PriceExcelColumn: "销售单价",
			QtyExcelColumn:   "销售数量",
			DBMatchField:     "docno",
			DBValueField:     "items_json",
			OutputColumnName: "完整SKU",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	lookup := &fakeExcelMatchLookup{value: map[string]string{
		"ORDER-1": `[{"no":"C09H1073AR752130","qty":1,"priceactual":319.2,"mProductName":"C09H1073A"}]`,
	}}
	stats, err := processExcelMatchFile(context.Background(), inputPath, outputPath, cfg, lookup)
	if err != nil {
		t.Fatalf("processExcelMatchFile returned error: %v", err)
	}
	if !reflect.DeepEqual(lookup.keys, []string{"ORDER-1", "ORDER-1"}) {
		t.Fatalf("lookup keys = %#v, want bounded detail lookup per batch", lookup.keys)
	}
	if stats.MatchedRows != 1 || stats.UnmatchedRows != 500 {
		t.Fatalf("stats = %+v, want one matched row and 500 unmatched rows", stats)
	}

	out, err := excelize.OpenFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = out.Close() }()
	if got, err := out.GetCellValue("Result_1", "E2"); err != nil || got != "C09H1073AR752130" {
		t.Fatalf("first matched SKU = %q, err=%v", got, err)
	}
	if got, err := out.GetCellValue("Result_1", "E502"); err != nil || got != "" {
		t.Fatalf("reused SKU in second batch = %q, err=%v", got, err)
	}
}

func TestProcessExcelMatchFileReservesExactSpecAcrossBatches(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "source.xlsx")
	outputPath := filepath.Join(dir, "result.xlsx")

	f := excelize.NewFile()
	if err := f.SetSheetRow("Sheet1", "A1", &[]interface{}{"订单号", "规格编码", "销售单价", "销售数量"}); err != nil {
		t.Fatal(err)
	}
	if err := f.SetSheetRow("Sheet1", "A2", &[]interface{}{"ORDER-1", "C09H1073", "319.20", "1"}); err != nil {
		t.Fatal(err)
	}
	for index := 1; index < 500; index++ {
		cell, err := excelize.CoordinatesToCellName(1, index+2)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.SetSheetRow("Sheet1", cell, &[]interface{}{"", "", "", ""}); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.SetSheetRow("Sheet1", "A502", &[]interface{}{"ORDER-1", "C09H1073A", "319.20", "1"}); err != nil {
		t.Fatal(err)
	}
	if err := f.SaveAs(inputPath); err != nil {
		t.Fatal(err)
	}

	cfg, err := normalizeExcelMatchConfig(ExcelMatchConfig{
		Operation: excelOperationExportMatch,
		BatchSize: 500,
		Steps: []ExcelMatchStep{{
			Name: "补全SKU", MatchMode: excelMatchModeOrderItemSKU, TableName: bojunRetailOrdersTable,
			MatchExcelColumn: "订单号", SpecExcelColumn: "规格编码", PriceExcelColumn: "销售单价", QtyExcelColumn: "销售数量",
			DBMatchField: "docno", DBValueField: "items_json", OutputColumnName: "完整SKU",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	lookup := &fakeExcelMatchLookup{value: map[string]string{
		"ORDER-1": `[
			{"no":"SKU-A","qty":1,"priceactual":319.2,"mProductName":"C09H1073A"},
			{"no":"SKU-B","qty":1,"priceactual":319.2,"mProductName":"C09H1073B"}
		]`,
	}}
	stats, err := processExcelMatchFile(context.Background(), inputPath, outputPath, cfg, lookup)
	if err != nil {
		t.Fatal(err)
	}
	if stats.MatchedRows != 2 {
		t.Fatalf("stats = %+v, want both fuzzy and exact rows matched", stats)
	}
	out, err := excelize.OpenFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = out.Close() }()
	if got, _ := out.GetCellValue("Result_1", "E2"); got != "SKU-B" {
		t.Fatalf("8-digit row SKU = %q, want unreserved SKU-B", got)
	}
	if got, _ := out.GetCellValue("Result_1", "E502"); got != "SKU-A" {
		t.Fatalf("9-digit row SKU = %q, want reserved exact SKU-A", got)
	}
}

func TestExcelOrderItemMatchSelectsExactNineDigitSpec(t *testing.T) {
	items, err := parseExcelOrderItems(`[
		{"no":"SKU-A","qty":1,"priceactual":319.2,"mProductName":"C09H1073A"},
		{"no":"SKU-B","qty":1,"priceactual":"319.20","mProductName":"C09H1073B"}
	]`)
	if err != nil {
		t.Fatal(err)
	}
	state := newExcelOrderItemMatchState(nil)
	got, reason, err := state.match(excelOrderItemDetail{items: items}, "ORDER-1", "c09h1073b", 31920, 1)
	if err != nil || got != "SKU-B" || reason != "" {
		t.Fatalf("match() = %q, %q, %v, want exact SKU-B", got, reason, err)
	}
}

func TestExcelOrderItemMatchReservesOnlyRequiredCandidateCapacity(t *testing.T) {
	items, err := parseExcelOrderItems(`[
		{"no":"SKU-A-1","qty":1,"priceactual":319.2,"mProductName":"C09H1073A"},
		{"no":"SKU-A-2","qty":1,"priceactual":319.2,"mProductName":"C09H1073A"}
	]`)
	if err != nil {
		t.Fatal(err)
	}
	reservations := excelOrderItemReservations{0: {}}
	reservations.add(0, "ORDER-1", "C09H1073A", 31920, 1)
	state := newExcelOrderItemMatchState(reservations)
	detail := excelOrderItemDetail{items: items}
	fuzzySKU, reason, err := state.match(detail, "ORDER-1", "C09H1073", 31920, 1)
	if err != nil || fuzzySKU != "SKU-A-1" || reason != "" {
		t.Fatalf("fuzzy match = %q, %q, %v, want one unreserved candidate", fuzzySKU, reason, err)
	}
	reservations.consume(0, "ORDER-1", "C09H1073A", 31920, 1)
	exactSKU, reason, err := state.match(detail, "ORDER-1", "C09H1073A", 31920, 1)
	if err != nil || exactSKU != "SKU-A-2" || reason != "" {
		t.Fatalf("exact match = %q, %q, %v, want remaining reserved candidate", exactSKU, reason, err)
	}
}

func TestExcelOrderItemMatchDoesNotCountDuplicateNoAsCapacity(t *testing.T) {
	items, err := parseExcelOrderItems(`[
		{"no":"SKU-A","qty":1,"priceactual":319.2,"mProductName":"C09H1073A"},
		{"no":"SKU-A","qty":1,"priceactual":319.2,"mProductName":"C09H1073A"}
	]`)
	if err != nil {
		t.Fatal(err)
	}
	reservations := excelOrderItemReservations{0: {}}
	reservations.add(0, "ORDER-1", "C09H1073A", 31920, 1)
	state := newExcelOrderItemMatchState(reservations)
	detail := excelOrderItemDetail{items: items}
	fuzzySKU, reason, err := state.match(detail, "ORDER-1", "C09H1073", 31920, 1)
	if err != nil || fuzzySKU != "" || reason != "符合条件的SKU已为9位精确规格编码行保留" {
		t.Fatalf("fuzzy match = %q, %q, %v, want duplicate no reserved once", fuzzySKU, reason, err)
	}
	reservations.consume(0, "ORDER-1", "C09H1073A", 31920, 1)
	exactSKU, reason, err := state.match(detail, "ORDER-1", "C09H1073A", 31920, 1)
	if err != nil || exactSKU != "SKU-A" || reason != "" {
		t.Fatalf("exact match = %q, %q, %v, want SKU-A", exactSKU, reason, err)
	}
}

func TestParseExcelOrderItemsSkipsInvalidItemAndRejectsTrailingJSON(t *testing.T) {
	items, err := parseExcelOrderItems(`[
		{"no":{"bad":true},"qty":1,"priceactual":319.2,"mProductName":"C09H1073A"},
		{"no":"SKU-B","qty":1,"priceactual":319.2,"mProductName":"C09H1073B"}
	]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].no != "SKU-B" {
		t.Fatalf("items = %#v, want only valid SKU-B", items)
	}
	if _, err := parseExcelOrderItems(`[] {}`); err == nil {
		t.Fatal("parseExcelOrderItems accepted trailing JSON")
	}
}

func TestProcessExcelMatchFileEvaluatesEachStepFilterAgainstAllRows(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "source.xlsx")
	outputPath := filepath.Join(dir, "result.xlsx")

	f := excelize.NewFile()
	rows := [][]interface{}{
		{"类型", "订单号"},
		{"A", "A001"},
		{"B", "B001"},
		{"C", "C001"},
	}
	for index, row := range rows {
		cell, err := excelize.CoordinatesToCellName(1, index+1)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.SetSheetRow("Sheet1", cell, &row); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.SaveAs(inputPath); err != nil {
		t.Fatal(err)
	}

	cfg, err := normalizeExcelMatchConfig(ExcelMatchConfig{
		Operation: "export_match",
		Steps: []ExcelMatchStep{
			{
				Name:             "匹配 A 类",
				Filters:          []ExcelMatchFilter{{Column: "类型", Op: "eq", Value: "A"}},
				TableName:        "a_orders",
				MatchExcelColumn: "订单号",
				DBMatchField:     "docno",
				DBValueField:     "result",
				OutputColumnName: "A结果",
			},
			{
				Name:             "匹配 B 类",
				Filters:          []ExcelMatchFilter{{Column: "类型", Op: "eq", Value: "B"}},
				TableName:        "b_orders",
				MatchExcelColumn: "订单号",
				DBMatchField:     "docno",
				DBValueField:     "result",
				OutputColumnName: "B结果",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	lookup := &fakeExcelMatchLookup{stepValues: map[string]map[string]string{
		"A结果": {"A001": "A命中"},
		"B结果": {"B001": "B命中"},
	}}

	stats, err := processExcelMatchFile(context.Background(), inputPath, outputPath, cfg, lookup)
	if err != nil {
		t.Fatalf("processExcelMatchFile returned error: %v", err)
	}
	if !reflect.DeepEqual(lookup.keys, []string{"A001", "B001"}) {
		t.Fatalf("lookup keys = %#v, want each step to use its own row set", lookup.keys)
	}
	if stats.TotalRows != 3 || stats.FilteredRows != 2 || stats.MatchedRows != 2 || stats.UnmatchedRows != 0 {
		t.Fatalf("stats = %+v, want total=3 filtered=2 matched=2 unmatched=0", stats)
	}

	out, err := excelize.OpenFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = out.Close() }()
	got, err := out.GetRows("Result_1")
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"类型", "订单号", "A结果", "B结果"},
		{"A", "A001", "A命中"},
		{"B", "B001", "", "B命中"},
		{"C", "C001"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("result rows = %#v, want %#v", got, want)
	}
}

func TestProcessExcelMatchFileUsesConfiguredDBMatchField(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "source.xlsx")
	outputPath := filepath.Join(dir, "result.xlsx")

	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	rows := [][]interface{}{
		{"店铺名称", "外部订单编号"},
		{"杭州恒隆", "EXT001"},
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
		value: map[string]string{"EXT001": "BOJUN001"},
	}
	cfg, err := normalizeExcelMatchConfig(ExcelMatchConfig{
		SheetName:        "Sheet1",
		Filters:          []ExcelMatchFilter{{Column: "店铺名称", Op: "eq", Value: "杭州恒隆"}},
		MatchExcelColumn: "外部订单编号",
		DBTemplate:       "bojun_retail_order",
		DBMatchField:     "otherdocno",
		DBValueField:     "docno",
		OutputColumnName: "伯俊单号",
	})
	if err != nil {
		t.Fatalf("normalizeExcelMatchConfig failed: %v", err)
	}

	stats, err := processExcelMatchFile(context.Background(), inputPath, outputPath, cfg, lookup)
	if err != nil {
		t.Fatalf("processExcelMatchFile failed: %v", err)
	}

	if !reflect.DeepEqual(lookup.matchFields, []string{"otherdocno"}) {
		t.Fatalf("lookup match fields = %#v, want otherdocno", lookup.matchFields)
	}
	if !reflect.DeepEqual(lookup.keys, []string{"EXT001"}) {
		t.Fatalf("lookup keys = %#v, want EXT001", lookup.keys)
	}
	if stats.MatchedRows != 1 || stats.UnmatchedRows != 0 {
		t.Fatalf("stats = %+v, want one matched row", stats)
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
		{"店铺名称", "外部订单编号", "伯俊单号"},
		{"杭州恒隆", "EXT001", "BOJUN001"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("result rows = %#v, want %#v", got, want)
	}
}

func TestProcessExcelMatchFileUsesConfiguredExportColumnFormat(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "source.xlsx")
	outputPath := filepath.Join(dir, "result.xlsx")

	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	rows := [][]interface{}{
		{"店铺名称", "订单号", "金额"},
		{"杭州恒隆", "B001", "123.45"},
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

	cfg, err := normalizeExcelMatchConfig(ExcelMatchConfig{
		SheetName:        "Sheet1",
		Filters:          []ExcelMatchFilter{{Column: "店铺名称", Op: "eq", Value: "杭州恒隆"}},
		MatchExcelColumn: "订单号",
		DBTemplate:       "bojun_retail_order",
		DBMatchField:     "docno",
		DBValueField:     "c_store_name",
		OutputColumnName: "线下金额",
		ExportColumnFormats: []ExcelExportColumnFormat{
			{Column: "金额", Format: "number"},
			{Column: "线下金额", Format: "number"},
		},
	})
	if err != nil {
		t.Fatalf("normalizeExcelMatchConfig failed: %v", err)
	}
	_, err = processExcelMatchFile(context.Background(), inputPath, outputPath, cfg, &fakeExcelMatchLookup{value: map[string]string{"B001": "456.78"}})
	if err != nil {
		t.Fatalf("processExcelMatchFile returned error: %v", err)
	}

	out, err := excelize.OpenFile(outputPath)
	if err != nil {
		t.Fatalf("OpenFile result failed: %v", err)
	}
	defer func() { _ = out.Close() }()

	rawAmount, err := out.GetCellValue("Result_1", "C2", excelize.Options{RawCellValue: true})
	if err != nil {
		t.Fatal(err)
	}
	if rawAmount != "123.45" {
		t.Fatalf("raw amount = %q, want numeric raw value 123.45", rawAmount)
	}
	cellType, err := out.GetCellType("Result_1", "C2")
	if err != nil {
		t.Fatal(err)
	}
	if cellType == excelize.CellTypeInlineString || cellType == excelize.CellTypeSharedString {
		t.Fatalf("amount cell type = %v, want numeric cell from configured format", cellType)
	}
	rawAppended, err := out.GetCellValue("Result_1", "D2", excelize.Options{RawCellValue: true})
	if err != nil {
		t.Fatal(err)
	}
	if rawAppended != "456.78" {
		t.Fatalf("raw appended = %q, want numeric raw value 456.78", rawAppended)
	}
	appendedType, err := out.GetCellType("Result_1", "D2")
	if err != nil {
		t.Fatal(err)
	}
	if appendedType == excelize.CellTypeInlineString || appendedType == excelize.CellTypeSharedString {
		t.Fatalf("appended cell type = %v, want numeric cell from configured format", appendedType)
	}
}

func TestProcessExcelMatchPreviewReturnsSamplesWithoutWritingOutput(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "source.xlsx")
	outputPath := filepath.Join(dir, "result.xlsx")

	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	rows := [][]interface{}{
		{"店铺名称", "外部订单编号", "金额"},
		{"杭州恒隆", "EXT001", "10"},
		{"上海前滩", "EXT002", "20"},
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

	input, err := excelize.OpenFile(inputPath)
	if err != nil {
		t.Fatalf("OpenFile source failed: %v", err)
	}
	defer func() { _ = input.Close() }()

	lookup := &fakeExcelMatchLookup{value: map[string]string{"EXT001": "BOJUN001"}}
	cfg, err := normalizeExcelMatchConfig(ExcelMatchConfig{
		SheetName:        "Sheet1",
		Filters:          []ExcelMatchFilter{{Column: "店铺名称", Op: "eq", Value: "杭州恒隆"}},
		MatchExcelColumn: "外部订单编号",
		DBTemplate:       "bojun_retail_order",
		DBMatchField:     "otherdocno",
		DBValueField:     "docno",
		OutputColumnName: "伯俊单号",
	})
	if err != nil {
		t.Fatalf("normalizeExcelMatchConfig failed: %v", err)
	}

	preview, err := processExcelMatchPreview(context.Background(), input, cfg, lookup, 100, 10)
	if err != nil {
		t.Fatalf("processExcelMatchPreview failed: %v", err)
	}

	if !reflect.DeepEqual(lookup.matchFields, []string{"otherdocno"}) {
		t.Fatalf("lookup match fields = %#v, want otherdocno", lookup.matchFields)
	}
	if !reflect.DeepEqual(lookup.keys, []string{"EXT001"}) {
		t.Fatalf("lookup keys = %#v, want only filtered row key", lookup.keys)
	}
	if preview.Stats.TotalRows != 2 || preview.Stats.FilteredRows != 1 || preview.Stats.MatchedRows != 1 || preview.Stats.UnmatchedRows != 0 {
		t.Fatalf("stats = %+v, want total=2 filtered=1 matched=1 unmatched=0", preview.Stats)
	}
	if len(preview.Samples) != 2 {
		t.Fatalf("samples length = %d, want 2", len(preview.Samples))
	}
	if preview.Samples[0].Status != "matched" || preview.Samples[0].MatchedValue != "BOJUN001" {
		t.Fatalf("first sample = %+v, want matched BOJUN001", preview.Samples[0])
	}
	if len(preview.Samples[0].StepResults) != 1 || preview.Samples[0].StepResults[0].StepName != "步骤 1" {
		t.Fatalf("preview step results = %#v", preview.Samples[0].StepResults)
	}
	if preview.Samples[1].Status != "skipped" {
		t.Fatalf("second sample = %+v, want skipped", preview.Samples[1])
	}
	if _, err := excelize.OpenFile(outputPath); err == nil {
		t.Fatal("preview unexpectedly wrote result.xlsx")
	}
}

func TestProcessExcelMatchPreviewReservesExactSpecsBeyondScanLimit(t *testing.T) {
	f := excelize.NewFile()
	if err := f.SetSheetRow("Sheet1", "A1", &[]interface{}{"订单号", "规格编码", "销售单价", "销售数量"}); err != nil {
		t.Fatal(err)
	}
	if err := f.SetSheetRow("Sheet1", "A2", &[]interface{}{"ORDER-1", "C09H1073", "319.20", "1"}); err != nil {
		t.Fatal(err)
	}
	for row := 3; row <= 101; row++ {
		cell, err := excelize.CoordinatesToCellName(1, row)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.SetSheetRow("Sheet1", cell, &[]interface{}{"", "", "", ""}); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.SetSheetRow("Sheet1", "A102", &[]interface{}{"ORDER-1", "C09H1073A", "319.20", "1"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := normalizeExcelMatchConfig(ExcelMatchConfig{
		Operation: excelOperationExportMatch,
		BatchSize: 500,
		Steps: []ExcelMatchStep{{
			Name: "补全SKU", MatchMode: excelMatchModeOrderItemSKU, TableName: bojunRetailOrdersTable,
			MatchExcelColumn: "订单号", SpecExcelColumn: "规格编码", PriceExcelColumn: "销售单价", QtyExcelColumn: "销售数量",
			DBMatchField: "docno", DBValueField: "items_json", OutputColumnName: "完整SKU",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	lookup := &fakeExcelMatchLookup{value: map[string]string{
		"ORDER-1": `[
			{"no":"SKU-A","qty":1,"priceactual":319.2,"mProductName":"C09H1073A"},
			{"no":"SKU-B","qty":1,"priceactual":319.2,"mProductName":"C09H1073B"}
		]`,
	}}
	preview, err := processExcelMatchPreview(context.Background(), f, cfg, lookup, 100, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Samples) != 1 || preview.Samples[0].MatchedValue != "SKU-B" {
		t.Fatalf("preview samples = %#v, want fuzzy row to preserve SKU-A beyond scan limit", preview.Samples)
	}
}

func TestExcelUploadAssemblesChunksAndRejectsInvalidID(t *testing.T) {
	uploadID := "0123456789abcdef0123456789abcdef"
	defer func() { _ = os.RemoveAll(excelUploadDir(uploadID)) }()

	meta := excelUploadMeta{
		UploadID:       uploadID,
		FileName:       "source.xlsx",
		TotalChunks:    2,
		UploadedChunks: 0,
		Complete:       false,
		CreatedAt:      time.Now(),
		ExpiresAt:      time.Now().Add(time.Hour),
	}
	if err := os.MkdirAll(excelUploadChunksDir(uploadID), 0700); err != nil {
		t.Fatal(err)
	}
	if err := writeExcelUploadMeta(uploadID, meta); err != nil {
		t.Fatalf("writeExcelUploadMeta failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(excelUploadChunksDir(uploadID), excelUploadChunkName(0)), []byte("hello "), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(excelUploadChunksDir(uploadID), excelUploadChunkName(1)), []byte("world"), 0600); err != nil {
		t.Fatal(err)
	}

	count, err := countExcelUploadChunks(uploadID, 2)
	if err != nil {
		t.Fatalf("countExcelUploadChunks failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("count = %d, want 2", count)
	}
	if err := assembleExcelUpload(uploadID, 2); err != nil {
		t.Fatalf("assembleExcelUpload failed: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(excelUploadDir(uploadID), excelUploadMergedFile))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello world" {
		t.Fatalf("merged content = %q, want hello world", string(got))
	}
	if _, err := readExcelUploadMeta("../bad"); err == nil {
		t.Fatal("readExcelUploadMeta accepted invalid upload id")
	}
}

func TestExcelTempRootDirCanBeConfigured(t *testing.T) {
	root := filepath.Join(t.TempDir(), "excel-work")
	t.Setenv(excelTempRootEnvName, root)

	jobDir := excelMatchJobDir(42)
	if !strings.HasPrefix(filepath.Clean(jobDir), filepath.Clean(root)) {
		t.Fatalf("excelMatchJobDir() = %s, want under %s", jobDir, root)
	}
	uploadDir := excelUploadRootDir()
	if !strings.HasPrefix(filepath.Clean(uploadDir), filepath.Clean(root)) {
		t.Fatalf("excelUploadRootDir() = %s, want under %s", uploadDir, root)
	}
}

func TestWithExcelizeTempDirSetsAndRestoresTempEnvironment(t *testing.T) {
	originalTmpDir, hadTmpDir := os.LookupEnv("TMPDIR")
	originalTmp, hadTmp := os.LookupEnv("TMP")
	originalTemp, hadTemp := os.LookupEnv("TEMP")
	tempDir := filepath.Join(t.TempDir(), "excelize")

	if err := withExcelizeTempDir(tempDir, func() error {
		for _, key := range []string{"TMPDIR", "TMP", "TEMP"} {
			if got := os.Getenv(key); filepath.Clean(got) != filepath.Clean(tempDir) {
				t.Fatalf("%s = %s, want %s", key, got, tempDir)
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("withExcelizeTempDir returned error: %v", err)
	}

	assertEnvRestored := func(key, want string, existed bool) {
		got, exists := os.LookupEnv(key)
		if exists != existed || got != want {
			t.Fatalf("%s restored to (%q,%v), want (%q,%v)", key, got, exists, want, existed)
		}
	}
	assertEnvRestored("TMPDIR", originalTmpDir, hadTmpDir)
	assertEnvRestored("TMP", originalTmp, hadTmp)
	assertEnvRestored("TEMP", originalTemp, hadTemp)
}

func TestExcelMatchOSSUploadTimeoutScalesWithFileSize(t *testing.T) {
	t.Setenv(excelMatchOSSUploadTimeoutEnvName, "")

	if got := excelMatchOSSUploadTimeout(0); got != excelMatchOSSUploadMinTimeout {
		t.Fatalf("timeout for empty size = %s, want %s", got, excelMatchOSSUploadMinTimeout)
	}
	largeFile := int64(500 * 1024 * 1024)
	got := excelMatchOSSUploadTimeout(largeFile)
	if got <= 90*time.Second {
		t.Fatalf("timeout for 500MB = %s, want greater than 90s", got)
	}
	if got < excelMatchOSSUploadMinTimeout {
		t.Fatalf("timeout for 500MB = %s, want at least %s", got, excelMatchOSSUploadMinTimeout)
	}
}

func TestExcelMatchOSSUploadTimeoutCanBeOverridden(t *testing.T) {
	t.Setenv(excelMatchOSSUploadTimeoutEnvName, "7200")

	got := excelMatchOSSUploadTimeout(500 * 1024 * 1024)
	if got != 2*time.Hour {
		t.Fatalf("timeout override = %s, want 2h", got)
	}
}

func TestExcelOSSProgressLoggerThrottlesAndFlushes(t *testing.T) {
	var logged []storage.UploadProgress
	logger := newExcelOSSProgressLogger(func(progress storage.UploadProgress, elapsed time.Duration) {
		logged = append(logged, progress)
		if elapsed < 0 {
			t.Fatalf("elapsed = %s, want non-negative", elapsed)
		}
	})

	logger.Handle(storage.UploadProgress{Transferred: 1024, Total: 100 * 1024 * 1024})
	if len(logged) != 0 {
		t.Fatalf("logged %d progress events before threshold, want 0", len(logged))
	}

	logger.Handle(storage.UploadProgress{Transferred: excelMatchOSSProgressBytes, Total: 100 * 1024 * 1024})
	if len(logged) != 1 {
		t.Fatalf("logged %d progress events at byte threshold, want 1", len(logged))
	}

	logger.Handle(storage.UploadProgress{Transferred: excelMatchOSSProgressBytes + 1024, Total: 100 * 1024 * 1024})
	if len(logged) != 1 {
		t.Fatalf("logged %d progress events below next threshold, want 1", len(logged))
	}

	logger.Flush()
	if len(logged) != 2 {
		t.Fatalf("logged %d progress events after flush, want 2", len(logged))
	}
	if logged[1].Transferred != excelMatchOSSProgressBytes+1024 {
		t.Fatalf("flushed transferred = %d, want %d", logged[1].Transferred, excelMatchOSSProgressBytes+1024)
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

func TestRefreshDownloadStateRejectsNonExportJobs(t *testing.T) {
	job := &model.ExcelMatchJob{
		ConfigJSON: `{"operation":"import_update"}`,
		Status:     excelMatchStatusSuccess,
	}

	(&ExcelMatchJobService{}).refreshDownloadState(context.Background(), job)

	if job.CanDownload {
		t.Fatal("CanDownload = true, want false for import_update job")
	}
	if job.DownloadMessage == "" {
		t.Fatal("DownloadMessage is empty, want reason for non-export job")
	}
}

func TestRefreshDownloadStateRequiresResultURL(t *testing.T) {
	jobID := uint(987654)
	workDir := excelMatchJobDir(jobID)
	if err := os.MkdirAll(workDir, 0700); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(workDir)

	resultPath := filepath.Join(workDir, excelMatchResultFileName)
	job := &model.ExcelMatchJob{
		BaseModel:      model.BaseModel{ID: jobID},
		ConfigJSON:     `{"operation":"export_match"}`,
		Status:         excelMatchStatusSuccess,
		ResultFilePath: resultPath,
	}

	(&ExcelMatchJobService{}).refreshDownloadState(context.Background(), job)
	if job.CanDownload {
		t.Fatal("CanDownload = true, want false when result file is missing")
	}
	if job.DownloadMessage == "" {
		t.Fatal("DownloadMessage is empty, want missing file reason")
	}

	if err := os.WriteFile(resultPath, []byte("xlsx"), 0600); err != nil {
		t.Fatal(err)
	}
	job.Status = excelMatchStatusSuccess
	(&ExcelMatchJobService{}).refreshDownloadState(context.Background(), job)
	if job.CanDownload {
		t.Fatal("CanDownload = true, want false before OSS result URL exists")
	}
	job.ResultURL = "https://warehouse.youlankids.com/data-warehouse/excel-match-results/987654/result.xlsx"
	(&ExcelMatchJobService{}).refreshDownloadState(context.Background(), job)
	if !job.CanDownload {
		t.Fatalf("CanDownload = false, want true when result URL exists: %s", job.DownloadMessage)
	}
}
