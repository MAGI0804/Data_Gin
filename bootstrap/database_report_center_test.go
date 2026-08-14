package bootstrap

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestReportVersionDatasourceMigrationUsesRollingExpandSequence(t *testing.T) {
	if addReportVersionDatasourceSQL != "ALTER TABLE `report_versions` ADD COLUMN `datasource_id` BIGINT UNSIGNED NULL AFTER `definition_id`" {
		t.Fatalf("add SQL = %q", addReportVersionDatasourceSQL)
	}
	for _, fragment := range []string{"JOIN report_definitions", "versions.definition_id", "definitions.datasource_id", "IS NULL", "= 0"} {
		if !strings.Contains(backfillReportVersionDatasourceSQL, fragment) {
			t.Fatalf("backfill SQL %q does not contain %q", backfillReportVersionDatasourceSQL, fragment)
		}
	}
	if strings.Contains(strings.ToUpper(backfillReportVersionDatasourceSQL), "NOT NULL") {
		t.Fatalf("expand migration prematurely contracts datasource_id: %q", backfillReportVersionDatasourceSQL)
	}
}

func TestReportGrantMigrationPreservesPublishedAndDraftSnapshots(t *testing.T) {
	if addReportGrantVersionSQL != "ALTER TABLE `report_grants` ADD COLUMN `version_id` BIGINT UNSIGNED NULL AFTER `definition_id`" {
		t.Fatalf("add SQL = %q", addReportGrantVersionSQL)
	}
	for _, fragment := range []string{
		"JOIN report_definitions", "current_published_version_id", "current_draft_version_id",
		"grants.version_id IS NULL", "grants.version_id = 0",
	} {
		if !strings.Contains(backfillReportGrantVersionSQL, fragment) {
			t.Fatalf("published backfill SQL %q does not contain %q", backfillReportGrantVersionSQL, fragment)
		}
	}
	for _, fragment := range []string{
		"INSERT INTO report_grants", "definitions.current_draft_version_id", "definitions.current_draft_version_id <> grants.version_id",
		"NOT EXISTS", "existing.subject_type = grants.subject_type", "existing.subject_id = grants.subject_id",
	} {
		if !strings.Contains(copyReportGrantDraftVersionSQL, fragment) {
			t.Fatalf("draft copy SQL %q does not contain %q", copyReportGrantDraftVersionSQL, fragment)
		}
	}
}

func TestReportJSONProcedureContractMigrationAddsOnlyMissingColumns(t *testing.T) {
	migrator := &fakeReportSchemaMigrator{
		tableExists: true,
		columns: map[string]bool{
			"ExecutionMode":       true,
			"JSONInputArgName":    true,
			"ResultCursorArgName": false,
			"InputSchemaJSON":     false,
		},
	}

	if err := migrateReportJSONProcedureContract(migrator); err != nil {
		t.Fatalf("migrateReportJSONProcedureContract() error = %v", err)
	}
	want := []string{"ResultCursorArgName", "InputSchemaJSON"}
	if !reflect.DeepEqual(migrator.added, want) {
		t.Fatalf("added columns = %#v, want %#v", migrator.added, want)
	}
}

func TestReportJSONProcedureContractMigrationCanResumeAfterPartialFailure(t *testing.T) {
	cause := errors.New("connection interrupted")
	first := &fakeReportSchemaMigrator{tableExists: true, columns: map[string]bool{}, addErrorAt: "ResultCursorArgName", addError: cause}
	if err := migrateReportJSONProcedureContract(first); !errors.Is(err, cause) {
		t.Fatalf("first migration error = %v, want %v", err, cause)
	}
	if want := []string{"ExecutionMode", "JSONInputArgName"}; !reflect.DeepEqual(first.added, want) {
		t.Fatalf("first added columns = %#v, want %#v", first.added, want)
	}

	second := &fakeReportSchemaMigrator{tableExists: true, columns: first.columns}
	if err := migrateReportJSONProcedureContract(second); err != nil {
		t.Fatalf("resumed migration error = %v", err)
	}
	if want := []string{"ResultCursorArgName", "InputSchemaJSON"}; !reflect.DeepEqual(second.added, want) {
		t.Fatalf("resumed added columns = %#v, want %#v", second.added, want)
	}
}

func TestReportJSONProcedureContractMigrationRequiresExistingBaselineTable(t *testing.T) {
	err := migrateReportJSONProcedureContract(&fakeReportSchemaMigrator{})
	if err == nil || !strings.Contains(err.Error(), "report_versions table is unavailable") {
		t.Fatalf("migration error = %v", err)
	}
}

func TestLegacyResultTableMigrationRejectsDuplicateOwnerAndTableAcrossDefinitions(t *testing.T) {
	candidates := []reportResultTableBindingCandidate{
		{DefinitionID: 7, Host: "primary.internal", ServiceName: "REPORT", Username: "USER_A", TableOwner: " report ", TableName: " sales_result "},
		{DefinitionID: 8, Host: "scan-alias.internal", ServiceName: "REPORT_ALIAS", Username: "USER_B", TableOwner: "REPORT", TableName: "SALES_RESULT"},
	}
	if err := validateLegacyResultTableBindings(candidates); err == nil || !strings.Contains(err.Error(), "reports 7 and 8") {
		t.Fatalf("validateLegacyResultTableBindings() error = %v", err)
	}
}

func TestLegacyResultTableMigrationAllowsDistinctTables(t *testing.T) {
	candidates := []reportResultTableBindingCandidate{
		{DefinitionID: 7, TableOwner: "REPORT", TableName: "SALES_RESULT"},
		{DefinitionID: 8, TableOwner: "REPORT", TableName: "INVENTORY_RESULT"},
	}
	if err := validateLegacyResultTableBindings(candidates); err != nil {
		t.Fatalf("validateLegacyResultTableBindings() error = %v", err)
	}
}

type fakeReportSchemaMigrator struct {
	tableExists bool
	columns     map[string]bool
	added       []string
	addErrorAt  string
	addError    error
}

func (migrator *fakeReportSchemaMigrator) HasTable(interface{}) bool { return migrator.tableExists }

func (migrator *fakeReportSchemaMigrator) HasColumn(_ interface{}, field string) bool {
	return migrator.columns[field]
}

func (migrator *fakeReportSchemaMigrator) AddColumn(_ interface{}, field string) error {
	if field == migrator.addErrorAt {
		return migrator.addError
	}
	if migrator.columns == nil {
		migrator.columns = make(map[string]bool)
	}
	migrator.columns[field] = true
	migrator.added = append(migrator.added, field)
	return nil
}
