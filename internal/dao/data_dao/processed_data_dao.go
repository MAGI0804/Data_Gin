package data_dao

import (
	"context"
	"time"

	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"

	"gorm.io/gorm"
)

type ProcessedDataDAO struct {
	db *gorm.DB
}

func NewProcessedDataDAO() *ProcessedDataDAO {
	return &ProcessedDataDAO{db: database.DB}
}

// Create 创建处理结果
func (dao *ProcessedDataDAO) Create(ctx context.Context, processedData *model.ProcessedData) (uint, error) {
	// 先将同原始数据的其他版本标记为非当前版本
	err := dao.db.WithContext(ctx).Model(&model.ProcessedData{}).
		Where("raw_data_id = ? AND is_current = ?", processedData.RawDataID, true).
		Update("is_current", false).Error

	if err != nil {
		return 0, err
	}

	// 创建新的处理结果
	err = dao.db.WithContext(ctx).Create(processedData).Error
	return processedData.ID, err
}

// FindByID 根据ID查找处理结果
func (dao *ProcessedDataDAO) FindByID(ctx context.Context, id uint) (*model.ProcessedData, error) {
	var processedData model.ProcessedData
	err := dao.db.WithContext(ctx).Where("id = ?", id).First(&processedData).Error
	return &processedData, err
}

// FindByRawDataID 根据原始数据ID查找处理结果
func (dao *ProcessedDataDAO) FindByRawDataID(ctx context.Context, rawDataID uint) (*model.ProcessedData, error) {
	var processedData model.ProcessedData
	err := dao.db.WithContext(ctx).Where("raw_data_id = ? AND is_current = ?", rawDataID, true).First(&processedData).Error
	return &processedData, err
}

// FindByDataType 根据数据类型查找处理结果
func (dao *ProcessedDataDAO) FindByDataType(ctx context.Context, dataType string, limit int) ([]model.ProcessedData, error) {
	var processedDataList []model.ProcessedData
	query := dao.db.WithContext(ctx).Where("data_type = ? AND is_current = ?", dataType, true)

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Order("created_at DESC").Find(&processedDataList).Error
	return processedDataList, err
}

// UpdateQualityScore 更新数据质量评分
func (dao *ProcessedDataDAO) UpdateQualityScore(ctx context.Context, id uint, qualityScore float64) error {
	return dao.db.WithContext(ctx).Model(&model.ProcessedData{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"quality_score": qualityScore,
			"updated_at":    time.Now().Unix(),
		}).Error
}
