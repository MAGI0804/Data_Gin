// package model 数据源配置模型
package model

// DataSource 数据源配置模型
// 存储外部API的配置信息，用于数据采集
type DataSource struct {
	*BaseModel

	// 数据源名称
	Name string `gorm:"column:name;size:100;not null" json:"name"`
	// 数据类型: api/database/file
	Type string `gorm:"column:type;size:50;not null" json:"type"`
	// 连接配置（API地址、认证信息等），使用JSON格式存储
	Config string `gorm:"column:config;type:json;not null" json:"config"`
	// 采集计划（cron表达式）
	Schedule string `gorm:"column:schedule;size:100" json:"schedule"`
	// 状态: active/inactive/error
	Status string `gorm:"column:status;type:enum('active','inactive','error');default:'active'" json:"status"`
	// 最后同步时间
	LastSyncTime *TimeNormal `gorm:"column:last_sync_time" json:"last_sync_time"`
	// 最后同步状态
	LastSyncStatus string `gorm:"column:last_sync_status;size:20" json:"last_sync_status"`

	*CommonTimestampsField
}

// TableName 指定表名
func (DataSource) TableName() string {
	return "data_sources"
}
