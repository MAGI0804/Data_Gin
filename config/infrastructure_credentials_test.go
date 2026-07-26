package config

import (
	"os"
	"path/filepath"
	"testing"

	pkgConfig "gin-biz-web-api/pkg/config"
)

func TestInfrastructureCredentialsRemainInProjectConfig(t *testing.T) {
	configDir := t.TempDir()
	configFile := filepath.Join(configDir, "config.yaml")
	contents := []byte(`
DB:
  Username: db-user
  Password: db-password
Redis:
  Username: redis-user
  Password: redis-password
Cache:
  Username: cache-user
  Password: cache-password
QueueJob:
  Redis:
    Username: queue-user
    Password: queue-password
JWT:
  Key: jwt-key
`)
	if err := os.WriteFile(configFile, contents, 0o600); err != nil {
		t.Fatalf("write test config: %v", err)
	}

	pkgConfig.NewConfig("", configDir+string(os.PathSeparator))

	want := map[string]string{
		"cfg.database.mysql.default.username": "db-user",
		"cfg.database.mysql.default.password": "db-password",
		"cfg.redis.default.username":          "redis-user",
		"cfg.redis.default.password":          "redis-password",
		"cfg.redis.cache.username":            "cache-user",
		"cfg.redis.cache.password":            "cache-password",
		"cfg.queue_job.redis.username":        "queue-user",
		"cfg.queue_job.redis.password":        "queue-password",
		"cfg.jwt.key":                         "jwt-key",
	}
	for key, expected := range want {
		if got := pkgConfig.GetString(key); got != expected {
			t.Errorf("%s = %q, want %q", key, got, expected)
		}
	}
}
