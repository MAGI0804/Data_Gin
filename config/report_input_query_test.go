package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadReportInputQueryConfigUsesDefaultsAndNamedSelects(t *testing.T) {
	setReportInputOracleEnvironment(t)
	t.Setenv(EnvReportInputQueriesJSON, `{"门店查询-2026_华东":"SELECT id, name FROM report_stores"}`)

	config, err := LoadReportInputQueryConfig()
	if err != nil {
		t.Fatalf("LoadReportInputQueryConfig() error = %v", err)
	}
	if config.Oracle.Port != 1521 || config.Oracle.QueryTimeout != 30*time.Second || config.Oracle.MaxOpenConnections != 20 {
		t.Fatalf("Oracle defaults = %#v", config.Oracle)
	}
	query, exists := config.Queries["门店查询-2026_华东"]
	if !exists || query.Select != "SELECT id, name FROM report_stores" {
		t.Fatalf("Queries = %#v", config.Queries)
	}
}

func TestValidateReportInputQueryNameSupportsUnicodeAndLimitsCharacters(t *testing.T) {
	for _, name := range []string{"门店查询-2026_华东", "É店9", strings.Repeat("店", 64)} {
		if !ValidateReportInputQueryName(name) {
			t.Errorf("ValidateReportInputQueryName(%q) = false", name)
		}
	}
	for _, name := range []string{"1门店", "_门店", "门店 查询", strings.Repeat("店", 65)} {
		if ValidateReportInputQueryName(name) {
			t.Errorf("ValidateReportInputQueryName(%q) = true", name)
		}
	}
}

func TestLoadReportInputQueryConfigRejectsUnsafeSelect(t *testing.T) {
	setReportInputOracleEnvironment(t)
	for _, value := range []string{
		`{"stores":"DELETE FROM report_stores"}`,
		`{"stores":"SELECT id, name FROM report_stores; DELETE FROM users"}`,
		`{"stores":"SELECT id, name FROM report_stores -- ignored"}`,
	} {
		t.Run(value, func(t *testing.T) {
			t.Setenv(EnvReportInputQueriesJSON, value)
			if _, err := LoadReportInputQueryConfig(); err == nil {
				t.Fatal("LoadReportInputQueryConfig() error = nil")
			}
		})
	}
}

func TestLoadReportInputQueryConfigRejectsTrailingJSON(t *testing.T) {
	setReportInputOracleEnvironment(t)
	for _, value := range []string{
		`{"stores":"SELECT id, name FROM report_stores"} trailing`,
		`{"stores":"SELECT id, name FROM report_stores"} {}`,
	} {
		t.Run(value, func(t *testing.T) {
			t.Setenv(EnvReportInputQueriesJSON, value)
			if _, err := LoadReportInputQueryConfig(); err == nil {
				t.Fatal("LoadReportInputQueryConfig() error = nil")
			}
		})
	}
}

func TestLoadReportInputQueryConfigRequiresOracleWhenQueriesExist(t *testing.T) {
	t.Setenv(EnvReportInputQueriesJSON, `{"stores":"SELECT id, name FROM report_stores"}`)
	if _, err := LoadReportInputQueryConfig(); err == nil {
		t.Fatal("LoadReportInputQueryConfig() error = nil")
	}
}

func setReportInputOracleEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv(EnvReportInputOracleHost, "oracle.internal")
	t.Setenv(EnvReportInputOracleServiceName, "REPORT")
	t.Setenv(EnvReportInputOracleUsername, "report_input")
	t.Setenv(EnvReportInputOraclePassword, "secret")
	t.Setenv(EnvReportInputOracleSID, "")
}
