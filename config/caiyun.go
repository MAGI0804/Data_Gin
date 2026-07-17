package config

import "gin-biz-web-api/pkg/config"

func init() {
	config.Add("cfg.caiyun", func() map[string]interface{} {
		return map[string]interface{}{
			"base_url":            config.Get("Caiyun.BaseURL", "https://api.caiyunapp.com"),
			"life_index_base_url": config.Get("Caiyun.LifeIndexBaseURL", "https://singer.caiyunhub.com"),
			"qps":                 config.Get("Caiyun.QPS", 0),
		}
	})
}
