package bootstrap

import (
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
