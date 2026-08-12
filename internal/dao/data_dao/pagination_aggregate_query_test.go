package data_dao

import (
	"context"
	"strings"
	"testing"

	"gorm.io/gorm"
)

func TestCleanRecordPaginationDoesNotReuseAggregateSelect(t *testing.T) {
	db := monitoringDryRunDB(t)
	statements := recordQueryStatements(t, db)
	dao := &CleanRecordDAO{db: db}

	if _, err := dao.FindWithPagination(context.Background(), CleanRecordListQuery{Page: 1, PageSize: 20}); err != nil {
		t.Fatalf("FindWithPagination() error = %v", err)
	}
	assertPaginationQueries(t, *statements, "clean_records")
}

func TestProcessedDataPaginationDoesNotReuseAggregateSelect(t *testing.T) {
	db := monitoringDryRunDB(t)
	statements := recordQueryStatements(t, db)
	dao := &ProcessedDataDAO{db: db}

	if _, err := dao.FindWithPagination(context.Background(), ProcessedDataListQuery{Page: 1, PageSize: 20}); err != nil {
		t.Fatalf("FindWithPagination() error = %v", err)
	}
	assertPaginationQueries(t, *statements, "processed_data")
}

func recordQueryStatements(t *testing.T, db *gorm.DB) *[]string {
	t.Helper()
	statements := make([]string, 0, 3)
	if err := db.Callback().Query().After("gorm:query").Register("test:record_pagination_queries", func(tx *gorm.DB) {
		statements = append(statements, tx.Statement.SQL.String())
	}); err != nil {
		t.Fatalf("register query recorder: %v", err)
	}
	return &statements
}

func assertPaginationQueries(t *testing.T, statements []string, table string) {
	t.Helper()
	if len(statements) != 3 {
		t.Fatalf("query count = %d, want 3; statements=%v", len(statements), statements)
	}
	if !strings.Contains(strings.ToUpper(statements[1]), "AVG(QUALITY_SCORE)") {
		t.Fatalf("aggregate query missing AVG: %s", statements[1])
	}
	listSQL := strings.ToUpper(statements[2])
	if strings.Contains(listSQL, "AVG(QUALITY_SCORE)") || !strings.Contains(statements[2], table) || !strings.Contains(listSQL, "ORDER BY CREATED_AT DESC") || !strings.Contains(listSQL, "LIMIT") {
		t.Fatalf("pagination query retained aggregate state: %s", statements[2])
	}
}
