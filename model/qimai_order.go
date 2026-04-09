package model

import (
	"time"

	"gorm.io/gorm"
)

// QIMAI_ORDER_DATA 企迈订单数据模型
type QIMAI_ORDER_DATA struct {
	ID              uint           `gorm:"primaryKey" json:"id"`
	RawDataID       uint           `gorm:"column:raw_data_id;not null" json:"rawDataId"`
	OrderType       int            `gorm:"column:order_type;not null" json:"orderType"`                        // 订单类型：11=堂食;12=自提、外带;13=外卖(餐饮)
	Source          int            `gorm:"column:source;not null" json:"source"`                               // 渠道类型：1=微信;2=支付宝;8=三方pos;9=代客点单;17=自助点餐屏;19=企迈门店助手;20=云闪付小程序
	UserID          string         `gorm:"column:user_id;size:50;null" json:"userId"`                          // 用户id
	MultiStoreID    string         `gorm:"column:multi_store_id;size:50;null" json:"multiStoreId"`             // 门店id
	Scene           string         `gorm:"column:scene;size:50;null" json:"scene"`                             // 订单场景值
	Status          string         `gorm:"column:status;size:50;null" json:"status"`                           // 订单状态
	OrderAt         time.Time      `gorm:"column:order_at;null" json:"orderAt"`                                // orderAt
	PayStatus       int            `gorm:"column:pay_status;not null;default:0" json:"payStatus"`              // 支付状态, 0：未支付 1:已支付
	OrderNo         string         `gorm:"column:order_no;size:100;not null;primaryKey" json:"orderNo"`        // 业务订单号
	PayNo           string         `gorm:"column:pay_no;size:100;null" json:"payNo"`                           // 交易订单号
	ThirdPayNo      string         `gorm:"column:third_pay_no;size:100;null" json:"thirdPayNo"`                // 官方交易流水号
	StoreOrderNo    string         `gorm:"column:store_order_no;size:50;null" json:"storeOrderNo"`             // 取餐号
	CreatedAt       time.Time      `gorm:"column:created_at;autoCreateTime" json:"createdAt"`                  // 下单时间
	CompletedAt     time.Time      `gorm:"column:completed_at;null" json:"completedAt"`                        // 订单完成时间
	TotalAmount     int            `gorm:"column:total_amount;not null;default:0" json:"totalAmount"`          // 订单总额，单位：分
	ActualAmount    int            `gorm:"column:actual_amount;not null;default:0" json:"actualAmount"`        // 实付金额，单位：分
	DiscountAmount  int            `gorm:"column:discount_amount;not null;default:0" json:"discountAmount"`    // 优惠总额，单位：分
	ItemAmount      int            `gorm:"column:item_amount;not null;default:0" json:"itemAmount"`            // 商品总额，单位：分
	PackAmount      int            `gorm:"column:pack_amount;not null;default:0" json:"packAmount"`            // 包装费总额，单位：分
	FreightAmount   int            `gorm:"column:freight_amount;not null;default:0" json:"freightAmount"`      // 配送费，单位：分
	CommisionAmount int            `gorm:"column:commision_amount;not null;default:0" json:"commision_amount"` // 平台服务费，单位：分
	InCome          int            `gorm:"column:in_come;not null;default:0" json:"inCome"`                    // 商家实收，单位：分
	BuyerRemarks    string         `gorm:"column:buyer_remarks;type:text;null" json:"buyerRemarks"`            // 用户备注说明
	ShopCode        string         `gorm:"column:shop_code;size:50;null" json:"shopCode"`                      // 门店编码
	ShopName        string         `gorm:"column:shop_name;size:100;null" json:"shopName"`                     // 门店名称
	Addr            AddrInfo       `gorm:"embedded;embeddedPrefix:addr_" json:"addr"`                          // 订单收货信息
	ItemList        []Item         `gorm:"-" json:"itemList"`                                                  // 商品列表
	PayList         []PayInfo      `gorm:"-" json:"payList"`                                                   // 支付信息
	DiscountList    []DiscountInfo `gorm:"-" json:"discountList"`                                              // 优惠信息
	ContactTel      string         `gorm:"column:contact_tel;size:20;null" json:"contactTel"`                  // 下单人手机号(脱敏)，如果有
	ContactName     string         `gorm:"column:contact_name;size:50;null" json:"contactName"`                // 下单人姓名、昵称
}

// AddrInfo 订单收货信息
type AddrInfo struct {
	AcceptName   string `gorm:"column:accept_name;size:50;null" json:"acceptName"`     // 收货人
	Mobile       string `gorm:"column:mobile;size:20;null" json:"mobile"`              // 联系电话(脱敏后)
	Sex          int    `gorm:"column:sex;not null;default:0" json:"sex"`              // 性别：0=未知；1=男；2=女
	ProvinceName string `gorm:"column:province_name;size:50;null" json:"provinceName"` // 收货人地址省份名称
	CityName     string `gorm:"column:city_name;size:50;null" json:"cityName"`         // 收货人地市名称
	AreaName     string `gorm:"column:area_name;size:50;null" json:"areaName"`         // 收货人县区名称
	AcceptAddr   string `gorm:"column:accept_addr;size:255;null" json:"acceptAddr"`    // 详细地址
}

// Item 商品信息
type Item struct {
	ItemName        string        `json:"itemName"`        // 商品名称
	ItemSign        string        `json:"itemSign"`        // 商品标识、编码
	ItemType        int           `json:"itemType"`        // 商品类型：0:普通商品 1:套餐商品 2:套餐子商品
	ItemUnit        string        `json:"itemUnit"`        // 商品单位
	ItemSpec        []ItemSpec    `json:"itemSpec"`        // 商品规格
	Num             string        `json:"num"`             // 购买数量
	PackAmount      int           `json:"packAmount"`      // 包装费、餐盒费。单位：分
	ItemPrice       int           `json:"itemPrice"`       // 商品成交价。包含附属商品价格的总价。单位：分
	AttachGoodsList []AttachGoods `json:"attachGoodsList"` // 附属商品列表加料
	SetMealItemList []Item        `json:"setMealItemList"` // 套餐⼦商品列表。重复itemList格式
}

// ItemSpec 商品规格
type ItemSpec struct {
	Name  string `json:"name"`  // 规格名称
	Value string `json:"value"` // 规格值
}

// AttachGoods 附属商品信息
type AttachGoods struct {
	Name  string `json:"name"`  // 商品名称
	Code  string `json:"code"`  // 加料商品编码
	Num   int    `json:"num"`   // 购买数量,单个商品的数量,不是单行总数
	Price int    `json:"price"` // 价格。单位：分
}

// PayInfo 支付信息
type PayInfo struct {
	PayType   int `json:"payType"`   // 支付方式：1=微信支付;2=支付宝支付;4=余额支付;16=免支付
	PayAmount int `json:"payAmount"` // 支付金额单位：分
}

// DiscountInfo 优惠信息
type DiscountInfo struct {
	DiscountName    string `json:"discountName"`    // 优惠名称
	DiscountType    int    `json:"discountType"`    // 优惠类型：枚举值(餐饮)说明如下： 2优惠券 3满减 4红包 5新客专项 6礼品卡 7伙伴优惠 8满赠 9支付优惠 10会员优惠 11新客专享 12第M份N折 13限时折扣 14特惠加购 20积分商品优惠 21积分抵现 22付费会员卡配送费折扣 23免排队券 24减配送费活动 25拼团活动优惠 29单品折扣优惠-POS 30整单折扣 31线下优惠 35接龙活动优惠 40其他支付记录 50风味卡优惠
	DiscountSummary string `json:"discountSummary"` // 优惠描述
	DiscountLevel   int    `json:"discountLevel"`   // 优惠级别
	DiscountAmount  int    `json:"discountAmount"`  // 优惠金额。单位：分
}

// TableName 设置表名
func (QIMAI_ORDER_DATA) TableName() string {
	return "qimai_order_data"
}

// BeforeSave GORM钩子，确保gorm包被使用
func (q *QIMAI_ORDER_DATA) BeforeSave(*gorm.DB) error {
	return nil
}
