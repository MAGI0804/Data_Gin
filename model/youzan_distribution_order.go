package model

// YouzanDistributionOrder stores orders pulled with the Youzan distribution credentials.
// The full upstream payload is retained so new business fields can be extracted later.
type YouzanDistributionOrder struct {
	BaseModel

	TID         string      `gorm:"column:tid;size:100;not null;unique" json:"tid"`
	Status      string      `gorm:"column:status;size:50;index" json:"status"`
	StatusStr   string      `gorm:"column:status_str;size:100" json:"status_str"`
	ShopName    string      `gorm:"column:shop_name;size:255;index" json:"shop_name"`
	NodeKdtID   int64       `gorm:"column:node_kdt_id;index" json:"node_kdt_id"`
	RootKdtID   int64       `gorm:"column:root_kdt_id;index" json:"root_kdt_id"`
	Payment     string      `gorm:"column:payment;size:50" json:"payment"`
	SuccessTime *TimeNormal `gorm:"column:success_time;index" json:"success_time"`
	CreatedTime *TimeNormal `gorm:"column:created_time;index" json:"created_time"`

	FansNicknameEncrypted string `gorm:"column:fans_nickname_encrypted;type:text" json:"fans_nickname_encrypted"`
	FansNickname          string `gorm:"column:fans_nickname;type:text" json:"fans_nickname"`

	ItemsJSON      string `gorm:"column:items_json;type:json" json:"items_json"`
	RawContentJSON string `gorm:"column:raw_content_json;type:json;not null" json:"raw_content_json"`

	CommonTimestampsField
}

func (YouzanDistributionOrder) TableName() string {
	return "youzan_distribution_orders"
}
