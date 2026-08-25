package reportoracle

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/godror/godror"
)

// JSONTableCallPlan is the validated call contract for a procedure that
// accepts one JSON document and writes rows into a run-scoped result table.
type JSONTableCallPlan struct {
	statement     string
	payloadIsCLOB bool
}

func (plan JSONTableCallPlan) Statement() string { return plan.statement }

func BuildJSONTableCallPlan(ref ProcedureRef, arguments []ProcedureArgument, inputArgName string) (JSONTableCallPlan, error) {
	normalized, err := NormalizeProcedureRef(ref)
	if err != nil {
		return JSONTableCallPlan{}, err
	}
	inputName, err := normalizeIdentifier(inputArgName, "JSON input argument")
	if err != nil {
		return JSONTableCallPlan{}, err
	}
	ordered := append([]ProcedureArgument(nil), arguments...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Position < ordered[j].Position })
	bindings := make([]string, 0, len(ordered))
	declarations := make([]string, 0, len(ordered))
	cursorCleanup := make([]string, 0, 1)
	inputFound, payloadIsCLOB := false, false
	for index, argument := range ordered {
		name, err := normalizeIdentifier(argument.Name, "procedure argument")
		if err != nil {
			return JSONTableCallPlan{}, err
		}
		direction := strings.ToUpper(strings.TrimSpace(argument.Direction))
		dataType := normalizeOracleMetadataType(argument.DataType)
		if name == inputName && direction == "IN" && jsonInputOracleType(dataType) {
			if inputFound {
				return JSONTableCallPlan{}, fmt.Errorf("%w: duplicate JSON input argument", ErrUnsupportedBinding)
			}
			inputFound = true
			payloadIsCLOB = dataType == "CLOB" || dataType == "NCLOB" || dataType == "JSON"
			bindings = append(bindings, name+" => :payload")
			continue
		}
		if direction != "OUT" {
			return JSONTableCallPlan{}, fmt.Errorf("%w: procedure contains an unsupported non-output argument %q", ErrUnsupportedBinding, name)
		}
		declarationType, cursor, ok := ignoredJSONTableOutputType(argument)
		if !ok {
			return JSONTableCallPlan{}, fmt.Errorf("%w: procedure output argument %q has unsupported type %q", ErrUnsupportedBinding, name, dataType)
		}
		variable := fmt.Sprintf("ignored_output_%d", index+1)
		declarations = append(declarations, variable+" "+declarationType+";")
		bindings = append(bindings, name+" => "+variable)
		if cursor {
			cursorCleanup = append(cursorCleanup, "IF "+variable+"%ISOPEN THEN CLOSE "+variable+"; END IF;")
		}
	}
	if !inputFound {
		return JSONTableCallPlan{}, fmt.Errorf("%w: procedure must contain the configured JSON input", ErrUnsupportedBinding)
	}
	target := normalized.Owner + "."
	if normalized.Package != "" {
		target += normalized.Package + "."
	}
	target += normalized.Name
	statement := fmt.Sprintf("BEGIN %s(%s); END;", target, strings.Join(bindings, ", "))
	if len(declarations) > 0 {
		body := fmt.Sprintf("BEGIN %s(%s);", target, strings.Join(bindings, ", "))
		if len(cursorCleanup) > 0 {
			body += " " + strings.Join(cursorCleanup, " ")
		}
		statement = "DECLARE " + strings.Join(declarations, " ") + " " + body + " END;"
	}
	return JSONTableCallPlan{statement: statement, payloadIsCLOB: payloadIsCLOB}, nil
}

// SupportsIgnoredJSONTableOutput reports whether an OUT argument can be bound
// to a local PL/SQL variable and safely discarded by the table-snapshot mode.
func SupportsIgnoredJSONTableOutput(argument ProcedureArgument) bool {
	if strings.ToUpper(strings.TrimSpace(argument.Direction)) != "OUT" {
		return false
	}
	_, _, ok := ignoredJSONTableOutputType(argument)
	return ok
}

func ignoredJSONTableOutputType(argument ProcedureArgument) (declaration string, cursor, ok bool) {
	dataType := normalizeOracleMetadataType(argument.DataType)
	typeName := normalizeOracleMetadataType(argument.TypeName)
	switch dataType {
	case "VARCHAR2", "CHAR", "LONG":
		return "VARCHAR2(32767)", false, true
	case "NVARCHAR2", "NCHAR":
		return "NVARCHAR2(32767)", false, true
	case "RAW", "LONG RAW":
		return "RAW(32767)", false, true
	case "NUMBER", "FLOAT", "DECIMAL", "INTEGER", "BINARY_INTEGER", "PLS_INTEGER":
		return "NUMBER", false, true
	case "BINARY_FLOAT", "BINARY_DOUBLE", "DATE", "TIMESTAMP", "TIMESTAMP WITH TIME ZONE", "TIMESTAMP WITH LOCAL TIME ZONE",
		"INTERVAL YEAR TO MONTH", "INTERVAL DAY TO SECOND", "CLOB", "NCLOB", "BLOB", "BFILE", "BOOLEAN", "ROWID", "UROWID":
		return dataType, false, true
	case "REF CURSOR", "SYS_REFCURSOR":
		return "SYS_REFCURSOR", true, true
	default:
		if typeName == "SYS_REFCURSOR" {
			return "SYS_REFCURSOR", true, true
		}
		return "", false, false
	}
}

func (adapter *Adapter) ExecuteJSONTable(ctx context.Context, tx *sql.Tx, plan JSONTableCallPlan, payload string) error {
	if adapter == nil || tx == nil || ctx == nil || strings.TrimSpace(plan.statement) == "" || !json.Valid([]byte(payload)) {
		return fmt.Errorf("execute JSON result-table procedure: invalid request")
	}
	payloadValue := interface{}(payload)
	if plan.payloadIsCLOB {
		payloadValue = godror.Lob{Reader: strings.NewReader(payload), IsClob: true}
	}
	if _, err := tx.ExecContext(ctx, plan.statement, sql.Named("payload", payloadValue)); err != nil {
		return mapExecutionError(ctx, err)
	}
	return nil
}
