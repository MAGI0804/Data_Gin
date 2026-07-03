package data_dao

import (
	"context"
	"time"

	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"

	"gorm.io/gorm"
)

type BojunRetailOrderDAO struct {
	db *gorm.DB
}

func NewBojunRetailOrderDAO() *BojunRetailOrderDAO {
	return &BojunRetailOrderDAO{db: database.DB}
}

func (dao *BojunRetailOrderDAO) Create(ctx context.Context, order *model.BojunRetailOrder) (uint, error) {
	now := int(time.Now().Unix())
	order.CreatedAt = now
	order.UpdatedAt = now

	err := dao.db.WithContext(ctx).Create(order).Error
	return order.ID, err
}

func (dao *BojunRetailOrderDAO) FindByDocNo(ctx context.Context, docNo string) (*model.BojunRetailOrder, error) {
	var order model.BojunRetailOrder
	err := dao.db.WithContext(ctx).
		Where("docno = ?", docNo).
		First(&order).
		Error
	return &order, err
}

func (dao *BojunRetailOrderDAO) FindByID(ctx context.Context, id uint) (*model.BojunRetailOrder, error) {
	var order model.BojunRetailOrder
	err := dao.db.WithContext(ctx).First(&order, id).Error
	return &order, err
}

func (dao *BojunRetailOrderDAO) Update(ctx context.Context, order *model.BojunRetailOrder) error {
	order.UpdatedAt = int(time.Now().Unix())
	return dao.db.WithContext(ctx).Save(order).Error
}

func (dao *BojunRetailOrderDAO) CreateOrUpdate(ctx context.Context, order *model.BojunRetailOrder) error {
	_, err := dao.CreateOrUpdateWithCreated(ctx, order)
	return err
}

func (dao *BojunRetailOrderDAO) CreateOrUpdateWithCreated(ctx context.Context, order *model.BojunRetailOrder) (bool, error) {
	existing, err := dao.FindByDocNo(ctx, order.DocNo)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			_, err = dao.Create(ctx, order)
			return err == nil, err
		}
		return false, err
	}

	order.ID = existing.ID
	order.CreatedAt = existing.CreatedAt
	return false, dao.Update(ctx, order)
}

func (dao *BojunRetailOrderDAO) UpdateSyncStatus(ctx context.Context, id uint, synced int) error {
	return dao.db.WithContext(ctx).
		Model(&model.BojunRetailOrder{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"synced":     synced,
			"updated_at": time.Now().Unix(),
		}).
		Error
}
