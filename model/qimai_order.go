package model

import (
	"time"
)

// QIMAI_ORDER_DATA 企迈订单数据模型
type QIMAI_ORDER_DATA struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	RawDataID       uint       `gorm:"column:raw_data_id;not null" json:"rawDataId"`
	OrderType       int        `gorm:"column:order_type;not null" json:"orderType"`                       // 订单类型：11=堂食;12=自提、外带;13=外卖(餐饮)
	Source          int        `gorm:"column:source;not null" json:"source"`                              // 渠道类型：1=微信;2=支付宝;8=三方pos;9=代客点单;17=自助点餐屏;19=企迈门店助手;20=云闪付小程序
	UserID          string     `gorm:"column:user_id;size:50;null" json:"userId"`                         // 用户id
	MultiStoreID    string     `gorm:"column:multi_store_id;size:50;null" json:"multiStoreId"`            // 门店id
	Scene           string     `gorm:"column:scene;size:50;null" json:"scene"`                            // 订单场景值
	Status          string     `gorm:"column:status;size:50;null" json:"status"`                          // 订单状态
	OrderAt         *time.Time `gorm:"column:order_at;null" json:"orderAt"`                               // orderAt
	PayStatus       int        `gorm:"column:pay_status;not null;default:0" json:"payStatus"`             // 支付状态, 0：未支付 1:已支付
	OrderNo         string     `gorm:"column:order_no;size:100;not null" json:"orderNo"`                  // 业务订单号
	PayNo           string     `gorm:"column:pay_no;size:100;null" json:"payNo"`                          // 交易订单号
	ThirdPayNo      string     `gorm:"column:third_pay_no;size:100;null" json:"thirdPayNo"`               // 官方交易流水号
	StoreOrderNo    string     `gorm:"column:store_order_no;size:50;null" json:"storeOrderNo"`            // 取餐号
	CreatedAt       time.Time  `gorm:"column:created_at;autoCreateTime" json:"createdAt"`                 // 下单时间
	CompletedAt     *time.Time `gorm:"column:completed_at;null" json:"completedAt"`                       // 订单完成时间
	TotalAmount     int        `gorm:"column:total_amount;not null;default:0" json:"totalAmount"`         // 订单总额，单位：分
	ActualAmount    int        `gorm:"column:actual_amount;not null;default:0" json:"actualAmount"`       // 实付金额，单位：分
	DiscountAmount  int        `gorm:"column:discount_amount;not null;default:0" json:"discountAmount"`   // 优惠总额，单位：分
	ItemAmount      int        `gorm:"column:item_amount;not null;default:0" json:"itemAmount"`           // 商品总额，单位：分
	PackAmount      int        `gorm:"column:pack_amount;not null;default:0" json:"packAmount"`           // 包装费总额，单位：分
	FreightAmount   int        `gorm:"column:freight_amount;not null;default:0" json:"freightAmount"`     // 配送费，单位：分
	CommisionAmount int        `gorm:"column:commision_amount;not null;default:0" json:"commisionAmount"` // 平台服务费，单位：分
	InCome          int        `gorm:"column:in_come;not null;default:0" json:"inCome"`                   // 商家实收，单位：分
	BuyerRemarks    string     `gorm:"column:buyer_remarks;type:text;null" json:"buyerRemarks"`           // 用户备注说明
	ShopCode        string     `gorm:"column:shop_code;size:50;null" json:"shopCode"`                     // 门店编码
	ShopName        string     `gorm:"column:shop_name;size:200;null" json:"shopName"`                    // 门店名称
	ContactTel      string     `gorm:"column:contact_tel;size:20;null" json:"contactTel"`                 // 下单人手机号(脱敏)
	ContactName     string     `gorm:"column:contact_name;size:50;null" json:"contactName"`               // 下单人姓名、昵称
	// 收货信息
	AddrAcceptName   string `gorm:"column:addr_accept_name;size:50;null" json:"addrAcceptName"`     // 收货人
	AddrMobile       string `gorm:"column:addr_mobile;size:20;null" json:"addrMobile"`              // 联系电话(脱敏后)
	AddrSex          int    `gorm:"column:addr_sex;not null;default:0" json:"addrSex"`              // 性别：0=未知；1=男；2=女
	AddrProvinceName string `gorm:"column:addr_province_name;size:50;null" json:"addrProvinceName"` // 收货人省份
	AddrCityName     string `gorm:"column:addr_city_name;size:50;null" json:"addrCityName"`         // 收货人地市
	AddrAreaName     string `gorm:"column:addr_area_name;size:50;null" json:"addrAreaName"`         // 收货人县区
	AddrAcceptAddr   string `gorm:"column:addr_accept_addr;size:255;null" json:"addrAcceptAddr"`    // 详细地址
	// 完整的JSON数据存储
	ItemListJSON     string `gorm:"column:item_list;type:text;null" json:"itemList"`         // 商品列表JSON
	PayListJSON      string `gorm:"column:pay_list;type:text;null" json:"payList"`           // 支付信息JSON
	DiscountListJSON string `gorm:"column:discount_list;type:text;null" json:"discountList"` // 优惠信息JSON
	ExtraDataJSON    string `gorm:"column:extra_data;type:text;null" json:"extraData"`       // 额外数据JSON
}

// TableName 设置表名
func (QIMAI_ORDER_DATA) TableName() string {
	return "qimai_order_data"
}
