package data_dao

import (
	"context"
	"time"

	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"

	"gorm.io/gorm"
)

type DataSourceDAO struct {
	db *gorm.DB
}

func NewDataSourceDAO() *DataSourceDAO {
	return &DataSourceDAO{db: database.DB}
}

// FindByID 根据ID查找数据源
func (dao *DataSourceDAO) FindByID(ctx context.Context, id uint) (*model.DataSource, error) {
	var source model.DataSource
	err := dao.db.WithContext(ctx).Where("id = ?", id).First(&source).Error
	return &source, err
}

// Create 创建数据源
func (dao *DataSourceDAO) Create(ctx context.Context, source *model.DataSource) (uint, error) {
	err := dao.db.WithContext(ctx).Create(source).Error
	return source.ID, err
}

// UpdateStatus 更新数据源状态
func (dao *DataSourceDAO) UpdateStatus(ctx context.Context, id uint, status, message string) error {
	return dao.db.WithContext(ctx).Model(&model.DataSource{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":           status,
			"last_sync_status": message,
			"last_sync_time":   time.Now(),
			"updated_at":       time.Now().Unix(),
		}).Error
}

// UpdateSyncStatus 更新同步状态
func (dao *DataSourceDAO) UpdateSyncStatus(ctx context.Context, id uint, syncTime time.Time, status string) error {
	return dao.db.WithContext(ctx).Model(&model.DataSource{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"last_sync_time":   syncTime,
			"last_sync_status": status,
			"status":           "active",
			"updated_at":       time.Now().Unix(),
		}).Error
}

// FindActive 查找所有活跃的数据源
func (dao *DataSourceDAO) FindActive(ctx context.Context) ([]model.DataSource, error) {
	var sources []model.DataSource
	err := dao.db.WithContext(ctx).Where("status = ?", "active").Find(&sources).Error
	return sources, err
}
