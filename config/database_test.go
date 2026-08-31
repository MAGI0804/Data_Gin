package config

import (
	"os"
	"path/filepath"
	"testing"

	pkgConfig "gin-biz-web-api/pkg/config"
)

func TestDatabaseReliabilityConfiguration(t *testing.T) {
	configDir := t.TempDir()
	configFile := filepath.Join(configDir, "config.yaml")
	contents := []byte(`
DB:
  MaxOpenConnections: 40
  MaxIdleConnections: 12
  MaxLifeSeconds: 600
  MaxIdleSeconds: 120
  ConnectTimeoutSeconds: 4
  ReadTimeoutSeconds: 45
  WriteTimeoutSeconds: 50
  RejectReadOnly: false
`)
	if err := os.WriteFile(configFile, contents, 0o600); err != nil {
		t.Fatalf("write test config: %v", err)
	}
	pkgConfig.NewConfig("", configDir+string(os.PathSeparator))

	prefix := "cfg.database.mysql.default."
	wantInts := map[string]int{
		"max_open_connections":    40,
		"max_idle_connections":    12,
		"max_life_seconds":        600,
		"max_idle_seconds":        120,
		"connect_timeout_seconds": 4,
		"read_timeout_seconds":    45,
		"write_timeout_seconds":   50,
	}
	for key, want := range wantInts {
		if got := pkgConfig.GetInt(prefix + key); got != want {
			t.Errorf("%s = %d, want %d", key, got, want)
		}
	}
	if pkgConfig.GetBool(prefix + "reject_read_only") {
		t.Fatal("reject_read_only = true, want false")
	}
}
