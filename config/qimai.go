package config

import (
	"gin-biz-web-api/pkg/config"
)

func init() {
	config.Add("cfg.qimai", func() map[string]interface{} {
		return map[string]interface{}{
			"open_id":     config.Get("Qimai.OpenID", ""),
			"grant_code":  config.Get("Qimai.GrantCode", ""),
			"open_key":    config.Get("Qimai.OpenKey", ""),
			"nonce":       config.Get("Qimai.Nonce", ""),

			"order_detail_url":      config.Get("Qimai.OrderDetailURL", "https://openapi.qmai.cn/v3/order/getDetail"),
			"business_record_url":   config.Get("Qimai.BusinessRecordURL", "https://openapi.qmai.cn/v3/dataone/finance/summary/businessRecord"),
		}
	})
}
