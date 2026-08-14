package reportoracle

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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
	if len(arguments) != 1 {
		return JSONTableCallPlan{}, fmt.Errorf("%w: procedure must have exactly one JSON input", ErrUnsupportedBinding)
	}
	argument := arguments[0]
	name := strings.ToUpper(strings.TrimSpace(argument.Name))
	direction := strings.ToUpper(strings.TrimSpace(argument.Direction))
	dataType := normalizeOracleMetadataType(argument.DataType)
	if name != inputName || direction != "IN" || !jsonInputOracleType(dataType) {
		return JSONTableCallPlan{}, fmt.Errorf("%w: procedure signature does not match the JSON result-table protocol", ErrUnsupportedBinding)
	}
	target := normalized.Owner + "."
	if normalized.Package != "" {
		target += normalized.Package + "."
	}
	target += normalized.Name
	return JSONTableCallPlan{
		statement:     fmt.Sprintf("BEGIN %s(%s => :payload); END;", target, inputName),
		payloadIsCLOB: dataType == "CLOB" || dataType == "NCLOB" || dataType == "JSON",
	}, nil
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
