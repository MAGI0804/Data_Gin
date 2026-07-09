package data_svc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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
	excelOperationExportMatch  = "export_match"
	excelOperationImportUpdate = "import_update"
	excelOperationClearMatched = "clear_matched_docno"

	excelMatchStatusPending = "pending"
	excelMatchStatusSuccess = "success"
	excelMatchStatusExpired = "expired"

	excelMatchRetention      = 24 * time.Hour
	excelMatchRootDirName    = "data-warehouse-excel-jobs"
	excelUploadRootDirName   = "data-warehouse-excel-uploads"
	excelMatchSourceFileName = "source.xlsx"
	excelMatchResultFileName = "result.xlsx"
	excelUploadChunksDirName = "chunks"
	excelUploadMetaFileName  = "meta.json"
	excelUploadMergedFile    = "source.xlsx"
	excelMaxRowsPerSheet     = 1048576
	defaultExcelMatchBatch   = 1000
	maxBufferedExcelRows     = 5000
	maxExcelUploadChunks     = 10000
	maxExcelUploadChunkBytes = 8 * 1024 * 1024
	defaultExcelSheetName    = "Sheet1"
	defaultExcelResultSheet  = "Result_1"
	bojunRetailOrderTemplate = "bojun_retail_order"
	bojunRetailOrdersTable   = "bojun_retail_orders"
	defaultExcelPreviewRows  = 5000
	defaultExcelPreviewItems = 50
)

var allowedBojunValueFields = map[string]struct{}{
	"billdate":             {},
	"c_store_code":         {},
	"c_store_name":         {},
	"docno":                {},
	"retailbilltype":       {},
	"order_type_code":      {},
	"order_type_name":      {},
	"otherdocno":           {},
	"tot_amt_actual":       {},
	"tot_amt_list":         {},
	"tot_qty":              {},
	"vipno":                {},
	"related_normal_docno": {},
	"o2o_so_docno":         {},
	"matched_docno":        {},
}

var allowedExcelImportTables = map[string]struct{}{
	bojunRetailOrdersTable:   {},
	bojunRetailOrderTemplate: {},
}

var allowedBojunImportMatchFields = map[string]struct{}{
	"docno":                {},
	"otherdocno":           {},
	"o2o_so_docno":         {},
	"related_normal_docno": {},
	"matched_docno":        {},
}

var allowedBojunImportWriteFields = map[string]struct{}{
	"matched_docno": {},
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
	Operation        string             `json:"operation"`
	SheetName        string             `json:"sheetName"`
	Filters          []ExcelMatchFilter `json:"filters"`
	MatchExcelColumn string             `json:"matchExcelColumn"`
	DBTemplate       string             `json:"dbTemplate"`
	DBMatchField     string             `json:"dbMatchField"`
	DBValueField     string             `json:"dbValueField"`
	TableName        string             `json:"tableName"`
	DBWriteField     string             `json:"dbWriteField"`
	WriteExcelColumn string             `json:"writeExcelColumn"`
	OutputColumnName string             `json:"outputColumnName"`
	BatchSize        int                `json:"batchSize"`
	DryRun           bool               `json:"dryRun"`
	ConfirmWrite     bool               `json:"confirmWrite"`
}

type ExcelUploadSession struct {
	UploadID       string `json:"uploadId"`
	FileName       string `json:"fileName"`
	TotalChunks    int    `json:"totalChunks"`
	UploadedChunks int    `json:"uploadedChunks"`
	Complete       bool   `json:"complete"`
	ExpiresAt      string `json:"expiresAt"`
}

type excelUploadMeta struct {
	UploadID       string    `json:"uploadId"`
	FileName       string    `json:"fileName"`
	TotalChunks    int       `json:"totalChunks"`
	UploadedChunks int       `json:"uploadedChunks"`
	Complete       bool      `json:"complete"`
	CreatedAt      time.Time `json:"createdAt"`
	ExpiresAt      time.Time `json:"expiresAt"`
}

type ExcelMatchPreviewResult struct {
	Config      ExcelMatchConfig          `json:"config"`
	Stats       ExcelMatchJobStats        `json:"stats"`
	ScanLimit   int                       `json:"scanLimit"`
	SampleLimit int                       `json:"sampleLimit"`
	Truncated   bool                      `json:"truncated"`
	Samples     []ExcelMatchPreviewSample `json:"samples"`
}

type ExcelMatchPreviewSample struct {
	RowNumber    int               `json:"rowNumber"`
	MatchKey     string            `json:"matchKey"`
	MatchedValue string            `json:"matchedValue"`
	Status       string            `json:"status"`
	Reason       string            `json:"reason"`
	Values       map[string]string `json:"values"`
}

type ExcelMatchJobStats = data_dao.ExcelMatchJobStats

type ExcelMatchLookup interface {
	Lookup(ctx context.Context, matchField string, keys []string, valueField string) (map[string]string, error)
}

type ExcelImportUpdater interface {
	FindKeys(ctx context.Context, matchField string, keys []string) (map[string]struct{}, error)
	UpdateByKey(ctx context.Context, matchField, key, writeField, value string) (int64, error)
}

type bojunExcelMatchLookup struct {
	dao *data_dao.ExcelMatchJobDAO
}

func (l bojunExcelMatchLookup) Lookup(ctx context.Context, matchField string, keys []string, valueField string) (map[string]string, error) {
	return l.dao.FindBojunFieldByKeys(ctx, matchField, keys, valueField)
}

type bojunExcelImportUpdater struct {
	dao *data_dao.ExcelMatchJobDAO
}

func (u bojunExcelImportUpdater) FindKeys(ctx context.Context, matchField string, keys []string) (map[string]struct{}, error) {
	return u.dao.FindBojunKeys(ctx, matchField, keys)
}

func (u bojunExcelImportUpdater) UpdateByKey(ctx context.Context, matchField, key, writeField, value string) (int64, error) {
	return u.dao.UpdateBojunFieldByKey(ctx, matchField, key, writeField, value)
}

type excelJobSource struct {
	fileName string
	save     func(dst string) error
}

func (s *ExcelMatchJobService) CreateJob(ctx context.Context, fileHeader *multipart.FileHeader, rawConfig string) (*model.ExcelMatchJob, error) {
	if fileHeader == nil {
		return nil, errors.New("上传文件不能为空")
	}
	if strings.ToLower(filepath.Ext(fileHeader.Filename)) != ".xlsx" {
		return nil, errors.New("仅支持 .xlsx 文件")
	}
	return s.createJobFromSource(ctx, excelJobSource{
		fileName: filepath.Base(fileHeader.Filename),
		save: func(dst string) error {
			return saveUploadedExcel(fileHeader, dst)
		},
	}, rawConfig)
}

func (s *ExcelMatchJobService) CreateJobFromUpload(ctx context.Context, uploadID, rawConfig string) (*model.ExcelMatchJob, error) {
	meta, sourcePath, err := s.requireCompletedUpload(uploadID)
	if err != nil {
		return nil, err
	}
	return s.createJobFromSource(ctx, excelJobSource{
		fileName: meta.FileName,
		save: func(dst string) error {
			return copyFile(sourcePath, dst)
		},
	}, rawConfig)
}

func (s *ExcelMatchJobService) createJobFromSource(ctx context.Context, source excelJobSource, rawConfig string) (*model.ExcelMatchJob, error) {
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
		SourceFileName: filepath.Base(source.fileName),
		ConfigJSON:     string(configBytes),
		Status:         excelMatchStatusPending,
		ExpiresAt:      &model.TimeNormal{Time: expiresAt},
	}
	if _, err := s.jobDAO.Create(ctx, matchJob); err != nil {
		return nil, err
	}
	s.logJob(ctx, matchJob.ID, "info", "Excel任务已创建", map[string]interface{}{
		"operation": normalizedConfig.Operation,
		"sheet":     normalizedConfig.SheetName,
		"file":      matchJob.SourceFileName,
	})

	workDir := excelMatchJobDir(matchJob.ID)
	sourcePath := filepath.Join(workDir, excelMatchSourceFileName)
	resultPath := filepath.Join(workDir, excelMatchResultFileName)
	if err := os.MkdirAll(workDir, 0700); err != nil {
		_ = s.jobDAO.MarkFailed(ctx, matchJob.ID, err.Error(), expiresAt)
		s.logJob(ctx, matchJob.ID, "error", "创建任务临时目录失败", map[string]interface{}{"error": err.Error()})
		return nil, err
	}
	if err := source.save(sourcePath); err != nil {
		_ = s.jobDAO.MarkFailed(ctx, matchJob.ID, err.Error(), expiresAt)
		_ = os.RemoveAll(workDir)
		s.logJob(ctx, matchJob.ID, "error", "保存上传Excel失败", map[string]interface{}{"error": err.Error()})
		return nil, err
	}
	s.logJob(ctx, matchJob.ID, "info", "上传Excel已保存到临时目录", map[string]interface{}{"source_file_name": matchJob.SourceFileName})
	if err := s.jobDAO.UpdatePaths(ctx, matchJob.ID, workDir, sourcePath, resultPath); err != nil {
		_ = s.jobDAO.MarkFailed(ctx, matchJob.ID, err.Error(), expiresAt)
		_ = os.RemoveAll(workDir)
		s.logJob(ctx, matchJob.ID, "error", "更新任务文件路径失败", map[string]interface{}{"error": err.Error()})
		return nil, err
	}

	task, err := job.NewExcelMatchExportTask(matchJob.ID)
	if err != nil {
		_ = s.jobDAO.MarkFailed(ctx, matchJob.ID, err.Error(), expiresAt)
		s.logJob(ctx, matchJob.ID, "error", "创建异步任务失败", map[string]interface{}{"error": err.Error()})
		return nil, err
	}
	if global.QueueJobClient == nil {
		err := errors.New("异步任务客户端未初始化")
		_ = s.jobDAO.MarkFailed(ctx, matchJob.ID, err.Error(), expiresAt)
		s.logJob(ctx, matchJob.ID, "error", "异步任务客户端未初始化", nil)
		return nil, err
	}
	if _, err := global.QueueJobClient.Enqueue(task, asynq.MaxRetry(1)); err != nil {
		_ = s.jobDAO.MarkFailed(ctx, matchJob.ID, err.Error(), expiresAt)
		s.logJob(ctx, matchJob.ID, "error", "投递异步任务失败", map[string]interface{}{"error": err.Error()})
		return nil, err
	}
	s.logJob(ctx, matchJob.ID, "info", "异步任务已投递", nil)

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

func (s *ExcelMatchJobService) GetJobLogs(ctx context.Context, id uint) ([]model.ExcelMatchJobLog, error) {
	return s.jobDAO.FindLogsByJobID(ctx, id, 200)
}

func (s *ExcelMatchJobService) CreateUploadSession(ctx context.Context, fileName string, totalChunks int) (*ExcelUploadSession, error) {
	if strings.ToLower(filepath.Ext(fileName)) != ".xlsx" {
		return nil, errors.New("仅支持 .xlsx 文件")
	}
	if totalChunks <= 0 || totalChunks > maxExcelUploadChunks {
		return nil, fmt.Errorf("分片数量必须在 1 到 %d 之间", maxExcelUploadChunks)
	}
	uploadID, err := newExcelUploadID()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	meta := excelUploadMeta{
		UploadID:       uploadID,
		FileName:       filepath.Base(fileName),
		TotalChunks:    totalChunks,
		UploadedChunks: 0,
		Complete:       false,
		CreatedAt:      now,
		ExpiresAt:      now.Add(excelMatchRetention),
	}
	if err := os.MkdirAll(excelUploadChunksDir(uploadID), 0700); err != nil {
		return nil, err
	}
	if err := writeExcelUploadMeta(uploadID, meta); err != nil {
		_ = os.RemoveAll(excelUploadDir(uploadID))
		return nil, err
	}
	_ = ctx
	return excelUploadSessionFromMeta(meta), nil
}

func (s *ExcelMatchJobService) SaveUploadChunk(ctx context.Context, uploadID string, index int, totalChunks int, fileHeader *multipart.FileHeader) (*ExcelUploadSession, error) {
	if fileHeader == nil {
		return nil, errors.New("分片文件不能为空")
	}
	if fileHeader.Size > maxExcelUploadChunkBytes {
		return nil, fmt.Errorf("单个分片不能超过 %d MB", maxExcelUploadChunkBytes/(1024*1024))
	}
	meta, err := readExcelUploadMeta(uploadID)
	if err != nil {
		return nil, err
	}
	if time.Now().After(meta.ExpiresAt) {
		_ = os.RemoveAll(excelUploadDir(uploadID))
		return nil, errors.New("上传会话已过期")
	}
	if totalChunks != meta.TotalChunks {
		return nil, errors.New("分片总数与上传会话不一致")
	}
	if index < 0 || index >= meta.TotalChunks {
		return nil, errors.New("分片序号超出范围")
	}

	dst := filepath.Join(excelUploadChunksDir(uploadID), excelUploadChunkName(index))
	if !isPathInside(excelUploadDir(uploadID), dst) {
		return nil, errors.New("分片路径非法")
	}
	if err := saveUploadedExcel(fileHeader, dst); err != nil {
		return nil, err
	}
	uploaded, err := countExcelUploadChunks(uploadID, meta.TotalChunks)
	if err != nil {
		return nil, err
	}
	meta.UploadedChunks = uploaded
	if err := writeExcelUploadMeta(uploadID, meta); err != nil {
		return nil, err
	}
	_ = ctx
	return excelUploadSessionFromMeta(meta), nil
}

func (s *ExcelMatchJobService) CompleteUpload(ctx context.Context, uploadID string, totalChunks int) (*ExcelUploadSession, error) {
	meta, err := readExcelUploadMeta(uploadID)
	if err != nil {
		return nil, err
	}
	if time.Now().After(meta.ExpiresAt) {
		_ = os.RemoveAll(excelUploadDir(uploadID))
		return nil, errors.New("上传会话已过期")
	}
	if totalChunks != meta.TotalChunks {
		return nil, errors.New("分片总数与上传会话不一致")
	}
	if err := assembleExcelUpload(uploadID, meta.TotalChunks); err != nil {
		return nil, err
	}
	meta.Complete = true
	meta.UploadedChunks = meta.TotalChunks
	if err := writeExcelUploadMeta(uploadID, meta); err != nil {
		return nil, err
	}
	_ = ctx
	return excelUploadSessionFromMeta(meta), nil
}

func (s *ExcelMatchJobService) Preview(ctx context.Context, fileHeader *multipart.FileHeader, rawConfig string) (*ExcelMatchPreviewResult, error) {
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
	config.Operation = excelOperationExportMatch
	normalizedConfig, err := normalizeExcelMatchConfig(config)
	if err != nil {
		return nil, err
	}

	src, err := fileHeader.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()

	input, err := excelize.OpenReader(src)
	if err != nil {
		return nil, err
	}
	defer func() { _ = input.Close() }()

	lookup := bojunExcelMatchLookup{dao: s.jobDAO}
	return processExcelMatchPreview(ctx, input, normalizedConfig, lookup, defaultExcelPreviewRows, defaultExcelPreviewItems)
}

func (s *ExcelMatchJobService) PreviewUploaded(ctx context.Context, uploadID, rawConfig string) (*ExcelMatchPreviewResult, error) {
	_, sourcePath, err := s.requireCompletedUpload(uploadID)
	if err != nil {
		return nil, err
	}
	var config ExcelMatchConfig
	if err := json.Unmarshal([]byte(rawConfig), &config); err != nil {
		return nil, fmt.Errorf("解析匹配配置失败: %w", err)
	}
	config.Operation = excelOperationExportMatch
	normalizedConfig, err := normalizeExcelMatchConfig(config)
	if err != nil {
		return nil, err
	}
	input, err := excelize.OpenFile(sourcePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = input.Close() }()

	lookup := bojunExcelMatchLookup{dao: s.jobDAO}
	return processExcelMatchPreview(ctx, input, normalizedConfig, lookup, defaultExcelPreviewRows, defaultExcelPreviewItems)
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
		s.logJob(ctx, id, "error", "任务配置校验失败", map[string]interface{}{"error": err.Error()})
		return err
	}

	if err := s.jobDAO.MarkRunning(ctx, id); err != nil {
		return err
	}
	s.logJob(ctx, id, "info", "任务开始处理", map[string]interface{}{
		"operation": config.Operation,
		"sheet":     config.SheetName,
	})

	var stats ExcelMatchJobStats
	if config.Operation == excelOperationImportUpdate || config.Operation == excelOperationClearMatched {
		updater := bojunExcelImportUpdater{dao: s.jobDAO}
		stats, err = processExcelImportUpdateFileWithProgress(ctx, matchJob.SourceFilePath, config, updater, func(stats ExcelMatchJobStats) {
			_ = s.jobDAO.UpdateProgress(ctx, id, stats)
			s.logJob(ctx, id, "info", "导入匹配进度", statsDetail(stats))
		})
	} else {
		lookup := bojunExcelMatchLookup{dao: s.jobDAO}
		stats, err = processExcelMatchFileWithProgress(ctx, matchJob.SourceFilePath, matchJob.ResultFilePath, config, lookup, func(stats ExcelMatchJobStats) {
			_ = s.jobDAO.UpdateProgress(ctx, id, stats)
			s.logJob(ctx, id, "info", "匹配导出进度", statsDetail(stats))
		})
	}
	expiresAt := time.Now().Add(excelMatchRetention)
	if err != nil {
		_ = s.jobDAO.MarkFailed(ctx, id, err.Error(), expiresAt)
		s.logJob(ctx, id, "error", "任务处理失败", map[string]interface{}{"error": err.Error()})
		return err
	}

	_ = os.Remove(matchJob.SourceFilePath)
	s.logJob(ctx, id, "info", "源文件已删除", nil)
	s.logJob(ctx, id, "info", "任务处理成功", statsDetail(stats))
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
	return s.CleanupExpiredUploads(ctx)
}

func (s *ExcelMatchJobService) CleanupExpiredUploads(ctx context.Context) error {
	root := excelUploadRootDir()
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	now := time.Now()
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !entry.IsDir() || !isValidExcelUploadID(entry.Name()) {
			continue
		}
		meta, err := readExcelUploadMeta(entry.Name())
		if err != nil || now.After(meta.ExpiresAt) {
			_ = os.RemoveAll(excelUploadDir(entry.Name()))
		}
	}
	return nil
}

func (s *ExcelMatchJobService) requireCompletedUpload(uploadID string) (excelUploadMeta, string, error) {
	meta, err := readExcelUploadMeta(uploadID)
	if err != nil {
		return excelUploadMeta{}, "", err
	}
	if time.Now().After(meta.ExpiresAt) {
		_ = os.RemoveAll(excelUploadDir(uploadID))
		return excelUploadMeta{}, "", errors.New("上传会话已过期")
	}
	if !meta.Complete {
		return excelUploadMeta{}, "", errors.New("上传会话尚未合并完成")
	}
	sourcePath := filepath.Join(excelUploadDir(uploadID), excelUploadMergedFile)
	if !isPathInside(excelUploadDir(uploadID), sourcePath) {
		return excelUploadMeta{}, "", errors.New("上传文件路径非法")
	}
	if _, err := os.Stat(sourcePath); err != nil {
		return excelUploadMeta{}, "", err
	}
	return meta, sourcePath, nil
}

func normalizeExcelMatchConfig(config ExcelMatchConfig) (ExcelMatchConfig, error) {
	config.Operation = strings.TrimSpace(config.Operation)
	config.SheetName = strings.TrimSpace(config.SheetName)
	config.MatchExcelColumn = strings.TrimSpace(config.MatchExcelColumn)
	config.DBTemplate = strings.TrimSpace(config.DBTemplate)
	config.DBMatchField = strings.TrimSpace(config.DBMatchField)
	config.DBValueField = strings.TrimSpace(config.DBValueField)
	config.TableName = strings.TrimSpace(config.TableName)
	config.DBWriteField = strings.TrimSpace(config.DBWriteField)
	config.WriteExcelColumn = strings.TrimSpace(config.WriteExcelColumn)
	config.OutputColumnName = strings.TrimSpace(config.OutputColumnName)

	if config.Operation == "" {
		config.Operation = excelOperationExportMatch
	}
	if config.SheetName == "" {
		config.SheetName = defaultExcelSheetName
	}
	if config.BatchSize == 0 {
		config.BatchSize = defaultExcelMatchBatch
	}
	if config.BatchSize < 500 || config.BatchSize > 2000 {
		return config, errors.New("batchSize 必须在 500 到 2000 之间")
	}

	switch config.Operation {
	case excelOperationExportMatch:
		return normalizeExcelExportConfig(config)
	case excelOperationImportUpdate, excelOperationClearMatched:
		return normalizeExcelImportConfig(config)
	default:
		return config, fmt.Errorf("暂不支持Excel任务操作: %s", config.Operation)
	}
}

func normalizeExcelExportConfig(config ExcelMatchConfig) (ExcelMatchConfig, error) {
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
	if _, ok := allowedBojunImportMatchFields[config.DBMatchField]; !ok {
		return config, fmt.Errorf("伯俊匹配字段不在白名单: %s", config.DBMatchField)
	}
	if _, ok := allowedBojunValueFields[config.DBValueField]; !ok {
		return config, fmt.Errorf("伯俊取值字段不在白名单: %s", config.DBValueField)
	}
	if config.OutputColumnName == "" {
		return config, errors.New("输出列名不能为空")
	}

	return config, nil
}

func normalizeExcelImportConfig(config ExcelMatchConfig) (ExcelMatchConfig, error) {
	if config.TableName == "" {
		config.TableName = bojunRetailOrdersTable
	}
	if _, ok := allowedExcelImportTables[config.TableName]; !ok {
		return config, fmt.Errorf("导入匹配表不在白名单: %s", config.TableName)
	}
	config.TableName = bojunRetailOrdersTable
	if config.DBMatchField == "" {
		config.DBMatchField = "docno"
	}
	if _, ok := allowedBojunImportMatchFields[config.DBMatchField]; !ok {
		return config, fmt.Errorf("导入匹配字段不在白名单: %s", config.DBMatchField)
	}
	if config.MatchExcelColumn == "" {
		return config, errors.New("匹配Excel列名不能为空")
	}
	if config.DBWriteField == "" {
		config.DBWriteField = "matched_docno"
	}
	if _, ok := allowedBojunImportWriteFields[config.DBWriteField]; !ok {
		return config, fmt.Errorf("导入写入字段不在白名单: %s", config.DBWriteField)
	}
	if config.Operation == excelOperationClearMatched {
		config.DBWriteField = "matched_docno"
		config.WriteExcelColumn = ""
	} else if config.WriteExcelColumn == "" {
		return config, errors.New("写入值Excel列名不能为空")
	}
	config.DryRun = !config.ConfirmWrite
	return config, nil
}

func processExcelMatchFile(ctx context.Context, inputPath, outputPath string, config ExcelMatchConfig, lookup ExcelMatchLookup) (ExcelMatchJobStats, error) {
	return processExcelMatchFileWithProgress(ctx, inputPath, outputPath, config, lookup, nil)
}

func processExcelMatchPreview(
	ctx context.Context,
	input *excelize.File,
	config ExcelMatchConfig,
	lookup ExcelMatchLookup,
	scanLimit int,
	sampleLimit int,
) (*ExcelMatchPreviewResult, error) {
	if scanLimit <= 0 || scanLimit > defaultExcelPreviewRows {
		scanLimit = defaultExcelPreviewRows
	}
	if sampleLimit <= 0 || sampleLimit > defaultExcelPreviewItems {
		sampleLimit = defaultExcelPreviewItems
	}

	sheets := input.GetSheetList()
	if len(sheets) == 0 {
		return nil, errors.New("Excel 没有可读取的 sheet")
	}
	if !sheetExists(sheets, config.SheetName) {
		return nil, fmt.Errorf("Excel 不存在 sheet: %s", config.SheetName)
	}
	rows, err := input.Rows(config.SheetName)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := &ExcelMatchPreviewResult{
		Config:      config,
		ScanLimit:   scanLimit,
		SampleLimit: sampleLimit,
	}
	var headers []string
	columnIndexes := map[string]int{}
	matchColumnIndex := -1

	type previewRow struct {
		rowNumber int
		values    []string
		eligible  bool
		key       string
	}
	var buffered []previewRow
	var lookupKeys []string
	lookupKeySet := map[string]struct{}{}

	addSample := func(row previewRow, matchedValue, status, reason string) {
		if len(result.Samples) >= sampleLimit {
			return
		}
		values := make(map[string]string, len(headers))
		for i, header := range headers {
			if i < len(row.values) {
				values[header] = row.values[i]
			} else {
				values[header] = ""
			}
		}
		result.Samples = append(result.Samples, ExcelMatchPreviewSample{
			RowNumber:    row.rowNumber,
			MatchKey:     row.key,
			MatchedValue: matchedValue,
			Status:       status,
			Reason:       reason,
			Values:       values,
		})
	}

	flush := func() error {
		if len(buffered) == 0 {
			return nil
		}
		matches := map[string]string{}
		if len(lookupKeys) > 0 {
			var err error
			matches, err = lookup.Lookup(ctx, config.DBMatchField, lookupKeys, config.DBValueField)
			if err != nil {
				return err
			}
		}
		for _, row := range buffered {
			if err := ctx.Err(); err != nil {
				return err
			}
			status := "skipped"
			reason := "未命中前置筛选"
			matchedValue := ""
			if row.eligible {
				if row.key == "" {
					status = "unmatched"
					reason = "匹配键为空"
					result.Stats.UnmatchedRows++
				} else if value, ok := matches[row.key]; ok {
					status = "matched"
					reason = "已匹配"
					matchedValue = value
					result.Stats.MatchedRows++
				} else {
					status = "unmatched"
					reason = "数据库无匹配记录"
					result.Stats.UnmatchedRows++
				}
			}
			addSample(row, matchedValue, status, reason)
			result.Stats.ProcessedRows++
		}
		buffered = buffered[:0]
		lookupKeys = lookupKeys[:0]
		lookupKeySet = map[string]struct{}{}
		return nil
	}

	headerRead := false
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		columns, err := rows.Columns()
		if err != nil {
			return result, err
		}
		if !headerRead {
			headers = normalizeHeaders(columns)
			if len(headers) == 0 {
				return result, errors.New("Excel 表头不能为空")
			}
			for i, header := range headers {
				columnIndexes[header] = i
			}
			for _, filter := range config.Filters {
				if _, ok := columnIndexes[filter.Column]; !ok {
					return result, fmt.Errorf("Excel 缺少筛选列: %s", filter.Column)
				}
			}
			var ok bool
			matchColumnIndex, ok = columnIndexes[config.MatchExcelColumn]
			if !ok {
				return result, fmt.Errorf("Excel 缺少订单号列: %s", config.MatchExcelColumn)
			}
			headerRead = true
			continue
		}

		result.Stats.TotalRows++
		normalized := normalizeExcelRow(columns, len(headers))
		eligible := excelRowMatchesFilters(normalized, columnIndexes, config.Filters)
		key := ""
		if eligible {
			result.Stats.FilteredRows++
			key = strings.TrimSpace(normalized[matchColumnIndex])
			if key != "" {
				if _, ok := lookupKeySet[key]; !ok {
					lookupKeys = append(lookupKeys, key)
					lookupKeySet[key] = struct{}{}
				}
			}
		}
		buffered = append(buffered, previewRow{
			rowNumber: result.Stats.TotalRows + 1,
			values:    normalized,
			eligible:  eligible,
			key:       key,
		})
		if len(lookupKeys) >= config.BatchSize || len(buffered) >= maxBufferedExcelRows {
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
	if !sheetExists(sheets, config.SheetName) {
		return ExcelMatchJobStats{}, fmt.Errorf("Excel 不存在 sheet: %s", config.SheetName)
	}
	rows, err := input.Rows(config.SheetName)
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
			matches, err = lookup.Lookup(ctx, config.DBMatchField, lookupKeys, config.DBValueField)
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

func processExcelImportUpdateFileWithProgress(
	ctx context.Context,
	inputPath string,
	config ExcelMatchConfig,
	updater ExcelImportUpdater,
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
	if !sheetExists(sheets, config.SheetName) {
		return ExcelMatchJobStats{}, fmt.Errorf("Excel 不存在 sheet: %s", config.SheetName)
	}
	rows, err := input.Rows(config.SheetName)
	if err != nil {
		return ExcelMatchJobStats{}, err
	}
	defer func() { _ = rows.Close() }()

	var headers []string
	columnIndexes := map[string]int{}
	matchColumnIndex := -1
	writeColumnIndex := -1
	stats := ExcelMatchJobStats{}

	type importRow struct {
		key   string
		value string
	}
	var buffered []importRow

	flush := func() error {
		if len(buffered) == 0 {
			return nil
		}
		keys := make([]string, 0, len(buffered))
		for _, row := range buffered {
			keys = append(keys, row.key)
		}
		existing, err := updater.FindKeys(ctx, config.DBMatchField, keys)
		if err != nil {
			return err
		}
		for _, row := range buffered {
			if _, ok := existing[row.key]; ok {
				stats.MatchedRows++
				if !config.DryRun {
					if _, err := updater.UpdateByKey(ctx, config.DBMatchField, row.key, config.DBWriteField, row.value); err != nil {
						return err
					}
				}
			} else {
				stats.UnmatchedRows++
			}
		}
		stats.ProcessedRows += len(buffered)
		if onProgress != nil {
			onProgress(stats)
		}
		buffered = buffered[:0]
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
			var ok bool
			matchColumnIndex, ok = columnIndexes[config.MatchExcelColumn]
			if !ok {
				return stats, fmt.Errorf("Excel 缺少匹配列: %s", config.MatchExcelColumn)
			}
			if config.Operation != excelOperationClearMatched {
				writeColumnIndex, ok = columnIndexes[config.WriteExcelColumn]
				if !ok {
					return stats, fmt.Errorf("Excel 缺少写入值列: %s", config.WriteExcelColumn)
				}
			}
			headerRead = true
			continue
		}

		stats.TotalRows++
		normalized := normalizeExcelRow(columns, len(headers))
		key := strings.TrimSpace(normalized[matchColumnIndex])
		value := ""
		if config.Operation != excelOperationClearMatched {
			value = strings.TrimSpace(normalized[writeColumnIndex])
		}
		if key == "" {
			stats.UnmatchedRows++
			stats.ProcessedRows++
			continue
		}
		buffered = append(buffered, importRow{key: key, value: value})
		if len(buffered) >= config.BatchSize {
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

func sheetExists(sheets []string, target string) bool {
	for _, sheet := range sheets {
		if sheet == target {
			return true
		}
	}
	return false
}

func (s *ExcelMatchJobService) logJob(ctx context.Context, jobID uint, level, message string, detail map[string]interface{}) {
	if jobID == 0 {
		return
	}
	detailJSON := "{}"
	if detail != nil {
		if data, err := json.Marshal(detail); err == nil {
			detailJSON = string(data)
		}
	}
	_ = s.jobDAO.CreateLog(ctx, &model.ExcelMatchJobLog{
		JobID:      jobID,
		Level:      level,
		Message:    message,
		DetailJSON: detailJSON,
	})
}

func statsDetail(stats ExcelMatchJobStats) map[string]interface{} {
	return map[string]interface{}{
		"total_rows":     stats.TotalRows,
		"processed_rows": stats.ProcessedRows,
		"filtered_rows":  stats.FilteredRows,
		"matched_rows":   stats.MatchedRows,
		"unmatched_rows": stats.UnmatchedRows,
	}
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

func copyFile(srcPath, dstPath string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)
	return err
}

func excelMatchJobDir(id uint) string {
	return filepath.Join(os.TempDir(), excelMatchRootDirName, strconv.FormatUint(uint64(id), 10))
}

func excelUploadRootDir() string {
	return filepath.Join(os.TempDir(), excelUploadRootDirName)
}

func excelUploadDir(uploadID string) string {
	return filepath.Join(excelUploadRootDir(), uploadID)
}

func excelUploadChunksDir(uploadID string) string {
	return filepath.Join(excelUploadDir(uploadID), excelUploadChunksDirName)
}

func excelUploadMetaPath(uploadID string) string {
	return filepath.Join(excelUploadDir(uploadID), excelUploadMetaFileName)
}

func excelUploadChunkName(index int) string {
	return fmt.Sprintf("%06d.part", index)
}

func newExcelUploadID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func isValidExcelUploadID(uploadID string) bool {
	if len(uploadID) != 32 {
		return false
	}
	for _, ch := range uploadID {
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') {
			return false
		}
	}
	return true
}

func writeExcelUploadMeta(uploadID string, meta excelUploadMeta) error {
	if !isValidExcelUploadID(uploadID) || uploadID != meta.UploadID {
		return errors.New("上传会话ID非法")
	}
	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	return os.WriteFile(excelUploadMetaPath(uploadID), data, 0600)
}

func readExcelUploadMeta(uploadID string) (excelUploadMeta, error) {
	if !isValidExcelUploadID(uploadID) {
		return excelUploadMeta{}, errors.New("上传会话ID非法")
	}
	metaPath := excelUploadMetaPath(uploadID)
	if !isPathInside(excelUploadDir(uploadID), metaPath) {
		return excelUploadMeta{}, errors.New("上传会话路径非法")
	}
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return excelUploadMeta{}, err
	}
	var meta excelUploadMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return excelUploadMeta{}, err
	}
	if meta.UploadID != uploadID {
		return excelUploadMeta{}, errors.New("上传会话元数据不一致")
	}
	return meta, nil
}

func countExcelUploadChunks(uploadID string, totalChunks int) (int, error) {
	count := 0
	for i := 0; i < totalChunks; i++ {
		chunkPath := filepath.Join(excelUploadChunksDir(uploadID), excelUploadChunkName(i))
		if !isPathInside(excelUploadDir(uploadID), chunkPath) {
			return 0, errors.New("分片路径非法")
		}
		if _, err := os.Stat(chunkPath); err == nil {
			count++
		} else if !os.IsNotExist(err) {
			return 0, err
		}
	}
	return count, nil
}

func assembleExcelUpload(uploadID string, totalChunks int) error {
	mergedPath := filepath.Join(excelUploadDir(uploadID), excelUploadMergedFile)
	if !isPathInside(excelUploadDir(uploadID), mergedPath) {
		return errors.New("合并文件路径非法")
	}
	out, err := os.OpenFile(mergedPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer out.Close()

	for i := 0; i < totalChunks; i++ {
		chunkPath := filepath.Join(excelUploadChunksDir(uploadID), excelUploadChunkName(i))
		if !isPathInside(excelUploadDir(uploadID), chunkPath) {
			return errors.New("分片路径非法")
		}
		in, err := os.Open(chunkPath)
		if err != nil {
			return fmt.Errorf("读取分片 %d 失败: %w", i, err)
		}
		if _, err := io.Copy(out, in); err != nil {
			_ = in.Close()
			return err
		}
		if err := in.Close(); err != nil {
			return err
		}
	}
	return nil
}

func excelUploadSessionFromMeta(meta excelUploadMeta) *ExcelUploadSession {
	return &ExcelUploadSession{
		UploadID:       meta.UploadID,
		FileName:       meta.FileName,
		TotalChunks:    meta.TotalChunks,
		UploadedChunks: meta.UploadedChunks,
		Complete:       meta.Complete,
		ExpiresAt:      meta.ExpiresAt.Format(time.RFC3339),
	}
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
