package reportoracle

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/godror/godror"
)

const (
	defaultResultPageSize = 100
	maxResultPageSize     = 1000
	maxResultColumns      = 512
	defaultPurgeBatchSize = 5000
	maxPurgeBatchSize     = 20000
)

type ResultSnapshotRef struct {
	Table       ResultTableRef
	RunIDColumn string
	RowIDColumn string
	Columns     []string
}

// ResultSnapshotContract is produced only after Oracle metadata proves the
// run and row columns are safe for snapshot keyset pagination.
type ResultSnapshotContract struct {
	table       ResultTableRef
	runIDColumn string
	rowIDColumn string
}

func ValidateResultSnapshotContract(contract ResultSnapshotContract, ref ResultSnapshotRef) error {
	table, runIDColumn, rowIDColumn, _, err := normalizeSnapshotRef(ref)
	if err != nil {
		return err
	}
	if contract.table != table || contract.runIDColumn != runIDColumn || contract.rowIDColumn != rowIDColumn {
		return configurationError("result snapshot contract does not match report version")
	}
	return nil
}

type ResultPagePlan struct {
	initialStatement string
	nextStatement    string
	columns          []string
}

func (plan ResultPagePlan) Columns() []string {
	return append([]string(nil), plan.columns...)
}

type ResultRow struct {
	RowID  int64
	Values []interface{}
}

type ResultPage struct {
	Columns   []string
	Rows      []ResultRow
	NextRowID int64
	HasNext   bool
}

type PurgePlan struct {
	statement string
}

type ResultCountPlan struct {
	statement string
}

func (adapter *Adapter) InspectResultSnapshotContract(
	ctx context.Context,
	ref ResultSnapshotRef,
) (ResultSnapshotContract, error) {
	normalizedTable, runIDColumn, rowIDColumn, _, err := normalizeSnapshotRef(ref)
	if err != nil {
		return ResultSnapshotContract{}, err
	}
	columns, err := adapter.InspectResultTable(ctx, normalizedTable)
	if err != nil {
		return ResultSnapshotContract{}, err
	}
	var uniqueIndexes int
	if err := adapter.db.QueryRowContext(ctx, uniqueResultKeySQL,
		normalizedTable.Owner, normalizedTable.Name, runIDColumn, rowIDColumn,
	).Scan(&uniqueIndexes); err != nil {
		return ResultSnapshotContract{}, fmt.Errorf("inspect oracle result key: %w", err)
	}
	return CompileResultSnapshotContract(ref, columns, uniqueIndexes > 0)
}

func CompileResultSnapshotContract(
	ref ResultSnapshotRef,
	columns []ResultColumn,
	hasUniqueKey bool,
) (ResultSnapshotContract, error) {
	normalizedTable, runIDColumn, rowIDColumn, _, err := normalizeSnapshotRef(ref)
	if err != nil {
		return ResultSnapshotContract{}, err
	}
	byName := make(map[string]ResultColumn, len(columns))
	for _, column := range columns {
		byName[strings.ToUpper(column.Name)] = column
	}
	runColumn, runExists := byName[runIDColumn]
	rowColumn, rowExists := byName[rowIDColumn]
	if !runExists || !rowExists || runColumn.Nullable || rowColumn.Nullable {
		return ResultSnapshotContract{}, configurationError("result key columns must exist and be not null")
	}
	if !supportedRunIDType(runColumn.DataType) {
		return ResultSnapshotContract{}, configurationError("result run id column type is unsupported")
	}
	if strings.ToUpper(rowColumn.DataType) != "NUMBER" || rowColumn.DataScale == nil || *rowColumn.DataScale != 0 {
		return ResultSnapshotContract{}, configurationError("result row id column must be NUMBER with scale 0")
	}
	if !hasUniqueKey {
		return ResultSnapshotContract{}, configurationError("result key columns require a two-column unique index")
	}
	return ResultSnapshotContract{table: normalizedTable, runIDColumn: runIDColumn, rowIDColumn: rowIDColumn}, nil
}

func BuildResultPagePlan(contract ResultSnapshotContract, columns []string) (ResultPagePlan, error) {
	if contract.table.Owner == "" || contract.table.Name == "" || contract.runIDColumn == "" || contract.rowIDColumn == "" {
		return ResultPagePlan{}, configurationError("result snapshot contract is not validated")
	}
	_, _, _, normalizedColumns, err := normalizeSnapshotRef(ResultSnapshotRef{
		Table: contract.table, RunIDColumn: contract.runIDColumn, RowIDColumn: contract.rowIDColumn, Columns: columns,
	})
	if err != nil {
		return ResultPagePlan{}, err
	}
	selectColumns := make([]string, 0, len(normalizedColumns)+1)
	selectColumns = append(selectColumns, contract.rowIDColumn)
	selectColumns = append(selectColumns, normalizedColumns...)
	initialStatement := fmt.Sprintf(
		"SELECT %s FROM %s.%s WHERE %s = :1 ORDER BY %s ASC FETCH NEXT :2 ROWS ONLY",
		strings.Join(selectColumns, ", "), contract.table.Owner, contract.table.Name,
		contract.runIDColumn, contract.rowIDColumn,
	)
	nextStatement := fmt.Sprintf(
		"SELECT %s FROM %s.%s WHERE %s = :1 AND %s > :2 ORDER BY %s ASC FETCH NEXT :3 ROWS ONLY",
		strings.Join(selectColumns, ", "), contract.table.Owner, contract.table.Name,
		contract.runIDColumn, contract.rowIDColumn, contract.rowIDColumn,
	)
	return ResultPagePlan{initialStatement: initialStatement, nextStatement: nextStatement, columns: normalizedColumns}, nil
}

func BuildPurgePlan(contract ResultSnapshotContract) (PurgePlan, error) {
	if contract.table.Owner == "" || contract.table.Name == "" || contract.runIDColumn == "" {
		return PurgePlan{}, configurationError("result snapshot contract is not validated")
	}
	statement := fmt.Sprintf(
		"DELETE FROM %s.%s WHERE ROWID IN (SELECT ROWID FROM %s.%s WHERE %s = :1 AND ROWNUM <= :2)",
		contract.table.Owner, contract.table.Name, contract.table.Owner, contract.table.Name, contract.runIDColumn,
	)
	return PurgePlan{statement: statement}, nil
}

func BuildResultCountPlan(contract ResultSnapshotContract) (ResultCountPlan, error) {
	if contract.table.Owner == "" || contract.table.Name == "" || contract.runIDColumn == "" {
		return ResultCountPlan{}, configurationError("result snapshot contract is not validated")
	}
	return ResultCountPlan{statement: fmt.Sprintf(
		"SELECT COUNT(*) FROM %s.%s WHERE %s = :1",
		contract.table.Owner, contract.table.Name, contract.runIDColumn,
	)}, nil
}

func (adapter *Adapter) CountResultRows(ctx context.Context, plan ResultCountPlan, runID string) (int64, error) {
	if adapter == nil || adapter.db == nil || strings.TrimSpace(plan.statement) == "" || strings.TrimSpace(runID) == "" {
		return 0, fmt.Errorf("count oracle report result rows: invalid request")
	}
	var count int64
	if err := adapter.db.QueryRowContext(ctx, plan.statement, runID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count oracle report result rows: %w", err)
	}
	if count < 0 {
		return 0, fmt.Errorf("count oracle report result rows: invalid count")
	}
	return count, nil
}

func (adapter *Adapter) CountResultRowsTx(ctx context.Context, tx *sql.Tx, plan ResultCountPlan, runID string) (int64, error) {
	if adapter == nil || tx == nil || strings.TrimSpace(plan.statement) == "" || strings.TrimSpace(runID) == "" {
		return 0, fmt.Errorf("count oracle report result rows: invalid transaction request")
	}
	var count int64
	if err := tx.QueryRowContext(ctx, plan.statement, runID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count oracle report result rows: %w", err)
	}
	if count < 0 {
		return 0, fmt.Errorf("count oracle report result rows: invalid count")
	}
	return count, nil
}

func (adapter *Adapter) ReadResultPage(
	ctx context.Context,
	plan ResultPagePlan,
	runID string,
	afterRowID *int64,
	pageSize int,
) (ResultPage, error) {
	if adapter == nil || adapter.db == nil || strings.TrimSpace(plan.initialStatement) == "" ||
		strings.TrimSpace(plan.nextStatement) == "" || len(plan.columns) == 0 || strings.TrimSpace(runID) == "" {
		return ResultPage{}, fmt.Errorf("read oracle report result page: invalid request")
	}
	if pageSize == 0 {
		pageSize = defaultResultPageSize
	}
	if pageSize < 1 || pageSize > maxResultPageSize {
		return ResultPage{}, fmt.Errorf("read oracle report result page: invalid page size")
	}
	statement := plan.initialStatement
	arguments := []interface{}{runID, pageSize + 1}
	if afterRowID != nil {
		statement = plan.nextStatement
		arguments = []interface{}{runID, *afterRowID, pageSize + 1}
	}
	arguments = append(arguments, godror.PrefetchCount(adapter.prefetchRows),
		godror.FetchArraySize(adapter.fetchArraySize), godror.ClobAsString())
	rows, err := adapter.db.QueryContext(ctx, statement, arguments...)
	if err != nil {
		return ResultPage{}, fmt.Errorf("read oracle report result page: %w", err)
	}
	defer rows.Close()

	result := ResultPage{Columns: plan.Columns(), Rows: make([]ResultRow, 0, pageSize)}
	for rows.Next() {
		values := make([]interface{}, len(plan.columns)+1)
		destinations := make([]interface{}, len(values))
		for index := range values {
			destinations[index] = &values[index]
		}
		if err := rows.Scan(destinations...); err != nil {
			return ResultPage{}, fmt.Errorf("scan oracle report result row: %w", err)
		}
		rowID, err := oracleRowID(values[0])
		if err != nil {
			return ResultPage{}, err
		}
		if len(result.Rows) == pageSize {
			result.HasNext = true
			break
		}
		result.Rows = append(result.Rows, ResultRow{RowID: rowID, Values: values[1:]})
		result.NextRowID = rowID
	}
	if err := rows.Err(); err != nil {
		return ResultPage{}, fmt.Errorf("iterate oracle report result rows: %w", err)
	}
	return result, nil
}

func (adapter *Adapter) PurgeResultBatch(
	ctx context.Context,
	tx *sql.Tx,
	plan PurgePlan,
	runID string,
	batchSize int,
) (int64, error) {
	if adapter == nil || tx == nil || strings.TrimSpace(plan.statement) == "" || strings.TrimSpace(runID) == "" {
		return 0, fmt.Errorf("purge oracle report result: invalid request")
	}
	if batchSize == 0 {
		batchSize = defaultPurgeBatchSize
	}
	if batchSize < 1 || batchSize > maxPurgeBatchSize {
		return 0, fmt.Errorf("purge oracle report result: invalid batch size")
	}
	result, err := tx.ExecContext(ctx, plan.statement, runID, batchSize)
	if err != nil {
		if contextError := ctx.Err(); contextError != nil {
			return 0, contextError
		}
		return 0, fmt.Errorf("purge oracle report result: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("purge oracle report result: read deleted rows: %w", err)
	}
	return deleted, nil
}

func supportedRunIDType(dataType string) bool {
	switch strings.ToUpper(strings.TrimSpace(dataType)) {
	case "CHAR", "NCHAR", "VARCHAR2", "NVARCHAR2":
		return true
	default:
		return false
	}
}

func normalizeSnapshotRef(ref ResultSnapshotRef) (ResultTableRef, string, string, []string, error) {
	table, err := NormalizeResultTableRef(ref.Table)
	if err != nil {
		return ResultTableRef{}, "", "", nil, err
	}
	runIDColumn, err := normalizeIdentifier(ref.RunIDColumn, "result run id column")
	if err != nil {
		return ResultTableRef{}, "", "", nil, err
	}
	rowIDColumn, err := normalizeIdentifier(ref.RowIDColumn, "result row id column")
	if err != nil {
		return ResultTableRef{}, "", "", nil, err
	}
	if runIDColumn == rowIDColumn {
		return ResultTableRef{}, "", "", nil, configurationError("result key columns must be distinct")
	}
	if len(ref.Columns) == 0 || len(ref.Columns) > maxResultColumns {
		return ResultTableRef{}, "", "", nil, configurationError("result columns are invalid")
	}
	columns := make([]string, len(ref.Columns))
	seen := map[string]struct{}{runIDColumn: {}, rowIDColumn: {}}
	for index, column := range ref.Columns {
		columns[index], err = normalizeIdentifier(column, "result column")
		if err != nil {
			return ResultTableRef{}, "", "", nil, err
		}
		if _, exists := seen[columns[index]]; exists {
			return ResultTableRef{}, "", "", nil, configurationError("result columns contain duplicate or key column")
		}
		seen[columns[index]] = struct{}{}
	}
	return table, runIDColumn, rowIDColumn, columns, nil
}

const uniqueResultKeySQL = `
WITH filters AS (
    SELECT :1 AS table_owner, :2 AS table_name, :3 AS run_id_column, :4 AS row_id_column
    FROM dual
)
SELECT COUNT(*)
FROM (
    SELECT indexes.index_name
    FROM all_indexes indexes
    JOIN all_ind_columns columns
      ON columns.index_owner = indexes.owner AND columns.index_name = indexes.index_name
    CROSS JOIN filters
    WHERE indexes.table_owner = filters.table_owner
      AND indexes.table_name = filters.table_name
      AND indexes.uniqueness = 'UNIQUE'
    GROUP BY indexes.index_name, filters.run_id_column, filters.row_id_column
    HAVING COUNT(*) = 2
       AND SUM(CASE WHEN columns.column_name IN (filters.run_id_column, filters.row_id_column) THEN 1 ELSE 0 END) = 2
)`

func oracleRowID(value interface{}) (int64, error) {
	switch typed := value.(type) {
	case int64:
		return typed, nil
	case int32:
		return int64(typed), nil
	case int:
		return int64(typed), nil
	case godror.Number:
		parsed, err := strconv.ParseInt(string(typed), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("scan oracle report result row: invalid row id")
		}
		return parsed, nil
	case string:
		parsed, err := strconv.ParseInt(typed, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("scan oracle report result row: invalid row id")
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("scan oracle report result row: unsupported row id type %T", value)
	}
}
