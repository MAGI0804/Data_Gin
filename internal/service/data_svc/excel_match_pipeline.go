package data_svc

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

type excelMatchPipelineRow struct {
	rowNumber   int
	values      []string
	eligible    bool
	stepResults []ExcelMatchPreviewStepResult
}

type excelMatchPipelineLayout struct {
	headers          []string
	columnIndexes    map[string]int
	stepInputIndexes []int
	columnFormats    []string
}

func processExcelMatchFile(ctx context.Context, inputPath, outputPath string, config ExcelMatchConfig, lookup ExcelMatchLookup) (ExcelMatchJobStats, error) {
	return processExcelMatchFileWithProgress(ctx, inputPath, outputPath, config, lookup, nil)
}

func prepareExcelMatchPipeline(headers []string, config ExcelMatchConfig) (excelMatchPipelineLayout, error) {
	layout := excelMatchPipelineLayout{columnIndexes: make(map[string]int, len(headers)+len(config.Steps))}
	layout.headers = append(layout.headers, headers...)
	for index, header := range headers {
		layout.columnIndexes[header] = index
	}
	for _, filter := range config.Filters {
		if _, ok := layout.columnIndexes[filter.Column]; !ok {
			return layout, fmt.Errorf("Excel 缺少筛选列: %s", filter.Column)
		}
	}
	for index, step := range config.Steps {
		inputIndex, ok := layout.columnIndexes[step.MatchExcelColumn]
		if !ok {
			return layout, fmt.Errorf("第 %d 个匹配步骤缺少输入列: %s", index+1, step.MatchExcelColumn)
		}
		if _, exists := layout.columnIndexes[step.OutputColumnName]; exists {
			return layout, fmt.Errorf("第 %d 个匹配步骤输出列已存在: %s", index+1, step.OutputColumnName)
		}
		layout.stepInputIndexes = append(layout.stepInputIndexes, inputIndex)
		layout.columnIndexes[step.OutputColumnName] = len(layout.headers)
		layout.headers = append(layout.headers, step.OutputColumnName)
	}
	formatByColumn := excelExportColumnFormatMap(config.ExportColumnFormats)
	for column := range formatByColumn {
		if _, ok := layout.columnIndexes[column]; !ok {
			return layout, fmt.Errorf("导出格式配置列不存在: %s", column)
		}
	}
	layout.columnFormats = excelExportColumnFormatsForHeaders(layout.headers, formatByColumn)
	return layout, nil
}

func runExcelMatchSteps(ctx context.Context, config ExcelMatchConfig, lookup ExcelMatchLookup, layout excelMatchPipelineLayout, rows []*excelMatchPipelineRow) error {
	for stepIndex, step := range config.Steps {
		keys := make([]string, 0, len(rows))
		seen := make(map[string]struct{}, len(rows))
		inputIndex := layout.stepInputIndexes[stepIndex]
		for _, row := range rows {
			if !row.eligible || inputIndex >= len(row.values) {
				continue
			}
			key := strings.TrimSpace(row.values[inputIndex])
			if key != "" {
				if _, ok := seen[key]; !ok {
					seen[key] = struct{}{}
					keys = append(keys, key)
				}
			}
		}
		matches := map[string]string{}
		if len(keys) > 0 {
			var err error
			matches, err = lookup.Lookup(ctx, step, keys)
			if err != nil {
				return fmt.Errorf("执行第 %d 个匹配步骤失败: %w", stepIndex+1, err)
			}
		}
		for _, row := range rows {
			if err := ctx.Err(); err != nil {
				return err
			}
			result := ExcelMatchPreviewStepResult{StepIndex: stepIndex + 1, StepName: step.Name, Status: "skipped", Reason: "未命中前置筛选"}
			value := ""
			if row.eligible {
				if inputIndex < len(row.values) {
					result.MatchKey = strings.TrimSpace(row.values[inputIndex])
				}
				if result.MatchKey == "" {
					result.Status = "unmatched"
					result.Reason = "匹配键为空"
				} else if matched, ok := matches[result.MatchKey]; ok {
					value = matched
					result.MatchedValue = matched
					result.Status = "matched"
					result.Reason = "已匹配"
				} else {
					result.Status = "unmatched"
					result.Reason = "数据库无匹配记录"
				}
			}
			row.values = append(row.values, value)
			row.stepResults = append(row.stepResults, result)
		}
	}
	return nil
}

func updateExcelMatchFinalStats(stats *ExcelMatchJobStats, rows []*excelMatchPipelineRow) {
	for _, row := range rows {
		if row.eligible {
			if len(row.stepResults) > 0 && row.stepResults[len(row.stepResults)-1].Status == "matched" {
				stats.MatchedRows++
			} else {
				stats.UnmatchedRows++
			}
		}
		stats.ProcessedRows++
	}
}

func processExcelMatchPreview(ctx context.Context, input *excelize.File, config ExcelMatchConfig, lookup ExcelMatchLookup, scanLimit, sampleLimit int) (*ExcelMatchPreviewResult, error) {
	if scanLimit <= 0 || scanLimit > defaultExcelPreviewRows {
		scanLimit = defaultExcelPreviewRows
	}
	if sampleLimit <= 0 || sampleLimit > defaultExcelPreviewItems {
		sampleLimit = defaultExcelPreviewItems
	}
	if len(config.Steps) == 0 {
		return nil, errors.New("至少需要一个匹配步骤")
	}
	if !sheetExists(input.GetSheetList(), config.SheetName) {
		return nil, fmt.Errorf("Excel 不存在 sheet: %s", config.SheetName)
	}
	rows, err := input.Rows(config.SheetName)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := &ExcelMatchPreviewResult{Config: config, ScanLimit: scanLimit, SampleLimit: sampleLimit}
	var layout excelMatchPipelineLayout
	var buffered []*excelMatchPipelineRow
	headerRead := false
	flush := func() error {
		if len(buffered) == 0 {
			return nil
		}
		if err := runExcelMatchSteps(ctx, config, lookup, layout, buffered); err != nil {
			return err
		}
		updateExcelMatchFinalStats(&result.Stats, buffered)
		for _, row := range buffered {
			if len(result.Samples) >= sampleLimit {
				break
			}
			sample := ExcelMatchPreviewSample{RowNumber: row.rowNumber, Values: make(map[string]string, len(layout.headers)), StepResults: row.stepResults}
			for index, header := range layout.headers {
				if index < len(row.values) {
					sample.Values[header] = row.values[index]
				}
			}
			if len(row.stepResults) > 0 {
				last := row.stepResults[len(row.stepResults)-1]
				sample.MatchKey, sample.MatchedValue, sample.Status, sample.Reason = last.MatchKey, last.MatchedValue, last.Status, last.Reason
			}
			result.Samples = append(result.Samples, sample)
		}
		buffered = buffered[:0]
		return nil
	}
	for rows.Next() {
		columns, err := rows.Columns()
		if err != nil {
			return result, err
		}
		if !headerRead {
			headers := normalizeHeaders(columns)
			if len(headers) == 0 {
				return result, errors.New("Excel 表头不能为空")
			}
			layout, err = prepareExcelMatchPipeline(headers, config)
			if err != nil {
				return result, err
			}
			headerRead = true
			continue
		}
		result.Stats.TotalRows++
		values := normalizeExcelRow(columns, len(layout.headers)-len(config.Steps))
		eligible := excelRowMatchesFilters(values, layout.columnIndexes, config.Filters)
		if eligible {
			result.Stats.FilteredRows++
		}
		buffered = append(buffered, &excelMatchPipelineRow{rowNumber: result.Stats.TotalRows + 1, values: values, eligible: eligible})
		if len(buffered) >= config.BatchSize {
			if err := flush(); err != nil {
				return result, err
			}
		}
		if result.Stats.TotalRows >= scanLimit {
			result.Truncated = true
			break
		}
	}
	if err := rows.Error(); err != nil {
		return result, err
	}
	if !headerRead {
		return result, errors.New("Excel 表头不能为空")
	}
	if err := flush(); err != nil {
		return result, err
	}
	return result, nil
}

func processExcelMatchFileWithProgress(ctx context.Context, inputPath, outputPath string, config ExcelMatchConfig, lookup ExcelMatchLookup, onProgress func(ExcelMatchJobStats)) (ExcelMatchJobStats, error) {
	input, err := excelize.OpenFile(inputPath)
	if err != nil {
		return ExcelMatchJobStats{}, err
	}
	defer func() { _ = input.Close() }()
	if len(config.Steps) == 0 {
		return ExcelMatchJobStats{}, errors.New("至少需要一个匹配步骤")
	}
	if !sheetExists(input.GetSheetList(), config.SheetName) {
		return ExcelMatchJobStats{}, fmt.Errorf("Excel 不存在 sheet: %s", config.SheetName)
	}
	rows, err := input.Rows(config.SheetName)
	if err != nil {
		return ExcelMatchJobStats{}, err
	}
	defer func() { _ = rows.Close() }()
	output := excelize.NewFile()
	defer func() { _ = output.Close() }()
	writer, currentSheet, err := initExcelMatchWriter(output)
	if err != nil {
		return ExcelMatchJobStats{}, err
	}

	stats := ExcelMatchJobStats{}
	var layout excelMatchPipelineLayout
	var buffered []*excelMatchPipelineRow
	rowInSheet, sheetIndex := 1, 1
	headerRead := false
	writeHeader := func() error {
		row := make([]interface{}, len(layout.headers))
		for index, header := range layout.headers {
			row[index] = header
		}
		if err := writer.SetRow("A1", row); err != nil {
			return err
		}
		rowInSheet = 2
		return nil
	}
	rotateSheet := func() error {
		if rowInSheet <= excelMaxRowsPerSheet {
			return nil
		}
		if err := writer.Flush(); err != nil {
			return err
		}
		sheetIndex++
		currentSheet = "Result_" + strconv.Itoa(sheetIndex)
		if _, err := output.NewSheet(currentSheet); err != nil {
			return err
		}
		writer, err = output.NewStreamWriter(currentSheet)
		if err != nil {
			return err
		}
		return writeHeader()
	}
	flush := func() error {
		if len(buffered) == 0 {
			return nil
		}
		if err := runExcelMatchSteps(ctx, config, lookup, layout, buffered); err != nil {
			return err
		}
		for _, bufferedRow := range buffered {
			if err := rotateSheet(); err != nil {
				return err
			}
			row := make([]interface{}, len(layout.headers))
			for index, value := range normalizeExcelRow(bufferedRow.values, len(layout.headers)) {
				row[index] = excelExportValueForFormat(value, layout.columnFormats[index])
			}
			cell, err := excelize.CoordinatesToCellName(1, rowInSheet)
			if err != nil {
				return err
			}
			if err := writer.SetRow(cell, row); err != nil {
				return err
			}
			rowInSheet++
		}
		updateExcelMatchFinalStats(&stats, buffered)
		if onProgress != nil {
			onProgress(stats)
		}
		buffered = buffered[:0]
		return nil
	}
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return stats, err
		}
		columns, err := rows.Columns()
		if err != nil {
			return stats, err
		}
		if !headerRead {
			headers := normalizeHeaders(columns)
			if len(headers) == 0 {
				return stats, errors.New("Excel 表头不能为空")
			}
			layout, err = prepareExcelMatchPipeline(headers, config)
			if err != nil {
				return stats, err
			}
			if err := writeHeader(); err != nil {
				return stats, err
			}
			headerRead = true
			continue
		}
		stats.TotalRows++
		values := normalizeExcelRow(columns, len(layout.headers)-len(config.Steps))
		eligible := excelRowMatchesFilters(values, layout.columnIndexes, config.Filters)
		if eligible {
			stats.FilteredRows++
		}
		buffered = append(buffered, &excelMatchPipelineRow{values: values, eligible: eligible})
		if len(buffered) >= config.BatchSize || len(buffered) >= maxBufferedExcelRows {
			if err := flush(); err != nil {
				return stats, err
			}
		}
	}
	if err := rows.Error(); err != nil {
		return stats, err
	}
	if !headerRead {
		return stats, errors.New("Excel 表头不能为空")
	}
	if err := flush(); err != nil {
		return stats, err
	}
	if err := writer.Flush(); err != nil {
		return stats, err
	}
	output.SetActiveSheet(0)
	if err := output.SaveAs(outputPath); err != nil {
		return stats, err
	}
	return stats, nil
}
