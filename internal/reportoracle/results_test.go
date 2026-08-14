package reportoracle

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"

	"gin-biz-web-api/internal/reportquery"

	"github.com/godror/godror"
)

func TestBuildResultQueryPlanUsesBoundFiltersAndStableSort(t *testing.T) {
	contract := testSnapshotContract()
	query := reportquery.Query{
		Filters: []reportquery.Filter{{Field: "store-id", Column: "STORE_CODE", OracleType: "VARCHAR2", Operator: "EQ", Values: []reportquery.Value{{Kind: "string", Text: "S001' OR 1=1 --"}}}},
		Sort:    []reportquery.Sort{{Field: "amount-id", Column: "AMOUNT", Direction: "DESC", Kind: "decimal"}},
	}
	plan, err := BuildResultQueryPlan(contract, []string{"STORE_CODE"}, query)
	if err != nil {
		t.Fatalf("BuildResultQueryPlan() error = %v", err)
	}
	for _, required := range []string{"STORE_CODE = :2", "ORDER BY AMOUNT DESC NULLS LAST, ID ASC", "FETCH NEXT :3 ROWS ONLY", "FETCH NEXT :10 ROWS ONLY"} {
		if !strings.Contains(plan.initialStatement+plan.nextStatement, required) {
			t.Fatalf("plan does not contain %q: %s / %s", required, plan.initialStatement, plan.nextStatement)
		}
	}
	if strings.Contains(plan.initialStatement, "OR 1=1") {
		t.Fatal("filter value was interpolated into SQL")
	}
	if len(plan.initialArguments) != 1 || plan.initialArguments[0] != "S001' OR 1=1 --" {
		t.Fatalf("initial arguments = %#v", plan.initialArguments)
	}
}

func TestBuildResultQueryPlanRejectsContractExternalColumn(t *testing.T) {
	query := reportquery.Query{Filters: []reportquery.Filter{{Field: "secret-id", Column: "SECRET_VALUE", OracleType: "VARCHAR2", Operator: "EQ", Values: []reportquery.Value{{Kind: "string", Text: "x"}}}}}
	if _, err := BuildResultQueryPlan(testSnapshotContract(), []string{"STORE_CODE"}, query); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("error = %v", err)
	}
}

func TestCompiledQueryRoundTrip(t *testing.T) {
	columns := []reportquery.Column{{FieldID: "store-id", LogicalCode: "store", DatabaseColumn: "STORE_CODE", ValueType: "string", SourceOracleType: "VARCHAR2", Filterable: true, AllowedOperators: []string{"EQ"}}}
	query, err := reportquery.Normalize(reportquery.Input{Filters: []reportquery.FilterInput{{Field: "store-id", Operator: "EQ", Value: json.RawMessage(`"S001"`)}}}, columns)
	if err != nil || reportquery.ValidateCompiled(query, columns) != nil {
		t.Fatalf("query=%#v err=%v", query, err)
	}
}

func TestBuildResultPagePlan(t *testing.T) {
	contract := testSnapshotContract()
	plan, err := BuildResultPagePlan(contract, []string{"store_code", "amount"})
	if err != nil {
		t.Fatalf("BuildResultPagePlan() error = %v", err)
	}
	want := "SELECT ID, STORE_CODE, AMOUNT FROM REPORT_OWNER.SALES_RESULT WHERE RUN_ID = :1 AND ID > :2 ORDER BY ID ASC FETCH NEXT :3 ROWS ONLY"
	if plan.nextStatement != want {
		t.Fatalf("statement = %q, want %q", plan.nextStatement, want)
	}
	if strings.Contains(plan.initialStatement, "ID >") {
		t.Fatalf("initial statement unexpectedly assumes a minimum row id: %q", plan.initialStatement)
	}
	columns := plan.Columns()
	columns[0] = "MUTATED"
	if plan.columns[0] != "STORE_CODE" {
		t.Fatal("Columns returned shared storage")
	}
}

func TestBuildPurgePlanIsScopedAndBounded(t *testing.T) {
	contract := ResultSnapshotContract{
		table: ResultTableRef{Owner: "REPORT", Name: "RESULT_ROWS"}, runIDColumn: "RUN_ID", rowIDColumn: "ID", columns: map[string]struct{}{"VALUE": {}},
	}
	plan, err := BuildPurgePlan(contract)
	if err != nil {
		t.Fatalf("BuildPurgePlan() error = %v", err)
	}
	for _, required := range []string{"WHERE RUN_ID = :1", "ROWNUM <= :2"} {
		if !strings.Contains(plan.statement, required) {
			t.Fatalf("purge statement %q does not contain %q", plan.statement, required)
		}
	}
}

func TestBuildResultCountPlanIsRunScoped(t *testing.T) {
	plan, err := BuildResultCountPlan(testSnapshotContract())
	if err != nil {
		t.Fatalf("BuildResultCountPlan() error = %v", err)
	}
	want := "SELECT COUNT(*) FROM REPORT_OWNER.SALES_RESULT WHERE RUN_ID = :1"
	if plan.statement != want {
		t.Fatalf("statement = %q, want %q", plan.statement, want)
	}
}

func TestResultPlansRejectUnsafeOrAmbiguousColumns(t *testing.T) {
	tests := []ResultSnapshotRef{
		{Table: ResultTableRef{Owner: "REPORT", Name: "ROWS; DROP TABLE X"}, RunIDColumn: "RUN_ID", RowIDColumn: "ID", Columns: []string{"VALUE"}},
		{Table: ResultTableRef{Owner: "REPORT", Name: "ROWS"}, RunIDColumn: "RUN_ID OR 1=1", RowIDColumn: "ID", Columns: []string{"VALUE"}},
		{Table: ResultTableRef{Owner: "REPORT", Name: "ROWS"}, RunIDColumn: "RUN_ID", RowIDColumn: "RUN_ID", Columns: []string{"VALUE"}},
		{Table: ResultTableRef{Owner: "REPORT", Name: "ROWS"}, RunIDColumn: "RUN_ID", RowIDColumn: "ID", Columns: []string{"VALUE", "value"}},
		{Table: ResultTableRef{Owner: "REPORT", Name: "ROWS"}, RunIDColumn: "RUN_ID", RowIDColumn: "ID", Columns: []string{"RUN_ID"}},
	}
	for index, ref := range tests {
		contract := ResultSnapshotContract{table: ref.Table, runIDColumn: ref.RunIDColumn, rowIDColumn: ref.RowIDColumn}
		if _, err := BuildResultPagePlan(contract, ref.Columns); !errors.Is(err, ErrInvalidConfiguration) {
			t.Fatalf("case %d error = %v, want ErrInvalidConfiguration", index, err)
		}
	}
}

func TestResultPlanRejectsUnvalidatedContract(t *testing.T) {
	if _, err := BuildResultPagePlan(ResultSnapshotContract{}, []string{"VALUE"}); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("BuildResultPagePlan() error = %v, want ErrInvalidConfiguration", err)
	}
	if _, err := BuildPurgePlan(ResultSnapshotContract{}); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("BuildPurgePlan() error = %v, want ErrInvalidConfiguration", err)
	}
	if _, err := BuildResultCountPlan(ResultSnapshotContract{}); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("BuildResultCountPlan() error = %v, want ErrInvalidConfiguration", err)
	}
}

func TestResultPlanSupportsConfiguredMaximumColumns(t *testing.T) {
	columns := make([]string, maxResultColumns)
	contract := testSnapshotContract()
	for index := range columns {
		columns[index] = "VALUE_" + strconv.Itoa(index)
		contract.columns[columns[index]] = struct{}{}
	}
	if _, err := BuildResultPagePlan(contract, columns); err != nil {
		t.Fatalf("BuildResultPagePlan() error = %v", err)
	}
	columns = append(columns, "TOO_MANY")
	if _, err := BuildResultPagePlan(contract, columns); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("BuildResultPagePlan() error = %v, want ErrInvalidConfiguration", err)
	}
}

func TestSupportedSystemResultIDTypes(t *testing.T) {
	zero, two, eighteen := int64(0), int64(2), int64(18)
	for _, column := range []ResultColumn{{DataType: "CHAR"}, {DataType: "VARCHAR2"}, {DataType: "NUMBER"}, {DataType: "NUMBER", DataScale: &zero}} {
		if _, supported := supportedReportIDColumn(column); !supported {
			t.Fatalf("supportedReportIDColumn(%#v) = false", column)
		}
	}
	if _, supported := supportedReportIDColumn(ResultColumn{DataType: "NUMBER", DataScale: &two}); supported {
		t.Fatal("supportedReportIDColumn accepted a decimal report id")
	}
	if !supportedRecordIDColumn(ResultColumn{DataType: "NUMBER"}) || !supportedRecordIDColumn(ResultColumn{DataType: "NUMBER", DataPrecision: &eighteen, DataScale: &zero}) {
		t.Fatal("supportedRecordIDColumn rejected an integer NUMBER")
	}
	if supportedRecordIDColumn(ResultColumn{DataType: "VARCHAR2"}) || supportedRecordIDColumn(ResultColumn{DataType: "NUMBER", DataScale: &two}) {
		t.Fatal("supportedRecordIDColumn accepted an unsupported type")
	}
}

func TestOracleRowID(t *testing.T) {
	for _, input := range []interface{}{int64(42), int32(42), int(42), godror.Number("42"), "42"} {
		got, err := oracleRowID(input)
		if err != nil || got != 42 {
			t.Fatalf("oracleRowID(%T) = %d, %v", input, got, err)
		}
	}
	if _, err := oracleRowID("42.1"); err == nil {
		t.Fatal("oracleRowID accepted a decimal row id")
	}
}

func testSnapshotContract() ResultSnapshotContract {
	return ResultSnapshotContract{
		table:       ResultTableRef{Owner: "REPORT_OWNER", Name: "SALES_RESULT"},
		runIDColumn: "RUN_ID", rowIDColumn: "ID",
		columns: map[string]struct{}{"STORE_CODE": {}, "AMOUNT": {}},
	}
}
