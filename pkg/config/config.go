// Package config 负责配置信息
// https://darjun.github.io/2020/01/18/godailylib/viper/
// https://github.com/spf13/viper
package config

import (
	"os"
	"regexp"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cast"
	"github.com/spf13/viper"

	"gin-biz-web-api/pkg/console"
	"gin-biz-web-api/pkg/file"
	"gin-biz-web-api/pkg/helper"
)

// vp viper 库实例
var vp *viper.Viper

// CfgFunc 动态加载配置信息
type CfgFunc func() map[string]interface{}

// CfgFuncS 先将预定义配置信息加载到此字典中，再通过 LoadConfig() 方法获取配置文件中的信息进行覆写
var CfgFuncS map[string]CfgFunc

// envVarPattern matches ${VAR} or ${VAR:-default}
var envVarPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(?::-([^}]*))?\}`)

// expandEnvVars recursively expands environment variable references in a string
func expandEnvVars(s string) string {
	return envVarPattern.ReplaceAllStringFunc(s, func(match string) string {
		parts := envVarPattern.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		envVar := parts[1]
		defaultVal := parts[2]

		if val, exists := os.LookupEnv(envVar); exists {
			return val
		}
		return defaultVal
	})
}

// expandEnvVarsInMap expands environment variables in a map[string]interface{}
func expandEnvVarsInMap(m map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m {
		switch val := v.(type) {
		case string:
			result[k] = expandEnvVars(val)
		case map[string]interface{}:
			result[k] = expandEnvVarsInMap(val)
		default:
			result[k] = val
		}
	}
	return result
}

// NewConfig 初始化 viper
func NewConfig(env string, configs ...string) {

	// 初始化 Viper 库
	vp = viper.New()

	// 配置类型，支持 "json", "toml", "yaml", "yml", "properties",
	//              "props", "prop", "env", "dotenv"
	// 需要注意的是：这个配置基本上是配合远程配置中心使用的，比如 etcd、consul、zookeeper 等，告诉 viper 当前的数据使用什么格式去解析
	// vp.SetConfigType("yaml")

	// 设置环境变量前缀，比如有一个环境变量为：`ALEX_AGE=18` 那么可以通过 vp.Get("age") 或者 vp.Get("AGE") 进行读取
	// vp.SetEnvPrefix("ALEX")

	// 绑定全部环境变量，比如读取 GOPATH 环境变量 vp.Get("GOPATH")
	vp.AutomaticEnv()

	fileName := FetchConfigFile(env) // 根据命令行参数 env 获取对应的配置文件名称（不带文件后缀）
	vp.SetConfigName(fileName)       // 设置配置文件的名称为 config

	// 添加多个配置文件目录
	for _, config := range configs {
		if config == "" {
			continue
		}
		configFile := config + fileName + ".yaml"
		if _, ok := file.IsExists(configFile); !ok {
			console.Exit("[%s] configuration file does not exist!", configFile)
		}

		// 设置配置文件路径为相对路径，相对于 main.go 比如：vp.AddConfigPath("config/")
		vp.AddConfigPath(config)
	}

	// 读取配置文件中的内容
	err := vp.ReadInConfig()
	console.ExitIf(err)

	// 先展开配置文件中的环境变量，确保动态配置回调读取到的是最终值。
	allSettings := vp.AllSettings()
	expanded := expandEnvVarsInMap(allSettings)
	for k, v := range expanded {
		vp.Set(k, v)
	}

	// 将读取的配置文件信息覆写自定义配置
	LoadConfig()

	WatchConfigurationChange()
}

// WatchConfigurationChange 热重载配置文件
func WatchConfigurationChange() {
	current := vp
	go func(config *viper.Viper) {
		config.WatchConfig()
		config.OnConfigChange(func(e fsnotify.Event) {
			loadConfig(config)
			console.Warning("Reload configuration file [ %v ] operation [ %v ]", e.Name, e.Op)
		})
	}(current)
}

// FetchConfigFile 根据环境变量获取对应的配置文件
func FetchConfigFile(env string) string {
	// 如果没设置环境变量，那么默认使用【etc/config.yaml 配置文件】
	if env == "" {
		return "config"
	}

	return env + "_config"
}

// LoadConfig 将配置文件中的内容读取后，覆写预定义配置信息
func LoadConfig() {
	loadConfig(vp)
}

func loadConfig(config *viper.Viper) {
	for name, fn := range CfgFuncS {
		// 设置键值：viper 支持在多个地方设置，读取顺序为：
		// 1. 调用 `viper.Set()` 显示设置的、2. 命令行选项、3. 环境变量、4. 配置文件、5. 默认值
		config.Set(name, fn())
	}
}

// Get 从 viper 中获取配置信息，支持默认值
func Get(path string, defaultValue ...interface{}) interface{} {
	// 配置文件中不存在的情况
	if !vp.IsSet(path) || helper.Empty(vp.Get(path)) {
		// 如果有默认参数，则使用默认参数
		if len(defaultValue) > 0 {
			return defaultValue[0]
		}
		// 没有默认参数，则直接返回空
		return nil
	}

	return vp.Get(path)
}

// Add 动态新增配置项
func Add(name string, configFn CfgFunc) {
	if CfgFuncS == nil {
		CfgFuncS = make(map[string]CfgFunc)
	}
	CfgFuncS[name] = configFn
}

// Instance 返回 viper 实例
func Instance() *viper.Viper {
	return vp
}

// BackupConfig 将所有的配置信息导出到指定文件中
func BackupConfig() error {
	// 如果想要覆盖，则调用
	return vp.WriteConfigAs("./etc/config_backup.yaml")
	// 如果备份配置文件存在，则不覆盖
	// return vp.SafeWriteConfigAs("./etc/config_backup.yaml")
}

// GetString 获取 string 类型的配置信息
func GetString(path string, defaultValue ...interface{}) string {
	return cast.ToString(Get(path, defaultValue...))
}

// GetInt 获取 int 类型的配置信息
func GetInt(path string, defaultValue ...interface{}) int {
	return cast.ToInt(Get(path, defaultValue...))
}

// GetInt64 获取 int64 类型的配置信息
func GetInt64(path string, defaultValue ...interface{}) int64 {
	return cast.ToInt64(Get(path, defaultValue...))
}

// GetUint 获取 uint 类型的配置信息
func GetUint(path string, defaultValue ...interface{}) uint {
	return cast.ToUint(Get(path, defaultValue...))
}

// GetFloat64 获取 float64 类型的配置信息
func GetFloat64(path string, defaultValue ...interface{}) float64 {
	return cast.ToFloat64(Get(path, defaultValue...))
}

// GetBool 获取 bool 类型的配置信息
func GetBool(path string, defaultValue ...interface{}) bool {
	return cast.ToBool(Get(path, defaultValue...))
}

// GetStringMapString 获取 map 类型的配置信息
func GetStringMapString(path string) map[string]string {
	return vp.GetStringMapString(path)
}

// GetStringSlice 获取切片类型的配置信息
func GetStringSlice(path string) []string {
	return vp.GetStringSlice(path)
}
