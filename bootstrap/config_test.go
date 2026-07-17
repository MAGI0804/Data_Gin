package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	pkgConfig "gin-biz-web-api/pkg/config"
)

func TestValidateMallWeatherConfig(t *testing.T) {
	tests := []struct {
		name      string
		yaml      string
		wantError string
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
			name:      "feishu cannot run independently",
			yaml:      "MallWeather:\n  FeishuEnabled: true\n",
			wantError: "requires mall weather",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configDir := t.TempDir()
			configFile := filepath.Join(configDir, "config.yaml")
			if err := os.WriteFile(configFile, []byte(tt.yaml), 0o600); err != nil {
				t.Fatalf("write test config: %v", err)
			}
			pkgConfig.NewConfig("", configDir+string(os.PathSeparator))

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
