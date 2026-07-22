package bootstrap

import (
	"fmt"
	"strings"

	_ "gin-biz-web-api/config"
	"gin-biz-web-api/global"
	pkgConfig "gin-biz-web-api/pkg/config"
	"gin-biz-web-api/pkg/console"
	"gin-biz-web-api/pkg/credential"

	"github.com/joho/godotenv"
)

// setupConfig 初始化配置文件信息
func setupConfig() {

	console.Info("init config ...")

	// 加载 .env 文件（如果存在）
	if err := godotenv.Load(); err != nil {
		console.Warning(".env file not found, using system environment variables")
	}

	// 通过匿名加载的方式自动加载了 config 包中所有的 init 函数

	// 加载配置文件
	pkgConfig.NewConfig(global.Env, strings.Split(global.ConfigPath, ",")...)

	if err := validateMallWeatherConfig(); err != nil {
		console.Exit("invalid mall weather configuration: %v", err)
	}

	credentials, err := credential.Load(credential.Requirements{
		Production:            pkgConfig.GetString("cfg.app.env") == "prod",
		RequireInfrastructure: true,
		RequireMallWeather:    pkgConfig.GetBool("cfg.mall_weather.enabled"),
		RequireFeishu:         pkgConfig.GetBool("cfg.mall_weather.feishu_enabled"),
		RequireOSS:            pkgConfig.GetBool("cfg.storage.oss.enabled"),
	})
	if err != nil {
		console.Exit("invalid credential configuration: %v", err)
	}
	global.Credentials = credentials
	logCredentialStatus(credentials)

}

func validateMallWeatherConfig() error {
	if !pkgConfig.GetBool("cfg.mall_weather.enabled") {
		if pkgConfig.GetBool("cfg.mall_weather.feishu_enabled") {
			return fmt.Errorf("feishu integration requires mall weather to be enabled")
		}
		return nil
	}
	if pkgConfig.GetString("cfg.mall_weather.provider") != "caiyun" {
		return fmt.Errorf("provider must be caiyun")
	}
	if pkgConfig.GetString("cfg.mall_weather.unit") != "metric:v2" {
		return fmt.Errorf("unit must be metric:v2")
	}
	if qps := pkgConfig.GetFloat64("cfg.caiyun.qps"); qps <= 0 || qps > 1000 {
		return fmt.Errorf("caiyun qps must be greater than zero and at most 1000")
	}
	if hourlySteps := pkgConfig.GetInt("cfg.mall_weather.hourly_steps"); hourlySteps < 1 || hourlySteps > 360 {
		return fmt.Errorf("hourly steps must be between 1 and 360")
	}
	if dailySteps := pkgConfig.GetInt("cfg.mall_weather.daily_steps"); dailySteps < 1 || dailySteps > 15 {
		return fmt.Errorf("daily steps must be between 1 and 15")
	}
	fetchTimeout := pkgConfig.GetInt("cfg.mall_weather.fetch_timeout_seconds")
	if fetchTimeout < 1 || fetchTimeout > 120 {
		return fmt.Errorf("fetch timeout must be between 1 and 120 seconds")
	}
	lockTTL := pkgConfig.GetInt("cfg.mall_weather.lock_ttl_seconds")
	taskTimeout := pkgConfig.GetInt("cfg.mall_weather.task_timeout_seconds")
	if taskTimeout <= fetchTimeout || taskTimeout > 1800 {
		return fmt.Errorf("weather task timeout must exceed fetch timeout and be at most 1800 seconds")
	}
	if lockTTL <= taskTimeout || lockTTL > 3600 {
		return fmt.Errorf("weather lock ttl must exceed task timeout and be at most 3600 seconds")
	}
	if releaseTimeout := pkgConfig.GetInt("cfg.mall_weather.lock_release_timeout_seconds"); releaseTimeout < 1 || releaseTimeout > 10 {
		return fmt.Errorf("weather lock release timeout must be between 1 and 10 seconds")
	}
	queues, ok := pkgConfig.Get("cfg.queue_job.config_opt.queues").(map[string]int)
	if !ok || queues["weather"] <= 0 {
		return fmt.Errorf("weather queue must be configured with a positive weight")
	}
	return nil
}

func logCredentialStatus(credentials credential.Config) {
	for _, name := range []string{
		credential.EnvAmapWebServiceKey,
		credential.EnvCaiyunAppKey,
		credential.EnvCaiyunAppSecret,
		credential.EnvFeishuAppID,
		credential.EnvFeishuAppSecret,
	} {
		console.Info("credential %s configured=%t source=environment", name, credentials.Configured(name))
	}
}
