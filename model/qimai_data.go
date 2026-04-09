package model

import (
	"time"
)

// QimaiData 企迈订单数据
type QimaiData struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	RawDataID    uint      `gorm:"not null" json:"raw_data_id"`
	OrderNo      string    `gorm:"size:100;not null" json:"order_no"`
	OrderDetails string    `gorm:"type:text" json:"order_details"` // JSON格式存储
	Status       string    `gorm:"size:50;not null" json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TableName 指定表名
func (QimaiData) TableName() string {
	return "qimai_data"
}
