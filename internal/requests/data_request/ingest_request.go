package data_request

import (
	"github.com/gin-gonic/gin"
	"github.com/thedevsaddam/govalidator"

	"gin-biz-web-api/pkg/validator"
)

// IngestRequest 数据接收请求
type IngestRequest struct {
	DataSource string                   `json:"data_source" valid:"data_source"`
	DataType   string                   `json:"data_type" valid:"data_type"`
	Data       []map[string]interface{} `json:"data" valid:"data"`
}

// IngestRequestValidator 数据接收请求验证器
func IngestRequestValidator(data interface{}, c *gin.Context) map[string][]string {
	rules := govalidator.MapData{
		"data_source": []string{"required", "max:100"},
		"data_type":   []string{"required", "max:50"},
		"data":        []string{"required", "min:1", "max:1000"},
	}

	messages := govalidator.MapData{
		"data_source": []string{
			"required:数据源不能为空",
			"max:数据源长度不能超过 100 个字符",
		},
		"data_type": []string{
			"required:数据类型不能为空",
			"max:数据类型长度不能超过 50 个字符",
		},
		"data": []string{
			"required:数据内容不能为空",
			"min:数据内容至少包含一条记录",
			"max:数据内容最多包含 1000 条记录",
		},
	}

	return validator.ValidateStruct(data, rules, messages)
}

// BatchIngestRequest 批量数据接收请求
type BatchIngestRequest struct {
	BatchID string `json:"batch_id" valid:"batch_id"`
	Items   []struct {
		DataSource string                   `json:"data_source" valid:"data_source"`
		DataType   string                   `json:"data_type" valid:"data_type"`
		Data       []map[string]interface{} `json:"data" valid:"data"`
	} `json:"items" valid:"items"`
}

// BatchIngestRequestValidator 批量数据接收请求验证器
func BatchIngestRequestValidator(data interface{}, c *gin.Context) map[string][]string {
	rules := govalidator.MapData{
		"batch_id": []string{"required", "max:100"},
		"items":    []string{"required", "min:1", "max:100"},
	}

	messages := govalidator.MapData{
		"batch_id": []string{
			"required:批次ID不能为空",
			"max:批次ID长度不能超过 100 个字符",
		},
		"items": []string{
			"required:批量数据不能为空",
			"min:批量数据至少包含一条记录",
			"max:批量数据最多包含 100 条记录",
		},
	}

	return validator.ValidateStruct(data, rules, messages)
}
