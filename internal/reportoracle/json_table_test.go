package reportoracle

import (
	"errors"
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

func TestBuildJSONTableCallPlanRejectsOtherSignatures(t *testing.T) {
	tests := []struct {
		name      string
		ref       ProcedureRef
		arguments []ProcedureArgument
	}{
		{name: "identifier injection", ref: ProcedureRef{Owner: "REPORT;DROP", Name: "RUN"}, arguments: []ProcedureArgument{{Name: "P_JSON", Direction: "IN", DataType: "CLOB"}}},
		{name: "cursor output", ref: ProcedureRef{Owner: "REPORT", Name: "RUN"}, arguments: []ProcedureArgument{{Name: "P_JSON", Direction: "IN", DataType: "CLOB"}, {Name: "P_RESULT", Direction: "OUT", DataType: "REF CURSOR"}}},
		{name: "wrong direction", ref: ProcedureRef{Owner: "REPORT", Name: "RUN"}, arguments: []ProcedureArgument{{Name: "P_JSON", Direction: "OUT", DataType: "CLOB"}}},
		{name: "unsupported type", ref: ProcedureRef{Owner: "REPORT", Name: "RUN"}, arguments: []ProcedureArgument{{Name: "P_JSON", Direction: "IN", DataType: "NUMBER"}}},
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
