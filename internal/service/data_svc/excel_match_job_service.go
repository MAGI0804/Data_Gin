package data_svc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gin-biz-web-api/global"
	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/job"
	"gin-biz-web-api/model"

	"github.com/hibiken/asynq"
	"github.com/xuri/excelize/v2"
)

const (
	excelMatchStatusPending = "pending"
	excelMatchStatusSuccess = "success"
	excelMatchStatusExpired = "expired"

	excelMatchRetention      = 24 * time.Hour
	excelMatchRootDirName    = "data-warehouse-excel-jobs"
	excelMatchSourceFileName = "source.xlsx"
	excelMatchResultFileName = "result.xlsx"
	excelMaxRowsPerSheet     = 1048576
	defaultExcelMatchBatch   = 1000
	maxBufferedExcelRows     = 5000
	defaultExcelResultSheet  = "Result_1"
	bojunRetailOrderTemplate = "bojun_retail_order"
)

var allowedBojunValueFields = map[string]struct{}{
	"billdate":             {},
	"c_store_code":         {},
	"c_store_name":         {},
	"retailbilltype":       {},
	"order_type_code":      {},
	"order_type_name":      {},
	"tot_amt_actual":       {},
	"tot_amt_list":         {},
	"tot_qty":              {},
	"vipno":                {},
	"related_normal_docno": {},
	"o2o_so_docno":         {},
}

type ExcelMatchJobService struct {
	jobDAO *data_dao.ExcelMatchJobDAO
}

func NewExcelMatchJobService() *ExcelMatchJobService {
	return &ExcelMatchJobService{
		jobDAO: data_dao.NewExcelMatchJobDAO(),
	}
}

type ExcelMatchFilter struct {
	Column string `json:"column"`
	Op     string `json:"op"`
	Value  string `json:"value"`
}

type ExcelMatchConfig struct {
	Filters          []ExcelMatchFilter `json:"filters"`
	MatchExcelColumn string             `json:"matchExcelColumn"`
	DBTemplate       string             `json:"dbTemplate"`
	DBMatchField     string             `json:"dbMatchField"`
	DBValueField     string             `json:"dbValueField"`
	OutputColumnName string             `json:"outputColumnName"`
	BatchSize        int                `json:"batchSize"`
}

type ExcelMatchJobStats = data_dao.ExcelMatchJobStats

type ExcelMatchLookup interface {
	Lookup(ctx context.Context, keys []string, valueField string) (map[string]string, error)
}

type bojunExcelMatchLookup struct {
	dao *data_dao.ExcelMatchJobDAO
}

func (l bojunExcelMatchLookup) Lookup(ctx context.Context, keys []string, valueField string) (map[string]string, error) {
	return l.dao.FindBojunFieldByDocNos(ctx, keys, valueField)
}

func (s *ExcelMatchJobService) CreateJob(ctx context.Context, fileHeader *multipart.FileHeader, rawConfig string) (*model.ExcelMatchJob, error) {
	if fileHeader == nil {
		return nil, errors.New("上传文件不能为空")
	}
	if strings.ToLower(filepath.Ext(fileHeader.Filename)) != ".xlsx" {
		return nil, errors.New("仅支持 .xlsx 文件")
	}

	var config ExcelMatchConfig
	if err := json.Unmarshal([]byte(rawConfig), &config); err != nil {
		return nil, fmt.Errorf("解析匹配配置失败: %w", err)
	}
	normalizedConfig, err := normalizeExcelMatchConfig(config)
	if err != nil {
		return nil, err
	}
	configBytes, err := json.Marshal(normalizedConfig)
	if err != nil {
		return nil, err
	}

	expiresAt := time.Now().Add(excelMatchRetention)
	matchJob := &model.ExcelMatchJob{
		SourceFileName: filepath.Base(fileHeader.Filename),
		ConfigJSON:     string(configBytes),
		Status:         excelMatchStatusPending,
		ExpiresAt:      &model.TimeNormal{Time: expiresAt},
	}
	if _, err := s.jobDAO.Create(ctx, matchJob); err != nil {
		return nil, err
	}

	workDir := excelMatchJobDir(matchJob.ID)
	sourcePath := filepath.Join(workDir, excelMatchSourceFileName)
	resultPath := filepath.Join(workDir, excelMatchResultFileName)
	if err := os.MkdirAll(workDir, 0700); err != nil {
		_ = s.jobDAO.MarkFailed(ctx, matchJob.ID, err.Error(), expiresAt)
		return nil, err
	}
	if err := saveUploadedExcel(fileHeader, sourcePath); err != nil {
		_ = s.jobDAO.MarkFailed(ctx, matchJob.ID, err.Error(), expiresAt)
		_ = os.RemoveAll(workDir)
		return nil, err
	}
	if err := s.jobDAO.UpdatePaths(ctx, matchJob.ID, workDir, sourcePath, resultPath); err != nil {
		_ = s.jobDAO.MarkFailed(ctx, matchJob.ID, err.Error(), expiresAt)
		_ = os.RemoveAll(workDir)
		return nil, err
	}

	task, err := job.NewExcelMatchExportTask(matchJob.ID)
	if err != nil {
		_ = s.jobDAO.MarkFailed(ctx, matchJob.ID, err.Error(), expiresAt)
		return nil, err
	}
	if global.QueueJobClient == nil {
		err := errors.New("异步任务客户端未初始化")
		_ = s.jobDAO.MarkFailed(ctx, matchJob.ID, err.Error(), expiresAt)
		return nil, err
	}
	if _, err := global.QueueJobClient.Enqueue(task, asynq.MaxRetry(1)); err != nil {
		_ = s.jobDAO.MarkFailed(ctx, matchJob.ID, err.Error(), expiresAt)
		return nil, err
	}

	return s.jobDAO.FindByID(ctx, matchJob.ID)
}

func (s *ExcelMatchJobService) GetJob(ctx context.Context, id uint) (*model.ExcelMatchJob, error) {
	matchJob, err := s.jobDAO.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if isExcelMatchJobExpired(matchJob) {
		_ = os.RemoveAll(matchJob.WorkDir)
		_ = s.jobDAO.MarkExpired(ctx, matchJob.ID)
		matchJob.Status = excelMatchStatusExpired
	}
	return matchJob, nil
}

func (s *ExcelMatchJobService) Download(ctx context.Context, id uint) (*model.ExcelMatchJob, string, string, error) {
	matchJob, err := s.GetJob(ctx, id)
	if err != nil {
		return nil, "", "", err
	}
	if matchJob.Status != excelMatchStatusSuccess {
		return nil, "", "", fmt.Errorf("任务状态为 %s，无法下载", matchJob.Status)
	}
	if !isPathInside(excelMatchJobDir(matchJob.ID), matchJob.ResultFilePath) {
		return nil, "", "", errors.New("结果文件路径非法")
	}
	if _, err := os.Stat(matchJob.ResultFilePath); err != nil {
		return nil, "", "", err
	}
	return matchJob, matchJob.ResultFilePath, fmt.Sprintf("excel_match_job_%d.xlsx", matchJob.ID), nil
}

func (s *ExcelMatchJobService) ProcessJob(ctx context.Context, id uint) error {
	matchJob, err := s.jobDAO.FindByID(ctx, id)
	if err != nil {
		return err
	}

	var config ExcelMatchConfig
	if err := json.Unmarshal([]byte(matchJob.ConfigJSON), &config); err != nil {
		_ = s.jobDAO.MarkFailed(ctx, id, err.Error(), time.Now().Add(excelMatchRetention))
		return err
	}
	config, err = normalizeExcelMatchConfig(config)
	if err != nil {
		_ = s.jobDAO.MarkFailed(ctx, id, err.Error(), time.Now().Add(excelMatchRetention))
		return err
	}

	if err := s.jobDAO.MarkRunning(ctx, id); err != nil {
		return err
	}

	lookup := bojunExcelMatchLookup{dao: s.jobDAO}
	stats, err := processExcelMatchFileWithProgress(ctx, matchJob.SourceFilePath, matchJob.ResultFilePath, config, lookup, func(stats ExcelMatchJobStats) {
		_ = s.jobDAO.UpdateProgress(ctx, id, stats)
	})
	expiresAt := time.Now().Add(excelMatchRetention)
	if err != nil {
		_ = s.jobDAO.MarkFailed(ctx, id, err.Error(), expiresAt)
		return err
	}

	_ = os.Remove(matchJob.SourceFilePath)
	return s.jobDAO.MarkSuccess(ctx, id, stats, expiresAt)
}

func (s *ExcelMatchJobService) CleanupExpiredJobs(ctx context.Context) error {
	jobs, err := s.jobDAO.FindExpired(ctx, time.Now(), 100)
	if err != nil {
		return err
	}
	for _, matchJob := range jobs {
		if matchJob.WorkDir != "" && isPathInside(excelMatchJobDir(matchJob.ID), matchJob.WorkDir) {
			_ = os.RemoveAll(matchJob.WorkDir)
		}
		_ = s.jobDAO.MarkExpired(ctx, matchJob.ID)
	}
	return nil
}

func normalizeExcelMatchConfig(config ExcelMatchConfig) (ExcelMatchConfig, error) {
	config.MatchExcelColumn = strings.TrimSpace(config.MatchExcelColumn)
	config.DBTemplate = strings.TrimSpace(config.DBTemplate)
	config.DBMatchField = strings.TrimSpace(config.DBMatchField)
	config.DBValueField = strings.TrimSpace(config.DBValueField)
	config.OutputColumnName = strings.TrimSpace(config.OutputColumnName)

	if len(config.Filters) == 0 {
		return config, errors.New("至少需要一个 Excel 前置筛选条件")
	}
	for i := range config.Filters {
		config.Filters[i].Column = strings.TrimSpace(config.Filters[i].Column)
		config.Filters[i].Op = strings.TrimSpace(config.Filters[i].Op)
		config.Filters[i].Value = strings.TrimSpace(config.Filters[i].Value)
		if config.Filters[i].Column == "" {
			return config, errors.New("筛选列不能为空")
		}
		if config.Filters[i].Op == "" {
			config.Filters[i].Op = "eq"
		}
		if config.Filters[i].Op != "eq" {
			return config, fmt.Errorf("暂不支持筛选操作: %s", config.Filters[i].Op)
		}
	}
	if config.MatchExcelColumn == "" {
		return config, errors.New("Excel 订单号列不能为空")
	}
	if config.DBTemplate != bojunRetailOrderTemplate {
		return config, fmt.Errorf("暂不支持数据库模板: %s", config.DBTemplate)
	}
	if config.DBMatchField == "" {
		config.DBMatchField = "docno"
	}
	if config.DBMatchField != "docno" {
		return config, errors.New("伯俊零售单仅支持按 docno 匹配")
	}
	if _, ok := allowedBojunValueFields[config.DBValueField]; !ok {
		return config, fmt.Errorf("伯俊写回字段不在白名单: %s", config.DBValueField)
	}
	if config.OutputColumnName == "" {
		return config, errors.New("输出列名不能为空")
	}
	if config.BatchSize == 0 {
		config.BatchSize = defaultExcelMatchBatch
	}
	if config.BatchSize < 500 || config.BatchSize > 2000 {
		return config, errors.New("batchSize 必须在 500 到 2000 之间")
	}

	return config, nil
}

func processExcelMatchFile(ctx context.Context, inputPath, outputPath string, config ExcelMatchConfig, lookup ExcelMatchLookup) (ExcelMatchJobStats, error) {
	return processExcelMatchFileWithProgress(ctx, inputPath, outputPath, config, lookup, nil)
}

func processExcelMatchFileWithProgress(
	ctx context.Context,
	inputPath string,
	outputPath string,
	config ExcelMatchConfig,
	lookup ExcelMatchLookup,
	onProgress func(ExcelMatchJobStats),
) (ExcelMatchJobStats, error) {
	input, err := excelize.OpenFile(inputPath)
	if err != nil {
		return ExcelMatchJobStats{}, err
	}
	defer func() { _ = input.Close() }()

	sheets := input.GetSheetList()
	if len(sheets) == 0 {
		return ExcelMatchJobStats{}, errors.New("Excel 没有可读取的 sheet")
	}
	rows, err := input.Rows(sheets[0])
	if err != nil {
		return ExcelMatchJobStats{}, err
	}
	defer func() { _ = rows.Close() }()

	output := excelize.NewFile()
	writer, currentSheet, err := initExcelMatchWriter(output)
	if err != nil {
		return ExcelMatchJobStats{}, err
	}

	var headers []string
	columnIndexes := map[string]int{}
	var matchColumnIndex int
	stats := ExcelMatchJobStats{}
	rowInSheet := 1
	sheetIndex := 1

	type bufferedRow struct {
		values   []string
		eligible bool
		key      string
	}
	var buffered []bufferedRow
	var lookupKeys []string
	lookupKeySet := map[string]struct{}{}

	writeHeader := func() error {
		row := make([]interface{}, 0, len(headers)+1)
		for _, header := range headers {
			row = append(row, header)
		}
		row = append(row, config.OutputColumnName)
		cell, err := excelize.CoordinatesToCellName(1, 1)
		if err != nil {
			return err
		}
		if err := writer.SetRow(cell, row); err != nil {
			return err
		}
		rowInSheet = 2
		return nil
	}

	rotateSheetIfNeeded := func() error {
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
		var err error
		writer, err = output.NewStreamWriter(currentSheet)
		if err != nil {
			return err
		}
		return writeHeader()
	}

	writeDataRow := func(values []string, appended string) error {
		if err := rotateSheetIfNeeded(); err != nil {
			return err
		}
		row := make([]interface{}, 0, len(headers)+1)
		for _, value := range normalizeExcelRow(values, len(headers)) {
			row = append(row, value)
		}
		row = append(row, appended)
		cell, err := excelize.CoordinatesToCellName(1, rowInSheet)
		if err != nil {
			return err
		}
		if err := writer.SetRow(cell, row); err != nil {
			return err
		}
		rowInSheet++
		return nil
	}

	flush := func() error {
		if len(buffered) == 0 {
			return nil
		}
		matches := map[string]string{}
		if len(lookupKeys) > 0 {
			var err error
			matches, err = lookup.Lookup(ctx, lookupKeys, config.DBValueField)
			if err != nil {
				return err
			}
		}
		for _, row := range buffered {
			if err := ctx.Err(); err != nil {
				return err
			}
			appendValue := ""
			if row.eligible {
				if value, ok := matches[row.key]; ok && row.key != "" {
					appendValue = value
					stats.MatchedRows++
				} else {
					stats.UnmatchedRows++
				}
			}
			if err := writeDataRow(row.values, appendValue); err != nil {
				return err
			}
			stats.ProcessedRows++
		}
		if onProgress != nil {
			onProgress(stats)
		}
		buffered = buffered[:0]
		lookupKeys = lookupKeys[:0]
		lookupKeySet = map[string]struct{}{}
		return nil
	}

	headerRead := false
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return stats, err
		}
		columns, err := rows.Columns()
		if err != nil {
			return stats, err
		}
		if !headerRead {
			headers = normalizeHeaders(columns)
			if len(headers) == 0 {
				return stats, errors.New("Excel 表头不能为空")
			}
			for i, header := range headers {
				columnIndexes[header] = i
			}
			for _, filter := range config.Filters {
				if _, ok := columnIndexes[filter.Column]; !ok {
					return stats, fmt.Errorf("Excel 缺少筛选列: %s", filter.Column)
				}
			}
			var ok bool
			matchColumnIndex, ok = columnIndexes[config.MatchExcelColumn]
			if !ok {
				return stats, fmt.Errorf("Excel 缺少订单号列: %s", config.MatchExcelColumn)
			}
			if err := writeHeader(); err != nil {
				return stats, err
			}
			headerRead = true
			continue
		}

		stats.TotalRows++
		normalized := normalizeExcelRow(columns, len(headers))
		eligible := excelRowMatchesFilters(normalized, columnIndexes, config.Filters)
		key := ""
		if eligible {
			stats.FilteredRows++
			key = strings.TrimSpace(normalized[matchColumnIndex])
			if key != "" {
				if _, ok := lookupKeySet[key]; !ok {
					lookupKeys = append(lookupKeys, key)
					lookupKeySet[key] = struct{}{}
				}
			}
		}
		buffered = append(buffered, bufferedRow{
			values:   normalized,
			eligible: eligible,
			key:      key,
		})
		if len(lookupKeys) >= config.BatchSize || len(buffered) >= maxBufferedExcelRows {
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

func initExcelMatchWriter(output *excelize.File) (*excelize.StreamWriter, string, error) {
	defaultSheet := output.GetSheetName(0)
	if err := output.SetSheetName(defaultSheet, defaultExcelResultSheet); err != nil {
		return nil, "", err
	}
	writer, err := output.NewStreamWriter(defaultExcelResultSheet)
	return writer, defaultExcelResultSheet, err
}

func normalizeHeaders(values []string) []string {
	headers := make([]string, len(values))
	for i, value := range values {
		headers[i] = strings.TrimSpace(value)
	}
	return headers
}

func normalizeExcelRow(values []string, width int) []string {
	row := make([]string, width)
	copy(row, values)
	return row
}

func excelRowMatchesFilters(row []string, columnIndexes map[string]int, filters []ExcelMatchFilter) bool {
	for _, filter := range filters {
		index, ok := columnIndexes[filter.Column]
		if !ok || index >= len(row) {
			return false
		}
		if strings.TrimSpace(row[index]) != filter.Value {
			return false
		}
	}
	return true
}

func saveUploadedExcel(fileHeader *multipart.FileHeader, dst string) error {
	src, err := fileHeader.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, src)
	return err
}

func excelMatchJobDir(id uint) string {
	return filepath.Join(os.TempDir(), excelMatchRootDirName, strconv.FormatUint(uint64(id), 10))
}

func isExcelMatchJobExpired(matchJob *model.ExcelMatchJob) bool {
	return matchJob.ExpiresAt != nil && time.Now().After(matchJob.ExpiresAt.Time)
}

func isPathInside(basePath, targetPath string) bool {
	if basePath == "" || targetPath == "" {
		return false
	}
	baseAbs, err := filepath.Abs(basePath)
	if err != nil {
		return false
	}
	targetAbs, err := filepath.Abs(targetPath)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(baseAbs, targetAbs)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel))
}
