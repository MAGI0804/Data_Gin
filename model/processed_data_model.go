// package model 处理结果模型
package model

// ProcessedData 处理结果模型
// 存储清洗和处理后的数据
type ProcessedData struct {
	*BaseModel

	// 原始数据ID
	RawDataID uint `gorm:"column:raw_data_id;not null" json:"raw_data_id"`
	// 数据类型
	DataType string `gorm:"column:data_type;size:50;not null" json:"data_type"`
	// 处理后的数据字段，使用JSON格式存储
	DataFields string `gorm:"column:data_fields;type:json;not null" json:"data_fields"`
	// 数据质量评分
	QualityScore float64 `gorm:"column:quality_score;type:decimal(5,2);default:100.00" json:"quality_score"`
	// 数据版本
	Version uint `gorm:"column:version;default:1" json:"version"`
	// 是否当前版本
	IsCurrent bool `gorm:"column:is_current;default:true" json:"is_current"`

	*CommonTimestampsField
}

// TableName 指定表名
func (ProcessedData) TableName() string {
	return "processed_data"
}
