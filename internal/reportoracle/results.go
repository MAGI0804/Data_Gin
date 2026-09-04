package reportoracle

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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
	Table   ResultTableRef
	Columns []string
}

// ResultTableSnapshotRef builds a result-table contract without imposing
// system columns on the Oracle schema.
func ResultTableSnapshotRef(table ResultTableRef, columns []string) ResultSnapshotRef {
	return ResultSnapshotRef{Table: table, Columns: columns}
}

// ResultSnapshotContract is produced after Oracle metadata proves that the
// configured output columns exist in the bound result table.
type ResultSnapshotContract struct {
	table   ResultTableRef
	columns map[string]struct{}
}

func ValidateResultSnapshotContract(contract ResultSnapshotContract, ref ResultSnapshotRef) error {
	table, columns, err := normalizeSnapshotRef(ref)
	if err != nil {
		return err
	}
	if contract.table != table || len(contract.columns) != len(columns) {
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
	Key       string
	Values    []interface{}
	SortValue *reportquery.Value
}

type ResultPage struct {
	Columns []string
	Rows    []ResultRow
	NextKey string
	HasNext bool
}

type PurgePlan struct {
	statement string
}

type ResultCountPlan struct {
	statement string
}

const resultTableStorageSQL = `
SELECT temporary, iot_type, row_movement
FROM all_tables
WHERE owner = :1 AND table_name = :2`

func (adapter *Adapter) InspectResultSnapshotContract(
	ctx context.Context,
	ref ResultSnapshotRef,
) (ResultSnapshotContract, error) {
	normalizedTable, _, err := normalizeSnapshotRef(ref)
	if err != nil {
		return ResultSnapshotContract{}, err
	}
	columns, err := adapter.InspectResultTable(ctx, normalizedTable)
	if err != nil {
		return ResultSnapshotContract{}, err
	}
	return CompileResultSnapshotContract(ref, columns)
}

// ValidateResultSnapshotTable proves that the bound object is a permanent
// heap table whose physical ROWID can be used for stable keyset pagination.
func (adapter *Adapter) ValidateResultSnapshotTable(ctx context.Context, ref ResultTableRef) error {
	if adapter == nil || adapter.db == nil {
		return fmt.Errorf("validate oracle result snapshot table: adapter is closed")
	}
	normalized, err := NormalizeResultTableRef(ref)
	if err != nil {
		return err
	}
	var temporary, rowMovement string
	var iotType sql.NullString
	err = adapter.db.QueryRowContext(ctx, resultTableStorageSQL, normalized.Owner, normalized.Name).
		Scan(&temporary, &iotType, &rowMovement)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: no matching result table", ErrMetadataMismatch)
	}
	if err != nil {
		return fmt.Errorf("inspect oracle result table storage: %w", err)
	}
	if err := validateResultTableStorage(temporary, iotType.String, rowMovement); err != nil {
		return err
	}
	probe := resultTableROWIDProbe(normalized)
	rows, err := adapter.db.QueryContext(ctx, probe)
	if err != nil {
		return fmt.Errorf("validate oracle result table ROWID: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		var rowID string
		if err := rows.Scan(&rowID); err != nil {
			return fmt.Errorf("scan oracle result table ROWID: %w", err)
		}
		if strings.TrimSpace(rowID) == "" {
			return fmt.Errorf("%w: result table returned an empty ROWID", ErrMetadataMismatch)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate oracle result table ROWID probe: %w", err)
	}
	return nil
}

func resultTableROWIDProbe(table ResultTableRef) string {
	return fmt.Sprintf("SELECT ROWIDTOCHAR(CHARTOROWID(ROWIDTOCHAR(ROWID))) FROM %s.%s WHERE ROWNUM <= 1", table.Owner, table.Name)
}

func validateResultTableStorage(temporary, iotType, rowMovement string) error {
	if strings.ToUpper(strings.TrimSpace(temporary)) != "N" {
		return fmt.Errorf("%w: %w", ErrMetadataMismatch, ErrTemporaryResultTable)
	}
	if strings.TrimSpace(iotType) != "" {
		return fmt.Errorf("%w: index-organized result tables do not expose stable physical ROWID", ErrMetadataMismatch)
	}
	if strings.ToUpper(strings.TrimSpace(rowMovement)) != "DISABLED" {
		return fmt.Errorf("%w: result table row movement must be disabled", ErrMetadataMismatch)
	}
	return nil
}

func CompileResultSnapshotContract(
	ref ResultSnapshotRef,
	columns []ResultColumn,
) (ResultSnapshotContract, error) {
	normalizedTable, configuredColumns, err := normalizeSnapshotRef(ref)
	if err != nil {
		return ResultSnapshotContract{}, err
	}
	byName := make(map[string]ResultColumn, len(columns))
	for _, column := range columns {
		byName[strings.ToUpper(column.Name)] = column
	}
	allowedColumns := make(map[string]struct{}, len(configuredColumns))
	for _, column := range configuredColumns {
		if _, exists := byName[column]; !exists {
			return ResultSnapshotContract{}, configurationError("configured result column does not exist")
		}
		allowedColumns[column] = struct{}{}
	}
	return ResultSnapshotContract{table: normalizedTable, columns: allowedColumns}, nil
}

func BuildResultPagePlan(contract ResultSnapshotContract, columns []string) (ResultPagePlan, error) {
	return BuildResultQueryPlan(contract, columns, reportquery.Query{})
}

func BuildResultQueryPlan(contract ResultSnapshotContract, columns []string, query reportquery.Query) (ResultPagePlan, error) {
	if contract.table.Owner == "" || contract.table.Name == "" || len(contract.columns) == 0 {
		return ResultPagePlan{}, configurationError("result snapshot contract is not validated")
	}
	_, normalizedColumns, err := normalizeSnapshotRef(ResultSnapshotRef{Table: contract.table, Columns: columns})
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
	selectColumns = append(selectColumns, "ROWIDTOCHAR(ROWID)")
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
	filterSQL, filterArguments, nextBind, err := buildResultFilters(query.Filters, contract.columns, 1)
	if err != nil {
		return ResultPagePlan{}, err
	}
	where := "1 = 1" + filterSQL
	order := "ROWID ASC"
	initialArguments := append([]interface{}(nil), filterArguments...)
	nextArguments := append([]interface{}(nil), filterArguments...)
	initialFetchBind := nextBind
	nextPredicate := ""
	if len(query.Sort) == 1 {
		sort := query.Sort[0]
		order = fmt.Sprintf("%s %s NULLS LAST, ROWID ASC", sort.Column, sort.Direction)
		comparison := ">"
		if sort.Direction == "DESC" {
			comparison = "<"
		}
		nextPredicate = fmt.Sprintf("((:%d = 1 AND %s IS NULL AND ROWID > CHARTOROWID(:%d)) OR (:%d = 0 AND ((%s %s :%d) OR %s IS NULL OR (%s = :%d AND ROWID > CHARTOROWID(:%d)))))",
			nextBind, sort.Column, nextBind+1, nextBind+2, sort.Column, comparison, nextBind+3, sort.Column, sort.Column, nextBind+4, nextBind+5)
		nextBind += 6
	} else {
		nextPredicate = fmt.Sprintf("ROWID > CHARTOROWID(:%d)", nextBind)
		nextBind++
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
	if contract.table.Owner == "" || contract.table.Name == "" {
		return PurgePlan{}, configurationError("result snapshot contract is not validated")
	}
	return buildTablePurgePlan(contract.table)
}

// BuildTablePurgePlan builds the bounded purge used when recovering an old
// table snapshot. Cleanup only depends on the published table identity: its
// historical presentation columns may legitimately differ from the current
// Oracle table after a newer report version is published.
func BuildTablePurgePlan(table ResultTableRef) (PurgePlan, error) {
	normalized, err := NormalizeResultTableRef(table)
	if err != nil {
		return PurgePlan{}, err
	}
	return buildTablePurgePlan(normalized)
}

func buildTablePurgePlan(table ResultTableRef) (PurgePlan, error) {
	statement := fmt.Sprintf(
		"DELETE FROM %s.%s WHERE ROWID IN (SELECT ROWID FROM %s.%s WHERE ROWNUM <= :1)",
		table.Owner, table.Name, table.Owner, table.Name,
	)
	return PurgePlan{statement: statement}, nil
}

func BuildResultCountPlan(contract ResultSnapshotContract) (ResultCountPlan, error) {
	if contract.table.Owner == "" || contract.table.Name == "" {
		return ResultCountPlan{}, configurationError("result snapshot contract is not validated")
	}
	return ResultCountPlan{statement: fmt.Sprintf(
		"SELECT COUNT(*) FROM %s.%s",
		contract.table.Owner, contract.table.Name,
	)}, nil
}

func (adapter *Adapter) CountResultRows(ctx context.Context, plan ResultCountPlan) (int64, error) {
	if adapter == nil || adapter.db == nil || strings.TrimSpace(plan.statement) == "" {
		return 0, fmt.Errorf("count oracle report result rows: invalid request")
	}
	var count int64
	if err := adapter.db.QueryRowContext(ctx, plan.statement).Scan(&count); err != nil {
		return 0, fmt.Errorf("count oracle report result rows: %w", err)
	}
	if count < 0 {
		return 0, fmt.Errorf("count oracle report result rows: invalid count")
	}
	return count, nil
}

func (adapter *Adapter) CountResultRowsTx(ctx context.Context, tx *sql.Tx, plan ResultCountPlan) (int64, error) {
	if adapter == nil || tx == nil || strings.TrimSpace(plan.statement) == "" {
		return 0, fmt.Errorf("count oracle report result rows: invalid transaction request")
	}
	var count int64
	if err := tx.QueryRowContext(ctx, plan.statement).Scan(&count); err != nil {
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
	after *ResultCursor,
	pageSize int,
) (ResultPage, error) {
	if adapter == nil || adapter.db == nil || strings.TrimSpace(plan.initialStatement) == "" ||
		strings.TrimSpace(plan.nextStatement) == "" || len(plan.columns) == 0 {
		return ResultPage{}, fmt.Errorf("read oracle report result page: invalid request")
	}
	if pageSize == 0 {
		pageSize = defaultResultPageSize
	}
	if pageSize < 1 || pageSize > maxResultPageSize {
		return ResultPage{}, fmt.Errorf("read oracle report result page: invalid page size")
	}
	statement := plan.initialStatement
	arguments := append([]interface{}(nil), plan.initialArguments...)
	if after != nil {
		statement = plan.nextStatement
		arguments = append([]interface{}(nil), plan.nextArguments...)
		if len(plan.query.Sort) == 0 {
			arguments = append(arguments, after.Key)
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
			arguments = append(arguments, isNull, after.Key, isNull, value, value, after.Key)
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
		rowKey, err := oracleRowKey(values[0])
		if err != nil {
			return ResultPage{}, err
		}
		if len(result.Rows) == pageSize {
			result.HasNext = true
			break
		}
		row := ResultRow{Key: rowKey, Values: values[hidden:]}
		if len(plan.query.Sort) == 1 && values[1] != nil {
			sortValue, valueErr := reportquery.ValueFromDatabase(plan.query.Sort[0].Kind, values[1])
			if valueErr != nil {
				return ResultPage{}, fmt.Errorf("read oracle report result page: normalize sort value: %w", valueErr)
			}
			row.SortValue = &sortValue
		}
		result.Rows = append(result.Rows, row)
		result.NextKey = rowKey
	}
	if err := rows.Err(); err != nil {
		return ResultPage{}, fmt.Errorf("iterate oracle report result rows: %w", err)
	}
	return result, nil
}

type ResultCursor struct {
	Key       string
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
	batchSize int,
) (int64, error) {
	if adapter == nil || tx == nil || strings.TrimSpace(plan.statement) == "" {
		return 0, fmt.Errorf("purge oracle report result: invalid request")
	}
	if batchSize == 0 {
		batchSize = defaultPurgeBatchSize
	}
	if batchSize < 1 || batchSize > maxPurgeBatchSize {
		return 0, fmt.Errorf("purge oracle report result: invalid batch size")
	}
	result, err := tx.ExecContext(ctx, plan.statement, batchSize)
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

func normalizeSnapshotRef(ref ResultSnapshotRef) (ResultTableRef, []string, error) {
	table, err := NormalizeResultTableRef(ref.Table)
	if err != nil {
		return ResultTableRef{}, nil, err
	}
	if len(ref.Columns) == 0 || len(ref.Columns) > maxResultColumns {
		return ResultTableRef{}, nil, configurationError("result columns are invalid")
	}
	columns := make([]string, len(ref.Columns))
	seen := make(map[string]struct{}, len(ref.Columns))
	for index, column := range ref.Columns {
		columns[index], err = normalizeIdentifier(column, "result column")
		if err != nil {
			return ResultTableRef{}, nil, err
		}
		if _, exists := seen[columns[index]]; exists {
			return ResultTableRef{}, nil, configurationError("result columns contain duplicate column")
		}
		seen[columns[index]] = struct{}{}
	}
	return table, columns, nil
}
func oracleRowKey(value interface{}) (string, error) {
	switch typed := value.(type) {
	case string:
		if strings.TrimSpace(typed) == "" {
			return "", fmt.Errorf("scan oracle report result row: empty row key")
		}
		return typed, nil
	case []byte:
		return oracleRowKey(string(typed))
	default:
		return "", fmt.Errorf("scan oracle report result row: unsupported row key type %T", value)
	}
}
