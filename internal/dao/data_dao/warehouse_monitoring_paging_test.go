package data_dao

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"gin-biz-web-api/model"
)

func TestMonitoringListFiltersUseBoundParameters(t *testing.T) {
	db := monitoringDryRunDB(t)
	start := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	success := false

	logDAO := &DeliveryLogDAO{db: db}
	logQuery := logDAO.applyListFilters(db.Model(&model.DeliveryLog{}), DeliveryLogListQuery{
		DestinationCode: "mall-a", SourceCode: "source-a", Success: &success, BusinessKey: "order-1", SentFrom: &start, SentTo: &end,
	})
	logStatement := logQuery.Find(&[]model.DeliveryLog{}).Statement
	for _, fragment := range []string{"destination_code = ?", "source_code = ?", "success = ?", "business_key = ?", "sent_at >= ?", "sent_at <= ?"} {
		if !strings.Contains(logStatement.SQL.String(), fragment) {
			t.Fatalf("delivery filter SQL missing %q: %s", fragment, logStatement.SQL.String())
		}
	}
	if strings.Contains(logStatement.SQL.String(), "mall-a") || len(logStatement.Vars) != 6 {
		t.Fatalf("delivery filter SQL did not bind inputs: %s; vars=%v", logStatement.SQL.String(), logStatement.Vars)
	}

	runDAO := &PipelineRunDAO{db: db}
	runQuery := runDAO.applyListFilters(db.Model(&model.PipelineRun{}), PipelineRunListQuery{
		Status: "failed", RunType: "delivery", TraceID: "trace-1", StartedAt: &start, EndedAt: &end,
	})
	runStatement := runQuery.Find(&[]model.PipelineRun{}).Statement
	for _, fragment := range []string{"status = ?", "run_type = ?", "trace_id = ?", "started_at >= ?", "started_at <= ?"} {
		if !strings.Contains(runStatement.SQL.String(), fragment) {
			t.Fatalf("run filter SQL missing %q: %s", fragment, runStatement.SQL.String())
		}
	}
	if strings.Contains(runStatement.SQL.String(), "trace-1") || len(runStatement.Vars) != 5 {
		t.Fatalf("run filter SQL did not bind inputs: %s; vars=%v", runStatement.SQL.String(), runStatement.Vars)
	}
}

func monitoringDryRunDB(t *testing.T) *gorm.DB {
	t.Helper()
	sqlDB, err := sql.Open("mysql", "")
	if err != nil {
		t.Fatalf("open dry-run mysql database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("open dry-run gorm database: %v", err)
	}
	return db
}
