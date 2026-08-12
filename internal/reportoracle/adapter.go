package reportoracle

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/godror/godror"
	"github.com/godror/godror/dsn"

	"gin-biz-web-api/internal/reporting"
)

var (
	identifierPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_$#]*$`)
	hostPattern       = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.-]*$`)
	servicePattern    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	overloadPattern   = regexp.MustCompile(`^[0-9]+$`)

	ErrInvalidConfiguration = errors.New("invalid oracle report configuration")
	ErrUnsupportedBinding   = errors.New("unsupported oracle report binding")
	ErrMetadataMismatch     = errors.New("oracle report metadata mismatch")
)

type Config struct {
	Host               string
	Port               int
	ServiceName        string
	SID                string
	Username           string
	Password           string
	Timezone           string
	ConnectTimeout     time.Duration
	MaxOpenConnections int
	MaxIdleConnections int
	ConnectionLifetime time.Duration
	ConnectionIdleTime time.Duration
	PrefetchRows       int
	FetchArraySize     int
}

type ProcedureRef struct {
	Owner    string
	Package  string
	Name     string
	Overload string
}

type ResultTableRef struct {
	Owner string
	Name  string
}

type ProcedureArgument struct {
	Name          string
	Position      int
	Sequence      int
	Direction     string
	DataType      string
	DataLength    *int64
	DataPrecision *int64
	DataScale     *int64
	TypeOwner     string
	TypeName      string
}

type ResultColumn struct {
	Name          string
	Position      int
	DataType      string
	DataLength    int64
	DataPrecision *int64
	DataScale     *int64
	Nullable      bool
}

type Adapter struct {
	db             *sql.DB
	prefetchRows   int
	fetchArraySize int
}

// CallPlan can only be produced by BuildCallPlan, preventing callers from
// supplying arbitrary PL/SQL to Execute.
type CallPlan struct {
	compiled reporting.CompiledCall
	bindings map[string]oracleBindKind
}

type oracleBindKind uint8

const (
	oracleBindScalar oracleBindKind = iota
	oracleBindNumber
	oracleBindCLOB
)

func (plan CallPlan) Statement() string { return plan.compiled.Statement }

func (plan CallPlan) Slots() []reporting.BindSlot {
	return append([]reporting.BindSlot(nil), plan.compiled.Slots...)
}

func Open(ctx context.Context, config Config) (*Adapter, error) {
	params, err := connectionParams(config)
	if err != nil {
		return nil, err
	}
	db := sql.OpenDB(godror.NewConnector(params))
	maxOpenConnections := positiveOrDefault(config.MaxOpenConnections, 20)
	maxIdleConnections := positiveOrDefault(config.MaxIdleConnections, 10)
	if maxIdleConnections > maxOpenConnections {
		maxIdleConnections = maxOpenConnections
	}
	db.SetMaxOpenConns(maxOpenConnections)
	db.SetMaxIdleConns(maxIdleConnections)
	db.SetConnMaxLifetime(durationOrDefault(config.ConnectionLifetime, 30*time.Minute))
	db.SetConnMaxIdleTime(durationOrDefault(config.ConnectionIdleTime, 5*time.Minute))

	pingCtx := ctx
	cancel := func() {}
	if config.ConnectTimeout > 0 {
		pingCtx, cancel = context.WithTimeout(ctx, config.ConnectTimeout)
	}
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping oracle report datasource: %w", err)
	}

	return &Adapter{
		db:             db,
		prefetchRows:   positiveOrDefault(config.PrefetchRows, 1000),
		fetchArraySize: positiveOrDefault(config.FetchArraySize, 1000),
	}, nil
}

// BeginTx starts the transaction used to execute a procedure and to keep its
// result snapshot atomic. Callers remain responsible for Commit or Rollback.
func (adapter *Adapter) BeginTx(ctx context.Context, options *sql.TxOptions) (*sql.Tx, error) {
	if adapter == nil || adapter.db == nil {
		return nil, fmt.Errorf("begin oracle report transaction: adapter is closed")
	}
	tx, err := adapter.db.BeginTx(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("begin oracle report transaction: %w", err)
	}
	return tx, nil
}

func (adapter *Adapter) Close() error {
	if adapter == nil || adapter.db == nil {
		return nil
	}
	return adapter.db.Close()
}

func (adapter *Adapter) InspectProcedure(ctx context.Context, ref ProcedureRef) ([]ProcedureArgument, error) {
	normalized, err := NormalizeProcedureRef(ref)
	if err != nil {
		return nil, err
	}
	rows, err := adapter.db.QueryContext(ctx, procedureArgumentsSQL,
		normalized.Owner, nullableQueryValue(normalized.Package), normalized.Name, nullableQueryValue(normalized.Overload),
		godror.PrefetchCount(adapter.prefetchRows), godror.FetchArraySize(adapter.fetchArraySize),
	)
	if err != nil {
		return nil, fmt.Errorf("inspect oracle procedure arguments: %w", err)
	}
	defer rows.Close()

	arguments := make([]ProcedureArgument, 0, 16)
	for rows.Next() {
		var argument ProcedureArgument
		var name, dataType, inOut, typeOwner, typeName sql.NullString
		var dataLength, precision, scale sql.NullInt64
		if err := rows.Scan(
			&name, &argument.Position, &argument.Sequence, &inOut, &dataType,
			&dataLength, &precision, &scale, &typeOwner, &typeName,
		); err != nil {
			return nil, fmt.Errorf("scan oracle procedure argument: %w", err)
		}
		argument.Name = name.String
		argument.Direction = inOut.String
		argument.DataType = dataType.String
		argument.DataLength = nullableInt64(dataLength)
		argument.DataPrecision = nullableInt64(precision)
		argument.DataScale = nullableInt64(scale)
		argument.TypeOwner = typeOwner.String
		argument.TypeName = typeName.String
		arguments = append(arguments, argument)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate oracle procedure arguments: %w", err)
	}
	if len(arguments) == 0 {
		return nil, fmt.Errorf("%w: no matching procedure", ErrMetadataMismatch)
	}
	return arguments, nil
}

func (adapter *Adapter) InspectResultTable(ctx context.Context, ref ResultTableRef) ([]ResultColumn, error) {
	normalized, err := NormalizeResultTableRef(ref)
	if err != nil {
		return nil, err
	}
	rows, err := adapter.db.QueryContext(ctx, resultColumnsSQL,
		normalized.Owner, normalized.Name,
		godror.PrefetchCount(adapter.prefetchRows), godror.FetchArraySize(adapter.fetchArraySize),
	)
	if err != nil {
		return nil, fmt.Errorf("inspect oracle result table: %w", err)
	}
	defer rows.Close()

	columns := make([]ResultColumn, 0, 32)
	for rows.Next() {
		var column ResultColumn
		var precision, scale sql.NullInt64
		var nullable string
		if err := rows.Scan(
			&column.Name, &column.Position, &column.DataType, &column.DataLength,
			&precision, &scale, &nullable,
		); err != nil {
			return nil, fmt.Errorf("scan oracle result column: %w", err)
		}
		column.DataPrecision = nullableInt64(precision)
		column.DataScale = nullableInt64(scale)
		column.Nullable = nullable == "Y"
		columns = append(columns, column)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate oracle result columns: %w", err)
	}
	if len(columns) == 0 {
		return nil, fmt.Errorf("%w: no matching result table", ErrMetadataMismatch)
	}
	return columns, nil
}

// BuildCanonicalCall creates the only PL/SQL block accepted by the execution
// adapter. The configured {{code}} template is retained in MySQL for review,
// while the published runtime statement is generated from validated objects.
func BuildCallPlan(ref ProcedureRef, definitions []reporting.ParameterDefinition) (CallPlan, error) {
	normalized, err := NormalizeProcedureRef(ref)
	if err != nil {
		return CallPlan{}, err
	}
	ordered := append([]reporting.ParameterDefinition(nil), definitions...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Position < ordered[j].Position })
	bindings := make([]string, 0, len(ordered))
	bindKinds := make(map[string]oracleBindKind, len(ordered))
	for _, definition := range ordered {
		if strings.ToUpper(strings.TrimSpace(definition.Direction)) != "IN" {
			return CallPlan{}, fmt.Errorf("%w: parameter %q must be an IN parameter", ErrUnsupportedBinding, definition.Code)
		}
		argName, err := normalizeIdentifier(definition.ProcedureArgName, "procedure argument")
		if err != nil {
			return CallPlan{}, err
		}
		bindKind, err := compileBindKind(definition)
		if err != nil {
			return CallPlan{}, err
		}
		bindings = append(bindings, fmt.Sprintf("%s => {{%s}}", argName, definition.Code))
		bindKinds[definition.Code] = bindKind
	}
	target := normalized.Owner + "."
	if normalized.Package != "" {
		target += normalized.Package + "."
	}
	target += normalized.Name
	if len(ordered) == 0 {
		return CallPlan{
			compiled: reporting.CompiledCall{Statement: fmt.Sprintf("BEGIN %s(); END;", target)},
			bindings: bindKinds,
		}, nil
	}
	template := fmt.Sprintf("BEGIN %s(%s); END;", target, strings.Join(bindings, ", "))
	compiled, err := reporting.CompileCallTemplate(template, ordered)
	if err != nil {
		return CallPlan{}, err
	}
	return CallPlan{compiled: compiled, bindings: bindKinds}, nil
}

func (adapter *Adapter) Execute(
	ctx context.Context,
	tx *sql.Tx,
	plan CallPlan,
	values map[string]interface{},
) error {
	if tx == nil {
		return fmt.Errorf("execute oracle report procedure: transaction is required")
	}
	if strings.TrimSpace(plan.compiled.Statement) == "" || plan.bindings == nil {
		return fmt.Errorf("%w: call plan is empty", ErrUnsupportedBinding)
	}
	arguments := make([]interface{}, 0, len(plan.compiled.Slots))
	for _, slot := range plan.compiled.Slots {
		value, ok := values[slot.Code]
		if !ok {
			return fmt.Errorf("%w: parameter %q has no database value", ErrUnsupportedBinding, slot.Code)
		}
		bindKind, ok := plan.bindings[slot.Code]
		if !ok {
			return fmt.Errorf("%w: parameter %q has no compiled oracle type", ErrUnsupportedBinding, slot.Code)
		}
		boundValue, err := bindOracleValue(bindKind, value)
		if err != nil {
			return fmt.Errorf("%w: parameter %q: %v", ErrUnsupportedBinding, slot.Code, err)
		}
		arguments = append(arguments, sql.Named(slot.BindName, boundValue))
	}
	if _, err := tx.ExecContext(ctx, plan.compiled.Statement, arguments...); err != nil {
		return mapExecutionError(ctx, err)
	}
	return nil
}

func mapExecutionError(ctx context.Context, err error) error {
	if contextError := ctx.Err(); contextError != nil {
		return contextError
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	var oracleError interface{ Code() int }
	if errors.As(err, &oracleError) && oracleError.Code() == 1013 {
		return context.Canceled
	}
	return fmt.Errorf("execute oracle report procedure: %w", err)
}

func compileBindKind(definition reporting.ParameterDefinition) (oracleBindKind, error) {
	oracleType := strings.ToUpper(strings.Join(strings.Fields(definition.OracleType), " "))
	if oracleType == "" {
		return 0, fmt.Errorf("%w: parameter %q has no oracle type", ErrUnsupportedBinding, definition.Code)
	}
	if definition.CollectionEncoding == reporting.CollectionEncodingJSONCLOB {
		if oracleType != "CLOB" {
			return 0, fmt.Errorf("%w: parameter %q with JSON_CLOB encoding must bind CLOB", ErrUnsupportedBinding, definition.Code)
		}
		return oracleBindCLOB, nil
	}
	if definition.LogicalType == reporting.LogicalTypeDecimal {
		if oracleType != "NUMBER" {
			return 0, fmt.Errorf("%w: decimal parameter %q must bind NUMBER", ErrUnsupportedBinding, definition.Code)
		}
		return oracleBindNumber, nil
	}
	if oracleType == "CLOB" || oracleType == "NCLOB" {
		return oracleBindCLOB, nil
	}
	return oracleBindScalar, nil
}

func bindOracleValue(kind oracleBindKind, value interface{}) (interface{}, error) {
	if value == nil {
		return nil, nil
	}
	switch kind {
	case oracleBindScalar:
		return value, nil
	case oracleBindNumber:
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("NUMBER value must be a normalized decimal string")
		}
		return godror.Number(text), nil
	case oracleBindCLOB:
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("CLOB value must be a string")
		}
		return godror.Lob{Reader: strings.NewReader(text), IsClob: true}, nil
	default:
		return nil, fmt.Errorf("oracle bind kind is invalid")
	}
}

func NormalizeProcedureRef(ref ProcedureRef) (ProcedureRef, error) {
	owner, err := normalizeIdentifier(ref.Owner, "procedure owner")
	if err != nil {
		return ProcedureRef{}, err
	}
	name, err := normalizeIdentifier(ref.Name, "procedure name")
	if err != nil {
		return ProcedureRef{}, err
	}
	packageName := ""
	if strings.TrimSpace(ref.Package) != "" {
		packageName, err = normalizeIdentifier(ref.Package, "package name")
		if err != nil {
			return ProcedureRef{}, err
		}
	}
	overload := strings.TrimSpace(ref.Overload)
	if overload != "" && !overloadPattern.MatchString(overload) {
		return ProcedureRef{}, configurationError("procedure overload is invalid")
	}
	return ProcedureRef{Owner: owner, Package: packageName, Name: name, Overload: overload}, nil
}

func NormalizeResultTableRef(ref ResultTableRef) (ResultTableRef, error) {
	owner, err := normalizeIdentifier(ref.Owner, "result table owner")
	if err != nil {
		return ResultTableRef{}, err
	}
	name, err := normalizeIdentifier(ref.Name, "result table name")
	if err != nil {
		return ResultTableRef{}, err
	}
	return ResultTableRef{Owner: owner, Name: name}, nil
}

func connectionParams(config Config) (dsn.ConnectionParams, error) {
	connectString, err := BuildConnectString(config)
	if err != nil {
		return dsn.ConnectionParams{}, err
	}
	username := strings.TrimSpace(config.Username)
	if username == "" || strings.TrimSpace(config.Password) == "" {
		return dsn.ConnectionParams{}, configurationError("username and password are required")
	}
	timezoneName := strings.TrimSpace(config.Timezone)
	if timezoneName == "" {
		timezoneName = "Asia/Shanghai"
	}
	location, err := time.LoadLocation(timezoneName)
	if err != nil {
		return dsn.ConnectionParams{}, configurationError("timezone is invalid")
	}
	return dsn.ConnectionParams{
		CommonParams: dsn.CommonParams{CommonSimpleParams: dsn.CommonSimpleParams{
			Username:           username,
			Password:           dsn.NewPassword(config.Password),
			ConnectString:      connectString,
			Timezone:           location,
			PerSessionTimezone: true,
		}},
		StandaloneConnection: godror.Bool(false),
	}, nil
}

func BuildConnectString(config Config) (string, error) {
	host := strings.TrimSpace(config.Host)
	if !hostPattern.MatchString(host) {
		return "", configurationError("host is invalid")
	}
	if config.Port < 1 || config.Port > 65535 {
		return "", configurationError("port is invalid")
	}
	serviceName := strings.TrimSpace(config.ServiceName)
	sid := strings.TrimSpace(config.SID)
	if (serviceName == "") == (sid == "") {
		return "", configurationError("exactly one of service name or SID is required")
	}
	if serviceName != "" {
		if !servicePattern.MatchString(serviceName) {
			return "", configurationError("service name is invalid")
		}
		return fmt.Sprintf("%s:%d/%s", host, config.Port, serviceName), nil
	}
	if !identifierPattern.MatchString(sid) {
		return "", configurationError("SID is invalid")
	}
	return fmt.Sprintf(
		"(DESCRIPTION=(ADDRESS=(PROTOCOL=TCP)(HOST=%s)(PORT=%d))(CONNECT_DATA=(SID=%s)))",
		host, config.Port, strings.ToUpper(sid),
	), nil
}

func normalizeIdentifier(value, label string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) > 128 || !identifierPattern.MatchString(trimmed) {
		return "", configurationError(label + " is invalid")
	}
	return strings.ToUpper(trimmed), nil
}

func nullableInt64(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	result := value.Int64
	return &result
}

func nullableQueryValue(value string) interface{} {
	if value == "" {
		return nil
	}
	return value
}

func positiveOrDefault(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func durationOrDefault(value, fallback time.Duration) time.Duration {
	if value > 0 {
		return value
	}
	return fallback
}

func configurationError(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalidConfiguration, message)
}

const procedureArgumentsSQL = `
WITH filters AS (
    SELECT :1 AS owner_name, :2 AS package_name, :3 AS object_name, :4 AS overload
    FROM dual
)
SELECT arguments.argument_name, arguments.position, arguments.sequence,
       arguments.in_out, arguments.data_type, arguments.data_length,
       arguments.data_precision, arguments.data_scale,
       arguments.type_owner, arguments.type_name
FROM all_arguments arguments
CROSS JOIN filters
WHERE arguments.owner = filters.owner_name
  AND ((filters.package_name IS NULL AND arguments.package_name IS NULL) OR arguments.package_name = filters.package_name)
  AND arguments.object_name = filters.object_name
  AND ((filters.overload IS NULL AND arguments.overload IS NULL) OR arguments.overload = filters.overload)
  AND arguments.data_level = 0
ORDER BY arguments.sequence`

const resultColumnsSQL = `
SELECT column_name, column_id, data_type, data_length,
       data_precision, data_scale, nullable
FROM all_tab_columns
WHERE owner = :1 AND table_name = :2
ORDER BY column_id`
