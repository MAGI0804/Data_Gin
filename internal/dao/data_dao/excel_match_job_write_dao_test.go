package data_dao

import (
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestExcelMatchJobDAOBatchUpdateBojunCompletedAtUsesTypedNullOnlyUpdate(t *testing.T) {
	t.Parallel()
	db := dryRunWeatherDAOTestDB(t)
	var statement string
	var statementVars []interface{}
	if err := db.Callback().Raw().After("gorm:raw").Register("test:capture_excel_bojun_completed_at_sql", func(tx *gorm.DB) {
		statement = tx.Statement.SQL.String()
		statementVars = append([]interface{}{}, tx.Statement.Vars...)
	}); err != nil {
		t.Fatalf("register SQL capture callback: %v", err)
	}

	_, err := (&ExcelMatchJobDAO{db: db}).BatchUpdateBojunFieldByKeys(
		t.Context(),
		"docno",
		"completed_at",
		map[string]string{"B001": "2026-07-11 10:31:22"},
	)
	if err != nil {
		t.Fatalf("BatchUpdateBojunFieldByKeys() error=%v", err)
	}
	for _, fragment := range []string{
		"UPDATE bojun_retail_orders",
		"SET `completed_at` = CASE `docno` WHEN ? THEN ? ELSE `completed_at` END",
		"updated_at = ?",
		"WHERE `docno` IN (?)",
		"completed_at IS NULL",
	} {
		if !strings.Contains(statement, fragment) {
			t.Fatalf("statement missing %q: %s", fragment, statement)
		}
	}
	if len(statementVars) != 4 {
		t.Fatalf("vars=%#v", statementVars)
	}
	completedAt, ok := statementVars[1].(time.Time)
	if !ok || completedAt.Format("2006-01-02 15:04:05") != "2026-07-11 10:31:22" ||
		completedAt.Location().String() != "Asia/Shanghai" {
		t.Fatalf("completed_at var=%#v", statementVars[1])
	}
}

func TestExcelMatchJobDAOBatchUpdateBojunCompletedAtRejectsInvalidValueAndMatchField(t *testing.T) {
	t.Parallel()
	dao := &ExcelMatchJobDAO{db: dryRunWeatherDAOTestDB(t)}
	tests := []struct {
		name       string
		matchField string
		value      string
	}{
		{name: "invalid time", matchField: "docno", value: "2026/07/11 10:31:22"},
		{name: "completed at cannot be a match key", matchField: "completed_at", value: "2026-07-11 10:31:22"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := dao.BatchUpdateBojunFieldByKeys(
				t.Context(),
				test.matchField,
				"completed_at",
				map[string]string{"B001": test.value},
			)
			if err == nil {
				t.Fatal("BatchUpdateBojunFieldByKeys() error=nil")
			}
		})
	}
}
