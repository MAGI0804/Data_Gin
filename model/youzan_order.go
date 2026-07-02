package model

import (
	"time"
)

// YOUZAN_ORDER_DATA 有赞订单数据表模型
// 用于存储从有赞API获取的订单数据
type YOUZAN_ORDER_DATA struct {
	ID              uint       `gorm:"primaryKey" json:"id"`                          // 主键ID
	CreatedAt       time.Time  `gorm:"column:created_at;autoCreateTime" json:"createdAt"` // 创建时间
	UpdatedAt       time.Time  `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"` // 更新时间

	// 订单基础信息
	TraceID         string     `gorm:"column:trace_id;size:100;null" json:"traceId"`           // 追踪ID
	TID             string     `gorm:"column:tid;size:100;not null" json:"tid"`               // 订单编号
	Status          string     `gorm:"column:status;size:50;null" json:"status"`               // 订单状态
	StatusStr       string     `gorm:"column:status_str;size:50;null" json:"statusStr"`       // 订单状态描述
	Type            int        `gorm:"column:type;not null;default:0" json:"type"`             // 订单类型
	ShopName        string     `gorm:"column:shop_name;size:200;null" json:"shopName"`         // 店铺名称
	NodeKdtID       int64      `gorm:"column:node_kdt_id;not null;default:0" json:"nodeKdtId"` // 店铺ID
	RootKdtID       int64      `gorm:"column:root_kdt_id;not null;default:0" json:"rootKdtId"` // 根店铺ID

	// 支付信息
	PayType         int        `gorm:"column:pay_type;not null;default:0" json:"payType"`         // 支付类型
	PayTypeStr      string     `gorm:"column:pay_type_str;size:50;null" json:"payTypeStr"`       // 支付类型描述
	PayTime         *time.Time `gorm:"column:pay_time;null" json:"payTime"`                     // 支付时间
	ConfirmTime     *time.Time `gorm:"column:confirm_time;null" json:"confirmTime"`             // 确认时间
	ConsignTime     *time.Time `gorm:"column:consign_time;null" json:"consignTime"`             // 发货时间
	SuccessTime     *time.Time `gorm:"column:success_time;null" json:"successTime"`             // 完成时间

	// 订单状态
	CloseType       int        `gorm:"column:close_type;not null;default:0" json:"closeType"`       // 关闭类型
	RefundState     int        `gorm:"column:refund_state;not null;default:0" json:"refundState"`   // 退款状态
	ExpressType     int        `gorm:"column:express_type;not null;default:0" json:"expressType"`   // 配送类型
	IsRetailOrder   bool       `gorm:"column:is_retail_order;not null;default:false" json:"isRetailOrder"` // 是否零售订单
	TeamType        int        `gorm:"column:team_type;not null;default:0" json:"teamType"`         // 团队类型

	// 时间信息
	ExpiredTime     *time.Time `gorm:"column:expired_time;null" json:"expiredTime"`         // 过期时间
	UpdateTime      *time.Time `gorm:"column:update_time;null" json:"updateTime"`          // 更新时间
	Created         *time.Time `gorm:"column:created;null" json:"created"`                  // 创建时间
	SerialNo        string     `gorm:"column:serial_no;size:50;null" json:"serialNo"`       // 序列号

	// 订单标签
	IsMember        bool       `gorm:"column:is_member;not null;default:false" json:"isMember"`       // 是否会员
	IsSettle        bool       `gorm:"column:is_settle;not null;default:false" json:"isSettle"`       // 是否已结算
	IsRefund        bool       `gorm:"column:is_refund;not null;default:false" json:"isRefund"`       // 是否退款
	IsPayed         bool       `gorm:"column:is_payed;not null;default:false" json:"isPayed"`         // 是否已支付

	// 金额信息
	TotalFee           float64    `gorm:"column:total_fee;not null;default:0" json:"totalFee"`         // 订单总金额
	Payment            float64    `gorm:"column:payment;not null;default:0" json:"payment"`             // 实际支付金额
	PostFee            float64    `gorm:"column:post_fee;not null;default:0" json:"postFee"`           // 运费
	TotalAmt           float64    `gorm:"column:total_amt;not null;default:0" json:"totalAmt"`         // 调整后金额（退款为负）
	AdjustmentPayment  float64    `gorm:"column:adjustment_payment;not null;default:0" json:"adjustmentPayment"` // 去掉储值后的金额（未调整则等于payment）

	// 扩展字段
	CashierID       string     `gorm:"column:cashier_id;size:50;null" json:"cashierId"`       // 收银员ID
	PayEndTime      string     `gorm:"column:pay_end_time;size:50;null" json:"payEndTime"`   // 支付结束时间

	// 来源信息
	Platform        string     `gorm:"column:platform;size:50;null" json:"platform"`               // 平台
	WxEntrance      string     `gorm:"column:wx_entrance;size:50;null" json:"wxEntrance"`         // 微信入口
	OrderMark       string     `gorm:"column:order_mark;size:50;null" json:"orderMark"`           // 订单标记
	IsOfflineOrder  bool       `gorm:"column:is_offline_order;not null;default:false" json:"isOfflineOrder"` // 是否线下订单

	// 买家信息
	BuyerPhone      string     `gorm:"column:buyer_phone;size:50;null" json:"buyerPhone"`       // 买家手机号
	FansNickname    string     `gorm:"column:fans_nickname;size:100;null" json:"fansNickname"`   // 买家昵称
	FansID          int64      `gorm:"column:fans_id;not null;default:0" json:"fansId"`         // 买家ID
	FansType        int        `gorm:"column:fans_type;not null;default:0" json:"fansType"`     // 买家类型

	// 收货地址
	DeliveryProvince string    `gorm:"column:delivery_province;size:50;null" json:"deliveryProvince"` // 省
	DeliveryCity     string    `gorm:"column:delivery_city;size:50;null" json:"deliveryCity"`         // 市
	DeliveryDistrict string    `gorm:"column:delivery_district;size:50;null" json:"deliveryDistrict"` // 区
	ReceiverName     string    `gorm:"column:receiver_name;size:50;null" json:"receiverName"`         // 收件人姓名
	ReceiverTel      string    `gorm:"column:receiver_tel;size:50;null" json:"receiverTel"`           // 收件人电话

	// JSON字段（存储复杂结构）
	ItemsJSON             string `gorm:"column:items;type:text;null" json:"items"`                     // 商品列表JSON
	TransactionJSON       string `gorm:"column:transaction;type:text;null" json:"transaction"`         // 交易号列表JSON
	OuterTransactionsJSON string `gorm:"column:outer_transactions;type:text;null" json:"outerTransactions"` // 外部交易号列表JSON

	// 同步状态
	Synced          int        `gorm:"column:synced;not null;default:0" json:"synced"` // 是否已同步到销售系统：0=未同步；1=已同步
}

// TableName 指定表名
func (YOUZAN_ORDER_DATA) TableName() string {
	return "youzan_order_data"
}

// YouzanOrderItem 订单商品项
type YouzanOrderItem struct {
	OID                string  `json:"oid"`                 // 子订单ID
	ItemID             int64   `json:"item_id"`             // 商品ID
	SkuID              int64   `json:"sku_id"`              // SKU ID
	Title              string  `json:"title"`               // 商品名称
	ItemNo             string  `json:"item_no"`             // 商品编号
	SkuNo              string  `json:"sku_no"`              // SKU编号
	OuterItemID        string  `json:"outer_item_id"`       // 外部商品ID
	OuterSkuID         string  `json:"outer_sku_id"`        // 外部SKU ID
	Alias              string  `json:"alias"`               // 商品别名
	Price              float64 `json:"price"`               // 单价
	DiscountPrice      float64 `json:"discount_price"`      // 折扣价
	Payment            float64 `json:"payment"`             // 实付金额
	TotalFee           float64 `json:"total_fee"`           // 商品总价
	Num                int     `json:"num"`                 // 数量
	GoodsURL           string  `json:"goods_url"`           // 商品链接
	PicPath            string  `json:"pic_path"`            // 商品图片
	SkuPropertiesName  string  `json:"sku_properties_name"` // SKU属性名称JSON
}
