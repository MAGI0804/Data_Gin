package data_dao

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"gin-biz-web-api/model"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestBuildOutboxClaimQueryScopesRegisteredTaskTypes(t *testing.T) {
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: &sql.DB{}, SkipInitializeWithVersion: true}), &gorm.Config{
		DryRun: true, DisableAutomaticPing: true, SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatalf("open dry-run database: %v", err)
	}
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	statement := buildOutboxClaimQuery(db, []string{"report:run"}, now, now.Add(-time.Minute), 20).Find(&[]model.AsyncJobOutbox{})
	if statement.Error != nil {
		t.Fatalf("build claim query: %v", statement.Error)
	}
	sqlText := statement.Statement.SQL.String()
	for _, fragment := range []string{"published_at IS NULL", "task_type IN", "available_at <=", "locked_at IS NULL", "ORDER BY id ASC", "LIMIT 20"} {
		if !strings.Contains(sqlText, fragment) {
			t.Fatalf("claim SQL %q does not contain %q", sqlText, fragment)
		}
	}
	if len(statement.Statement.Vars) < 1 || statement.Statement.Vars[0] != "report:run" {
		t.Fatalf("claim vars = %#v", statement.Statement.Vars)
	}
}
