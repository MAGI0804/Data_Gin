package model

import (
	"encoding/json"
	"reflect"
	"strings"
	"sync"
	"testing"

	"gorm.io/gorm/schema"
)

func TestReportCenterTableNames(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "datasources", got: (ReportDatasource{}).TableName(), want: "report_datasources"},
		{name: "input query definitions", got: (ReportInputQueryDefinition{}).TableName(), want: "report_input_query_definitions"},
		{name: "result table bindings", got: (ReportResultTableBinding{}).TableName(), want: "report_result_table_bindings"},
		{name: "definitions", got: (ReportDefinition{}).TableName(), want: "report_definitions"},
		{name: "versions", got: (ReportVersion{}).TableName(), want: "report_versions"},
		{name: "parameters", got: (ReportParameter{}).TableName(), want: "report_parameters"},
		{name: "columns", got: (ReportColumn{}).TableName(), want: "report_columns"},
		{name: "grants", got: (ReportGrant{}).TableName(), want: "report_grants"},
		{name: "runs", got: (ReportRun{}).TableName(), want: "report_runs"},
		{name: "exports", got: (ReportExport{}).TableName(), want: "report_exports"},
		{name: "result read leases", got: (ReportResultReadLease{}).TableName(), want: "report_result_read_leases"},
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

func TestReportGrantUniqueSubjectIsVersionScoped(t *testing.T) {
	parsed, err := schema.Parse(&ReportGrant{}, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatalf("schema.Parse() error = %v", err)
	}
	index, ok := parsed.ParseIndexes()["uk_report_grant_subject"]
	if !ok {
		t.Fatal("version-scoped grant subject index not found")
	}
	if index.Class != "UNIQUE" {
		t.Fatalf("index class = %q, want UNIQUE", index.Class)
	}
	got := make([]string, 0, len(index.Fields))
	for _, field := range index.Fields {
		got = append(got, field.DBName)
	}
	want := []string{"version_id", "subject_type", "subject_id"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("index fields = %v, want %v", got, want)
	}
}

func TestReportResultTableBindingHasAtomicUniquenessConstraints(t *testing.T) {
	parsed, err := schema.Parse(&ReportResultTableBinding{}, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatalf("schema.Parse() error = %v", err)
	}
	tableIndex, ok := parsed.ParseIndexes()["uk_report_result_table_binding"]
	if !ok || tableIndex.Class != "UNIQUE" {
		t.Fatalf("result table index = %#v", tableIndex)
	}
	fields := make([]string, 0, len(tableIndex.Fields))
	for _, field := range tableIndex.Fields {
		fields = append(fields, field.DBName)
	}
	if want := []string{"connection_fingerprint", "table_owner", "table_name"}; !reflect.DeepEqual(fields, want) {
		t.Fatalf("result table index fields = %v, want %v", fields, want)
	}
	definitionIndex, ok := parsed.ParseIndexes()["idx_report_result_table_bindings_definition_id"]
	if !ok || definitionIndex.Class != "UNIQUE" {
		t.Fatalf("definition index = %#v", definitionIndex)
	}
	identityField := parsed.LookUpField("IdentitySource")
	if identityField == nil || identityField.DBName != "identity_source" || identityField.DefaultValue != "LEGACY_DATASOURCE_V1" {
		t.Fatalf("identity source field = %#v", identityField)
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
		RefreshNonce:                  "refresh-secret",
		NormalizedParametersJSON:      JSONText(`{"customer":"secret"}`),
		SensitiveParametersCipher:     "encrypted-parameters",
		SensitiveParametersKeyVersion: "parameter-key-v1",
		WorkerID:                      "worker-internal",
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
		"parameter-key-v1",
		"worker-internal",
		"private/object.xlsx",
		"123",
	} {
		if strings.Contains(combined, secret) {
			t.Fatalf("marshaled report model leaked %q: %s", secret, combined)
		}
	}
}

func TestReportVersionHidesInternalResultSnapshotKeyColumns(t *testing.T) {
	version := ReportVersion{ResultRunIDColumn: "RUN_ID", ResultRowIDColumn: "ID"}
	encoded, err := json.Marshal(version)
	if err != nil {
		t.Fatalf("marshal report version: %v", err)
	}
	text := string(encoded)
	if strings.Contains(text, "resultRunIdColumn") || strings.Contains(text, "resultRowIdColumn") {
		t.Fatalf("report version exposed internal result snapshot key columns: %s", text)
	}
}
