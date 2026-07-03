package config

import (
	"gin-biz-web-api/pkg/config"
)

func init() {
	config.Add("cfg.bojun", func() map[string]interface{} {
		return map[string]interface{}{
			"base_url":        config.Get("Bojun.BaseURL", "http://47.116.189.190:9100/bos/standard"),
			"appkey":          config.Get("Bojun.AppKey", ""),
			"secret":          config.Get("Bojun.Secret", ""),
			"format":          config.Get("Bojun.Format", "json"),
			"timeout_seconds": config.GetInt("Bojun.TimeoutSeconds", 10),
		}
	})
}
