package data_dao

import (
	"context"

	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"

	"gorm.io/gorm"
)

// QimaiDataDAO qimai_order_data表的DAO
type QimaiDataDAO struct {
	db *gorm.DB
}

// NewQimaiDataDAO 创建QimaiDataDAO实例
func NewQimaiDataDAO() *QimaiDataDAO {
	return &QimaiDataDAO{
		db: database.DB,
	}
}

// Create 创建企迈订单数据
func (dao *QimaiDataDAO) Create(ctx context.Context, qimaiData *model.QIMAI_ORDER_DATA) (uint, error) {
	result := dao.db.WithContext(ctx).Create(qimaiData)
	return qimaiData.ID, result.Error
}

// Update 更新企迈订单数据
func (dao *QimaiDataDAO) Update(ctx context.Context, qimaiData *model.QIMAI_ORDER_DATA) error {
	return dao.db.WithContext(ctx).Save(qimaiData).Error
}

// FindByID 根据ID查询企迈订单数据
func (dao *QimaiDataDAO) FindByID(ctx context.Context, id uint) (*model.QIMAI_ORDER_DATA, error) {
	var qimaiData model.QIMAI_ORDER_DATA
	err := dao.db.WithContext(ctx).First(&qimaiData, id).Error
	if err != nil {
		return nil, err
	}
	return &qimaiData, nil
}

// FindByOrderNo 根据订单号查询企迈订单数据
func (dao *QimaiDataDAO) FindByOrderNo(ctx context.Context, orderNo string) (*model.QIMAI_ORDER_DATA, error) {
	var qimaiData model.QIMAI_ORDER_DATA
	err := dao.db.WithContext(ctx).Where("order_no = ?", orderNo).First(&qimaiData).Error
	if err != nil {
		return nil, err
	}
	return &qimaiData, nil
}

// FindAll 查询所有企迈订单数据
func (dao *QimaiDataDAO) FindAll(ctx context.Context) ([]model.QIMAI_ORDER_DATA, error) {
	var qimaiDataList []model.QIMAI_ORDER_DATA
	err := dao.db.WithContext(ctx).Find(&qimaiDataList).Error
	return qimaiDataList, err
}

// Delete 删除企迈订单数据
func (dao *QimaiDataDAO) Delete(ctx context.Context, id uint) error {
	return dao.db.WithContext(ctx).Delete(&model.QIMAI_ORDER_DATA{}, id).Error
}
