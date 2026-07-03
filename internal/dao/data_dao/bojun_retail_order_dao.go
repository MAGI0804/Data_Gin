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

func (dao *BojunRetailOrderDAO) Update(ctx context.Context, order *model.BojunRetailOrder) error {
	order.UpdatedAt = int(time.Now().Unix())
	return dao.db.WithContext(ctx).Save(order).Error
}

func (dao *BojunRetailOrderDAO) CreateOrUpdate(ctx context.Context, order *model.BojunRetailOrder) error {
	existing, err := dao.FindByDocNo(ctx, order.DocNo)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			_, err = dao.Create(ctx, order)
			return err
		}
		return err
	}

	order.ID = existing.ID
	order.CreatedAt = existing.CreatedAt
	return dao.Update(ctx, order)
}
