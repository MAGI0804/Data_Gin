package config

import (
	"gin-biz-web-api/pkg/config"
)

func init() {
	config.Add("cfg.xian", func() map[string]interface{} {
		return map[string]interface{}{
			"app_secret": config.Get("Xian.AppSecret", ""),
			"shop_id":    config.Get("Xian.ShopID", ""),
			"shop_name":  config.Get("Xian.ShopName", ""),
			"url":        config.Get("Xian.URL", ""),

			"sync": map[string]interface{}{
				"shop_code": config.Get("Xian.Sync.ShopCode", ""),
				"status":    config.Get("Xian.Sync.Status", "70"),
			},
		}
	})
}
