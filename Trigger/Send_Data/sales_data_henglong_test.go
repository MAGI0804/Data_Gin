package send

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "gin-biz-web-api/config"
	pkgConfig "gin-biz-web-api/pkg/config"
)

func setupSalesDataTestConfig(t *testing.T) {
	t.Helper()
	configDir := t.TempDir()
	configBody := []byte("HengLong:\n  TillID: \"01\"\n")
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), configBody, 0o600); err != nil {
		t.Fatalf("write test config: %v", err)
	}
	pkgConfig.NewConfig("", configDir+string(os.PathSeparator))
}

func TestBuildSoapXMLUsesProvidedIssueTimeAndMallItemCode(t *testing.T) {
	setupSalesDataTestConfig(t)
	issuedAt := time.Date(2026, 7, 3, 15, 45, 11, 0, time.FixedZone("CST", 8*60*60))

	xml := buildSoapXML(118.15, "ABCN002A001P12607031545110012", &issuedAt, "416201", "E6600000099", "SA")

	for _, want := range []string{
		"<issuedate_yyyymmdd>20260703</issuedate_yyyymmdd>",
		"<issuetime_hhmmss>154511</issuetime_hhmmss>",
		"<txtime_hhmmss>154511</txtime_hhmmss>",
		"<mallitemcode>E6600000099</mallitemcode>",
	} {
		if !strings.Contains(xml, want) {
			t.Fatalf("xml missing %s", want)
		}
	}
	if strings.Contains(xml, "<issuetime_hhmmss>000000</issuetime_hhmmss>") {
		t.Fatal("xml contains zero issue time")
	}
}
