package data_dao

import (
	"context"

	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type YouzanDistributionOrderDAO struct {
	db *gorm.DB
}

func NewYouzanDistributionOrderDAO() *YouzanDistributionOrderDAO {
	return &YouzanDistributionOrderDAO{db: database.DB}
}

func (dao *YouzanDistributionOrderDAO) FindExistingTIDs(ctx context.Context, tids []string) (map[string]bool, error) {
	existing := make(map[string]bool)
	if len(tids) == 0 {
		return existing, nil
	}

	var values []string
	err := dao.db.WithContext(ctx).
		Model(&model.YouzanDistributionOrder{}).
		Where("tid IN ?", tids).
		Pluck("tid", &values).Error
	if err != nil {
		return nil, err
	}
	for _, tid := range values {
		existing[tid] = true
	}
	return existing, nil
}

// CreateBatchIfNotExists inserts new orders and leaves existing TIDs untouched.
func (dao *YouzanDistributionOrderDAO) CreateBatchIfNotExists(ctx context.Context, orders []model.YouzanDistributionOrder) (int64, error) {
	if len(orders) == 0 {
		return 0, nil
	}

	result := dao.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		CreateInBatches(&orders, 100)
	return result.RowsAffected, result.Error
}
