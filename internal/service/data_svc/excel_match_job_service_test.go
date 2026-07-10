package data_svc

import (
	"context"
	"gin-biz-web-api/model"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

type fakeExcelMatchLookup struct {
	matchFields []string
	keys        []string
	value       map[string]string
}

func (f *fakeExcelMatchLookup) Lookup(ctx context.Context, matchField string, keys []string, valueField string) (map[string]string, error) {
	f.matchFields = append(f.matchFields, matchField)
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
	if preview.Samples[1].Status != "skipped" {
		t.Fatalf("second sample = %+v, want skipped", preview.Samples[1])
	}
	if _, err := excelize.OpenFile(outputPath); err == nil {
		t.Fatal("preview unexpectedly wrote result.xlsx")
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

func TestRefreshDownloadStateRequiresExistingResultFile(t *testing.T) {
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
	if job.Status != excelMatchStatusExpired {
		t.Fatalf("Status = %s, want expired when result file is missing", job.Status)
	}
	if job.DownloadMessage == "" {
		t.Fatal("DownloadMessage is empty, want missing file reason")
	}

	if err := os.WriteFile(resultPath, []byte("xlsx"), 0600); err != nil {
		t.Fatal(err)
	}
	job.Status = excelMatchStatusSuccess
	(&ExcelMatchJobService{}).refreshDownloadState(context.Background(), job)
	if !job.CanDownload {
		t.Fatalf("CanDownload = false, want true when result file exists: %s", job.DownloadMessage)
	}
}
