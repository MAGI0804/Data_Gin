// package model 原始数据模型
package model

import (
	"time"
)

// RawData 原始数据模型
// 存储从外部API获取或接收的原始数据
type RawData struct {
	*BaseModel

	// 数据源ID
	DataSourceID uint `gorm:"column:data_source_id;not null" json:"data_source_id"`
	// 外部系统ID
	ExternalID string `gorm:"column:external_id;size:255" json:"external_id"`
	// 数据类型
	DataType string `gorm:"column:data_type;size:50;not null" json:"data_type"`
	// 原始数据内容，使用JSON格式存储
	RawContent string `gorm:"column:raw_content;type:json;not null" json:"raw_content"`
	// 元数据（来源、时间戳等），使用JSON格式存储
	Metadata string `gorm:"column:metadata;type:json" json:"metadata"`
	// 状态: pending/processing/processed/error
	Status string `gorm:"column:status;type:enum('pending','processing','processed','error');default:'pending'" json:"status"`
	// 错误信息
	ErrorMessage string `gorm:"column:error_message;type:text" json:"error_message"`
	// 处理完成时间
	ProcessedAt int `gorm:"column:processed_at;default:0" json:"processed_at"`
	// 备注
	Remark string `gorm:"column:remark;size:255" json:"remark"`
	// 来源
	Source string `gorm:"column:source;size:100" json:"source"`
	// 客户端IP
	ClientIP string `gorm:"column:client_ip;size:50" json:"client_ip"`
	// 接收时间，格式：yyyy-mm-dd hh:mm:ss
	IngestedAt *time.Time `gorm:"column:ingested_at" json:"ingested_at"`

	*CommonTimestampsField
}

// TableName 指定表名
func (RawData) TableName() string {
	return "raw_data"
}
