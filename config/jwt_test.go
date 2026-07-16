package config

import (
	"os"
	"path/filepath"
	"testing"

	pkgConfig "gin-biz-web-api/pkg/config"
)

func TestJWTDefaultExpiryIsOneDay(t *testing.T) {
	configDir := t.TempDir()
	configFile := filepath.Join(configDir, "config.yaml")
	if err := os.WriteFile(configFile, []byte("JWT:\n  Key: test-key\n"), 0600); err != nil {
		t.Fatalf("write test config: %v", err)
	}

	pkgConfig.NewConfig("", configDir+string(os.PathSeparator))

	if got := pkgConfig.GetInt64("cfg.jwt.expire_time"); got != 24*60 {
		t.Fatalf("jwt default expiry = %d minutes, want %d", got, 24*60)
	}
}
