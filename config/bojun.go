package config

import (
	"gin-biz-web-api/pkg/config"
)

func init() {
	config.Add("cfg.bojun", func() map[string]interface{} {
		return map[string]interface{}{
			"base_url":               config.Get("Bojun.BaseURL", "http://47.116.189.190:9100/bos/standard"),
			"appkey":                 config.Get("Bojun.AppKey", ""),
			"secret":                 config.Get("Bojun.Secret", ""),
			"format":                 config.Get("Bojun.Format", "json"),
			"timeout_seconds":        config.GetInt("Bojun.TimeoutSeconds", 10),
			"order_method":           config.Get("Bojun.OrderMethod", "/retail/middleretail.query"),
			"order_page_size":        config.GetInt("Bojun.OrderPageSize", 100),
			"order_lookback_minutes": config.GetInt("Bojun.OrderLookbackMinutes", 1),
			"order_max_pages":        config.GetInt("Bojun.OrderMaxPages", 20),
			"order_cron_expr":        config.Get("Bojun.OrderCronExpr", "0 */1 * * * *"),
		}
	})
}
