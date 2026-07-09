// 静态文件存储配置
package config

import "gin-biz-web-api/pkg/config"

func init() {
	config.Add("cfg.storage", func() map[string]interface{} {
		return map[string]interface{}{
			"driver": config.Get("Storage.Driver", "local"),
			"oss": map[string]interface{}{
				"enabled":            config.Get("Storage.OSS.Enabled", false),
				"region":             config.Get("Storage.OSS.Region", ""),
				"endpoint":           config.Get("Storage.OSS.Endpoint", ""),
				"bucket":             config.Get("Storage.OSS.Bucket", ""),
				"cdn_base_url":       config.Get("Storage.OSS.CDNBaseURL", ""),
				"prefix":             config.Get("Storage.OSS.Prefix", "data-warehouse"),
				"use_internal":       config.Get("Storage.OSS.UseInternal", false),
				"use_cname":          config.Get("Storage.OSS.UseCName", false),
				"disable_ssl":        config.Get("Storage.OSS.DisableSSL", false),
				"connect_timeout":    config.Get("Storage.OSS.ConnectTimeoutSeconds", 10),
				"read_write_timeout": config.Get("Storage.OSS.ReadWriteTimeoutSeconds", 300),
			},
		}
	})
}
