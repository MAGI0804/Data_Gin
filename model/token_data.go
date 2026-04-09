package model

import (
	"time"
)

// TokenData 各平台的token信息
type TokenData struct {
	ID               uint        `gorm:"primaryKey" json:"id"`
	PlatformName     string      `gorm:"size:100;not null" json:"platform_name"`
	AccountName      string      `gorm:"size:100;not null" json:"account_name"`
	VerificationInfo interface{} `gorm:"type:text" json:"verification_info"` // JSON格式存储
	Remark           string      `gorm:"type:text" json:"remark"`
	CreatedAt        time.Time   `json:"created_at"`
	UpdatedAt        time.Time   `json:"updated_at"`
}

// TableName 指定表名
func (TokenData) TableName() string {
	return "token_data"
}
