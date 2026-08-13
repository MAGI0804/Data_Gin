package reportrepo

import (
	"strings"
	"testing"

	"gin-biz-web-api/model"
	"gorm.io/gorm"
)

func TestPublishedVersionListQueryScopesOwnerReportAndCursor(t *testing.T) {
	db := newDryRunDB(t).Session(&gorm.Session{DryRun: true})
	statement := buildPublishedVersionListQuery(db, 17, 9, VersionListQuery{AfterID: 23, Limit: 50}).Scan(&[]versionSummaryRecord{})
	sqlText := statement.Statement.SQL.String()
	for _, fragment := range []string{"definitions.owner_user_id = ?", "versions.definition_id = ?", "versions.status = ?", "versions.id < ?", "versions.id DESC", "LIMIT 51"} {
		if !strings.Contains(sqlText, fragment) {
			t.Fatalf("SQL %q missing %q", sqlText, fragment)
		}
	}
	wantVars := []interface{}{uint(17), uint(9), model.ReportVersionStatusPublished, uint(23)}
	for index, want := range wantVars {
		if statement.Statement.Vars[index] != want {
			t.Fatalf("vars[%d] = %#v, want %#v", index, statement.Statement.Vars[index], want)
		}
	}
}

func TestPublishedVersionSummaryQueryScopesOwnerReportStatusAndVersion(t *testing.T) {
	db := newDryRunDB(t).Session(&gorm.Session{DryRun: true})
	statement := publishedVersionSummaryQuery(db, 17, 9).Where("versions.id = ?", uint(23)).Take(&versionSummaryRecord{})
	sqlText := statement.Statement.SQL.String()
	for _, fragment := range []string{"definitions.owner_user_id = ?", "versions.definition_id = ?", "versions.status = ?", "versions.id = ?"} {
		if !strings.Contains(sqlText, fragment) {
			t.Fatalf("SQL %q missing %q", sqlText, fragment)
		}
	}
	wantVars := []interface{}{uint(17), uint(9), model.ReportVersionStatusPublished, uint(23)}
	for index, want := range wantVars {
		if statement.Statement.Vars[index] != want {
			t.Fatalf("vars[%d] = %#v, want %#v", index, statement.Statement.Vars[index], want)
		}
	}
}
