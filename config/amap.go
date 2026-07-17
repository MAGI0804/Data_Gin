package config

import "gin-biz-web-api/pkg/config"

func init() {
	config.Add("cfg.amap", func() map[string]interface{} {
		return map[string]interface{}{
			"base_url": config.Get("Amap.BaseURL", "https://restapi.amap.com"),
		}
	})
}
