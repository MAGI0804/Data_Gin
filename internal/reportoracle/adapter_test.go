package reportoracle

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/godror/godror"

	"gin-biz-web-api/internal/reporting"
)

func TestBuildConnectString(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		want   string
	}{
		{
			name:   "service name",
			config: Config{Host: "oracle.internal", Port: 1521, ServiceName: "REPORT_PDB"},
			want:   "oracle.internal:1521/REPORT_PDB",
		},
		{
			name:   "SID",
			config: Config{Host: "10.0.0.8", Port: 1521, SID: "orcl"},
			want:   "(DESCRIPTION=(ADDRESS=(PROTOCOL=TCP)(HOST=10.0.0.8)(PORT=1521))(CONNECT_DATA=(SID=ORCL)))",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := BuildConnectString(test.config)
			if err != nil {
				t.Fatalf("BuildConnectString() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("BuildConnectString() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestProcedureInspectionUsesEachBindOnce(t *testing.T) {
	for _, bind := range []string{":1", ":2", ":3", ":4"} {
		if count := strings.Count(procedureArgumentsSQL, bind); count != 1 {
			t.Fatalf("bind %s occurs %d times, want once", bind, count)
		}
	}
}

func TestProcedureCatalogUsesBoundFiltersAndVisibleMetadata(t *testing.T) {
	for _, bind := range []string{":1", ":2", ":3", ":4"} {
		if count := strings.Count(procedureCatalogSQL, bind); count != 1 {
			t.Fatalf("bind %s occurs %d times, want once", bind, count)
		}
	}
	for _, fragment := range []string{"all_procedures", "all_arguments", "all_objects", "objects.status = 'VALID'", "INSTR("} {
		if !strings.Contains(procedureCatalogSQL, fragment) {
			t.Fatalf("procedure catalog SQL is missing %q", fragment)
		}
	}
}

func TestProcedureCursorRoundTrip(t *testing.T) {
	ref := ProcedureRef{Owner: "report_owner", Package: "report_pkg", Name: "run_report", Overload: "2"}
	key, err := ProcedureCursorKey(ref)
	if err != nil {
		t.Fatalf("ProcedureCursorKey() error = %v", err)
	}
	parsed, err := ParseProcedureCursorKey(key)
	if err != nil {
		t.Fatalf("ParseProcedureCursorKey() error = %v", err)
	}
	if parsed.Owner != "REPORT_OWNER" || parsed.Package != "REPORT_PKG" || parsed.Name != "RUN_REPORT" || parsed.Overload != "2" {
		t.Fatalf("parsed cursor = %+v", parsed)
	}
	if _, err := ParseProcedureCursorKey("REPORT_OWNER.RUN_REPORT"); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("invalid cursor error = %v", err)
	}
}

func TestResultTableCatalogUsesBoundFiltersAndVisibleTables(t *testing.T) {
	for _, bind := range []string{":1", ":2", ":3", ":4"} {
		if count := strings.Count(resultTableCatalogSQL, bind); count != 1 {
			t.Fatalf("bind %s occurs %d times, want once", bind, count)
		}
	}
	for _, fragment := range []string{"all_tables", "all_tab_columns", "INSTR(", "REGEXP_LIKE", "CHR(31)"} {
		if !strings.Contains(resultTableCatalogSQL, fragment) {
			t.Fatalf("result table catalog SQL is missing %q", fragment)
		}
	}
	if strings.Contains(resultTableCatalogSQL, "all_views") {
		t.Fatal("result table catalog must not include Oracle views")
	}
}

func TestDatabaseIdentityQueriesPreferDBIDAndRetainLeastPrivilegeFallback(t *testing.T) {
	for _, fragment := range []string{"v$database", "dbid", "db_unique_name", "CON_ID", "CON_NAME"} {
		if !strings.Contains(databaseIdentitySQL, fragment) {
			t.Fatalf("database identity SQL is missing %q", fragment)
		}
	}
	if strings.Contains(databaseIdentitySQL, "CON_UID") || strings.Contains(databaseIdentityFallbackSQL, "CON_UID") {
		t.Fatal("Oracle 19c database identity queries must not request unsupported USERENV CON_UID")
	}
	if !strings.Contains(databaseIdentityFallbackSQL, "DB_UNIQUE_NAME") || !strings.Contains(databaseIdentityFallbackSQL, "FROM dual") {
		t.Fatalf("database identity fallback is incomplete: %s", databaseIdentityFallbackSQL)
	}
}

func TestResultTableCursorRoundTrip(t *testing.T) {
	ref := ResultTableRef{Owner: "report_owner", Name: "daily_result_rows"}
	key, err := ResultTableCursorKey(ref)
	if err != nil {
		t.Fatalf("ResultTableCursorKey() error = %v", err)
	}
	parsed, err := ParseResultTableCursorKey(key)
	if err != nil {
		t.Fatalf("ParseResultTableCursorKey() error = %v", err)
	}
	if parsed.Owner != "REPORT_OWNER" || parsed.Name != "DAILY_RESULT_ROWS" {
		t.Fatalf("parsed cursor = %+v", parsed)
	}
	if _, err := ParseResultTableCursorKey("REPORT_OWNER.DAILY_RESULT_ROWS"); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("invalid cursor error = %v", err)
	}
}

func TestBuildConnectStringRejectsUnsafeConfiguration(t *testing.T) {
	tests := []Config{
		{Host: "db)(PORT=9999", Port: 1521, ServiceName: "REPORT"},
		{Host: "db.internal", Port: 0, ServiceName: "REPORT"},
		{Host: "db.internal", Port: 1521},
		{Host: "db.internal", Port: 1521, ServiceName: "REPORT", SID: "ORCL"},
		{Host: "db.internal", Port: 1521, ServiceName: "REPORT/ADMIN"},
	}
	for index, config := range tests {
		if _, err := BuildConnectString(config); !errors.Is(err, ErrInvalidConfiguration) {
			t.Fatalf("case %d error = %v, want ErrInvalidConfiguration", index, err)
		}
	}
}

func TestBuildCallPlan(t *testing.T) {
	call, err := BuildCallPlan(ProcedureRef{
		Owner: "report_owner", Package: "report_pkg", Name: "order_report",
	}, []reporting.ParameterDefinition{
		{Code: "startTime", ProcedureArgName: "P_START_TIME", Position: 2, Direction: "IN", LogicalType: reporting.LogicalTypeDateTime, OracleType: "TIMESTAMP WITH TIME ZONE"},
		{Code: "runId", ProcedureArgName: "P_RUN_ID", Position: 1, Direction: "IN", LogicalType: reporting.LogicalTypeString, OracleType: "VARCHAR2"},
	})
	if err != nil {
		t.Fatalf("BuildCallPlan() error = %v", err)
	}
	want := "BEGIN REPORT_OWNER.REPORT_PKG.ORDER_REPORT(P_RUN_ID => :p1, P_START_TIME => :p2); END;"
	if call.Statement() != want {
		t.Fatalf("Statement = %q, want %q", call.Statement(), want)
	}
	slots := call.Slots()
	if len(slots) != 2 || slots[0].Code != "runId" || slots[1].Code != "startTime" {
		t.Fatalf("Slots = %#v", slots)
	}
}

func TestBuildCallPlanRejectsIdentifierInjection(t *testing.T) {
	_, err := BuildCallPlan(ProcedureRef{
		Owner: "REPORT; DROP TABLE USERS", Name: "RUN",
	}, []reporting.ParameterDefinition{
		{Code: "runId", ProcedureArgName: "P_RUN_ID", Position: 1, Direction: "IN", LogicalType: reporting.LogicalTypeString, OracleType: "VARCHAR2"},
	})
	if !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("error = %v, want ErrInvalidConfiguration", err)
	}
}

func TestBuildCallPlanRejectsOutputParameter(t *testing.T) {
	_, err := BuildCallPlan(ProcedureRef{Owner: "REPORT", Name: "RUN"}, []reporting.ParameterDefinition{
		{Code: "cursor", ProcedureArgName: "P_CURSOR", Position: 1, Direction: "OUT", LogicalType: reporting.LogicalTypeString, OracleType: "VARCHAR2"},
	})
	if !errors.Is(err, ErrUnsupportedBinding) {
		t.Fatalf("error = %v, want ErrUnsupportedBinding", err)
	}
}

func TestBuildCallPlanRejectsUnbindableLogicalOracleTypes(t *testing.T) {
	tests := []reporting.ParameterDefinition{
		{Code: "amount", ProcedureArgName: "P_AMOUNT", Position: 1, Direction: "IN", LogicalType: reporting.LogicalTypeDecimal, OracleType: "BINARY_DOUBLE"},
		{Code: "enabled", ProcedureArgName: "P_ENABLED", Position: 1, Direction: "IN", LogicalType: reporting.LogicalTypeBoolean, OracleType: "NUMBER"},
		{Code: "count", ProcedureArgName: "P_COUNT", Position: 1, Direction: "IN", LogicalType: reporting.LogicalTypeInteger, OracleType: "VARCHAR2"},
	}
	for _, definition := range tests {
		if _, err := BuildCallPlan(ProcedureRef{Owner: "REPORT", Name: "RUN"}, []reporting.ParameterDefinition{definition}); !errors.Is(err, ErrUnsupportedBinding) {
			t.Fatalf("BuildCallPlan(%s) error = %v", definition.Code, err)
		}
	}
}

func TestBuildCallPlanSupportsProcedureWithoutArguments(t *testing.T) {
	call, err := BuildCallPlan(ProcedureRef{Owner: "REPORT", Name: "REFRESH_ALL"}, nil)
	if err != nil {
		t.Fatalf("BuildCallPlan() error = %v", err)
	}
	if got, want := call.Statement(), "BEGIN REPORT.REFRESH_ALL(); END;"; got != want {
		t.Fatalf("Statement = %q, want %q", got, want)
	}
}

func TestBuildCallPlanCompilesOracleBindingTypes(t *testing.T) {
	call, err := BuildCallPlan(ProcedureRef{Owner: "REPORT", Name: "RUN"}, []reporting.ParameterDefinition{
		{Code: "amount", ProcedureArgName: "P_AMOUNT", Position: 1, Direction: "IN", LogicalType: reporting.LogicalTypeDecimal, OracleType: "number"},
		{Code: "orgCodes", ProcedureArgName: "P_ORG_CODES", Position: 2, Direction: "IN", LogicalType: reporting.LogicalTypeMultiEnum, OracleType: "CLOB", Cardinality: reporting.CardinalityMultiple, CollectionEncoding: reporting.CollectionEncodingJSONCLOB},
	})
	if err != nil {
		t.Fatalf("BuildCallPlan() error = %v", err)
	}
	if call.bindings["amount"] != oracleBindNumber || call.bindings["orgCodes"] != oracleBindCLOB {
		t.Fatalf("bindings = %#v", call.bindings)
	}

	number, err := bindOracleValue(call.bindings["amount"], "1234567890.0123456789")
	if err != nil {
		t.Fatalf("bindOracleValue(number) error = %v", err)
	}
	if got, ok := number.(godror.Number); !ok || got != godror.Number("1234567890.0123456789") {
		t.Fatalf("number = %#v", number)
	}

	clob, err := bindOracleValue(call.bindings["orgCodes"], `["NORTH","SOUTH"]`)
	if err != nil {
		t.Fatalf("bindOracleValue(clob) error = %v", err)
	}
	lob, ok := clob.(godror.Lob)
	if !ok || !lob.IsClob {
		t.Fatalf("clob = %#v", clob)
	}
}

func TestBuildCallPlanRejectsIncompatibleOracleBindings(t *testing.T) {
	tests := []reporting.ParameterDefinition{
		{Code: "amount", ProcedureArgName: "P_AMOUNT", Position: 1, Direction: "IN", LogicalType: reporting.LogicalTypeDecimal, OracleType: "VARCHAR2"},
		{Code: "orgCodes", ProcedureArgName: "P_ORG_CODES", Position: 1, Direction: "IN", LogicalType: reporting.LogicalTypeMultiEnum, OracleType: "VARCHAR2", Cardinality: reporting.CardinalityMultiple, CollectionEncoding: reporting.CollectionEncodingJSONCLOB},
	}
	for _, definition := range tests {
		_, err := BuildCallPlan(ProcedureRef{Owner: "REPORT", Name: "RUN"}, []reporting.ParameterDefinition{definition})
		if !errors.Is(err, ErrUnsupportedBinding) {
			t.Fatalf("definition %q error = %v, want ErrUnsupportedBinding", definition.Code, err)
		}
	}
}

func TestExecutionCancellationPreservesDeadline(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	if err := mapExecutionError(ctx, oracleTestError{code: 1013}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("mapExecutionError() = %v, want DeadlineExceeded", err)
	}
}

func TestExecutionCancellationMapsOracleCancel(t *testing.T) {
	if err := mapExecutionError(context.Background(), oracleTestError{code: 1013}); !errors.Is(err, context.Canceled) {
		t.Fatalf("mapExecutionError() = %v, want Canceled", err)
	}
}

type oracleTestError struct {
	code int
}

func (err oracleTestError) Error() string { return "oracle test error" }
func (err oracleTestError) Code() int     { return err.code }
