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
