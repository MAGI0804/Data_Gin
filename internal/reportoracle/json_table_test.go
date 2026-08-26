package reportoracle

import (
	"errors"
	"strings"
	"testing"
)

func TestBuildJSONTableCallPlanAcceptsSingleJSONInput(t *testing.T) {
	tests := []struct {
		name       string
		oracleType string
		wantCLOB   bool
	}{
		{name: "varchar2", oracleType: "VARCHAR2"},
		{name: "clob", oracleType: "CLOB", wantCLOB: true},
		{name: "native JSON", oracleType: "JSON", wantCLOB: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan, err := BuildJSONTableCallPlan(
				ProcedureRef{Owner: "report", Package: "pkg_sales", Name: "build_report"},
				[]ProcedureArgument{{Name: "p_payload", Position: 1, Sequence: 1, Direction: "IN", DataType: test.oracleType}},
				"p_payload",
			)
			if err != nil {
				t.Fatalf("BuildJSONTableCallPlan() error = %v", err)
			}
			if got, want := plan.Statement(), "BEGIN REPORT.PKG_SALES.BUILD_REPORT(P_PAYLOAD => :payload); END;"; got != want {
				t.Fatalf("Statement() = %q, want %q", got, want)
			}
			if plan.payloadIsCLOB != test.wantCLOB {
				t.Fatalf("payloadIsCLOB = %t, want %t", plan.payloadIsCLOB, test.wantCLOB)
			}
		})
	}
}

func TestBuildJSONTableCallPlanAcceptsErrorOutput(t *testing.T) {
	plan, err := BuildJSONTableCallPlan(
		ProcedureRef{Owner: "report", Package: "pkg_sales", Name: "build_report"},
		[]ProcedureArgument{
			{Name: "p_payload", Position: 1, Sequence: 1, Direction: "IN", DataType: "CLOB"},
			{Name: "r_error", Position: 2, Sequence: 2, Direction: "OUT", DataType: "VARCHAR2"},
		},
		"p_payload",
	)
	if err != nil {
		t.Fatalf("BuildJSONTableCallPlan() error = %v", err)
	}
	want := "DECLARE error_output_2 VARCHAR2(32767); BEGIN REPORT.PKG_SALES.BUILD_REPORT(P_PAYLOAD => :payload, R_ERROR => error_output_2); IF error_output_2 IS NOT NULL AND TRIM(SUBSTR(error_output_2, 1, 500)) <> '执行成功' THEN RAISE_APPLICATION_ERROR(-20001, SUBSTR(error_output_2, 1, 500)); END IF; END;"
	if plan.Statement() != want {
		t.Fatalf("Statement() = %q, want %q", plan.Statement(), want)
	}
}

func TestBuildJSONTableCallPlanChecksCLOBErrorOutput(t *testing.T) {
	plan, err := BuildJSONTableCallPlan(
		ProcedureRef{Owner: "report", Name: "build_report"},
		[]ProcedureArgument{
			{Name: "p_payload", Position: 1, Direction: "IN", DataType: "CLOB"},
			{Name: "r_error", Position: 2, Direction: "OUT", DataType: "CLOB"},
		},
		"p_payload",
	)
	if err != nil {
		t.Fatalf("BuildJSONTableCallPlan() error = %v", err)
	}
	want := "IF error_output_2 IS NOT NULL AND TRIM(DBMS_LOB.SUBSTR(error_output_2, 500, 1)) <> '执行成功' THEN RAISE_APPLICATION_ERROR(-20001, DBMS_LOB.SUBSTR(error_output_2, 500, 1)); END IF;"
	if !strings.Contains(plan.Statement(), want) {
		t.Fatalf("Statement() = %q, want fragment %q", plan.Statement(), want)
	}
}

func TestBuildJSONTableCallPlanRejectsOtherSignatures(t *testing.T) {
	tests := []struct {
		name      string
		ref       ProcedureRef
		arguments []ProcedureArgument
	}{
		{name: "identifier injection", ref: ProcedureRef{Owner: "REPORT;DROP", Name: "RUN"}, arguments: []ProcedureArgument{{Name: "P_JSON", Direction: "IN", DataType: "CLOB"}}},
		{name: "wrong direction", ref: ProcedureRef{Owner: "REPORT", Name: "RUN"}, arguments: []ProcedureArgument{{Name: "P_JSON", Direction: "OUT", DataType: "CLOB"}}},
		{name: "unsupported type", ref: ProcedureRef{Owner: "REPORT", Name: "RUN"}, arguments: []ProcedureArgument{{Name: "P_JSON", Direction: "IN", DataType: "NUMBER"}}},
		{name: "required extra input", ref: ProcedureRef{Owner: "REPORT", Name: "RUN"}, arguments: []ProcedureArgument{{Name: "P_JSON", Direction: "IN", DataType: "CLOB"}, {Name: "P_OTHER", Direction: "IN", DataType: "VARCHAR2"}}},
		{name: "result cursor output", ref: ProcedureRef{Owner: "REPORT", Name: "RUN"}, arguments: []ProcedureArgument{{Name: "P_JSON", Direction: "IN", DataType: "CLOB"}, {Name: "P_RESULT", Direction: "OUT", DataType: "REF CURSOR"}}},
		{name: "unrecognized output name", ref: ProcedureRef{Owner: "REPORT", Name: "RUN"}, arguments: []ProcedureArgument{{Name: "P_JSON", Direction: "IN", DataType: "CLOB"}, {Name: "P_STATUS", Direction: "OUT", DataType: "VARCHAR2"}}},
		{name: "unsupported output type", ref: ProcedureRef{Owner: "REPORT", Name: "RUN"}, arguments: []ProcedureArgument{{Name: "P_JSON", Direction: "IN", DataType: "CLOB"}, {Name: "P_OBJECT", Direction: "OUT", DataType: "OBJECT", TypeOwner: "REPORT", TypeName: "RESULT_OBJECT"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := BuildJSONTableCallPlan(test.ref, test.arguments, "P_JSON")
			if !errors.Is(err, ErrInvalidConfiguration) && !errors.Is(err, ErrUnsupportedBinding) {
				t.Fatalf("BuildJSONTableCallPlan() error = %v", err)
			}
		})
	}
}
