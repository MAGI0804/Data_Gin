package data_dao

import (
	"strings"
	"testing"
	"time"
)

func TestBuildFetchRunQueryUsesFiltersAndDescendingKeyset(t *testing.T) {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	cursor := time.Date(2026, 7, 22, 1, 2, 3, 0, time.UTC)
	statement, args, err := buildFetchRunQuery(FetchRunQuery{
		MallID: 7, StartUTC: start, EndUTC: start.Add(31 * 24 * time.Hour),
		CorrelationID: "manual:opaque-123", TaskKind: "mall:weather:full",
		EndpointKind: "v26_weather", Status: "partial_success",
		AfterCreatedAt: &cursor, AfterID: 9, Limit: 201,
	})
	if err != nil {
		t.Fatalf("buildFetchRunQuery() error=%v", err)
	}
	for _, fragment := range []string{
		"r.mall_id = ?", "r.created_at >= ?", "r.created_at < ?", "r.task_window = ?", "r.task_kind = ?",
		"r.endpoint_kind = ?", "r.status = ?", "r.created_at < ?", "r.id < ?",
		"ORDER BY r.created_at DESC, r.id DESC", "LIMIT ?",
	} {
		if !strings.Contains(statement, fragment) {
			t.Fatalf("query does not contain %q:\n%s", fragment, statement)
		}
	}
	if len(args) != 11 || strings.Contains(statement, "manual:opaque-123") ||
		strings.Contains(statement, "mall:weather:full") || strings.Contains(statement, "partial_success") ||
		args[3] != "manual:opaque-123" {
		t.Fatalf("statement=%s args=%v", statement, args)
	}
}

func TestBuildFetchRunQueryRejectsIncompleteCursor(t *testing.T) {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	if _, _, err := buildFetchRunQuery(FetchRunQuery{
		MallID: 7, StartUTC: start, EndUTC: start.Add(time.Hour), AfterID: 1,
	}); err == nil {
		t.Fatal("buildFetchRunQuery() accepted cursor without created-at")
	}
}
