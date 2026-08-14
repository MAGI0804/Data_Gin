package config

import (
	"os"
	"strconv"
	"strings"

	"gin-biz-web-api/pkg/config"
)

const EnvReportCenterEnabled = "REPORT_CENTER_ENABLED"

func init() {
	config.Add("cfg.report_center", func() map[string]interface{} {
		return map[string]interface{}{
			"enabled": reportCenterEnabled(),
		}
	})
}

func reportCenterEnabled() bool {
	raw, exists := os.LookupEnv(EnvReportCenterEnabled)
	if exists && strings.TrimSpace(raw) != "" {
		enabled, err := strconv.ParseBool(strings.TrimSpace(raw))
		if err != nil {
			return false
		}
		return enabled
	}
	if instance := config.Instance(); instance != nil && instance.IsSet("ReportCenter.Enabled") {
		return instance.GetBool("ReportCenter.Enabled")
	}
	return true
}
