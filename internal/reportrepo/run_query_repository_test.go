package reportrepo

import (
	"strings"
	"testing"

	"gin-biz-web-api/model"

	"gorm.io/gorm"
)

func TestActorRunQueryAlwaysScopesRequestedBy(t *testing.T) {
	db := newDryRunDB(t).Session(&gorm.Session{DryRun: true})
	query := buildActorRunQuery(db.Model(&model.ReportRun{}), 17, 31).Find(&model.ReportRun{})
	statement := query.Statement.SQL.String()
	for _, fragment := range []string{"id = ?", "requested_by = ?"} {
		if !strings.Contains(statement, fragment) {
			t.Fatalf("actor run query %q does not contain %q", statement, fragment)
		}
	}
}
