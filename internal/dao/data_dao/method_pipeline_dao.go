package data_dao

import (
	"context"
	"time"

	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"

	"gorm.io/gorm"
)

type PipelineDefinitionDAO struct {
	db *gorm.DB
}

func NewPipelineDefinitionDAO() *PipelineDefinitionDAO {
	return &PipelineDefinitionDAO{db: database.DB}
}

func (dao *PipelineDefinitionDAO) Create(ctx context.Context, pipeline *model.PipelineDefinition) (uint, error) {
	now := int(time.Now().Unix())
	pipeline.CreatedAt = now
	pipeline.UpdatedAt = now
	err := dao.db.WithContext(ctx).Create(pipeline).Error
	return pipeline.ID, err
}

func (dao *PipelineDefinitionDAO) FindAll(ctx context.Context) ([]model.PipelineDefinition, error) {
	var pipelines []model.PipelineDefinition
	err := dao.db.WithContext(ctx).Order("id DESC").Find(&pipelines).Error
	return pipelines, err
}

func (dao *PipelineDefinitionDAO) FindByID(ctx context.Context, id uint) (*model.PipelineDefinition, error) {
	var pipeline model.PipelineDefinition
	err := dao.db.WithContext(ctx).Where("id = ?", id).First(&pipeline).Error
	return &pipeline, err
}

func (dao *PipelineDefinitionDAO) Update(ctx context.Context, pipeline *model.PipelineDefinition) error {
	pipeline.UpdatedAt = int(time.Now().Unix())
	return dao.db.WithContext(ctx).Save(pipeline).Error
}

type PipelineStageDAO struct {
	db *gorm.DB
}

func NewPipelineStageDAO() *PipelineStageDAO {
	return &PipelineStageDAO{db: database.DB}
}

func (dao *PipelineStageDAO) Create(ctx context.Context, stage *model.PipelineStage) (uint, error) {
	now := int(time.Now().Unix())
	stage.CreatedAt = now
	stage.UpdatedAt = now
	err := dao.db.WithContext(ctx).Create(stage).Error
	return stage.ID, err
}

func (dao *PipelineStageDAO) FindByID(ctx context.Context, id uint) (*model.PipelineStage, error) {
	var stage model.PipelineStage
	err := dao.db.WithContext(ctx).Where("id = ?", id).First(&stage).Error
	return &stage, err
}

func (dao *PipelineStageDAO) FindByPipelineID(ctx context.Context, pipelineID uint) ([]model.PipelineStage, error) {
	var stages []model.PipelineStage
	err := dao.db.WithContext(ctx).
		Where("pipeline_id = ?", pipelineID).
		Order("order_index ASC, id ASC").
		Find(&stages).
		Error
	return stages, err
}

func (dao *PipelineStageDAO) Update(ctx context.Context, stage *model.PipelineStage) error {
	stage.UpdatedAt = int(time.Now().Unix())
	return dao.db.WithContext(ctx).Save(stage).Error
}

type MethodStepDAO struct {
	db *gorm.DB
}

func NewMethodStepDAO() *MethodStepDAO {
	return &MethodStepDAO{db: database.DB}
}

func (dao *MethodStepDAO) Create(ctx context.Context, step *model.MethodStep) (uint, error) {
	now := int(time.Now().Unix())
	step.CreatedAt = now
	step.UpdatedAt = now
	err := dao.db.WithContext(ctx).Create(step).Error
	return step.ID, err
}

func (dao *MethodStepDAO) FindByID(ctx context.Context, id uint) (*model.MethodStep, error) {
	var step model.MethodStep
	err := dao.db.WithContext(ctx).Where("id = ?", id).First(&step).Error
	return &step, err
}

func (dao *MethodStepDAO) FindByPipelineID(ctx context.Context, pipelineID uint) ([]model.MethodStep, error) {
	var steps []model.MethodStep
	err := dao.db.WithContext(ctx).
		Where("pipeline_id = ?", pipelineID).
		Order("order_index ASC, id ASC").
		Find(&steps).
		Error
	return steps, err
}

func (dao *MethodStepDAO) FindByStageID(ctx context.Context, stageID uint) ([]model.MethodStep, error) {
	var steps []model.MethodStep
	err := dao.db.WithContext(ctx).
		Where("stage_id = ?", stageID).
		Order("order_index ASC, id ASC").
		Find(&steps).
		Error
	return steps, err
}

func (dao *MethodStepDAO) Update(ctx context.Context, step *model.MethodStep) error {
	step.UpdatedAt = int(time.Now().Unix())
	return dao.db.WithContext(ctx).Save(step).Error
}

func (dao *MethodStepDAO) UpdateLegacy(ctx context.Context, step *model.MethodStep) (bool, error) {
	step.UpdatedAt = int(time.Now().Unix())
	result := dao.db.WithContext(ctx).
		Model(&model.MethodStep{}).
		Where("id = ? AND pipeline_id = ? AND stage_id = 0", step.ID, step.PipelineID).
		Updates(map[string]interface{}{
			"stage_id": step.StageID, "code": step.Code, "name": step.Name, "method_type": step.MethodType,
			"order_index": step.OrderIndex, "enabled": step.Enabled, "timeout_seconds": step.TimeoutSeconds,
			"generated_config_json": step.GeneratedConfigJSON, "updated_at": step.UpdatedAt,
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

type MethodParamDAO struct {
	db *gorm.DB
}

func NewMethodParamDAO() *MethodParamDAO {
	return &MethodParamDAO{db: database.DB}
}

func (dao *MethodParamDAO) ReplaceByStepID(ctx context.Context, stepID uint, params []model.MethodParam) error {
	return dao.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("step_id = ?", stepID).Delete(&model.MethodParam{}).Error; err != nil {
			return err
		}
		now := int(time.Now().Unix())
		for index := range params {
			params[index].StepID = stepID
			params[index].CreatedAt = now
			params[index].UpdatedAt = now
		}
		if len(params) == 0 {
			return nil
		}
		return tx.Create(&params).Error
	})
}

func (dao *MethodParamDAO) FindByStepIDs(ctx context.Context, stepIDs []uint) ([]model.MethodParam, error) {
	var params []model.MethodParam
	if len(stepIDs) == 0 {
		return params, nil
	}
	err := dao.db.WithContext(ctx).
		Where("step_id IN ?", stepIDs).
		Order("step_id ASC, order_index ASC, id ASC").
		Find(&params).
		Error
	return params, err
}

type MethodOutputDAO struct {
	db *gorm.DB
}

func NewMethodOutputDAO() *MethodOutputDAO {
	return &MethodOutputDAO{db: database.DB}
}

func (dao *MethodOutputDAO) ReplaceByStepID(ctx context.Context, stepID uint, outputs []model.MethodOutput) error {
	return dao.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("step_id = ?", stepID).Delete(&model.MethodOutput{}).Error; err != nil {
			return err
		}
		now := int(time.Now().Unix())
		for index := range outputs {
			outputs[index].StepID = stepID
			outputs[index].CreatedAt = now
			outputs[index].UpdatedAt = now
		}
		if len(outputs) == 0 {
			return nil
		}
		return tx.Create(&outputs).Error
	})
}

func (dao *MethodOutputDAO) FindByStepIDs(ctx context.Context, stepIDs []uint) ([]model.MethodOutput, error) {
	var outputs []model.MethodOutput
	if len(stepIDs) == 0 {
		return outputs, nil
	}
	err := dao.db.WithContext(ctx).
		Where("step_id IN ?", stepIDs).
		Order("step_id ASC, order_index ASC, id ASC").
		Find(&outputs).
		Error
	return outputs, err
}

type StageGeneratedConfigDAO struct {
	db *gorm.DB
}

func NewStageGeneratedConfigDAO() *StageGeneratedConfigDAO {
	return &StageGeneratedConfigDAO{db: database.DB}
}

func (dao *StageGeneratedConfigDAO) Create(ctx context.Context, cfg *model.StageGeneratedConfig) (uint, error) {
	now := int(time.Now().Unix())
	cfg.CreatedAt = now
	cfg.UpdatedAt = now
	err := dao.db.WithContext(ctx).Create(cfg).Error
	return cfg.ID, err
}

func (dao *StageGeneratedConfigDAO) FindLatestByStageID(ctx context.Context, stageID uint) (*model.StageGeneratedConfig, error) {
	var cfg model.StageGeneratedConfig
	err := dao.db.WithContext(ctx).
		Where("stage_id = ?", stageID).
		Order("version DESC, id DESC").
		First(&cfg).
		Error
	return &cfg, err
}

func (dao *StageGeneratedConfigDAO) FindByPipelineID(ctx context.Context, pipelineID uint) ([]model.StageGeneratedConfig, error) {
	var configs []model.StageGeneratedConfig
	err := dao.db.WithContext(ctx).
		Where("pipeline_id = ?", pipelineID).
		Order("stage_id ASC, version DESC, id DESC").
		Find(&configs).
		Error
	return configs, err
}

func (dao *StageGeneratedConfigDAO) Update(ctx context.Context, cfg *model.StageGeneratedConfig) error {
	cfg.UpdatedAt = int(time.Now().Unix())
	return dao.db.WithContext(ctx).Save(cfg).Error
}

func (dao *StageGeneratedConfigDAO) NextVersion(ctx context.Context, stageID uint) (int, error) {
	var cfg model.StageGeneratedConfig
	err := dao.db.WithContext(ctx).
		Where("stage_id = ?", stageID).
		Order("version DESC, id DESC").
		First(&cfg).
		Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return 1, nil
		}
		return 0, err
	}
	return cfg.Version + 1, nil
}

type StepRunDAO struct {
	db *gorm.DB
}

func NewStepRunDAO() *StepRunDAO {
	return &StepRunDAO{db: database.DB}
}

func (dao *StepRunDAO) Create(ctx context.Context, run *model.StepRun) (uint, error) {
	now := int(time.Now().Unix())
	run.CreatedAt = now
	run.UpdatedAt = now
	err := dao.db.WithContext(ctx).Create(run).Error
	return run.ID, err
}

func (dao *StepRunDAO) Finish(ctx context.Context, id uint, status, outputJSON, errorMessage string) error {
	updates := map[string]interface{}{
		"status":      status,
		"output_json": outputJSON,
		"finished_at": model.TimeNormal{Time: time.Now()},
		"updated_at":  time.Now().Unix(),
	}
	if errorMessage != "" {
		updates["error_message"] = errorMessage
	}
	return dao.db.WithContext(ctx).Model(&model.StepRun{}).Where("id = ?", id).Updates(updates).Error
}

func (dao *StepRunDAO) FindByRunID(ctx context.Context, runID uint) ([]model.StepRun, error) {
	var runs []model.StepRun
	err := dao.db.WithContext(ctx).
		Where("run_id = ?", runID).
		Order("id ASC").
		Find(&runs).
		Error
	return runs, err
}
