package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	appConfig "gin-biz-web-api/config"
	pkgConfig "gin-biz-web-api/pkg/config"
)

func TestValidateMallWeatherConfig(t *testing.T) {
	tests := []struct {
		name         string
		yaml         string
		enabledEnv   string
		checkEnabled bool
		wantEnabled  bool
		wantError    string
	}{
		{
			name: "disabled by default",
			yaml: "App:\n  Env: local\n",
		},
		{
			name:      "enabled requires explicit qps",
			yaml:      "MallWeather:\n  Enabled: true\n",
			wantError: "qps",
		},
		{
			name: "enabled configuration is valid",
			yaml: "MallWeather:\n  Enabled: true\nCaiyun:\n  QPS: 2\n",
		},
		{
			name:       "environment enables workers",
			yaml:       "MallWeather:\n  Enabled: false\nCaiyun:\n  QPS: 2\n",
			enabledEnv: "true", checkEnabled: true, wantEnabled: true,
		},
		{
			name:       "environment disables workers",
			yaml:       "MallWeather:\n  Enabled: true\n",
			enabledEnv: "false", checkEnabled: true, wantEnabled: false,
		},
		{
			name:       "invalid environment flag fails closed",
			yaml:       "App:\n  Env: local\n",
			enabledEnv: "sometimes", wantError: "MALL_WEATHER_ENABLED",
		},
		{
			name:      "task timeout must exceed fetch timeout",
			yaml:      "MallWeather:\n  Enabled: true\n  FetchTimeoutSeconds: 30\n  TaskTimeoutSeconds: 30\nCaiyun:\n  QPS: 2\n",
			wantError: "task timeout",
		},
		{
			name:      "lock ttl must exceed task timeout",
			yaml:      "MallWeather:\n  Enabled: true\n  TaskTimeoutSeconds: 600\n  LockTTLSeconds: 600\nCaiyun:\n  QPS: 2\n",
			wantError: "lock ttl",
		},
		{
			name:      "repair rounds are bounded",
			yaml:      "MallWeather:\n  Enabled: true\n  RepairMaxRounds: 11\nCaiyun:\n  QPS: 2\n",
			wantError: "repair max rounds",
		},
		{
			name:      "repair spread is bounded",
			yaml:      "MallWeather:\n  Enabled: true\n  RepairSpreadSeconds: 3601\nCaiyun:\n  QPS: 2\n",
			wantError: "repair spread",
		},
		{
			name:      "enabled requires weather queue consumer",
			yaml:      "MallWeather:\n  Enabled: true\nCaiyun:\n  QPS: 2\nQueueJob:\n  ConfigOpt:\n    Queues:\n      default: 1\n",
			wantError: "weather queue",
		},
		{
			name:      "feishu cannot run independently",
			yaml:      "MallWeather:\n  FeishuEnabled: true\n",
			wantError: "requires mall weather",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(appConfig.EnvMallWeatherEnabled, tt.enabledEnv)
			configDir := t.TempDir()
			configFile := filepath.Join(configDir, "config.yaml")
			if err := os.WriteFile(configFile, []byte(tt.yaml), 0o600); err != nil {
				t.Fatalf("write test config: %v", err)
			}
			pkgConfig.NewConfig("", configDir+string(os.PathSeparator))
			if tt.checkEnabled && pkgConfig.GetBool("cfg.mall_weather.enabled") != tt.wantEnabled {
				t.Fatalf("cfg.mall_weather.enabled = %t, want %t", pkgConfig.GetBool("cfg.mall_weather.enabled"), tt.wantEnabled)
			}

			err := validateMallWeatherConfig()
			if tt.wantError == "" {
				if err != nil {
					t.Fatalf("validateMallWeatherConfig() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("validateMallWeatherConfig() error = %v, want %q", err, tt.wantError)
			}
		})
	}
}
