package data_dao

import (
	"context"
	"time"

	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"

	"gorm.io/gorm"
)

type RawDataDAO struct {
	db *gorm.DB
}

func NewRawDataDAO() *RawDataDAO {
	return &RawDataDAO{db: database.DB}
}

// Create 创建原始数据
func (dao *RawDataDAO) Create(ctx context.Context, rawData *model.RawData) (uint, error) {
	err := dao.db.WithContext(ctx).Create(rawData).Error
	return rawData.ID, err
}

// FindByID 根据ID查找原始数据
func (dao *RawDataDAO) FindByID(ctx context.Context, id uint) (*model.RawData, error) {
	var rawData model.RawData
	err := dao.db.WithContext(ctx).Where("id = ?", id).First(&rawData).Error
	return &rawData, err
}

// UpdateStatus 更新原始数据状态
func (dao *RawDataDAO) UpdateStatus(ctx context.Context, id uint, status, errorMessage string) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now().Unix(),
	}

	if errorMessage != "" {
		updates["error_message"] = errorMessage
	}

	if status == "processed" {
		updates["processed_at"] = time.Now().Unix()
	}

	return dao.db.WithContext(ctx).Model(&model.RawData{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// FindByStatus 根据状态查找原始数据
func (dao *RawDataDAO) FindByStatus(ctx context.Context, status string, limit int) ([]model.RawData, error) {
	var rawDataList []model.RawData
	query := dao.db.WithContext(ctx).Where("status = ?", status)

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Order("created_at ASC").Find(&rawDataList).Error
	return rawDataList, err
}

// FindByDataSource 根据数据源ID查找原始数据
func (dao *RawDataDAO) FindByDataSource(ctx context.Context, dataSourceID uint, limit int) ([]model.RawData, error) {
	var rawDataList []model.RawData
	query := dao.db.WithContext(ctx).Where("data_source_id = ?", dataSourceID)

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Order("created_at DESC").Find(&rawDataList).Error
	return rawDataList, err
}
