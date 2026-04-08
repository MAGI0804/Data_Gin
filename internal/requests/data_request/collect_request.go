package data_request

import (
	"github.com/gin-gonic/gin"
	"github.com/thedevsaddam/govalidator"

	"gin-biz-web-api/pkg/validator"
)

// CollectRequest 数据采集请求
type CollectRequest struct {
	SourceID uint `json:"source_id" valid:"source_id"`
}

// CollectRequestValidator 数据采集请求验证器
func CollectRequestValidator(data interface{}, c *gin.Context) map[string][]string {
	rules := govalidator.MapData{
		"source_id": []string{"required", "min:1"},
	}

	messages := govalidator.MapData{
		"source_id": []string{
			"required:数据源ID不能为空",
			"min:数据源ID必须大于 0",
		},
	}

	return validator.ValidateStruct(data, rules, messages)
}

// CollectStatusRequest 数据采集状态查询请求
type CollectStatusRequest struct {
	JobID string `json:"job_id" valid:"job_id"`
}

// CollectStatusRequestValidator 数据采集状态查询请求验证器
func CollectStatusRequestValidator(data interface{}, c *gin.Context) map[string][]string {
	rules := govalidator.MapData{
		"job_id": []string{"required", "max:100"},
	}

	messages := govalidator.MapData{
		"job_id": []string{
			"required:任务ID不能为空",
			"max:任务ID长度不能超过 100 个字符",
		},
	}

	return validator.ValidateStruct(data, rules, messages)
}
