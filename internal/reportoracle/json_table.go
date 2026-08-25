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
	outputChecks := make([]string, 0, 1)
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
		declarationType, messageExpression, ok := jsonTableErrorOutputType(argument)
		if !ok {
			return JSONTableCallPlan{}, fmt.Errorf("%w: procedure output argument %q has unsupported type %q", ErrUnsupportedBinding, name, dataType)
		}
		variable := fmt.Sprintf("error_output_%d", index+1)
		declarations = append(declarations, variable+" "+declarationType+";")
		bindings = append(bindings, name+" => "+variable)
		outputChecks = append(outputChecks, "IF "+variable+" IS NOT NULL THEN RAISE_APPLICATION_ERROR(-20001, "+strings.ReplaceAll(messageExpression, "$VALUE", variable)+"); END IF;")
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
		if len(outputChecks) > 0 {
			body += " " + strings.Join(outputChecks, " ")
		}
		statement = "DECLARE " + strings.Join(declarations, " ") + " " + body + " END;"
	}
	return JSONTableCallPlan{statement: statement, payloadIsCLOB: payloadIsCLOB}, nil
}

// SupportsJSONTableErrorOutput reports whether an R_ERROR OUT argument can be
// checked by the table-snapshot call before the transaction is committed.
func SupportsJSONTableErrorOutput(argument ProcedureArgument) bool {
	if strings.ToUpper(strings.TrimSpace(argument.Direction)) != "OUT" {
		return false
	}
	_, _, ok := jsonTableErrorOutputType(argument)
	return ok
}

func jsonTableErrorOutputType(argument ProcedureArgument) (declaration, messageExpression string, ok bool) {
	if strings.ToUpper(strings.TrimSpace(argument.Name)) != "R_ERROR" || strings.TrimSpace(argument.TypeOwner) != "" || strings.TrimSpace(argument.TypeName) != "" {
		return "", "", false
	}
	dataType := normalizeOracleMetadataType(argument.DataType)
	switch dataType {
	case "VARCHAR2", "CHAR", "LONG":
		return "VARCHAR2(32767)", "SUBSTR($VALUE, 1, 500)", true
	case "NVARCHAR2", "NCHAR":
		return "NVARCHAR2(32767)", "SUBSTR($VALUE, 1, 500)", true
	case "CLOB", "NCLOB":
		return dataType, "DBMS_LOB.SUBSTR($VALUE, 500, 1)", true
	default:
		return "", "", false
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
