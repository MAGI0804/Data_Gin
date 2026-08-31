package database

import (
	"testing"
	"time"

	"gorm.io/driver/mysql"
)

func TestValidateDBClientConfig(t *testing.T) {
	valid := DBClientConfig{
		DBConfig:        mysql.Open("user:pass@tcp(127.0.0.1:3306)/db"),
		MaxOpenConns:    25,
		MaxIdleConns:    10,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 90 * time.Second,
		ConnectTimeout:  5 * time.Second,
		ReadTimeout:     30 * time.Second,
		WriteTimeout:    30 * time.Second,
	}
	tests := []struct {
		name   string
		mutate func(*DBClientConfig)
	}{
		{name: "valid", mutate: func(*DBClientConfig) {}},
		{name: "missing dialector", mutate: func(cfg *DBClientConfig) { cfg.DBConfig = nil }},
		{name: "nonpositive max open", mutate: func(cfg *DBClientConfig) { cfg.MaxOpenConns = 0 }},
		{name: "idle exceeds open", mutate: func(cfg *DBClientConfig) { cfg.MaxIdleConns = 26 }},
		{name: "nonpositive lifetime", mutate: func(cfg *DBClientConfig) { cfg.ConnMaxLifetime = 0 }},
		{name: "idle time exceeds lifetime", mutate: func(cfg *DBClientConfig) { cfg.ConnMaxIdleTime = 6 * time.Minute }},
		{name: "nonpositive connect timeout", mutate: func(cfg *DBClientConfig) { cfg.ConnectTimeout = 0 }},
		{name: "nonpositive read timeout", mutate: func(cfg *DBClientConfig) { cfg.ReadTimeout = 0 }},
		{name: "nonpositive write timeout", mutate: func(cfg *DBClientConfig) { cfg.WriteTimeout = 0 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := valid
			tt.mutate(&cfg)
			err := validateDBClientConfig(&cfg)
			if tt.name == "valid" && err != nil {
				t.Fatalf("validateDBClientConfig() error = %v", err)
			}
			if tt.name != "valid" && err == nil {
				t.Fatal("validateDBClientConfig() error = nil")
			}
		})
	}
}
