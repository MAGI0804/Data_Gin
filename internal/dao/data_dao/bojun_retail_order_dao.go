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

type OpenBojunOrderQuery struct {
	StartBillDate  int
	EndBillDate    int
	StoreCodes     []string
	OrderTypes     []string
	BeforeBillDate int
	BeforeID       uint
	Limit          int
}

func NewBojunRetailOrderDAO(databases ...*gorm.DB) *BojunRetailOrderDAO {
	db := database.DB
	if len(databases) > 0 && databases[0] != nil {
		db = databases[0]
	}
	return &BojunRetailOrderDAO{db: db}
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

func (dao *BojunRetailOrderDAO) ExistsByDocNo(ctx context.Context, docNo string) (bool, error) {
	var count int64
	err := dao.db.WithContext(ctx).
		Model(&model.BojunRetailOrder{}).
		Where("docno = ?", docNo).
		Count(&count).
		Error
	return count > 0, err
}

func (dao *BojunRetailOrderDAO) CreateIfNotExists(ctx context.Context, order *model.BojunRetailOrder) (bool, error) {
	exists, err := dao.ExistsByDocNo(ctx, order.DocNo)
	if err != nil {
		return false, err
	}
	if exists {
		return false, nil
	}

	_, err = dao.Create(ctx, order)
	return err == nil, err
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

func (dao *BojunRetailOrderDAO) ListOpenOrders(
	ctx context.Context,
	query OpenBojunOrderQuery,
) ([]model.BojunRetailOrder, error) {
	if dao == nil || dao.db == nil || ctx == nil || query.StartBillDate <= 0 ||
		query.EndBillDate < query.StartBillDate || len(query.StoreCodes) == 0 ||
		query.Limit <= 0 {
		return nil, gorm.ErrInvalidData
	}

	dbQuery := dao.db.WithContext(ctx).
		Model(&model.BojunRetailOrder{}).
		Select([]string{
			"id", "otherdocno", "docno", "billdate", "c_store_code", "c_store_name",
			"order_type_code", "order_type_name", "tot_lines", "tot_qty", "tot_amt_list",
			"tot_amt_actual", "avg_discount", "related_normal_docno", "items_json",
		}).
		Where("billdate BETWEEN ? AND ?", query.StartBillDate, query.EndBillDate).
		Where("c_store_code IN ?", query.StoreCodes)
	if len(query.OrderTypes) > 0 {
		dbQuery = dbQuery.Where("order_type_code IN ?", query.OrderTypes)
	}
	if query.BeforeBillDate > 0 && query.BeforeID > 0 {
		dbQuery = dbQuery.Where(
			"billdate < ? OR (billdate = ? AND id < ?)",
			query.BeforeBillDate,
			query.BeforeBillDate,
			query.BeforeID,
		)
	}

	orders := make([]model.BojunRetailOrder, 0)
	err := dbQuery.Order("billdate DESC").Order("id DESC").Limit(query.Limit).Find(&orders).Error
	return orders, err
}
