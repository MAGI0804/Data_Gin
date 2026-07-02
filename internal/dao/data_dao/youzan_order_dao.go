package data_dao

import (
	"context"
	"time"

	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"

	"gorm.io/gorm"
)

type YouzanOrderDAO struct {
	db *gorm.DB
}

func NewYouzanOrderDAO() *YouzanOrderDAO {
	return &YouzanOrderDAO{
		db: database.DB,
	}
}

func (dao *YouzanOrderDAO) Create(ctx context.Context, order *model.YOUZAN_ORDER_DATA) (uint, error) {
	result := dao.db.WithContext(ctx).Create(order)
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected == 0 {
		return 0, gorm.ErrRecordNotFound
	}
	return order.ID, nil
}

func (dao *YouzanOrderDAO) Update(ctx context.Context, order *model.YOUZAN_ORDER_DATA) error {
	updates := map[string]interface{}{
		"updated_at":         order.UpdatedAt,
		"trace_id":           order.TraceID,
		"tid":                order.TID,
		"status":             order.Status,
		"status_str":         order.StatusStr,
		"type":               order.Type,
		"shop_name":          order.ShopName,
		"node_kdt_id":        order.NodeKdtID,
		"root_kdt_id":        order.RootKdtID,
		"pay_type":           order.PayType,
		"pay_type_str":       order.PayTypeStr,
		"pay_time":           order.PayTime,
		"confirm_time":       order.ConfirmTime,
		"consign_time":       order.ConsignTime,
		"success_time":       order.SuccessTime,
		"close_type":         order.CloseType,
		"refund_state":       order.RefundState,
		"express_type":       order.ExpressType,
		"is_retail_order":    order.IsRetailOrder,
		"team_type":          order.TeamType,
		"expired_time":       order.ExpiredTime,
		"update_time":        order.UpdateTime,
		"created":            order.Created,
		"serial_no":          order.SerialNo,
		"is_member":          order.IsMember,
		"is_settle":          order.IsSettle,
		"is_refund":          order.IsRefund,
		"is_payed":           order.IsPayed,
		"total_fee":          order.TotalFee,
		"payment":            order.Payment,
		"post_fee":           order.PostFee,
		"total_amt":          order.TotalAmt,
		"adjustment_payment": order.AdjustmentPayment,
		"cashier_id":         order.CashierID,
		"pay_end_time":       order.PayEndTime,
		"platform":           order.Platform,
		"wx_entrance":        order.WxEntrance,
		"order_mark":         order.OrderMark,
		"is_offline_order":   order.IsOfflineOrder,
		"buyer_phone":        order.BuyerPhone,
		"fans_nickname":      order.FansNickname,
		"fans_id":            order.FansID,
		"fans_type":          order.FansType,
		"delivery_province":  order.DeliveryProvince,
		"delivery_city":      order.DeliveryCity,
		"delivery_district":  order.DeliveryDistrict,
		"receiver_name":      order.ReceiverName,
		"receiver_tel":       order.ReceiverTel,
	}

	if order.UpdatedAt.IsZero() {
		updates["updated_at"] = time.Now()
	}

	result := dao.db.WithContext(ctx).Model(&model.YOUZAN_ORDER_DATA{}).Where("id = ?", order.ID).Updates(updates)
	return result.Error
}

func (dao *YouzanOrderDAO) FindByID(ctx context.Context, id uint) (*model.YOUZAN_ORDER_DATA, error) {
	var order model.YOUZAN_ORDER_DATA
	err := dao.db.WithContext(ctx).First(&order, id).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func (dao *YouzanOrderDAO) FindByTID(ctx context.Context, tid string) (*model.YOUZAN_ORDER_DATA, error) {
	var order model.YOUZAN_ORDER_DATA
	err := dao.db.WithContext(ctx).Where("tid = ?", tid).First(&order).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func (dao *YouzanOrderDAO) FindAll(ctx context.Context) ([]model.YOUZAN_ORDER_DATA, error) {
	var orders []model.YOUZAN_ORDER_DATA
	err := dao.db.WithContext(ctx).Find(&orders).Error
	return orders, err
}

func (dao *YouzanOrderDAO) FindByStatus(ctx context.Context, status string) ([]model.YOUZAN_ORDER_DATA, error) {
	var orders []model.YOUZAN_ORDER_DATA
	err := dao.db.WithContext(ctx).Where("status = ?", status).Find(&orders).Error
	return orders, err
}

func (dao *YouzanOrderDAO) FindUnsynced(ctx context.Context) ([]model.YOUZAN_ORDER_DATA, error) {
	var orders []model.YOUZAN_ORDER_DATA
	err := dao.db.WithContext(ctx).Where("synced = ?", 0).Find(&orders).Error
	return orders, err
}

func (dao *YouzanOrderDAO) FindUnsyncedByNodeKdtID(ctx context.Context, nodeKdtID int64) ([]model.YOUZAN_ORDER_DATA, error) {
	var orders []model.YOUZAN_ORDER_DATA
	err := dao.db.WithContext(ctx).Where("synced = 0 AND node_kdt_id = ? AND status = ?", nodeKdtID, "TRADE_SUCCESS").Find(&orders).Error
	return orders, err
}

func (dao *YouzanOrderDAO) MarkAsSynced(ctx context.Context, id uint) error {
	return dao.db.WithContext(ctx).Model(&model.YOUZAN_ORDER_DATA{}).Where("id = ?", id).Update("synced", 1).Error
}

func (dao *YouzanOrderDAO) Delete(ctx context.Context, id uint) error {
	return dao.db.WithContext(ctx).Delete(&model.YOUZAN_ORDER_DATA{}, id).Error
}

func (dao *YouzanOrderDAO) CreateOrUpdate(ctx context.Context, order *model.YOUZAN_ORDER_DATA) error {
	existing, err := dao.FindByTID(ctx, order.TID)
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
	order.UpdatedAt = time.Now()
	return dao.Update(ctx, order)
}

func (dao *YouzanOrderDAO) CountByTID(ctx context.Context, tid string) (int64, error) {
	var count int64
	err := dao.db.WithContext(ctx).Model(&model.YOUZAN_ORDER_DATA{}).Where("tid = ?", tid).Count(&count).Error
	return count, err
}
