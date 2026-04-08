package data_dao

import (
	"context"
	"time"

	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"

	"gorm.io/gorm"
)

type StatisticsDAO struct {
	db *gorm.DB
}

func NewStatisticsDAO() *StatisticsDAO {
	return &StatisticsDAO{db: database.DB}
}

// Create 创建或更新统计数据
func (dao *StatisticsDAO) Create(ctx context.Context, stats *model.DataStatistics) (uint, error) {
	// 先尝试查找是否已存在相同日期、类型和数据源的统计记录
	var existingStats model.DataStatistics
	err := dao.db.WithContext(ctx).Where(
		"stat_date = ? AND data_type = ? AND data_source_id = ?",
		stats.StatDate, stats.DataType, stats.DataSourceID,
	).First(&existingStats).Error

	if err == nil {
		// 已存在，更新记录
		stats.ID = existingStats.ID
		err = dao.db.WithContext(ctx).Save(stats).Error
	} else if err == gorm.ErrRecordNotFound {
		// 不存在，创建新记录
		err = dao.db.WithContext(ctx).Create(stats).Error
	}

	return stats.ID, err
}

// FindByDate 根据日期查找统计数据
func (dao *StatisticsDAO) FindByDate(ctx context.Context, statDate string, dataType string) (*model.DataStatistics, error) {
	var stats model.DataStatistics
	err := dao.db.WithContext(ctx).Where("stat_date = ? AND data_type = ?", statDate, dataType).First(&stats).Error
	return &stats, err
}

// FindByDateRange 根据日期范围查找统计数据
func (dao *StatisticsDAO) FindByDateRange(ctx context.Context, startDate, endDate string, dataType string) ([]model.DataStatistics, error) {
	var statsList []model.DataStatistics
	query := dao.db.WithContext(ctx).Where("stat_date BETWEEN ? AND ?", startDate, endDate)

	if dataType != "" {
		query = query.Where("data_type = ?", dataType)
	}

	err := query.Order("stat_date ASC").Find(&statsList).Error
	return statsList, err
}

// UpdateCounts 更新统计计数
func (dao *StatisticsDAO) UpdateCounts(ctx context.Context, id uint, totalCount, processedCount, errorCount uint) error {
	return dao.db.WithContext(ctx).Model(&model.DataStatistics{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"total_count":     totalCount,
			"processed_count": processedCount,
			"error_count":     errorCount,
			"updated_at":      time.Now().Unix(),
		}).Error
}

// UpdateQualityScore 更新平均质量评分
func (dao *StatisticsDAO) UpdateQualityScore(ctx context.Context, id uint, avgQualityScore float64) error {
	return dao.db.WithContext(ctx).Model(&model.DataStatistics{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"avg_quality_score": avgQualityScore,
			"updated_at":        time.Now().Unix(),
		}).Error
}
