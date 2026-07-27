package config

import (
	"os"
	"strconv"
	"strings"

	"gin-biz-web-api/pkg/config"
)

const EnvCaiyunQPS = "CAIYUN_QPS"

func init() {
	config.Add("cfg.caiyun", func() map[string]interface{} {
		return map[string]interface{}{
			"base_url":            config.Get("Caiyun.BaseURL", "https://api.caiyunapp.com"),
			"life_index_base_url": config.Get("Caiyun.LifeIndexBaseURL", "https://singer.caiyunhub.com"),
			"qps":                 caiyunQPS(),
		}
	})
}

func caiyunQPS() float64 {
	fallback := config.GetFloat64("Caiyun.QPS", 0)
	raw, exists := os.LookupEnv(EnvCaiyunQPS)
	if !exists || strings.TrimSpace(raw) == "" {
		return fallback
	}
	qps, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0
	}
	return qps
}
