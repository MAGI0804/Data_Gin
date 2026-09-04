package auth_svc

import (
	"database/sql"
	"reflect"
	"strings"
	"testing"

	"gin-biz-web-api/internal/reportrepo"
	"gin-biz-web-api/model"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestNormalizeAccessReportActions(t *testing.T) {
	want := []string{reportrepo.ReportActionQuery, reportrepo.ReportActionExport}
	got, err := normalizeAccessReportActions([]string{" export ", "query", "QUERY"})
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeAccessReportActions() = %#v, %v, want %#v", got, err, want)
	}
	for _, actions := range [][]string{{"EXPORT"}, {"DELETE"}} {
		if _, err := normalizeAccessReportActions(actions); err == nil {
			t.Fatalf("normalizeAccessReportActions(%#v) accepted invalid actions", actions)
		}
	}
}

func TestSplitAccountReportCategoryActionsSeparatesDirectAndInherited(t *testing.T) {
	direct, inherited, err := splitAccountReportCategoryActions([]model.ReportCategoryGrant{
		{SubjectType: reportCategorySubjectUser, ActionsJSON: model.JSONText(`["QUERY"]`)},
		{SubjectType: reportCategorySubjectRole, ActionsJSON: model.JSONText(`["QUERY","EXPORT"]`)},
	})
	if err != nil || !reflect.DeepEqual(direct, []string{"QUERY"}) || !reflect.DeepEqual(inherited, []string{"QUERY", "EXPORT"}) {
		t.Fatalf("splitAccountReportCategoryActions() = %#v, %#v, %v", direct, inherited, err)
	}
}

func newAuthServiceDryRunDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn: &sql.DB{}, SkipInitializeWithVersion: true,
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true, SkipDefaultTransaction: true})
	if err != nil {
		t.Fatalf("open dry-run database: %v", err)
	}
	return db
}

func TestAccessAccountReportCategoryQueryKeepsConfiguredCategoriesWithoutReports(t *testing.T) {
	db := newAuthServiceDryRunDB(t)
	statement := accessAccountReportCategoryQuery(db.Session(&gorm.Session{DryRun: true})).Scan(&[]accessAccountReportCategoryRecord{}).Statement
	sqlText := statement.SQL.String()
	for _, fragment := range []string{"SELECT category FROM report_category_access", "LEFT JOIN report_definitions", "LEFT JOIN report_category_access"} {
		if !strings.Contains(sqlText, fragment) {
			t.Fatalf("report category query %q does not contain %q", sqlText, fragment)
		}
	}
}
