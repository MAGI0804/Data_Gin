package reportoracle

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"

	"gin-biz-web-api/internal/reportquery"
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
	for _, required := range []string{"STORE_CODE = :1", "ORDER BY AMOUNT DESC NULLS LAST, ROWID ASC", "FETCH NEXT :2 ROWS ONLY", "FETCH NEXT :8 ROWS ONLY", "ROWIDTOCHAR(ROWID)"} {
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

func TestResultTableROWIDProbeValidatesBothConversions(t *testing.T) {
	probe := resultTableROWIDProbe(ResultTableRef{Owner: "REPORT", Name: "RESULTS"})
	for _, fragment := range []string{"ROWIDTOCHAR", "CHARTOROWID", "ROWNUM <= 1"} {
		if !strings.Contains(probe, fragment) {
			t.Fatalf("ROWID probe is missing %q", fragment)
		}
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
	want := "SELECT ROWIDTOCHAR(ROWID), STORE_CODE, AMOUNT FROM REPORT_OWNER.SALES_RESULT WHERE 1 = 1 AND ROWID > CHARTOROWID(:1) ORDER BY ROWID ASC FETCH NEXT :2 ROWS ONLY"
	if plan.nextStatement != want {
		t.Fatalf("statement = %q, want %q", plan.nextStatement, want)
	}
	if strings.Contains(plan.initialStatement, "CHARTOROWID") {
		t.Fatalf("initial statement unexpectedly assumes a row cursor: %q", plan.initialStatement)
	}
	columns := plan.Columns()
	columns[0] = "MUTATED"
	if plan.columns[0] != "STORE_CODE" {
		t.Fatal("Columns returned shared storage")
	}
}

func TestBuildPurgePlanIsBounded(t *testing.T) {
	contract := ResultSnapshotContract{
		table: ResultTableRef{Owner: "REPORT", Name: "RESULT_ROWS"}, columns: map[string]struct{}{"VALUE": {}},
	}
	plan, err := BuildPurgePlan(contract)
	if err != nil {
		t.Fatalf("BuildPurgePlan() error = %v", err)
	}
	for _, required := range []string{"WHERE ROWID IN", "ROWNUM <= :1"} {
		if !strings.Contains(plan.statement, required) {
			t.Fatalf("purge statement %q does not contain %q", plan.statement, required)
		}
	}
}

func TestBuildResultCountPlanCountsWholeTable(t *testing.T) {
	plan, err := BuildResultCountPlan(testSnapshotContract())
	if err != nil {
		t.Fatalf("BuildResultCountPlan() error = %v", err)
	}
	want := "SELECT COUNT(*) FROM REPORT_OWNER.SALES_RESULT"
	if plan.statement != want {
		t.Fatalf("statement = %q, want %q", plan.statement, want)
	}
}

func TestResultPlansRejectUnsafeOrAmbiguousColumns(t *testing.T) {
	tests := []ResultSnapshotRef{
		{Table: ResultTableRef{Owner: "REPORT", Name: "ROWS; DROP TABLE X"}, Columns: []string{"VALUE"}},
		{Table: ResultTableRef{Owner: "REPORT", Name: "ROWS"}, Columns: []string{"VALUE", "value"}},
		{Table: ResultTableRef{Owner: "REPORT", Name: "ROWS"}, Columns: nil},
	}
	for index, ref := range tests {
		contract := ResultSnapshotContract{table: ref.Table, columns: map[string]struct{}{"VALUE": {}}}
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

func TestValidateResultTableStorageRequiresStablePhysicalROWID(t *testing.T) {
	tests := []struct {
		name        string
		temporary   string
		iotType     string
		rowMovement string
		wantError   bool
	}{
		{name: "heap table", temporary: "N", rowMovement: "DISABLED"},
		{name: "temporary table", temporary: "Y", rowMovement: "DISABLED", wantError: true},
		{name: "index organized table", temporary: "N", iotType: "IOT", rowMovement: "DISABLED", wantError: true},
		{name: "row movement enabled", temporary: "N", rowMovement: "ENABLED", wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateResultTableStorage(test.temporary, test.iotType, test.rowMovement)
			if test.wantError && !errors.Is(err, ErrMetadataMismatch) {
				t.Fatalf("validateResultTableStorage() error = %v, want ErrMetadataMismatch", err)
			}
			if test.name == "temporary table" && !errors.Is(err, ErrTemporaryResultTable) {
				t.Fatalf("validateResultTableStorage() error = %v, want ErrTemporaryResultTable", err)
			}
			if !test.wantError && err != nil {
				t.Fatalf("validateResultTableStorage() error = %v", err)
			}
		})
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

func TestOracleRowKey(t *testing.T) {
	for _, input := range []interface{}{"AAAPr9AAEAAAAGrAAA", []byte("AAAPr9AAEAAAAGrAAA")} {
		got, err := oracleRowKey(input)
		if err != nil || got != "AAAPr9AAEAAAAGrAAA" {
			t.Fatalf("oracleRowKey(%T) = %q, %v", input, got, err)
		}
	}
	if _, err := oracleRowKey(42); err == nil {
		t.Fatal("oracleRowKey accepted an unsupported value")
	}
}

func testSnapshotContract() ResultSnapshotContract {
	return ResultSnapshotContract{
		table:   ResultTableRef{Owner: "REPORT_OWNER", Name: "SALES_RESULT"},
		columns: map[string]struct{}{"STORE_CODE": {}, "AMOUNT": {}},
	}
}
