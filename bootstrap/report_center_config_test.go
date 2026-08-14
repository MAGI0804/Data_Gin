package bootstrap

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gin-biz-web-api/global"
	pkgConfig "gin-biz-web-api/pkg/config"
)

func TestValidateReportCenterRuntimeRequiresCompleteDependencies(t *testing.T) {
	t.Setenv("REPORT_WORKER_ENABLED", "")
	loadReportCenterConfig(t, `
ReportCenter:
  Enabled: true
QueueJob:
  ReportWorker:
    Enabled: true
    QueueWeight: 2
Storage:
  Driver: oss
  OSS:
    Enabled: true
    Region: cn-shanghai
    Endpoint: https://oss-cn-shanghai.aliyuncs.com
    Bucket: report-test
`)
	previousEnabled := global.ReportCenterEnabledAtStartup
	global.ReportCenterEnabledAtStartup = true
	t.Cleanup(func() { global.ReportCenterEnabledAtStartup = previousEnabled })

	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	t.Setenv("REPORT_CREDENTIAL_KEY_VERSION", "credential-v1")
	t.Setenv("REPORT_CREDENTIAL_KEYS_JSON", `{"credential-v1":"`+key+`"}`)
	t.Setenv("REPORT_PARAMETER_KEY_VERSION", "parameter-v1")
	t.Setenv("REPORT_PARAMETER_KEYS_JSON", `{"parameter-v1":"`+key+`"}`)
	t.Setenv("ALIYUN_OSS_ACCESS_KEY_ID", "test-access-key")
	t.Setenv("ALIYUN_OSS_ACCESS_KEY_SECRET", "test-access-secret")
	if err := validateReportCenterRuntime(); err != nil {
		t.Fatalf("validateReportCenterRuntime() error = %v", err)
	}

	t.Setenv("REPORT_PARAMETER_KEYS_JSON", "")
	if err := validateReportCenterRuntime(); err == nil {
		t.Fatal("validateReportCenterRuntime() accepted an empty parameter keyring")
	}
}

func TestValidateReportCenterRuntimeRejectsHalfEnabledRuntime(t *testing.T) {
	t.Setenv("REPORT_WORKER_ENABLED", "")
	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	for name, value := range map[string]string{
		"REPORT_CREDENTIAL_KEY_VERSION": "credential-v1",
		"REPORT_CREDENTIAL_KEYS_JSON":   `{"credential-v1":"` + key + `"}`,
		"REPORT_PARAMETER_KEY_VERSION":  "parameter-v1",
		"REPORT_PARAMETER_KEYS_JSON":    `{"parameter-v1":"` + key + `"}`,
		"ALIYUN_OSS_ACCESS_KEY_ID":      "test-access-key",
		"ALIYUN_OSS_ACCESS_KEY_SECRET":  "test-access-secret",
	} {
		t.Setenv(name, value)
	}
	previousEnabled := global.ReportCenterEnabledAtStartup
	global.ReportCenterEnabledAtStartup = true
	t.Cleanup(func() { global.ReportCenterEnabledAtStartup = previousEnabled })

	tests := []struct {
		name      string
		yaml      string
		wantError string
	}{
		{
			name: "worker disabled",
			yaml: `
QueueJob:
  ReportWorker:
    Enabled: false
    QueueWeight: 2
Storage:
  Driver: oss
  OSS:
    Enabled: true
    Region: cn-shanghai
    Endpoint: https://oss-cn-shanghai.aliyuncs.com
    Bucket: report-test
`,
			wantError: "worker must be enabled",
		},
		{
			name: "OSS disabled",
			yaml: `
QueueJob:
  ReportWorker:
    Enabled: true
    QueueWeight: 2
Storage:
  Driver: local
  OSS:
    Enabled: false
`,
			wantError: "OSS 存储未启用",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loadReportCenterConfig(t, tt.yaml)
			err := validateReportCenterRuntime()
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("validateReportCenterRuntime() error = %v, want %q", err, tt.wantError)
			}
		})
	}
}

func TestValidateReportCenterRuntimeSkipsDisabledModule(t *testing.T) {
	previousEnabled := global.ReportCenterEnabledAtStartup
	global.ReportCenterEnabledAtStartup = false
	t.Cleanup(func() { global.ReportCenterEnabledAtStartup = previousEnabled })
	if err := validateReportCenterRuntime(); err != nil {
		t.Fatalf("validateReportCenterRuntime() disabled error = %v", err)
	}
}

func TestValidateReportCenterFlagsRejectsInvalidEnvironmentValue(t *testing.T) {
	for _, name := range []string{"REPORT_CENTER_ENABLED", "REPORT_WORKER_ENABLED"} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("REPORT_CENTER_ENABLED", "")
			t.Setenv("REPORT_WORKER_ENABLED", "")
			t.Setenv(name, "sometimes")
			if err := validateReportCenterFlags(); err == nil {
				t.Fatalf("validateReportCenterFlags() accepted invalid %s", name)
			}
		})
	}
}

func loadReportCenterConfig(t *testing.T, contents string) {
	t.Helper()
	configDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(contents), 0o600); err != nil {
		t.Fatalf("write report center config: %v", err)
	}
	pkgConfig.NewConfig("", configDir+string(os.PathSeparator))
}
