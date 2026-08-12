package reportrepo

import (
	"strings"
	"testing"

	"gorm.io/gorm"
)

func TestActorExportListQueryScopesActorAndUsesDescendingCursor(t *testing.T) {
	db := newDryRunDB(t).Session(&gorm.Session{DryRun: true})
	query := buildActorExportListQuery(db, 17, ExportListQuery{AfterID: 41, Limit: 20, Status: "READY"})
	statement := query.Scan(&[]ExportListRecord{}).Statement
	sqlText := statement.SQL.String()
	for _, fragment := range []string{
		"JOIN report_runs runs ON runs.id = exports.run_id AND runs.requested_by = ?",
		"JOIN report_definitions definitions ON definitions.id = runs.definition_id",
		"exports.created_by = ?",
		"exports.id < ?",
		"exports.status = ?",
		"ORDER BY exports.id DESC",
		"LIMIT 21",
	} {
		if !strings.Contains(sqlText, fragment) {
			t.Fatalf("export list SQL %q does not contain %q", sqlText, fragment)
		}
	}
	wantVars := []interface{}{uint(17), uint(17), uint(41), "READY"}
	if len(statement.Vars) != len(wantVars) {
		t.Fatalf("export list vars = %#v, want %#v", statement.Vars, wantVars)
	}
	for index, want := range wantVars {
		if statement.Vars[index] != want {
			t.Fatalf("export list vars[%d] = %#v, want %#v", index, statement.Vars[index], want)
		}
	}
}
