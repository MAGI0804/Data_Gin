package reportoracle

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/godror/godror"
)

// QuerySelect executes one validated SELECT with bound arguments. The caller
// owns the returned rows and must close them.
func (adapter *Adapter) QuerySelect(ctx context.Context, statement string, arguments ...interface{}) (*sql.Rows, error) {
	if adapter == nil || adapter.db == nil || ctx == nil {
		return nil, fmt.Errorf("query oracle export select: adapter is closed")
	}
	if !ValidateSelect(statement) {
		return nil, fmt.Errorf("%w: export query must contain one SELECT statement", ErrInvalidConfiguration)
	}
	arguments = append(arguments, godror.PrefetchCount(adapter.prefetchRows), godror.FetchArraySize(adapter.fetchArraySize), godror.ClobAsString())
	rows, err := adapter.db.QueryContext(ctx, strings.TrimSpace(statement), arguments...)
	if err != nil {
		return nil, fmt.Errorf("query oracle export select: %w", err)
	}
	return rows, nil
}

// ValidateSelect keeps configured SQL at a read-only boundary. Values must be
// supplied separately as named binds by the caller.
func ValidateSelect(statement string) bool {
	trimmed := strings.TrimSpace(statement)
	lower := strings.ToLower(trimmed)
	fields := strings.Fields(lower)
	if len(fields) < 2 || fields[0] != "select" || strings.Contains(trimmed, ";") ||
		strings.Contains(trimmed, "--") || strings.Contains(trimmed, "/*") || strings.Contains(trimmed, "*/") {
		return false
	}
	normalized := " " + strings.Join(fields, " ") + " "
	return !strings.Contains(normalized, " for update ")
}
