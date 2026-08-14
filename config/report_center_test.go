package config

import (
	"os"
	"path/filepath"
	"testing"

	pkgConfig "gin-biz-web-api/pkg/config"
)

func TestReportCenterRuntimeFlagsPreserveExplicitFalseAndZero(t *testing.T) {
	t.Setenv(EnvReportCenterEnabled, "")
	t.Setenv(EnvReportWorkerEnabled, "")
	loadReportCenterTestConfig(t, `
ReportCenter:
  Enabled: false
QueueJob:
  ReportWorker:
    Enabled: false
    QueueWeight: 0
`)
	if pkgConfig.GetBool("cfg.report_center.enabled") {
		t.Fatal("cfg.report_center.enabled = true, want false")
	}
	if pkgConfig.GetBool("cfg.queue_job.report_worker.enabled") {
		t.Fatal("cfg.queue_job.report_worker.enabled = true, want false")
	}
	if weight := pkgConfig.GetInt("cfg.queue_job.report_worker.queue_weight"); weight != 0 {
		t.Fatalf("cfg.queue_job.report_worker.queue_weight = %d, want 0", weight)
	}
}

func TestReportCenterRuntimeFlagsUseEnvironmentOverrides(t *testing.T) {
	t.Setenv(EnvReportCenterEnabled, "true")
	t.Setenv(EnvReportWorkerEnabled, "true")
	loadReportCenterTestConfig(t, `
ReportCenter:
  Enabled: false
QueueJob:
  ReportWorker:
    Enabled: false
`)
	if !pkgConfig.GetBool("cfg.report_center.enabled") {
		t.Fatal("cfg.report_center.enabled = false, want environment override")
	}
	if !pkgConfig.GetBool("cfg.queue_job.report_worker.enabled") {
		t.Fatal("cfg.queue_job.report_worker.enabled = false, want environment override")
	}
}

func loadReportCenterTestConfig(t *testing.T, contents string) {
	t.Helper()
	configDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(contents), 0o600); err != nil {
		t.Fatalf("write report center test config: %v", err)
	}
	pkgConfig.NewConfig("", configDir+string(os.PathSeparator))
}
