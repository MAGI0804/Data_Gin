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
