package model

import (
	"time"
)

// YOUZAN_RETURN_DATA 有赞退款订单数据表模型
// 用于存储从有赞API获取的退款订单数据
type YOUZAN_RETURN_DATA struct {
	ID              uint       `gorm:"primaryKey" json:"id"`                          // 主键ID
	CreatedAt       time.Time  `gorm:"column:created_at;autoCreateTime" json:"createdAt"` // 创建时间
	UpdatedAt       time.Time  `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"` // 更新时间

	// 退款基础信息
	TraceID         string     `gorm:"column:trace_id;size:100;null" json:"traceId"`           // 追踪ID
	RefundID        string     `gorm:"column:refund_id;size:100;not null" json:"refundId"`     // 退款ID
	TID             string     `gorm:"column:tid;size:100;not null" json:"tid"`               // 原订单编号
	Status          string     `gorm:"column:status;size:50;null" json:"status"`               // 退款状态（SUCCESS/FAIL等）
	NodeKdtID       int64      `gorm:"column:node_kdt_id;not null;default:0" json:"nodeKdtId"` // 店铺ID
	KdtID           int64      `gorm:"column:kdt_id;not null;default:0" json:"kdtId"`         // 根店铺ID

	// 退款原因和类型
	Reason          int        `gorm:"column:reason;not null;default:0" json:"reason"`             // 退款原因
	ReturnGoods     bool       `gorm:"column:return_goods;not null;default:false" json:"returnGoods"` // 是否退货

	// 状态信息
	CSStatus        int        `gorm:"column:cs_status;not null;default:0" json:"csStatus"`           // 客服状态
	DeliveryStatus  int        `gorm:"column:delivery_status;not null;default:0" json:"deliveryStatus"` // 配送状态

	// 金额信息
	RefundFee       float64    `gorm:"column:refund_fee;not null;default:0" json:"refundFee"`       // 退款金额

	// 时间信息
	Created         *time.Time `gorm:"column:created;null" json:"created"`                  // 退款创建时间
	Modified        *time.Time `gorm:"column:modified;null" json:"modified"`                // 退款修改时间

	// 同步状态
	Synced          int        `gorm:"column:synced;not null;default:0" json:"synced"` // 是否已同步到销售系统：0=未同步；1=已同步
}

// TableName 指定表名
func (YOUZAN_RETURN_DATA) TableName() string {
	return "youzan_return_data"
}
