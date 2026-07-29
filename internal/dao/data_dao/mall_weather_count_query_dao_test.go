package data_dao

import (
	"strings"
	"testing"
)

func TestBuildWeatherCountStatementRemovesOrderingAndLimit(t *testing.T) {
	statement := "SELECT ranked.* FROM (SELECT w.*, ROW_NUMBER() OVER (ORDER BY w.id) AS version_rank FROM weather AS w) AS ranked\nWHERE ranked.version_rank = 1\nORDER BY ranked.id ASC\nLIMIT ?"
	countStatement, args, err := buildWeatherCountStatement(statement, []interface{}{7, 200})
	if err != nil {
		t.Fatalf("buildWeatherCountStatement() error = %v", err)
	}
	if len(args) != 1 || args[0] != 7 || strings.Contains(countStatement, "ORDER BY ranked.id") ||
		strings.Contains(countStatement, "LIMIT") || !strings.Contains(countStatement, "version_rank = 1") {
		t.Fatalf("statement=%q args=%v", countStatement, args)
	}
}

func TestBuildWeatherCountStatementRejectsUnexpectedQuery(t *testing.T) {
	if _, _, err := buildWeatherCountStatement("SELECT 1", nil); err == nil {
		t.Fatal("buildWeatherCountStatement() accepted an unbounded source")
	}
}
