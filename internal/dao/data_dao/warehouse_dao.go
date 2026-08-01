package data_dao

import (
	"context"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"

	"gorm.io/gorm"
)

type SourceDefinitionDAO struct {
	db *gorm.DB
}

func NewSourceDefinitionDAO() *SourceDefinitionDAO {
	return &SourceDefinitionDAO{db: database.DB}
}

func (dao *SourceDefinitionDAO) Create(ctx context.Context, source *model.SourceDefinition) (uint, error) {
	now := int(time.Now().Unix())
	source.CreatedAt = now
	source.UpdatedAt = now

	err := dao.db.WithContext(ctx).Create(source).Error
	return source.ID, err
}

func (dao *SourceDefinitionDAO) FindByID(ctx context.Context, id uint) (*model.SourceDefinition, error) {
	var source model.SourceDefinition
	err := dao.db.WithContext(ctx).
		Where("id = ?", id).
		First(&source).
		Error
	return &source, err
}

func (dao *SourceDefinitionDAO) FindByCode(ctx context.Context, code string) (*model.SourceDefinition, error) {
	var source model.SourceDefinition
	err := dao.db.WithContext(ctx).
		Where("code = ?", code).
		First(&source).
		Error
	return &source, err
}

func (dao *SourceDefinitionDAO) FindAll(ctx context.Context) ([]model.SourceDefinition, error) {
	var sources []model.SourceDefinition
	err := dao.db.WithContext(ctx).
		Order("id DESC").
		Find(&sources).
		Error
	return sources, err
}

func (dao *SourceDefinitionDAO) Update(ctx context.Context, source *model.SourceDefinition) error {
	source.UpdatedAt = int(time.Now().Unix())
	return dao.db.WithContext(ctx).Save(source).Error
}

func (dao *SourceDefinitionDAO) FindEnabledWithQueryKey(ctx context.Context) ([]model.SourceDefinition, error) {
	var sources []model.SourceDefinition
	err := dao.db.WithContext(ctx).
		Where("enabled = ? AND source_query_key <> ?", true, "").
		Find(&sources).
		Error
	return sources, err
}

type RawRecordDAO struct {
	db *gorm.DB
}

// RawRecordListQuery contains validated raw_records list filters.
type RawRecordListQuery struct {
	Page      int
	PageSize  int
	Source    string
	Status    string
	TraceID   string
	StartTime *time.Time
	EndTime   *time.Time
	Origin    string
}

// RawRecordListItem is the database projection safe for management list views.
// It intentionally excludes raw request data, metadata and error details.
type RawRecordListItem struct {
	ID         uint              `gorm:"column:id"`
	SourceID   uint              `gorm:"column:source_id"`
	SourceCode string            `gorm:"column:source_code"`
	Status     string            `gorm:"column:status"`
	TraceID    string            `gorm:"column:trace_id"`
	ReceivedAt *model.TimeNormal `gorm:"column:received_at"`
	CreatedAt  int               `gorm:"column:created_at"`
}

type RawRecordListPage struct {
	List  []RawRecordListItem
	Total int64
}

func NewRawRecordDAO() *RawRecordDAO {
	return &RawRecordDAO{db: database.DB}
}

func (dao *RawRecordDAO) Create(ctx context.Context, rawRecord *model.RawRecord) (uint, error) {
	now := int(time.Now().Unix())
	rawRecord.CreatedAt = now
	rawRecord.UpdatedAt = now

	err := dao.db.WithContext(ctx).Create(rawRecord).Error
	return rawRecord.ID, err
}

func (dao *RawRecordDAO) FindByID(ctx context.Context, id uint) (*model.RawRecord, error) {
	var rawRecord model.RawRecord
	err := dao.db.WithContext(ctx).
		Where("id = ?", id).
		First(&rawRecord).
		Error
	return &rawRecord, err
}

func (dao *RawRecordDAO) UpdateStatus(ctx context.Context, id uint, status, errorMessage string) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now().Unix(),
	}
	if errorMessage != "" {
		updates["error_message"] = errorMessage
	}

	return dao.db.WithContext(ctx).
		Model(&model.RawRecord{}).
		Where("id = ?", id).
		Updates(updates).
		Error
}

func (dao *RawRecordDAO) FindPage(ctx context.Context, params RawRecordListQuery) (*RawRecordListPage, error) {
	query := dao.db.WithContext(ctx).Model(&model.RawRecord{})
	if params.Source != "" {
		query = query.Where("source_code = ?", params.Source)
	}
	if params.Status != "" {
		query = query.Where("status = ?", params.Status)
	}
	if params.TraceID != "" {
		query = query.Where("trace_id = ?", params.TraceID)
	}
	if params.StartTime != nil {
		query = query.Where("received_at >= ?", *params.StartTime)
	}
	if params.EndTime != nil {
		query = query.Where("received_at <= ?", *params.EndTime)
	}
	if condition, args := rawRecordOriginCondition(params.Origin); condition != "" {
		query = query.Where(condition, args...)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	items := make([]RawRecordListItem, 0)
	offset := (params.Page - 1) * params.PageSize
	if err := query.
		Select(rawRecordListColumns()).
		Order("received_at DESC, id DESC").
		Offset(offset).
		Limit(params.PageSize).
		Find(&items).
		Error; err != nil {
		return nil, err
	}

	return &RawRecordListPage{List: items, Total: total}, nil
}

func rawRecordListColumns() []string {
	return []string{
		"id",
		"source_id",
		"source_code",
		"status",
		"trace_id",
		"received_at",
		"created_at",
	}
}

func rawRecordOriginCondition(origin string) (string, []interface{}) {
	const fetchedFormat = "%\"format\":\"fetch\"%"
	switch origin {
	case "pull":
		return "metadata_json LIKE ?", []interface{}{fetchedFormat}
	case "receive":
		return "(metadata_json IS NULL OR metadata_json NOT LIKE ?)", []interface{}{fetchedFormat}
	default:
		return "", nil
	}
}

type TransformRuleDAO struct {
	db *gorm.DB
}

func NewTransformRuleDAO() *TransformRuleDAO {
	return &TransformRuleDAO{db: database.DB}
}

func (dao *TransformRuleDAO) Create(ctx context.Context, rule *model.TransformRule) (uint, error) {
	now := int(time.Now().Unix())
	rule.CreatedAt = now
	rule.UpdatedAt = now

	err := dao.db.WithContext(ctx).Create(rule).Error
	return rule.ID, err
}

func (dao *TransformRuleDAO) FindEnabledBySourceID(ctx context.Context, sourceID uint) ([]model.TransformRule, error) {
	var rules []model.TransformRule
	err := dao.db.WithContext(ctx).
		Where("source_id = ? AND enabled = ?", sourceID, true).
		Order("order_index ASC, id ASC").
		Find(&rules).
		Error
	return rules, err
}

func (dao *TransformRuleDAO) FindAll(ctx context.Context) ([]model.TransformRule, error) {
	var rules []model.TransformRule
	err := dao.db.WithContext(ctx).
		Order("source_id ASC, order_index ASC, id DESC").
		Find(&rules).
		Error
	return rules, err
}

func (dao *TransformRuleDAO) FindByID(ctx context.Context, id uint) (*model.TransformRule, error) {
	var rule model.TransformRule
	err := dao.db.WithContext(ctx).
		Where("id = ?", id).
		First(&rule).
		Error
	return &rule, err
}

func (dao *TransformRuleDAO) Update(ctx context.Context, rule *model.TransformRule) error {
	rule.UpdatedAt = int(time.Now().Unix())
	return dao.db.WithContext(ctx).Save(rule).Error
}

type CleanRecordDAO struct {
	db *gorm.DB
}

type CleanRecordListQuery struct {
	SourceID    uint
	TableName   string
	BusinessKey string
	Status      string
	MinQuality  *float64
	MaxQuality  *float64
	CreatedFrom int64
	CreatedTo   int64
	Page        int
	PageSize    int
}

type CleanRecordWithTotal struct {
	List           []model.CleanRecord
	Total          int64
	AverageQuality float64
}

func NewCleanRecordDAO() *CleanRecordDAO {
	return &CleanRecordDAO{db: database.DB}
}

func (dao *CleanRecordDAO) Create(ctx context.Context, cleanRecord *model.CleanRecord) (uint, error) {
	now := int(time.Now().Unix())
	cleanRecord.CreatedAt = now
	cleanRecord.UpdatedAt = now

	err := dao.db.WithContext(ctx).Create(cleanRecord).Error
	return cleanRecord.ID, err
}

func (dao *CleanRecordDAO) FindReadyBySourceAndTable(ctx context.Context, sourceID uint, tableName string, limit int) ([]model.CleanRecord, error) {
	var records []model.CleanRecord
	query := dao.db.WithContext(ctx).
		Where("source_id = ? AND table_name = ? AND status = ?", sourceID, tableName, "ready")
	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Order("id ASC").Find(&records).Error
	return records, err
}

func (dao *CleanRecordDAO) FindWithPagination(ctx context.Context, params CleanRecordListQuery) (*CleanRecordWithTotal, error) {
	query := dao.applyListFilters(dao.db.WithContext(ctx).Model(&model.CleanRecord{}), params)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var average struct{ Value float64 }
	if err := query.Select("COALESCE(AVG(quality_score), 0) AS value").Scan(&average).Error; err != nil {
		return nil, err
	}
	var list []model.CleanRecord
	offset := (params.Page - 1) * params.PageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(params.PageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	return &CleanRecordWithTotal{List: list, Total: total, AverageQuality: average.Value}, nil
}

func (dao *CleanRecordDAO) applyListFilters(query *gorm.DB, params CleanRecordListQuery) *gorm.DB {
	if params.SourceID > 0 {
		query = query.Where("source_id = ?", params.SourceID)
	}
	if params.TableName != "" {
		query = query.Where("table_name = ?", params.TableName)
	}
	if params.BusinessKey != "" {
		query = query.Where("business_key LIKE ?", "%"+params.BusinessKey+"%")
	}
	if params.Status != "" {
		query = query.Where("status = ?", params.Status)
	}
	if params.MinQuality != nil {
		query = query.Where("quality_score >= ?", *params.MinQuality)
	}
	if params.MaxQuality != nil {
		query = query.Where("quality_score <= ?", *params.MaxQuality)
	}
	if params.CreatedFrom > 0 {
		query = query.Where("created_at >= ?", params.CreatedFrom)
	}
	if params.CreatedTo > 0 {
		query = query.Where("created_at <= ?", params.CreatedTo)
	}
	return query
}

func (dao *CleanRecordDAO) MarkDelivered(ctx context.Context, id uint) error {
	return dao.db.WithContext(ctx).
		Model(&model.CleanRecord{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     "delivered",
			"updated_at": time.Now().Unix(),
		}).
		Error
}

type DestinationDefinitionDAO struct {
	db *gorm.DB
}

func NewDestinationDefinitionDAO() *DestinationDefinitionDAO {
	return &DestinationDefinitionDAO{db: database.DB}
}

func (dao *DestinationDefinitionDAO) Create(ctx context.Context, destination *model.DestinationDefinition) (uint, error) {
	now := int(time.Now().Unix())
	destination.CreatedAt = now
	destination.UpdatedAt = now

	err := dao.db.WithContext(ctx).Create(destination).Error
	return destination.ID, err
}

func (dao *DestinationDefinitionDAO) FindByID(ctx context.Context, id uint) (*model.DestinationDefinition, error) {
	var destination model.DestinationDefinition
	err := dao.db.WithContext(ctx).
		Where("id = ?", id).
		First(&destination).
		Error
	return &destination, err
}

func (dao *DestinationDefinitionDAO) FindAll(ctx context.Context) ([]model.DestinationDefinition, error) {
	var destinations []model.DestinationDefinition
	err := dao.db.WithContext(ctx).
		Order("id DESC").
		Find(&destinations).
		Error
	return destinations, err
}

func (dao *DestinationDefinitionDAO) Update(ctx context.Context, destination *model.DestinationDefinition) error {
	destination.UpdatedAt = int(time.Now().Unix())
	return dao.db.WithContext(ctx).Save(destination).Error
}

type DeliveryTaskDAO struct {
	db *gorm.DB
}

func NewDeliveryTaskDAO() *DeliveryTaskDAO {
	return &DeliveryTaskDAO{db: database.DB}
}

func (dao *DeliveryTaskDAO) Create(ctx context.Context, task *model.DeliveryTask) (uint, error) {
	now := int(time.Now().Unix())
	task.CreatedAt = now
	task.UpdatedAt = now

	err := dao.db.WithContext(ctx).Create(task).Error
	return task.ID, err
}

func (dao *DeliveryTaskDAO) FindByID(ctx context.Context, id uint) (*model.DeliveryTask, error) {
	var task model.DeliveryTask
	err := dao.db.WithContext(ctx).
		Where("id = ?", id).
		First(&task).
		Error
	return &task, err
}

func (dao *DeliveryTaskDAO) FindEnabledScheduled(ctx context.Context) ([]model.DeliveryTask, error) {
	var tasks []model.DeliveryTask
	err := dao.db.WithContext(ctx).
		Where("enabled = ? AND trigger_type = ? AND cron_expr <> ?", true, "schedule", "").
		Find(&tasks).
		Error
	return tasks, err
}

func (dao *DeliveryTaskDAO) FindAll(ctx context.Context) ([]model.DeliveryTask, error) {
	var tasks []model.DeliveryTask
	err := dao.db.WithContext(ctx).
		Order("id DESC").
		Find(&tasks).
		Error
	return tasks, err
}

func (dao *DeliveryTaskDAO) Update(ctx context.Context, task *model.DeliveryTask) error {
	task.UpdatedAt = int(time.Now().Unix())
	return dao.db.WithContext(ctx).Save(task).Error
}

type DeliveryLogDAO struct {
	db *gorm.DB
}

// DeliveryLogListQuery contains bounded management list filters.
type DeliveryLogListQuery struct {
	Page            int
	PageSize        int
	DestinationCode string
	SourceCode      string
	Success         *bool
	BusinessKey     string
	SentFrom        *time.Time
	SentTo          *time.Time
}

type DeliveryLogListPage struct {
	List  []model.DeliveryLog
	Total int64
}

func NewDeliveryLogDAO(databases ...*gorm.DB) *DeliveryLogDAO {
	db := database.DB
	if len(databases) > 0 && databases[0] != nil {
		db = databases[0]
	}
	return &DeliveryLogDAO{db: db}
}

func (dao *DeliveryLogDAO) WithDB(db *gorm.DB) *DeliveryLogDAO {
	return &DeliveryLogDAO{db: db}
}

func (dao *DeliveryLogDAO) Create(ctx context.Context, log *model.DeliveryLog) (uint, error) {
	now := int(time.Now().Unix())
	log.CreatedAt = now
	log.UpdatedAt = now

	err := dao.db.WithContext(ctx).Create(log).Error
	return log.ID, err
}

func (dao *DeliveryLogDAO) FindByID(ctx context.Context, id uint) (*model.DeliveryLog, error) {
	var log model.DeliveryLog
	err := dao.db.WithContext(ctx).First(&log, id).Error
	return &log, err
}

func (dao *DeliveryLogDAO) IncrementRetryCount(ctx context.Context, id uint) error {
	return dao.db.WithContext(ctx).
		Model(&model.DeliveryLog{}).
		Where("id = ?", id).
		UpdateColumn("retry_count", gorm.Expr("retry_count + ?", 1)).
		Error
}

func (dao *DeliveryLogDAO) FindRecent(ctx context.Context, limit int) ([]model.DeliveryLog, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	var logs []model.DeliveryLog
	err := dao.db.WithContext(ctx).
		Order("id DESC").
		Limit(limit).
		Find(&logs).
		Error
	return logs, err
}

func (dao *DeliveryLogDAO) FindPage(ctx context.Context, params DeliveryLogListQuery) (*DeliveryLogListPage, error) {
	query := dao.applyListFilters(dao.db.WithContext(ctx).Model(&model.DeliveryLog{}), params)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	logs := make([]model.DeliveryLog, 0)
	offset := (params.Page - 1) * params.PageSize
	if err := query.Order("id DESC").Offset(offset).Limit(params.PageSize).Find(&logs).Error; err != nil {
		return nil, err
	}
	return &DeliveryLogListPage{List: logs, Total: total}, nil
}

func (dao *DeliveryLogDAO) applyListFilters(query *gorm.DB, params DeliveryLogListQuery) *gorm.DB {
	if params.DestinationCode != "" {
		query = query.Where("destination_code = ?", params.DestinationCode)
	}
	if params.SourceCode != "" {
		query = query.Where("source_code = ?", params.SourceCode)
	}
	if params.Success != nil {
		query = query.Where("success = ?", *params.Success)
	}
	if params.BusinessKey != "" {
		query = query.Where("business_key = ?", params.BusinessKey)
	}
	if params.SentFrom != nil {
		query = query.Where("sent_at >= ?", *params.SentFrom)
	}
	if params.SentTo != nil {
		query = query.Where("sent_at <= ?", *params.SentTo)
	}
	return query
}

type DeliveryLogBatchFinish struct {
	Status          string
	RowStart        int64
	RowEnd          int64
	HTTPStatus      int
	FeishuCode      int
	Success         bool
	ResponseSummary string
	SafeError       string
	FinishedAt      time.Time
}

func (dao *DeliveryLogDAO) FindLatestWeatherBatch(
	ctx context.Context,
	runID uint,
	destinationID uint,
	datasetKind string,
	batchNo int,
) (*model.DeliveryLog, error) {
	datasetKind = strings.TrimSpace(datasetKind)
	if dao == nil || dao.db == nil || ctx == nil || runID == 0 || destinationID == 0 || datasetKind == "" ||
		len(datasetKind) > 32 || batchNo < 1 {
		return nil, errors.New("delivery log: invalid weather batch query")
	}
	var log model.DeliveryLog
	err := dao.db.WithContext(ctx).
		Where(
			"run_id = ? AND destination_id = ? AND dataset_kind = ? AND batch_no = ?",
			runID,
			destinationID,
			datasetKind,
			batchNo,
		).
		Order("attempt DESC, id DESC").
		First(&log).
		Error
	return &log, err
}

func (dao *DeliveryLogDAO) FinishWeatherBatch(
	ctx context.Context,
	id uint,
	finish DeliveryLogBatchFinish,
) error {
	finish.Status = strings.TrimSpace(finish.Status)
	if dao == nil || dao.db == nil || ctx == nil || id == 0 || !validDeliveryLogBatchFinish(finish) {
		return errors.New("delivery log: invalid weather batch completion")
	}
	finishedAt := model.TimeNormal{Time: finish.FinishedAt.UTC()}
	updates := map[string]interface{}{
		"status":           finish.Status,
		"http_status":      finish.HTTPStatus,
		"feishu_code":      finish.FeishuCode,
		"success":          finish.Success,
		"response_summary": finish.ResponseSummary,
		"error_message":    finish.SafeError,
		"finished_at":      finishedAt,
		"sent_at":          finishedAt,
		"updated_at":       finish.FinishedAt.UTC().Unix(),
	}
	if finish.RowStart > 0 {
		updates["row_start"] = finish.RowStart
		updates["row_end"] = finish.RowEnd
	}
	result := dao.db.WithContext(ctx).
		Model(&model.DeliveryLog{}).
		Where("id = ? AND status = ?", id, "running").
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("delivery log: weather batch is not running")
	}
	return nil
}

func (dao *DeliveryLogDAO) ReconcileWeatherBatchSuccess(
	ctx context.Context,
	id uint,
	requestChecksum string,
	rowStart int64,
	rowEnd int64,
	finishedAt time.Time,
) error {
	checksum, checksumErr := hex.DecodeString(requestChecksum)
	if dao == nil || dao.db == nil || ctx == nil || id == 0 || checksumErr != nil || len(checksum) != 32 ||
		rowStart < 1 || rowEnd < rowStart || finishedAt.IsZero() {
		return errors.New("delivery log: invalid weather batch reconciliation")
	}
	finished := model.TimeNormal{Time: finishedAt.UTC()}
	result := dao.db.WithContext(ctx).
		Model(&model.DeliveryLog{}).
		Where("id = ? AND request_checksum = ? AND status IN ?", id, requestChecksum, []string{"running", "unknown"}).
		Updates(map[string]interface{}{
			"status":           "success",
			"row_start":        rowStart,
			"row_end":          rowEnd,
			"success":          true,
			"response_summary": "remote range checksum matched",
			"error_message":    "",
			"finished_at":      finished,
			"sent_at":          finished,
			"updated_at":       finishedAt.UTC().Unix(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("delivery log: weather batch cannot be reconciled")
	}
	return nil
}

func validDeliveryLogBatchFinish(finish DeliveryLogBatchFinish) bool {
	return !finish.FinishedAt.IsZero() &&
		(finish.Status == "success" || finish.Status == "failed" || finish.Status == "unknown") &&
		((finish.RowStart == 0 && finish.RowEnd == 0) || (finish.RowStart > 0 && finish.RowEnd >= finish.RowStart)) &&
		finish.HTTPStatus >= 0 && finish.FeishuCode >= 0 && len(finish.ResponseSummary) <= 512 &&
		len(finish.SafeError) <= 2048 && (finish.Status == "success") == finish.Success
}

type PipelineRunDAO struct {
	db *gorm.DB
}

type PipelineRunListQuery struct {
	Page      int
	PageSize  int
	Status    string
	RunType   string
	TraceID   string
	StartedAt *time.Time
	EndedAt   *time.Time
}

type PipelineRunListPage struct {
	List  []model.PipelineRun
	Total int64
}

func NewPipelineRunDAO() *PipelineRunDAO {
	return &PipelineRunDAO{db: database.DB}
}

func (dao *PipelineRunDAO) Create(ctx context.Context, run *model.PipelineRun) (uint, error) {
	now := int(time.Now().Unix())
	run.CreatedAt = now
	run.UpdatedAt = now

	err := dao.db.WithContext(ctx).Create(run).Error
	return run.ID, err
}

func (dao *PipelineRunDAO) Finish(ctx context.Context, id uint, status string, successCount, failedCount int, errorMessage string) error {
	updates := map[string]interface{}{
		"status":        status,
		"total_count":   successCount + failedCount,
		"success_count": successCount,
		"failed_count":  failedCount,
		"finished_at":   model.TimeNormal{Time: time.Now()},
		"updated_at":    time.Now().Unix(),
	}
	if errorMessage != "" {
		updates["error_message"] = errorMessage
	}

	return dao.db.WithContext(ctx).
		Model(&model.PipelineRun{}).
		Where("id = ?", id).
		Updates(updates).
		Error
}

func (dao *PipelineRunDAO) FindRecent(ctx context.Context, limit int) ([]model.PipelineRun, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	var runs []model.PipelineRun
	err := dao.db.WithContext(ctx).
		Order("id DESC").
		Limit(limit).
		Find(&runs).
		Error
	return runs, err
}

func (dao *PipelineRunDAO) FindPage(ctx context.Context, params PipelineRunListQuery) (*PipelineRunListPage, error) {
	query := dao.applyListFilters(dao.db.WithContext(ctx).Model(&model.PipelineRun{}), params)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	runs := make([]model.PipelineRun, 0)
	offset := (params.Page - 1) * params.PageSize
	if err := query.Order("id DESC").Offset(offset).Limit(params.PageSize).Find(&runs).Error; err != nil {
		return nil, err
	}
	return &PipelineRunListPage{List: runs, Total: total}, nil
}

func (dao *PipelineRunDAO) applyListFilters(query *gorm.DB, params PipelineRunListQuery) *gorm.DB {
	if params.Status != "" {
		query = query.Where("status = ?", params.Status)
	}
	if params.RunType != "" {
		query = query.Where("run_type = ?", params.RunType)
	}
	if params.TraceID != "" {
		query = query.Where("trace_id = ?", params.TraceID)
	}
	if params.StartedAt != nil {
		query = query.Where("started_at >= ?", *params.StartedAt)
	}
	if params.EndedAt != nil {
		query = query.Where("started_at <= ?", *params.EndedAt)
	}
	return query
}
