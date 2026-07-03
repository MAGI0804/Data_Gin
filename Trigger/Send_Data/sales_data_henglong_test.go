package send

import (
	"strings"
	"sync"
	"testing"
	"time"

	_ "gin-biz-web-api/config"
	pkgConfig "gin-biz-web-api/pkg/config"
)

var salesDataTestConfigOnce sync.Once

func setupSalesDataTestConfig() {
	salesDataTestConfigOnce.Do(func() {
		pkgConfig.NewConfig("", "../../etc/")
	})
}

func TestBuildSoapXMLUsesProvidedIssueTimeAndMallItemCode(t *testing.T) {
	setupSalesDataTestConfig()
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
