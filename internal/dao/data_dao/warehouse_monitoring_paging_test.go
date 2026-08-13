package data_dao

import (
	"database/sql"
	"fmt"
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

	enabled := true
	sourceDAO := &SourceDefinitionDAO{db: db}
	sourceStatement := sourceDAO.applyListFilters(db.Model(&model.SourceDefinition{}), SourceDefinitionListQuery{
		Keyword: "source-a", Enabled: &enabled, SourceType: "api_poll",
	}).Find(&[]model.SourceDefinition{}).Statement
	for _, fragment := range []string{"name LIKE ?", "code LIKE ?", "auth_type LIKE ?", "enabled = ?", "source_type = ?"} {
		if !strings.Contains(sourceStatement.SQL.String(), fragment) {
			t.Fatalf("source filter SQL missing %q: %s", fragment, sourceStatement.SQL.String())
		}
	}
	if strings.Contains(sourceStatement.SQL.String(), "source-a") || len(sourceStatement.Vars) != 5 {
		t.Fatalf("source filter SQL did not bind inputs: %s; vars=%v", sourceStatement.SQL.String(), sourceStatement.Vars)
	}

	ruleDAO := &TransformRuleDAO{db: db}
	ruleStatement := ruleDAO.applyListFilters(db.Model(&model.TransformRule{}), TransformRuleListQuery{
		Keyword: "rule-a", Enabled: &enabled, RuleType: "mapping", SourceID: 7,
	}).Find(&[]model.TransformRule{}).Statement
	for _, fragment := range []string{"name LIKE ?", "enabled = ?", "rule_type = ?", "source_id = ?"} {
		if !strings.Contains(ruleStatement.SQL.String(), fragment) {
			t.Fatalf("rule filter SQL missing %q: %s", fragment, ruleStatement.SQL.String())
		}
	}
	if strings.Contains(ruleStatement.SQL.String(), "rule-a") || len(ruleStatement.Vars) != 4 {
		t.Fatalf("rule filter SQL did not bind inputs: %s; vars=%v", ruleStatement.SQL.String(), ruleStatement.Vars)
	}

	destinationDAO := &DestinationDefinitionDAO{db: db}
	destinationStatement := destinationDAO.applyListFilters(db.Model(&model.DestinationDefinition{}), DestinationDefinitionListQuery{
		Keyword: "target-a", Enabled: &enabled, DestinationType: "http",
	}).Find(&[]model.DestinationDefinition{}).Statement
	for _, fragment := range []string{"name LIKE ?", "code LIKE ?", "enabled = ?", "destination_type = ?"} {
		if !strings.Contains(destinationStatement.SQL.String(), fragment) {
			t.Fatalf("destination filter SQL missing %q: %s", fragment, destinationStatement.SQL.String())
		}
	}
	if strings.Contains(destinationStatement.SQL.String(), "target-a") || len(destinationStatement.Vars) != 4 {
		t.Fatalf("destination filter SQL did not bind inputs: %s; vars=%v", destinationStatement.SQL.String(), destinationStatement.Vars)
	}

	taskDAO := &DeliveryTaskDAO{db: db}
	taskStatement := taskDAO.applyListFilters(db.Model(&model.DeliveryTask{}), DeliveryTaskListQuery{
		Keyword: "task-a", Enabled: &enabled, DestinationID: 9,
	}).Find(&[]model.DeliveryTask{}).Statement
	for _, fragment := range []string{"name LIKE ?", "clean_table LIKE ?", "enabled = ?", "destination_id = ?"} {
		if !strings.Contains(taskStatement.SQL.String(), fragment) {
			t.Fatalf("task filter SQL missing %q: %s", fragment, taskStatement.SQL.String())
		}
	}
	if strings.Contains(taskStatement.SQL.String(), "task-a") || len(taskStatement.Vars) != 4 {
		t.Fatalf("task filter SQL did not bind inputs: %s; vars=%v", taskStatement.SQL.String(), taskStatement.Vars)
	}
}

func TestConfigEnabledUpdateChangesOnlyStatusColumns(t *testing.T) {
	db := monitoringDryRunDB(t).Session(&gorm.Session{SkipDefaultTransaction: true})
	tests := []struct {
		name     string
		resource interface{}
		table    string
	}{
		{name: "source", resource: &model.SourceDefinition{}, table: "source_definitions"},
		{name: "rule", resource: &model.TransformRule{}, table: "transform_rules"},
		{name: "destination", resource: &model.DestinationDefinition{}, table: "destination_definitions"},
		{name: "task", resource: &model.DeliveryTask{}, table: "delivery_tasks"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statement := configEnabledUpdate(t.Context(), db, tt.resource, 17, false).Statement
			sql := statement.SQL.String()
			for _, fragment := range []string{"UPDATE `" + tt.table + "`", "`enabled`=?", "`updated_at`=?", "WHERE id = ?"} {
				if !strings.Contains(sql, fragment) {
					t.Fatalf("status update SQL missing %q: %s", fragment, sql)
				}
			}
			for _, forbidden := range []string{"`name`=", "`code`=", "`config_json`=", "`schema_json`=", "`payload_template`="} {
				if strings.Contains(sql, forbidden) {
					t.Fatalf("status update SQL includes %q: %s", forbidden, sql)
				}
			}
			if len(statement.Vars) != 3 || statement.Vars[0] != false || fmt.Sprint(statement.Vars[2]) != "17" {
				t.Fatalf("status update vars = %v", statement.Vars)
			}
		})
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
