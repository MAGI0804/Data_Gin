package data_dao

import (
	"context"
	"time"

	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"

	"gorm.io/gorm"
)

type YouzanReturnDAO struct {
	db *gorm.DB
}

func NewYouzanReturnDAO() *YouzanReturnDAO {
	return &YouzanReturnDAO{
		db: database.DB,
	}
}

func (dao *YouzanReturnDAO) Create(ctx context.Context, order *model.YOUZAN_RETURN_DATA) (uint, error) {
	result := dao.db.WithContext(ctx).Create(order)
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected == 0 {
		return 0, gorm.ErrRecordNotFound
	}
	return order.ID, nil
}

func (dao *YouzanReturnDAO) Update(ctx context.Context, order *model.YOUZAN_RETURN_DATA) error {
	updates := map[string]interface{}{
		"updated_at":   order.UpdatedAt,
		"trace_id":    order.TraceID,
		"refund_id":   order.RefundID,
		"tid":         order.TID,
		"status":      order.Status,
		"node_kdt_id": order.NodeKdtID,
		"kdt_id":      order.KdtID,
		"reason":      order.Reason,
		"return_goods": order.ReturnGoods,
		"cs_status":     order.CSStatus,
		"delivery_status": order.DeliveryStatus,
		"refund_fee":    order.RefundFee,
		"created":        order.Created,
		"modified":       order.Modified,
	}

	if order.UpdatedAt.IsZero() {
		updates["updated_at"] = time.Now()
	}

	result := dao.db.WithContext(ctx).Model(&model.YOUZAN_RETURN_DATA{}).Where("id = ?", order.ID).Updates(updates)
	return result.Error
}

func (dao *YouzanReturnDAO) FindByID(ctx context.Context, id uint) (*model.YOUZAN_RETURN_DATA, error) {
	var order model.YOUZAN_RETURN_DATA
	err := dao.db.WithContext(ctx).First(&order, id).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func (dao *YouzanReturnDAO) FindByTID(ctx context.Context, tid string) (*model.YOUZAN_RETURN_DATA, error) {
	var order model.YOUZAN_RETURN_DATA
	err := dao.db.WithContext(ctx).Where("tid = ?", tid).First(&order).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func (dao *YouzanReturnDAO) FindByRefundID(ctx context.Context, refundID string) (*model.YOUZAN_RETURN_DATA, error) {
	var order model.YOUZAN_RETURN_DATA
	err := dao.db.WithContext(ctx).Where("refund_id = ?", refundID).First(&order).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func (dao *YouzanReturnDAO) FindAll(ctx context.Context) ([]model.YOUZAN_RETURN_DATA, error) {
	var orders []model.YOUZAN_RETURN_DATA
	err := dao.db.WithContext(ctx).Find(&orders).Error
	return orders, err
}

func (dao *YouzanReturnDAO) FindByStatus(ctx context.Context, status string) ([]model.YOUZAN_RETURN_DATA, error) {
	var orders []model.YOUZAN_RETURN_DATA
	err := dao.db.WithContext(ctx).Where("status = ?", status).Find(&orders).Error
	return orders, err
}

func (dao *YouzanReturnDAO) FindUnsynced(ctx context.Context) ([]model.YOUZAN_RETURN_DATA, error) {
	var orders []model.YOUZAN_RETURN_DATA
	err := dao.db.WithContext(ctx).Where("synced = ?", 0).Find(&orders).Error
	return orders, err
}

func (dao *YouzanReturnDAO) FindUnsyncedByNodeKdtIDAndStatus(ctx context.Context, nodeKdtID int64, status string) ([]model.YOUZAN_RETURN_DATA, error) {
	var orders []model.YOUZAN_RETURN_DATA
	err := dao.db.WithContext(ctx).Where("synced = 0 AND node_kdt_id = ? AND status = ?", nodeKdtID, status).Find(&orders).Error
	return orders, err
}

func (dao *YouzanReturnDAO) FindUnsyncedByNodeKdtID(ctx context.Context, nodeKdtID int64) ([]model.YOUZAN_RETURN_DATA, error) {
	var orders []model.YOUZAN_RETURN_DATA
	err := dao.db.WithContext(ctx).Where("synced = 0 AND node_kdt_id = ?", nodeKdtID).Find(&orders).Error
	return orders, err
}

func (dao *YouzanReturnDAO) MarkAsSynced(ctx context.Context, id uint) error {
	return dao.db.WithContext(ctx).Model(&model.YOUZAN_RETURN_DATA{}).Where("id = ?", id).Update("synced", 1).Error
}

func (dao *YouzanReturnDAO) Delete(ctx context.Context, id uint) error {
	return dao.db.WithContext(ctx).Delete(&model.YOUZAN_RETURN_DATA{}, id).Error
}

func (dao *YouzanReturnDAO) CreateOrUpdate(ctx context.Context, order *model.YOUZAN_RETURN_DATA) error {
	existing, err := dao.FindByRefundID(ctx, order.RefundID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			_, err = dao.Create(ctx, order)
			return err
		}
		return err
	}

	if existing.Synced == 1 {
		return nil
	}

	order.ID = existing.ID
	order.CreatedAt = existing.CreatedAt
	return dao.Update(ctx, order)
}

func (dao *YouzanReturnDAO) CountByRefundID(ctx context.Context, refundID string) (int64, error) {
	var count int64
	err := dao.db.WithContext(ctx).Model(&model.YOUZAN_RETURN_DATA{}).Where("refund_id = ?", refundID).Count(&count).Error
	return count, err
}
