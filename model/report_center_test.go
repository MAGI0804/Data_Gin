package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestReportCenterTableNames(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "datasources", got: (ReportDatasource{}).TableName(), want: "report_datasources"},
		{name: "definitions", got: (ReportDefinition{}).TableName(), want: "report_definitions"},
		{name: "versions", got: (ReportVersion{}).TableName(), want: "report_versions"},
		{name: "parameters", got: (ReportParameter{}).TableName(), want: "report_parameters"},
		{name: "columns", got: (ReportColumn{}).TableName(), want: "report_columns"},
		{name: "grants", got: (ReportGrant{}).TableName(), want: "report_grants"},
		{name: "runs", got: (ReportRun{}).TableName(), want: "report_runs"},
		{name: "exports", got: (ReportExport{}).TableName(), want: "report_exports"},
		{name: "audits", got: (ReportAudit{}).TableName(), want: "report_audits"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.got != test.want {
				t.Fatalf("TableName() = %q, want %q", test.got, test.want)
			}
		})
	}
}

func TestReportCenterModelsDoNotMarshalSecrets(t *testing.T) {
	datasourceJSON, err := json.Marshal(ReportDatasource{
		PasswordCiphertext:   "encrypted-password",
		CredentialKeyVersion: "key-v1",
		SessionInitJSON:      JSONText(`{"sql":"ALTER SESSION"}`),
	})
	if err != nil {
		t.Fatalf("marshal report datasource: %v", err)
	}

	runJSON, err := json.Marshal(ReportRun{
		RefreshNonce:              "refresh-secret",
		NormalizedParametersJSON:  JSONText(`{"customer":"secret"}`),
		SensitiveParametersCipher: "encrypted-parameters",
		WorkerID:                  "worker-internal",
	})
	if err != nil {
		t.Fatalf("marshal report run: %v", err)
	}

	exportJSON, err := json.Marshal(ReportExport{
		ResultObjectKey: "private/object.xlsx",
		PurgeCursor:     123,
	})
	if err != nil {
		t.Fatalf("marshal report export: %v", err)
	}

	combined := string(datasourceJSON) + string(runJSON) + string(exportJSON)
	for _, secret := range []string{
		"encrypted-password",
		"key-v1",
		"ALTER SESSION",
		"refresh-secret",
		"secret",
		"encrypted-parameters",
		"worker-internal",
		"private/object.xlsx",
		"123",
	} {
		if strings.Contains(combined, secret) {
			t.Fatalf("marshaled report model leaked %q: %s", secret, combined)
		}
	}
}
