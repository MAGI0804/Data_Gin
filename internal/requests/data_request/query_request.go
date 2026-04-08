package data_request

import (
	"github.com/gin-gonic/gin"
	"github.com/thedevsaddam/govalidator"

	"gin-biz-web-api/pkg/validator"
)

// RawDataQueryRequest 原始数据查询请求
type RawDataQueryRequest struct {
	DataType     string `form:"data_type" valid:"data_type"`
	DataSourceID uint   `form:"data_source_id" valid:"data_source_id"`
	Status       string `form:"status" valid:"status"`
	Limit        int    `form:"limit" valid:"limit"`
}

// RawDataQueryRequestValidator 原始数据查询请求验证器
func RawDataQueryRequestValidator(data interface{}, c *gin.Context) map[string][]string {
	rules := govalidator.MapData{
		"data_type":      []string{"max:50"},
		"data_source_id": []string{"min:0"},
		"status":         []string{"max:20"},
		"limit":          []string{"min:1", "max:1000"},
	}

	messages := govalidator.MapData{
		"data_type": []string{
			"max:数据类型长度不能超过 50 个字符",
		},
		"data_source_id": []string{
			"min:数据源ID必须大于等于 0",
		},
		"status": []string{
			"max:状态长度不能超过 20 个字符",
		},
		"limit": []string{
			"min:查询数量必须大于 0",
			"max:查询数量不能超过 1000",
		},
	}

	return validator.ValidateStruct(data, rules, messages)
}

// ProcessedDataQueryRequest 处理后数据查询请求
type ProcessedDataQueryRequest struct {
	DataType string `form:"data_type" valid:"data_type"`
	Limit    int    `form:"limit" valid:"limit"`
}

// ProcessedDataQueryRequestValidator 处理后数据查询请求验证器
func ProcessedDataQueryRequestValidator(data interface{}, c *gin.Context) map[string][]string {
	rules := govalidator.MapData{
		"data_type": []string{"max:50"},
		"limit":     []string{"min:1", "max:1000"},
	}

	messages := govalidator.MapData{
		"data_type": []string{
			"max:数据类型长度不能超过 50 个字符",
		},
		"limit": []string{
			"min:查询数量必须大于 0",
			"max:查询数量不能超过 1000",
		},
	}

	return validator.ValidateStruct(data, rules, messages)
}

// StatisticsQueryRequest 统计数据查询请求
type StatisticsQueryRequest struct {
	StartDate string `form:"start_date" valid:"start_date"`
	EndDate   string `form:"end_date" valid:"end_date"`
	DataType  string `form:"data_type" valid:"data_type"`
}

// StatisticsQueryRequestValidator 统计数据查询请求验证器
func StatisticsQueryRequestValidator(data interface{}, c *gin.Context) map[string][]string {
	rules := govalidator.MapData{
		"start_date": []string{"max:10"},
		"end_date":   []string{"max:10"},
		"data_type":  []string{"max:50"},
	}

	messages := govalidator.MapData{
		"start_date": []string{
			"max:开始日期长度不能超过 10 个字符",
		},
		"end_date": []string{
			"max:结束日期长度不能超过 10 个字符",
		},
		"data_type": []string{
			"max:数据类型长度不能超过 50 个字符",
		},
	}

	return validator.ValidateStruct(data, rules, messages)
}
