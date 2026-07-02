package data_dao

import (
	"context"
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

type CleanRecordDAO struct {
	db *gorm.DB
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

type DeliveryLogDAO struct {
	db *gorm.DB
}

func NewDeliveryLogDAO() *DeliveryLogDAO {
	return &DeliveryLogDAO{db: database.DB}
}

func (dao *DeliveryLogDAO) Create(ctx context.Context, log *model.DeliveryLog) (uint, error) {
	now := int(time.Now().Unix())
	log.CreatedAt = now
	log.UpdatedAt = now

	err := dao.db.WithContext(ctx).Create(log).Error
	return log.ID, err
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

type PipelineRunDAO struct {
	db *gorm.DB
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
