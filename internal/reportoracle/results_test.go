package reportoracle

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/godror/godror"
)

func TestBuildResultPagePlan(t *testing.T) {
	contract := testSnapshotContract()
	plan, err := BuildResultPagePlan(contract, []string{"store_code", "amount"})
	if err != nil {
		t.Fatalf("BuildResultPagePlan() error = %v", err)
	}
	want := "SELECT ROW_NO, STORE_CODE, AMOUNT FROM REPORT_OWNER.SALES_RESULT WHERE RUN_ID = :1 AND ROW_NO > :2 ORDER BY ROW_NO ASC FETCH NEXT :3 ROWS ONLY"
	if plan.nextStatement != want {
		t.Fatalf("statement = %q, want %q", plan.nextStatement, want)
	}
	if strings.Contains(plan.initialStatement, "ROW_NO >") {
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
		table: ResultTableRef{Owner: "REPORT", Name: "RESULT_ROWS"}, runIDColumn: "RUN_ID", rowIDColumn: "ROW_NO",
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

func TestResultPlansRejectUnsafeOrAmbiguousColumns(t *testing.T) {
	tests := []ResultSnapshotRef{
		{Table: ResultTableRef{Owner: "REPORT", Name: "ROWS; DROP TABLE X"}, RunIDColumn: "RUN_ID", RowIDColumn: "ROW_NO", Columns: []string{"VALUE"}},
		{Table: ResultTableRef{Owner: "REPORT", Name: "ROWS"}, RunIDColumn: "RUN_ID OR 1=1", RowIDColumn: "ROW_NO", Columns: []string{"VALUE"}},
		{Table: ResultTableRef{Owner: "REPORT", Name: "ROWS"}, RunIDColumn: "RUN_ID", RowIDColumn: "RUN_ID", Columns: []string{"VALUE"}},
		{Table: ResultTableRef{Owner: "REPORT", Name: "ROWS"}, RunIDColumn: "RUN_ID", RowIDColumn: "ROW_NO", Columns: []string{"VALUE", "value"}},
		{Table: ResultTableRef{Owner: "REPORT", Name: "ROWS"}, RunIDColumn: "RUN_ID", RowIDColumn: "ROW_NO", Columns: []string{"RUN_ID"}},
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
}

func TestResultPlanSupportsConfiguredMaximumColumns(t *testing.T) {
	columns := make([]string, maxResultColumns)
	for index := range columns {
		columns[index] = "VALUE_" + strconv.Itoa(index)
	}
	if _, err := BuildResultPagePlan(testSnapshotContract(), columns); err != nil {
		t.Fatalf("BuildResultPagePlan() error = %v", err)
	}
	columns = append(columns, "TOO_MANY")
	if _, err := BuildResultPagePlan(testSnapshotContract(), columns); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("BuildResultPagePlan() error = %v, want ErrInvalidConfiguration", err)
	}
}

func TestSupportedRunIDType(t *testing.T) {
	for _, dataType := range []string{"CHAR", "VARCHAR2", "NVARCHAR2"} {
		if !supportedRunIDType(dataType) {
			t.Fatalf("supportedRunIDType(%q) = false", dataType)
		}
	}
	for _, dataType := range []string{"NUMBER", "BLOB", "BFILE"} {
		if supportedRunIDType(dataType) {
			t.Fatalf("supportedRunIDType(%q) = true", dataType)
		}
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
		runIDColumn: "RUN_ID", rowIDColumn: "ROW_NO",
	}
}
