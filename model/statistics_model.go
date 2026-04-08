// package model 数据统计模型
package model

// DataStatistics 数据统计模型
// 存储数据统计信息，便于快速查询
type DataStatistics struct {
	*BaseModel

	// 统计日期
	StatDate string `gorm:"column:stat_date;type:date;not null" json:"stat_date"`
	// 数据类型
	DataType string `gorm:"column:data_type;size:50;not null" json:"data_type"`
	// 数据源ID
	DataSourceID uint `gorm:"column:data_source_id" json:"data_source_id"`
	// 总数据量
	TotalCount uint `gorm:"column:total_count;default:0" json:"total_count"`
	// 已处理数量
	ProcessedCount uint `gorm:"column:processed_count;default:0" json:"processed_count"`
	// 错误数量
	ErrorCount uint `gorm:"column:error_count;default:0" json:"error_count"`
	// 平均质量分
	AvgQualityScore float64 `gorm:"column:avg_quality_score;type:decimal(5,2);default:0" json:"avg_quality_score"`

	*CommonTimestampsField
}

// TableName 指定表名
func (DataStatistics) TableName() string {
	return "data_statistics"
}
