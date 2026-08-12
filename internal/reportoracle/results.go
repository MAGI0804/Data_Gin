package reportoracle

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gin-biz-web-api/internal/reportquery"

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
	columns     map[string]struct{}
}

func ValidateResultSnapshotContract(contract ResultSnapshotContract, ref ResultSnapshotRef) error {
	table, runIDColumn, rowIDColumn, columns, err := normalizeSnapshotRef(ref)
	if err != nil {
		return err
	}
	if contract.table != table || contract.runIDColumn != runIDColumn || contract.rowIDColumn != rowIDColumn || len(contract.columns) != len(columns) {
		return configurationError("result snapshot contract does not match report version")
	}
	for _, column := range columns {
		if _, ok := contract.columns[column]; !ok {
			return configurationError("result snapshot contract columns do not match report version")
		}
	}
	return nil
}

type ResultPagePlan struct {
	initialStatement string
	nextStatement    string
	columns          []string
	query            reportquery.Query
	initialArguments []interface{}
	nextArguments    []interface{}
}

func (plan ResultPagePlan) Columns() []string {
	return append([]string(nil), plan.columns...)
}

type ResultRow struct {
	RowID     int64
	Values    []interface{}
	SortValue *reportquery.Value
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
	if strings.ToUpper(rowColumn.DataType) != "NUMBER" || rowColumn.DataScale == nil || *rowColumn.DataScale != 0 || rowColumn.DataPrecision == nil || *rowColumn.DataPrecision < 1 || *rowColumn.DataPrecision > 18 {
		return ResultSnapshotContract{}, configurationError("result row id column must be NUMBER(1..18,0)")
	}
	if !hasUniqueKey {
		return ResultSnapshotContract{}, configurationError("result key columns require a two-column unique index")
	}
	allowedColumns := make(map[string]struct{})
	for _, column := range ref.Columns {
		allowedColumns[strings.ToUpper(strings.TrimSpace(column))] = struct{}{}
	}
	return ResultSnapshotContract{table: normalizedTable, runIDColumn: runIDColumn, rowIDColumn: rowIDColumn, columns: allowedColumns}, nil
}

func BuildResultPagePlan(contract ResultSnapshotContract, columns []string) (ResultPagePlan, error) {
	return BuildResultQueryPlan(contract, columns, reportquery.Query{})
}

func BuildResultQueryPlan(contract ResultSnapshotContract, columns []string, query reportquery.Query) (ResultPagePlan, error) {
	if contract.table.Owner == "" || contract.table.Name == "" || contract.runIDColumn == "" || contract.rowIDColumn == "" {
		return ResultPagePlan{}, configurationError("result snapshot contract is not validated")
	}
	_, _, _, normalizedColumns, err := normalizeSnapshotRef(ResultSnapshotRef{
		Table: contract.table, RunIDColumn: contract.runIDColumn, RowIDColumn: contract.rowIDColumn, Columns: columns,
	})
	if err != nil {
		return ResultPagePlan{}, err
	}
	for _, column := range normalizedColumns {
		if _, allowed := contract.columns[column]; !allowed {
			return ResultPagePlan{}, configurationError("result output column is outside the published contract")
		}
	}
	if len(query.Filters) > reportquery.MaxFilters || len(query.Sort) > reportquery.MaxSorts {
		return ResultPagePlan{}, configurationError("result query is excessive")
	}
	selectColumns := make([]string, 0, len(normalizedColumns)+2)
	selectColumns = append(selectColumns, contract.rowIDColumn)
	if len(query.Sort) == 1 {
		sortColumn, sortErr := normalizeIdentifier(query.Sort[0].Column, "result sort column")
		if sortErr != nil || (query.Sort[0].Direction != "ASC" && query.Sort[0].Direction != "DESC") {
			return ResultPagePlan{}, configurationError("result sort is invalid")
		}
		query.Sort[0].Column = sortColumn
		if _, allowed := contract.columns[sortColumn]; !allowed {
			return ResultPagePlan{}, configurationError("result sort column is outside the published contract")
		}
		selectColumns = append(selectColumns, query.Sort[0].Column)
	}
	selectColumns = append(selectColumns, normalizedColumns...)
	filterSQL, filterArguments, nextBind, err := buildResultFilters(query.Filters, contract.columns, 2)
	if err != nil {
		return ResultPagePlan{}, err
	}
	where := contract.runIDColumn + " = :1" + filterSQL
	order := contract.rowIDColumn + " ASC"
	initialArguments := append([]interface{}(nil), filterArguments...)
	nextArguments := append([]interface{}(nil), filterArguments...)
	initialFetchBind := nextBind
	nextPredicate := fmt.Sprintf("%s > :%d", contract.rowIDColumn, nextBind)
	nextBind++
	if len(query.Sort) == 1 {
		sort := query.Sort[0]
		order = fmt.Sprintf("%s %s NULLS LAST, %s ASC", sort.Column, sort.Direction, contract.rowIDColumn)
		comparison := ">"
		if sort.Direction == "DESC" {
			comparison = "<"
		}
		nextPredicate = fmt.Sprintf("((:%d = 1 AND %s IS NULL AND %s > :%d) OR (:%d = 0 AND ((%s %s :%d) OR %s IS NULL OR (%s = :%d AND %s > :%d))))",
			nextBind, sort.Column, contract.rowIDColumn, nextBind+1, nextBind+2, sort.Column, comparison, nextBind+3, sort.Column, sort.Column, nextBind+4, contract.rowIDColumn, nextBind+5)
		nextBind += 6
	}
	initialStatement := fmt.Sprintf(
		"SELECT %s FROM %s.%s WHERE %s ORDER BY %s FETCH NEXT :%d ROWS ONLY",
		strings.Join(selectColumns, ", "), contract.table.Owner, contract.table.Name,
		where, order, initialFetchBind,
	)
	nextStatement := fmt.Sprintf(
		"SELECT %s FROM %s.%s WHERE %s AND %s ORDER BY %s FETCH NEXT :%d ROWS ONLY",
		strings.Join(selectColumns, ", "), contract.table.Owner, contract.table.Name,
		where, nextPredicate, order, nextBind,
	)
	return ResultPagePlan{initialStatement: initialStatement, nextStatement: nextStatement, columns: normalizedColumns, query: query, initialArguments: initialArguments, nextArguments: nextArguments}, nil
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
	after *ResultCursor,
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
	arguments := []interface{}{runID}
	arguments = append(arguments, plan.initialArguments...)
	if after != nil {
		statement = plan.nextStatement
		arguments = []interface{}{runID}
		arguments = append(arguments, plan.nextArguments...)
		if len(plan.query.Sort) == 0 {
			arguments = append(arguments, after.RowID)
		} else {
			isNull := 0
			var value interface{}
			if after.SortValue == nil {
				isNull = 1
			} else {
				var err error
				value, err = resultQueryBind(*after.SortValue, "")
				if err != nil {
					return ResultPage{}, err
				}
			}
			arguments = append(arguments, isNull, after.RowID, isNull, value, value, after.RowID)
		}
	}
	arguments = append(arguments, pageSize+1)
	arguments = append(arguments, godror.PrefetchCount(adapter.prefetchRows),
		godror.FetchArraySize(adapter.fetchArraySize), godror.ClobAsString())
	rows, err := adapter.db.QueryContext(ctx, statement, arguments...)
	if err != nil {
		return ResultPage{}, fmt.Errorf("read oracle report result page: %w", err)
	}
	defer rows.Close()

	result := ResultPage{Columns: plan.Columns(), Rows: make([]ResultRow, 0, pageSize)}
	for rows.Next() {
		hidden := 1
		if len(plan.query.Sort) == 1 {
			hidden++
		}
		values := make([]interface{}, len(plan.columns)+hidden)
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
		row := ResultRow{RowID: rowID, Values: values[hidden:]}
		if len(plan.query.Sort) == 1 && values[1] != nil {
			sortValue, valueErr := reportquery.ValueFromDatabase(plan.query.Sort[0].Kind, values[1])
			if valueErr != nil {
				return ResultPage{}, fmt.Errorf("read oracle report result page: normalize sort value: %w", valueErr)
			}
			row.SortValue = &sortValue
		}
		result.Rows = append(result.Rows, row)
		result.NextRowID = rowID
	}
	if err := rows.Err(); err != nil {
		return ResultPage{}, fmt.Errorf("iterate oracle report result rows: %w", err)
	}
	return result, nil
}

type ResultCursor struct {
	RowID     int64
	SortValue *reportquery.Value
}

func buildResultFilters(filters []reportquery.Filter, allowedColumns map[string]struct{}, startBind int) (string, []interface{}, int, error) {
	clauses := make([]string, 0, len(filters))
	arguments := make([]interface{}, 0)
	bind := startBind
	for _, filter := range filters {
		column, err := normalizeIdentifier(filter.Column, "result filter column")
		if err != nil {
			return "", nil, 0, configurationError("result filter column is invalid")
		}
		if _, allowed := allowedColumns[column]; !allowed {
			return "", nil, 0, configurationError("result filter column is outside the published contract")
		}
		operator := strings.ToUpper(filter.Operator)
		appendValues := func() ([]string, error) {
			placeholders := make([]string, len(filter.Values))
			for index, value := range filter.Values {
				bound, err := resultQueryBind(value, filter.OracleType)
				if err != nil {
					return nil, err
				}
				placeholders[index] = fmt.Sprintf(":%d", bind)
				bind++
				arguments = append(arguments, bound)
			}
			return placeholders, nil
		}
		switch operator {
		case "IS_NULL", "IS_NOT_NULL":
			if len(filter.Values) != 0 {
				return "", nil, 0, configurationError("null result filter has values")
			}
			if operator == "IS_NULL" {
				clauses = append(clauses, column+" IS NULL")
			} else {
				clauses = append(clauses, column+" IS NOT NULL")
			}
		case "EQ", "NE", "GT", "GTE", "LT", "LTE":
			if len(filter.Values) != 1 {
				return "", nil, 0, configurationError("scalar result filter has invalid values")
			}
			operators := map[string]string{"EQ": "=", "NE": "<>", "GT": ">", "GTE": ">=", "LT": "<", "LTE": "<="}
			values, err := appendValues()
			if err != nil {
				return "", nil, 0, err
			}
			clauses = append(clauses, fmt.Sprintf("%s %s %s", column, operators[operator], values[0]))
		case "IN", "NOT_IN":
			if len(filter.Values) == 0 || len(filter.Values) > reportquery.MaxSetValues {
				return "", nil, 0, configurationError("set result filter has invalid values")
			}
			values, err := appendValues()
			if err != nil {
				return "", nil, 0, err
			}
			keyword := "IN"
			if operator == "NOT_IN" {
				keyword = "NOT IN"
			}
			clauses = append(clauses, fmt.Sprintf("%s %s (%s)", column, keyword, strings.Join(values, ", ")))
		case "BETWEEN":
			if len(filter.Values) != 2 {
				return "", nil, 0, configurationError("between result filter has invalid values")
			}
			values, err := appendValues()
			if err != nil {
				return "", nil, 0, err
			}
			clauses = append(clauses, fmt.Sprintf("%s BETWEEN %s AND %s", column, values[0], values[1]))
		case "CONTAINS", "STARTS_WITH":
			if len(filter.Values) != 1 || filter.Values[0].Kind != "string" {
				return "", nil, 0, configurationError("like result filter has invalid value")
			}
			value := strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(filter.Values[0].Text)
			if operator == "CONTAINS" {
				value = "%" + value + "%"
			} else {
				value += "%"
			}
			clauses = append(clauses, fmt.Sprintf("%s LIKE :%d ESCAPE '\\'", column, bind))
			bind++
			arguments = append(arguments, value)
		default:
			return "", nil, 0, configurationError("result filter operator is invalid")
		}
	}
	if len(clauses) == 0 {
		return "", arguments, bind, nil
	}
	return " AND " + strings.Join(clauses, " AND "), arguments, bind, nil
}

func resultQueryBind(value reportquery.Value, oracleType string) (interface{}, error) {
	switch value.Kind {
	case "integer", "decimal":
		return godror.Number(value.Text), nil
	case "boolean":
		if value.Text == "true" {
			return godror.Number("1"), nil
		}
		if value.Text == "false" {
			return godror.Number("0"), nil
		}
	case "date":
		parsed, err := time.Parse("2006-01-02", value.Text)
		if err == nil {
			return parsed, nil
		}
	case "datetime":
		parsed, err := time.Parse(time.RFC3339Nano, value.Text)
		if err == nil {
			return parsed, nil
		}
	case "string":
		return value.Text, nil
	}
	return nil, configurationError("result query bind value is invalid")
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
	   AND MAX(CASE WHEN columns.column_position = 1 AND columns.column_name = filters.run_id_column THEN 1 ELSE 0 END) = 1
	   AND MAX(CASE WHEN columns.column_position = 2 AND columns.column_name = filters.row_id_column THEN 1 ELSE 0 END) = 1
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
