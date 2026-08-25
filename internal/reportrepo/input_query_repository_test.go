package reportrepo

import (
	"strings"
	"testing"

	"gorm.io/gorm"
)

func TestInputQueryReferenceScopeUsesCurrentVersionsAndExactJSONPath(t *testing.T) {
	db := newDryRunDB(t)
	statement := buildReportInputQueryReferenceQuery(db.Session(&gorm.Session{DryRun: true}), "stores").Count(new(int64))
	sqlText := statement.Statement.SQL.String()
	for _, fragment := range []string{
		"definitions.current_draft_version_id = versions.id",
		"definitions.current_published_version_id = versions.id",
		"JSON_SEARCH(versions.input_schema_json, 'one', ?, NULL, '$.*.queryName') IS NOT NULL",
	} {
		if !strings.Contains(sqlText, fragment) {
			t.Fatalf("reference SQL %q does not contain %q", sqlText, fragment)
		}
	}
	if !containsSQLVariable(statement.Statement.Vars, "stores") {
		t.Fatalf("reference vars %#v do not contain query name", statement.Statement.Vars)
	}
}
