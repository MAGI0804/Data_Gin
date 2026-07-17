package bootstrap

import (
	"fmt"

	"gin-biz-web-api/global"
	"gin-biz-web-api/pkg/config"
	"gin-biz-web-api/pkg/console"
	"gin-biz-web-api/pkg/redis"
)

// setupRedis 初始化 redis
func setupRedis() {

	console.Info("init redis ...")

	// 初始化配置信息组
	rdsConfigs := make(redis.RdsConfigs)
	// 获取 config/redis.go 中的所有配置信息组
	configs := config.GetStringMapString("cfg.redis")

	for group := range configs {
		cfgPrefix := "cfg.redis." + group + "."
		username, password := global.Credentials.RedisUsername(), global.Credentials.RedisPassword()
		if group == "cache" {
			username, password = global.Credentials.CacheUsername(), global.Credentials.CachePassword()
		}
		rdsConfigs[group] = &redis.RdsClientConfig{
			Addr: fmt.Sprintf(
				"%v:%v",
				config.GetString(cfgPrefix+"host"),
				config.GetString(cfgPrefix+"port")),
			Username: username,
			Password: password,
			DB:       config.GetInt(cfgPrefix + "db"),
		}
	}

	// 连接 redis
	redis.ConnectRedis(rdsConfigs)

}
