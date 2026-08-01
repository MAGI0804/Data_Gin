package data_dao

import (
	"context"
	"strings"
	"testing"

	"gin-biz-web-api/model"
)

func TestIsSafeExcelSQLIdentifier(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "table", value: "bojun_retail_orders", want: true},
		{name: "column", value: "matched_docno", want: true},
		{name: "starts with number", value: "1_orders", want: false},
		{name: "contains dot", value: "warehouse.orders", want: false},
		{name: "contains sql", value: "orders` WHERE 1=1 --", want: false},
		{name: "empty", value: "", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSafeExcelSQLIdentifier(tt.value); got != tt.want {
				t.Fatalf("isSafeExcelSQLIdentifier(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestExcelMatchJobPageQueryUsesBoundFilters(t *testing.T) {
	dao := &ExcelMatchJobDAO{db: dryRunWeatherDAOTestDB(t)}
	params := ExcelMatchJobListQuery{Page: 2, PageSize: 20, Keyword: "orders", Status: "failed"}
	query := dao.applyListFilters(dao.db.WithContext(context.Background()).Model(&model.ExcelMatchJob{}), params).
		Order("id DESC").Offset((params.Page - 1) * params.PageSize).Limit(params.PageSize).
		Find(&[]model.ExcelMatchJob{})
	if query.Error != nil {
		t.Fatalf("build Excel job page query: %v", query.Error)
	}
	statement := query.Statement.SQL.String()
	for _, fragment := range []string{"source_file_name LIKE ?", "status = ?", "LIMIT 20", "OFFSET 20"} {
		if !strings.Contains(statement, fragment) {
			t.Fatalf("Excel job page query missing %q: %s", fragment, statement)
		}
	}
	if strings.Contains(statement, "orders") || strings.Contains(statement, "failed") {
		t.Fatalf("Excel job page query interpolated filters: %s", statement)
	}
	if len(query.Statement.Vars) != 2 || query.Statement.Vars[0] != "%orders%" || query.Statement.Vars[1] != "failed" {
		t.Fatalf("Excel job page query vars = %#v", query.Statement.Vars)
	}
}
