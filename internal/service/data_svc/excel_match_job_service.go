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
	"sync"
	"time"

	"gin-biz-web-api/global"
	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/job"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/storage"

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

	excelMatchRetention               = 24 * time.Hour
	excelMatchRootDirName             = "data-warehouse-excel-jobs"
	excelUploadRootDirName            = "data-warehouse-excel-uploads"
	excelPreviewRootDirName           = "data-warehouse-excel-previews"
	excelizeTempDirName               = "excelize-tmp"
	excelTempRootEnvName              = "DATA_WAREHOUSE_EXCEL_TEMP_DIR"
	excelMatchSourceFileName          = "source.xlsx"
	excelMatchResultFileName          = "result.xlsx"
	excelUploadChunksDirName          = "chunks"
	excelUploadMetaFileName           = "meta.json"
	excelUploadMergedFile             = "source.xlsx"
	excelMaxRowsPerSheet              = 1048576
	defaultExcelMatchBatch            = 1000
	maxBufferedExcelRows              = 5000
	maxExcelUploadChunks              = 10000
	maxExcelUploadChunkBytes          = 8 * 1024 * 1024
	defaultExcelSheetName             = "Sheet1"
	defaultExcelResultSheet           = "Result_1"
	bojunRetailOrderTemplate          = "bojun_retail_order"
	bojunRetailOrdersTable            = "bojun_retail_orders"
	defaultExcelPreviewRows           = 5000
	defaultExcelPreviewItems          = 50
	excelMatchOSSUploadTimeoutEnvName = "EXCEL_MATCH_OSS_UPLOAD_TIMEOUT_SECONDS"
	excelMatchOSSUploadMinTimeout     = 10 * time.Minute
	excelMatchOSSUploadMaxTimeout     = 2 * time.Hour
	excelMatchOSSUploadBytesPerMinute = 32 * 1024 * 1024
	excelMatchOSSProgressBytes        = 32 * 1024 * 1024
	excelMatchOSSProgressInterval     = 15 * time.Second
)

var excelizeTempMu sync.Mutex

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

type ExcelMatchScheme struct {
	ID         uint             `json:"id"`
	Name       string           `json:"name"`
	Operation  string           `json:"operation"`
	Config     ExcelMatchConfig `json:"config"`
	ConfigJSON string           `json:"config_json"`
	CreatedAt  int              `json:"created_at"`
	UpdatedAt  int              `json:"updated_at"`
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
	s.refreshExpiredJob(ctx, matchJob)
	s.refreshDownloadState(ctx, matchJob)
	return matchJob, nil
}

func (s *ExcelMatchJobService) GetJobLogs(ctx context.Context, id uint) ([]model.ExcelMatchJobLog, error) {
	return s.jobDAO.FindLogsByJobID(ctx, id, 200)
}

func (s *ExcelMatchJobService) ListJobs(ctx context.Context, limit int) ([]model.ExcelMatchJob, error) {
	jobs, err := s.jobDAO.ListJobs(ctx, limit)
	if err != nil {
		return nil, err
	}
	for i := range jobs {
		s.refreshExpiredJob(ctx, &jobs[i])
		s.refreshDownloadState(ctx, &jobs[i])
	}
	return jobs, nil
}

func (s *ExcelMatchJobService) ListSchemes(ctx context.Context, operation string) ([]ExcelMatchScheme, error) {
	operation = strings.TrimSpace(operation)
	if operation != "" && operation != excelOperationExportMatch && operation != excelOperationImportUpdate {
		return nil, fmt.Errorf("暂不支持方案类型: %s", operation)
	}
	rows, err := s.jobDAO.ListSchemes(ctx, operation)
	if err != nil {
		return nil, err
	}
	schemes := make([]ExcelMatchScheme, 0, len(rows))
	for _, row := range rows {
		scheme, err := excelMatchSchemeFromModel(row)
		if err != nil {
			return nil, err
		}
		schemes = append(schemes, scheme)
	}
	return schemes, nil
}

func (s *ExcelMatchJobService) SaveScheme(ctx context.Context, name, operation, rawConfig string) (*ExcelMatchScheme, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("方案名称不能为空")
	}
	if len([]rune(name)) > 100 {
		return nil, errors.New("方案名称不能超过 100 个字符")
	}
	operation = strings.TrimSpace(operation)
	if operation != excelOperationExportMatch && operation != excelOperationImportUpdate {
		return nil, fmt.Errorf("暂不支持方案类型: %s", operation)
	}
	var config ExcelMatchConfig
	if err := json.Unmarshal([]byte(rawConfig), &config); err != nil {
		return nil, fmt.Errorf("解析方案配置失败: %w", err)
	}
	config.Operation = operation
	normalizedConfig, err := normalizeExcelMatchConfig(config)
	if err != nil {
		return nil, err
	}
	configBytes, err := json.Marshal(normalizedConfig)
	if err != nil {
		return nil, err
	}
	row := &model.ExcelMatchScheme{
		Name:       name,
		Operation:  operation,
		ConfigJSON: string(configBytes),
	}
	if err := s.jobDAO.SaveScheme(ctx, row); err != nil {
		return nil, err
	}
	rows, err := s.jobDAO.ListSchemes(ctx, operation)
	if err != nil {
		return nil, err
	}
	for _, item := range rows {
		if item.Name == name {
			scheme, err := excelMatchSchemeFromModel(item)
			if err != nil {
				return nil, err
			}
			return &scheme, nil
		}
	}
	scheme, err := excelMatchSchemeFromModel(*row)
	if err != nil {
		return nil, err
	}
	return &scheme, nil
}

func (s *ExcelMatchJobService) DeleteScheme(ctx context.Context, id uint) error {
	if id == 0 {
		return errors.New("方案ID不能为空")
	}
	return s.jobDAO.DeleteScheme(ctx, id)
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

	lookup := bojunExcelMatchLookup{dao: s.jobDAO}
	var result *ExcelMatchPreviewResult
	previewID, err := newExcelUploadID()
	if err != nil {
		return nil, err
	}
	tempDir := filepath.Join(excelPreviewRootDir(), previewID, excelizeTempDirName)
	defer func() { _ = os.RemoveAll(filepath.Dir(tempDir)) }()
	err = withExcelizeTempDir(tempDir, func() error {
		input, err := excelize.OpenReader(src)
		if err != nil {
			return err
		}
		defer func() { _ = input.Close() }()
		result, err = processExcelMatchPreview(ctx, input, normalizedConfig, lookup, defaultExcelPreviewRows, defaultExcelPreviewItems)
		return err
	})
	return result, err
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
	lookup := bojunExcelMatchLookup{dao: s.jobDAO}
	var result *ExcelMatchPreviewResult
	err = withExcelizeTempDir(excelizeTempDirForPath(sourcePath), func() error {
		input, err := excelize.OpenFile(sourcePath)
		if err != nil {
			return err
		}
		defer func() { _ = input.Close() }()
		result, err = processExcelMatchPreview(ctx, input, normalizedConfig, lookup, defaultExcelPreviewRows, defaultExcelPreviewItems)
		return err
	})
	return result, err
}

func (s *ExcelMatchJobService) Download(ctx context.Context, id uint) (*model.ExcelMatchJob, string, string, error) {
	matchJob, err := s.GetJob(ctx, id)
	if err != nil {
		return nil, "", "", err
	}
	if matchJob.Status != excelMatchStatusSuccess {
		return nil, "", "", fmt.Errorf("任务状态为 %s，无法下载", matchJob.Status)
	}
	if excelJobOperationFromConfigJSON(matchJob.ConfigJSON) != excelOperationExportMatch {
		return nil, "", "", errors.New("该任务不是匹配导出任务，不会生成结果文件")
	}
	if strings.TrimSpace(matchJob.ResultURL) != "" {
		return matchJob, "", fmt.Sprintf("excel_match_job_%d.xlsx", matchJob.ID), nil
	}
	if !isPathInside(excelMatchJobDir(matchJob.ID), matchJob.ResultFilePath) {
		return nil, "", "", errors.New("结果文件路径非法")
	}
	if _, err := os.Stat(matchJob.ResultFilePath); err != nil {
		if os.IsNotExist(err) {
			if s.jobDAO != nil {
				_ = s.jobDAO.MarkExpired(ctx, matchJob.ID)
			}
			s.logJob(ctx, matchJob.ID, "warn", "结果文件不存在，任务已标记为过期", nil)
			return nil, "", "", errors.New("结果文件不存在或已被清理，请重新创建匹配导出任务")
		}
		return nil, "", "", errors.New("读取结果文件失败，请稍后重试")
	}
	return nil, "", "", errors.New("结果文件尚未上传到OSS，上传成功后才能下载，请稍后刷新任务状态")
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
	excelTempDir := filepath.Join(matchJob.WorkDir, excelizeTempDirName)
	s.logJob(ctx, id, "info", "Excel临时目录已设置", map[string]interface{}{"temp_dir": excelTempDir})
	err = withExcelizeTempDir(excelTempDir, func() error {
		if config.Operation == excelOperationImportUpdate || config.Operation == excelOperationClearMatched {
			updater := bojunExcelImportUpdater{dao: s.jobDAO}
			var processErr error
			stats, processErr = processExcelImportUpdateFileWithProgress(ctx, matchJob.SourceFilePath, config, updater, func(stats ExcelMatchJobStats) {
				_ = s.jobDAO.UpdateProgress(ctx, id, stats)
				s.logJob(ctx, id, "info", "导入匹配进度", statsDetail(stats))
			})
			return processErr
		}
		lookup := bojunExcelMatchLookup{dao: s.jobDAO}
		var processErr error
		stats, processErr = processExcelMatchFileWithProgress(ctx, matchJob.SourceFilePath, matchJob.ResultFilePath, config, lookup, func(stats ExcelMatchJobStats) {
			_ = s.jobDAO.UpdateProgress(ctx, id, stats)
			s.logJob(ctx, id, "info", "匹配导出进度", statsDetail(stats))
		})
		return processErr
	})
	expiresAt := time.Now().Add(excelMatchRetention)
	if err != nil {
		_ = s.jobDAO.MarkFailed(ctx, id, err.Error(), expiresAt)
		s.logJob(ctx, id, "error", "任务处理失败", map[string]interface{}{"error": err.Error()})
		return err
	}
	_ = os.Remove(matchJob.SourceFilePath)
	s.logJob(ctx, id, "info", "源文件已删除", nil)

	if config.Operation == excelOperationExportMatch && storage.OSSStorageEnabled() {
		fileSize := fileSizeOrZero(matchJob.ResultFilePath)
		uploadTimeout := excelMatchOSSUploadTimeout(fileSize)
		objectKey := excelMatchResultObjectKey(id, time.Now())
		s.logJob(ctx, id, "info", "Excel匹配完成，开始上传结果文件到OSS", map[string]interface{}{
			"object_key":      objectKey,
			"file_size":       fileSize,
			"timeout_seconds": int(uploadTimeout.Seconds()),
		})
		result, err := s.uploadExcelResultToOSS(ctx, matchJob, objectKey, uploadTimeout)
		if err != nil {
			errorMessage := fmt.Sprintf("上传结果文件到OSS失败: %v", err)
			_ = s.jobDAO.MarkFailed(ctx, id, errorMessage, expiresAt)
			s.logJob(ctx, id, "error", "上传结果文件到OSS失败", map[string]interface{}{
				"object_key":      objectKey,
				"file_size":       fileSize,
				"timeout_seconds": int(uploadTimeout.Seconds()),
				"error":           err.Error(),
			})
			return nil
		}
		s.logJob(ctx, id, "info", "结果文件已上传OSS", map[string]interface{}{
			"object_key": result.ObjectKey,
			"result_url": result.URL,
			"file_size":  fileSize,
		})
		_ = os.Remove(matchJob.ResultFilePath)
	}
	if err := s.jobDAO.MarkSuccess(ctx, id, stats, expiresAt); err != nil {
		s.logJob(ctx, id, "error", "更新任务成功状态失败", map[string]interface{}{"error": err.Error()})
		return err
	}
	s.logJob(ctx, id, "info", "任务处理成功", statsDetail(stats))
	return nil
}

func (s *ExcelMatchJobService) CleanupExpiredJobs(ctx context.Context) error {
	jobs, err := s.jobDAO.FindExpired(ctx, time.Now(), 100)
	if err != nil {
		return err
	}
	for _, matchJob := range jobs {
		s.cleanupResultObject(ctx, matchJob)
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
	defer func() { _ = output.Close() }()
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
	if jobID == 0 || s.jobDAO == nil {
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

func (s *ExcelMatchJobService) uploadExcelResultToOSS(ctx context.Context, matchJob *model.ExcelMatchJob, objectKey string, uploadTimeout time.Duration) (storage.UploadResult, error) {
	client, err := storage.NewOSSClientFromConfig()
	if err != nil {
		return storage.UploadResult{}, err
	}
	fileSize := fileSizeOrZero(matchJob.ResultFilePath)
	uploadPlan := client.UploadPlan(fileSize)
	s.logJob(ctx, matchJob.ID, "info", "OSS上传配置", map[string]interface{}{
		"endpoint":                  uploadPlan.Endpoint,
		"use_internal":              uploadPlan.UseInternal,
		"multipart":                 uploadPlan.Multipart,
		"multipart_threshold_bytes": uploadPlan.MultipartThresholdBytes,
		"part_size_bytes":           uploadPlan.PartSizeBytes,
		"parallel_num":              uploadPlan.ParallelNum,
		"enable_checkpoint":         uploadPlan.EnableCheckpoint,
		"checkpoint_dir":            uploadPlan.CheckpointDir,
	})
	uploadCtx, cancel := context.WithTimeout(ctx, uploadTimeout)
	defer cancel()
	progressLogger := newExcelOSSProgressLogger(func(progress storage.UploadProgress, elapsed time.Duration) {
		s.logJob(ctx, matchJob.ID, "info", "OSS结果文件上传进度", map[string]interface{}{
			"object_key":  objectKey,
			"transferred": progress.Transferred,
			"total":       progress.Total,
			"percent":     fmt.Sprintf("%.2f", progress.Percent),
			"increment":   progress.Increment,
			"file_size":   fileSize,
			"timeout_sec": int(uploadTimeout.Seconds()),
			"elapsed_sec": int(elapsed.Seconds()),
		})
	})
	defer progressLogger.Flush()
	result, err := client.UploadFileWithProgress(uploadCtx, objectKey, matchJob.ResultFilePath, fmt.Sprintf("excel_match_job_%d.xlsx", matchJob.ID), progressLogger.Handle)
	if err != nil {
		return storage.UploadResult{}, err
	}
	if err := s.jobDAO.UpdateResultStorage(ctx, matchJob.ID, result.ObjectKey, result.URL); err != nil {
		return storage.UploadResult{}, err
	}
	matchJob.ResultObjectKey = result.ObjectKey
	matchJob.ResultURL = result.URL
	return result, nil
}

func excelMatchResultObjectKey(jobID uint, now time.Time) string {
	return storage.BuildObjectKey(
		"excel-match-results",
		now.Format("2006/01/02"),
		strconv.FormatUint(uint64(jobID), 10),
		excelMatchResultFileName,
	)
}

func (s *ExcelMatchJobService) cleanupResultObject(ctx context.Context, matchJob model.ExcelMatchJob) {
	if strings.TrimSpace(matchJob.ResultObjectKey) == "" || !storage.OSSStorageEnabled() {
		return
	}
	client, err := storage.NewOSSClientFromConfig()
	if err != nil {
		s.logJob(ctx, matchJob.ID, "warn", "初始化OSS清理客户端失败", map[string]interface{}{"error": err.Error()})
		return
	}
	if err := client.DeleteObject(ctx, matchJob.ResultObjectKey); err != nil {
		s.logJob(ctx, matchJob.ID, "warn", "删除OSS结果文件失败", map[string]interface{}{"error": err.Error()})
	}
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

func fileSizeOrZero(filePath string) int64 {
	info, err := os.Stat(filePath)
	if err != nil {
		return 0
	}
	return info.Size()
}

func excelMatchOSSUploadTimeout(fileSize int64) time.Duration {
	if override := strings.TrimSpace(os.Getenv(excelMatchOSSUploadTimeoutEnvName)); override != "" {
		if seconds, err := strconv.Atoi(override); err == nil && seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
	}
	if fileSize <= 0 {
		return excelMatchOSSUploadMinTimeout
	}
	minutes := fileSize / excelMatchOSSUploadBytesPerMinute
	if fileSize%excelMatchOSSUploadBytesPerMinute != 0 {
		minutes++
	}
	timeout := time.Duration(minutes) * time.Minute
	if timeout < excelMatchOSSUploadMinTimeout {
		return excelMatchOSSUploadMinTimeout
	}
	if timeout > excelMatchOSSUploadMaxTimeout {
		return excelMatchOSSUploadMaxTimeout
	}
	return timeout
}

type excelOSSProgressLogger struct {
	startedAt       time.Time
	lastLoggedAt    time.Time
	lastLoggedBytes int64
	latest          storage.UploadProgress
	log             func(storage.UploadProgress, time.Duration)
}

func newExcelOSSProgressLogger(log func(storage.UploadProgress, time.Duration)) *excelOSSProgressLogger {
	now := time.Now()
	return &excelOSSProgressLogger{
		startedAt:    now,
		lastLoggedAt: now,
		log:          log,
	}
}

func (l *excelOSSProgressLogger) Handle(progress storage.UploadProgress) {
	l.latest = progress
	now := time.Now()
	if progress.Total > 0 && progress.Transferred >= progress.Total {
		l.emit(progress, now)
		return
	}
	if progress.Transferred-l.lastLoggedBytes >= excelMatchOSSProgressBytes || now.Sub(l.lastLoggedAt) >= excelMatchOSSProgressInterval {
		l.emit(progress, now)
	}
}

func (l *excelOSSProgressLogger) Flush() {
	if l.latest.Transferred <= 0 || l.latest.Transferred == l.lastLoggedBytes {
		return
	}
	l.emit(l.latest, time.Now())
}

func (l *excelOSSProgressLogger) emit(progress storage.UploadProgress, now time.Time) {
	if l.log == nil {
		return
	}
	l.lastLoggedAt = now
	l.lastLoggedBytes = progress.Transferred
	l.log(progress, now.Sub(l.startedAt))
}

func excelMatchSchemeFromModel(row model.ExcelMatchScheme) (ExcelMatchScheme, error) {
	var config ExcelMatchConfig
	if err := json.Unmarshal([]byte(row.ConfigJSON), &config); err != nil {
		return ExcelMatchScheme{}, err
	}
	return ExcelMatchScheme{
		ID:         row.ID,
		Name:       row.Name,
		Operation:  row.Operation,
		Config:     config,
		ConfigJSON: row.ConfigJSON,
		CreatedAt:  row.CreatedAt,
		UpdatedAt:  row.UpdatedAt,
	}, nil
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
	return filepath.Join(excelTempRootDir(), excelMatchRootDirName, strconv.FormatUint(uint64(id), 10))
}

func excelUploadRootDir() string {
	return filepath.Join(excelTempRootDir(), excelUploadRootDirName)
}

func excelPreviewRootDir() string {
	return filepath.Join(excelTempRootDir(), excelPreviewRootDirName)
}

func excelTempRootDir() string {
	for _, key := range []string{excelTempRootEnvName, "EXCEL_MATCH_TEMP_DIR"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return filepath.Clean(value)
		}
	}
	if abs, err := filepath.Abs(filepath.Join("storage", "tmp", "data-warehouse-excel")); err == nil {
		return abs
	}
	return filepath.Join(os.TempDir(), "data-warehouse-excel")
}

func excelizeTempDirForPath(filePath string) string {
	base := filepath.Dir(filePath)
	if base == "." || strings.TrimSpace(base) == "" {
		base = excelTempRootDir()
	}
	return filepath.Join(base, excelizeTempDirName)
}

func withExcelizeTempDir(tempDir string, fn func() error) error {
	if err := os.MkdirAll(tempDir, 0700); err != nil {
		return err
	}
	excelizeTempMu.Lock()
	defer excelizeTempMu.Unlock()

	keys := []string{"TMPDIR", "TMP", "TEMP"}
	previous := make(map[string]string, len(keys))
	present := make(map[string]bool, len(keys))
	for _, key := range keys {
		value, ok := os.LookupEnv(key)
		previous[key] = value
		present[key] = ok
		if err := os.Setenv(key, tempDir); err != nil {
			return err
		}
	}
	defer func() {
		for _, key := range keys {
			if present[key] {
				_ = os.Setenv(key, previous[key])
			} else {
				_ = os.Unsetenv(key)
			}
		}
	}()

	return fn()
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

func (s *ExcelMatchJobService) refreshExpiredJob(ctx context.Context, matchJob *model.ExcelMatchJob) {
	if !isExcelMatchJobExpired(matchJob) || matchJob.Status == excelMatchStatusExpired {
		return
	}
	_ = os.RemoveAll(matchJob.WorkDir)
	_ = s.jobDAO.MarkExpired(ctx, matchJob.ID)
	matchJob.Status = excelMatchStatusExpired
}

func (s *ExcelMatchJobService) refreshDownloadState(ctx context.Context, matchJob *model.ExcelMatchJob) {
	matchJob.CanDownload = false
	matchJob.DownloadMessage = ""
	if matchJob.Status != excelMatchStatusSuccess {
		return
	}
	if excelJobOperationFromConfigJSON(matchJob.ConfigJSON) != excelOperationExportMatch {
		matchJob.DownloadMessage = "该任务不会生成结果文件"
		return
	}
	if strings.TrimSpace(matchJob.ResultURL) != "" {
		matchJob.CanDownload = true
		return
	}
	matchJob.DownloadMessage = "结果文件正在上传OSS，上传成功后才能下载，请稍后刷新任务状态"
}

func excelJobOperationFromConfigJSON(rawConfig string) string {
	var config ExcelMatchConfig
	if err := json.Unmarshal([]byte(rawConfig), &config); err != nil {
		return ""
	}
	config.Operation = strings.TrimSpace(config.Operation)
	if config.Operation == "" {
		return excelOperationExportMatch
	}
	return config.Operation
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
