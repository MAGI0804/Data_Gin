package reportrepo

import (
	"encoding/json"
	"strings"
	"testing"

	"gin-biz-web-api/model"
)

func TestDatasourceAuditDetailExcludesCredentialAndConnectionSecrets(t *testing.T) {
	detail := datasourceAuditDetailFrom(model.ReportDatasource{
		Code: "report_oracle", Name: "经营报表库", Driver: model.ReportDatasourceDriverOracle,
		Host: "private.internal", Username: "report_user", PasswordCiphertext: "cipher-secret",
		CredentialKeyVersion: "key-v1", SessionInitJSON: model.JSONText(`{"sql":"secret"}`), Enabled: true,
	})
	encoded, err := json.Marshal(detail)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	for _, secret := range []string{"private.internal", "report_user", "cipher-secret", "key-v1", "session", "sql"} {
		if strings.Contains(strings.ToLower(string(encoded)), strings.ToLower(secret)) {
			t.Fatalf("audit detail leaked %q: %s", secret, encoded)
		}
	}
	if detail.Code != "report_oracle" || detail.Driver != model.ReportDatasourceDriverOracle {
		t.Fatalf("audit detail = %#v", detail)
	}
}

func TestDatasourceConnectionChangedSeparatesOperationalAndDisplayUpdates(t *testing.T) {
	current := model.ReportDatasource{Host: "oracle.internal", Port: 1521, ServiceName: "REPORT", Username: "report_user", SessionTimezone: "Asia/Shanghai", ConnectTimeoutSeconds: 5, QueryTimeoutSeconds: 300, MaxOpenConnections: 10, MaxIdleConnections: 2, PrefetchRows: 1000, ArraySize: 1000}
	displayOnly := current
	displayOnly.Name = "新名称"
	displayOnly.Enabled = false
	if datasourceConnectionChanged(current, displayOnly) {
		t.Fatal("display-only update was treated as a connection change")
	}
	connectionUpdate := current
	connectionUpdate.Host = "other.internal"
	if !datasourceConnectionChanged(current, connectionUpdate) {
		t.Fatal("host update was not treated as a connection change")
	}
	credentialUpdate := current
	credentialUpdate.PasswordCiphertext = "new-ciphertext"
	if !datasourceConnectionChanged(current, credentialUpdate) {
		t.Fatal("credential rotation was not treated as a connection change")
	}
}
