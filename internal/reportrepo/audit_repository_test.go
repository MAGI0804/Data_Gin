package reportrepo

import (
	"context"
	"strings"
	"testing"

	"gin-biz-web-api/model"

	"gorm.io/gorm"
)

func TestReportAuditListQueryUsesBoundFiltersAndDescendingCursor(t *testing.T) {
	db := newDryRunDB(t).Session(&gorm.Session{DryRun: true})
	query := buildReportAuditListQuery(db, ReportAuditListQuery{
		AfterID: 90, Limit: 20, Action: "REPORT_RESULT_QUERY_SUCCESS",
		TargetType: "REPORT_RUN", TargetID: 31,
	})
	statement := query.Find(&[]model.ReportAudit{}).Statement
	sqlText := statement.SQL.String()
	for _, fragment := range []string{
		"FROM `report_audits`", "id < ?", "action = ?", "target_type = ?", "target_id = ?",
		"ORDER BY id DESC", "LIMIT 21",
	} {
		if !strings.Contains(sqlText, fragment) {
			t.Fatalf("audit list SQL %q does not contain %q", sqlText, fragment)
		}
	}
	wantVars := []interface{}{uint(90), "REPORT_RESULT_QUERY_SUCCESS", "REPORT_RUN", uint(31)}
	for index, want := range wantVars {
		if statement.Vars[index] != want {
			t.Fatalf("vars[%d] = %#v, want %#v", index, statement.Vars[index], want)
		}
	}
}

func TestReportAuditActorValidationSupportsUsersAndSystemWorkers(t *testing.T) {
	valid := []model.ReportAudit{
		{ActorType: model.ReportAuditActorUser, ActorUserID: 7},
		{ActorType: model.ReportAuditActorSystem, ActorUserID: 0},
		{ActorUserID: 7},
	}
	for _, audit := range valid {
		if !validReportAuditActor(audit) {
			t.Fatalf("validReportAuditActor(%#v)=false", audit)
		}
	}
	for _, audit := range []model.ReportAudit{
		{ActorType: model.ReportAuditActorUser},
		{ActorType: model.ReportAuditActorSystem, ActorUserID: 7},
		{ActorType: "UNKNOWN", ActorUserID: 7},
	} {
		if validReportAuditActor(audit) {
			t.Fatalf("validReportAuditActor(%#v)=true", audit)
		}
	}
	repository := &Repository{}
	if err := repository.WriteReportAudit(context.Background(), model.ReportAudit{}); err == nil {
		t.Fatal("invalid repository accepted audit")
	}
}
