package source

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
)

type DatabaseConnector struct{}

func (DatabaseConnector) Code() string {
	return "database"
}

func (connector DatabaseConnector) Test(ctx context.Context, cfg Config) error {
	db, err := connector.openDB(cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	return db.PingContext(ctx)
}

func (connector DatabaseConnector) Fetch(ctx context.Context, cfg Config, cursor FetchCursor) (*FetchResult, error) {
	db, err := connector.openDB(cfg)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	query := StringValue(cfg, "query")
	if query == "" {
		return nil, errorsf("database source config query is required")
	}

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query database source: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("read database source columns: %w", err)
	}

	records := []map[string]interface{}{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("scan database source row: %w", err)
		}

		record := make(map[string]interface{}, len(columns))
		for i, column := range columns {
			record[column] = normalizeDBValue(values[i])
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate database source rows: %w", err)
	}

	return &FetchResult{
		Records: records,
		Cursor:  cursor,
	}, nil
}

func (DatabaseConnector) openDB(cfg Config) (*sql.DB, error) {
	driver := StringValue(cfg, "driver")
	if driver == "" {
		driver = "mysql"
	}
	if driver != "mysql" {
		return nil, fmt.Errorf("unsupported database source driver %q", driver)
	}

	dsn := StringValue(cfg, "dsn")
	if dsn == "" {
		return nil, errorsf("database source config dsn is required")
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("open database source: %w", err)
	}
	return db, nil
}

func normalizeDBValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case []byte:
		return string(typed)
	default:
		return typed
	}
}
