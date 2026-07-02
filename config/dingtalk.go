package config

import (
	"gin-biz-web-api/pkg/config"
)

func init() {
	config.Add("cfg.dingtalk", func() map[string]interface{} {
		return map[string]interface{}{
			"default": map[string]interface{}{
				"token":  config.Get("DingTalk.Default.Token", ""),
				"secret": config.Get("DingTalk.Default.Secret", ""),
			},
			"xian": map[string]interface{}{
				"token":  config.Get("DingTalk.Xian.Token", ""),
				"secret": config.Get("DingTalk.Xian.Secret", ""),
			},
		}
	})
}
