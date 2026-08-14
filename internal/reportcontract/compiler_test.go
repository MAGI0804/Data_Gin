package reportcontract

import (
	"errors"
	"strings"
	"testing"

	"gin-biz-web-api/internal/reportoracle"
	"gin-biz-web-api/model"
)

func TestCompileProducesStablePublicationContract(t *testing.T) {
	version, parameters, columns, grants, arguments, result := validContract()
	contract := validSnapshotContract(t, version, result, columns)
	first, err := Compile(version, parameters, columns, grants, arguments, result, contract)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	second, err := Compile(version, []model.ReportParameter{parameters[1], parameters[0]}, columns,
		[]model.ReportGrant{grants[1], grants[0]}, []reportoracle.ProcedureArgument{arguments[1], arguments[0]}, result, contract)
	if err != nil {
		t.Fatalf("Compile(shuffled) error = %v", err)
	}
	if first.Hashes != second.Hashes || string(first.SpecJSON) != string(second.SpecJSON) {
		t.Fatalf("publication contract is not deterministic\nfirst=%+v\nsecond=%+v", first.Hashes, second.Hashes)
	}
	for name, value := range map[string]string{
		"contract": first.Hashes.Contract, "parameters": first.Hashes.ParameterSchema,
		"procedure": first.Hashes.ProcedureSignature, "result": first.Hashes.ResultSchema,
		"permission": first.Hashes.Permission, "export": first.Hashes.ExportSchema,
	} {
		if len(value) != 64 {
			t.Fatalf("%s hash length = %d", name, len(value))
		}
	}
}

func TestCompileRefCursorUsesJSONSchemaAndDiscoveredSignature(t *testing.T) {
	version, _, columns, grants, _, _ := validContract()
	version.ExecutionMode = model.ReportExecutionModeRefCursor
	version.JSONInputArgName = "P_JSON"
	version.ResultCursorArgName = "P_RESULT"
	version.InputSchemaJSON = model.JSONText(`{"store_id":{"type":"VARCHAR2","displayName":"门店","required":true}}`)
	version.ResultTableOwner, version.ResultTableName = "", ""
	version.ResultRunIDColumn, version.ResultRowIDColumn = "", ""
	version.CallTemplate = "BEGIN REPORT.PKG.SALES(P_JSON => :payload, P_RESULT => :resultCursor); END;"
	arguments := []reportoracle.ProcedureArgument{
		{Name: "P_JSON", Position: 1, Sequence: 1, Direction: "IN", DataType: "CLOB"},
		{Name: "P_RESULT", Position: 2, Sequence: 2, Direction: "OUT", DataType: "REF CURSOR"},
	}

	compiled, err := Compile(version, nil, columns, grants, arguments, nil, reportoracle.ResultSnapshotContract{})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if len(compiled.SpecJSON) == 0 || len(compiled.Hashes.Contract) != 64 || len(compiled.Hashes.ParameterSchema) != 64 {
		t.Fatalf("compiled = %#v", compiled)
	}
	if err := VerifyRuntimeProcedureMetadata(compiled.SpecJSON, compiled.Hashes.Contract, compiled.Hashes.ProcedureSignature, arguments); err != nil {
		t.Fatalf("VerifyRuntimeProcedureMetadata() error = %v", err)
	}
}

func TestCompileRefCursorRejectsMissingExcelMapping(t *testing.T) {
	version, _, _, grants, _, _ := validContract()
	version.ExecutionMode = model.ReportExecutionModeRefCursor
	version.JSONInputArgName, version.ResultCursorArgName = "P_JSON", "P_RESULT"
	version.InputSchemaJSON = model.JSONText(`{"store_id":{"type":"VARCHAR2","displayName":"门店"}}`)
	arguments := []reportoracle.ProcedureArgument{
		{Name: "P_JSON", Position: 1, Sequence: 1, Direction: "IN", DataType: "CLOB"},
		{Name: "P_RESULT", Position: 2, Sequence: 2, Direction: "OUT", DataType: "REF CURSOR"},
	}

	if _, err := Compile(version, nil, nil, grants, arguments, nil, reportoracle.ResultSnapshotContract{}); !errors.Is(err, ErrInvalidContract) {
		t.Fatalf("Compile() error = %v, want ErrInvalidContract", err)
	}
}

func TestCompileJSONInputResultTableUsesOneInputAndPublishedTable(t *testing.T) {
	version, _, columns, grants, _, result := validContract()
	version.ExecutionMode = model.ReportExecutionModeTableSnapshot
	version.JSONInputArgName = "P_JSON"
	version.InputSchemaJSON = model.JSONText(`{"store_id":{"type":"VARCHAR2","displayName":"门店","required":true}}`)
	version.CallTemplate = "BEGIN REPORT.PKG.SALES(P_JSON => :payload); END;"
	arguments := []reportoracle.ProcedureArgument{
		{Name: "P_JSON", Position: 1, Sequence: 1, Direction: "IN", DataType: "CLOB"},
	}
	contract := validSnapshotContract(t, version, result, columns)

	compiled, err := Compile(version, nil, columns, grants, arguments, result, contract)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if len(compiled.SpecJSON) == 0 || len(compiled.Hashes.ParameterSchema) != 64 || len(compiled.Hashes.ResultSchema) != 64 {
		t.Fatalf("compiled = %#v", compiled)
	}
	if !strings.Contains(string(compiled.SpecJSON), `"jsonInputArgName":"P_JSON"`) || !strings.Contains(string(compiled.SpecJSON), `"resultTableName":"SALES_RESULT"`) {
		t.Fatalf("compiled spec = %s", compiled.SpecJSON)
	}
	if err := VerifyRuntimeMetadata(compiled.SpecJSON, compiled.Hashes.Contract, compiled.Hashes.ProcedureSignature, compiled.Hashes.ResultSchema, arguments, result); err != nil {
		t.Fatalf("VerifyRuntimeMetadata() error = %v", err)
	}
}

func TestCompileJSONInputResultTableRejectsCursorOutput(t *testing.T) {
	version, _, columns, grants, _, result := validContract()
	version.ExecutionMode = model.ReportExecutionModeTableSnapshot
	version.JSONInputArgName = "P_JSON"
	version.InputSchemaJSON = model.JSONText(`{"store_id":{"type":"VARCHAR2","displayName":"门店"}}`)
	version.CallTemplate = "BEGIN REPORT.PKG.SALES(P_JSON => :payload); END;"
	arguments := []reportoracle.ProcedureArgument{
		{Name: "P_JSON", Position: 1, Sequence: 1, Direction: "IN", DataType: "CLOB"},
		{Name: "P_RESULT", Position: 2, Sequence: 2, Direction: "OUT", DataType: "REF CURSOR"},
	}

	_, err := Compile(version, nil, columns, grants, arguments, result, validSnapshotContract(t, version, result, columns))
	if !errors.Is(err, ErrInvalidContract) {
		t.Fatalf("Compile() error = %v, want ErrInvalidContract", err)
	}
}

func TestVerifyRuntimeMetadataAcceptsPublishedContractAndRejectsDrift(t *testing.T) {
	version, parameters, columns, grants, procedure, result := validContract()
	contract := validSnapshotContract(t, version, result, columns)
	compiled, err := Compile(version, parameters, columns, grants, procedure, result, contract)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if err := VerifyRuntimeMetadata(compiled.SpecJSON, compiled.Hashes.Contract, compiled.Hashes.ProcedureSignature, compiled.Hashes.ResultSchema, procedure, result); err != nil {
		t.Fatalf("VerifyRuntimeMetadata() error = %v", err)
	}
	procedure[0].Name = "P_WRONG"
	if err := VerifyRuntimeMetadata(compiled.SpecJSON, compiled.Hashes.Contract, compiled.Hashes.ProcedureSignature, compiled.Hashes.ResultSchema, procedure, result); !errors.Is(err, ErrInvalidContract) {
		t.Fatalf("procedure drift error = %v", err)
	}
}

func TestVerifyRuntimeResultMetadataRejectsDriftAfterRun(t *testing.T) {
	version, parameters, columns, grants, procedure, result := validContract()
	compiled, err := Compile(version, parameters, columns, grants, procedure, result, validSnapshotContract(t, version, result, columns))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if err := VerifyRuntimeResultMetadata(compiled.SpecJSON, compiled.Hashes.Contract, compiled.Hashes.ResultSchema, result); err != nil {
		t.Fatalf("VerifyRuntimeResultMetadata() error = %v", err)
	}
	drifted := append([]reportoracle.ResultColumn(nil), result...)
	drifted[len(drifted)-1].DataType = "CLOB"
	if err := VerifyRuntimeResultMetadata(compiled.SpecJSON, compiled.Hashes.Contract, compiled.Hashes.ResultSchema, drifted); !errors.Is(err, ErrInvalidContract) {
		t.Fatalf("drift error = %v", err)
	}
	if err := VerifyRuntimeResultMetadata(compiled.SpecJSON, strings.Repeat("0", 64), compiled.Hashes.ResultSchema, result); !errors.Is(err, ErrInvalidContract) {
		t.Fatalf("contract hash error = %v", err)
	}
}

func TestCompileRejectsProcedureAndTemplateDrift(t *testing.T) {
	version, parameters, columns, grants, arguments, result := validContract()
	arguments[0].Name = "P_WRONG"
	contract := validSnapshotContract(t, version, result, columns)
	if _, err := Compile(version, parameters, columns, grants, arguments, result, contract); !errors.Is(err, ErrInvalidContract) {
		t.Fatalf("procedure drift error = %v", err)
	}

	version, parameters, columns, grants, arguments, result = validContract()
	version.CallTemplate = "BEGIN {{runId}}; END;"
	if _, err := Compile(version, parameters, columns, grants, arguments, result, contract); !errors.Is(err, ErrInvalidContract) {
		t.Fatalf("template drift error = %v", err)
	}
}

func TestCompileRejectsResultAndExcelMappingDrift(t *testing.T) {
	version, parameters, columns, grants, arguments, result := validContract()
	columns[0].DatabaseColumn = "MISSING_COLUMN"
	contract := validSnapshotContract(t, version, result, columns)
	if _, err := Compile(version, parameters, columns, grants, arguments, result, contract); !errors.Is(err, ErrInvalidContract) {
		t.Fatalf("result drift error = %v", err)
	}

	version, parameters, columns, grants, arguments, result = validContract()
	columns[1].ExcelHeader = columns[0].ExcelHeader
	if _, err := Compile(version, parameters, columns, grants, arguments, result, contract); !errors.Is(err, ErrInvalidContract) {
		t.Fatalf("duplicate header error = %v", err)
	}
}

func TestCompileRejectsContractWithoutExportableColumns(t *testing.T) {
	version, parameters, columns, grants, arguments, result := validContract()
	for index := range columns {
		columns[index].ExportAllowed = false
	}
	contract := validSnapshotContract(t, version, result, columns)
	if _, err := Compile(version, parameters, columns, grants, arguments, result, contract); !errors.Is(err, ErrInvalidContract) {
		t.Fatalf("Compile() error = %v, want ErrInvalidContract", err)
	}
}

func TestCompileIgnoresNonExportableFieldsInExcelHeaderUniqueness(t *testing.T) {
	version, parameters, columns, grants, arguments, result := validContract()
	columns[0].ExportAllowed = false
	columns[1].ExcelHeader = columns[0].ExcelHeader
	columns[1].ExportOrder = columns[0].ExportOrder
	contract := validSnapshotContract(t, version, result, columns)
	if _, err := Compile(version, parameters, columns, grants, arguments, result, contract); err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
}

func TestCanonicalJSONPreservesExactNumbers(t *testing.T) {
	left := canonicalJSON(model.JSONText(`{"value":9007199254740992,"decimal":0.1234567890123456789}`))
	right := canonicalJSON(model.JSONText(`{"value":9007199254740993,"decimal":0.1234567890123456790}`))
	if string(left) == string(right) {
		t.Fatalf("canonical JSON collapsed distinct exact numbers: %s", left)
	}
	if string(left) != `{"decimal":0.1234567890123456789,"value":9007199254740992}` {
		t.Fatalf("canonical JSON changed numeric precision: %s", left)
	}
}

func TestCompileRejectsDuplicateOracleMetadataPositions(t *testing.T) {
	version, parameters, columns, grants, arguments, result := validContract()
	contract := validSnapshotContract(t, version, result, columns)
	arguments[1].Sequence = arguments[0].Sequence
	if _, err := Compile(version, parameters, columns, grants, arguments, result, contract); !errors.Is(err, ErrInvalidContract) {
		t.Fatalf("duplicate argument sequence error = %v", err)
	}

	version, parameters, columns, grants, arguments, result = validContract()
	contract = validSnapshotContract(t, version, result, columns)
	result[3].Position = result[2].Position
	if _, err := Compile(version, parameters, columns, grants, arguments, result, contract); !errors.Is(err, ErrInvalidContract) {
		t.Fatalf("duplicate result position error = %v", err)
	}
}

func TestCompileRejectsSensitiveDefaultAndUnknownGrantAction(t *testing.T) {
	version, parameters, columns, grants, arguments, result := validContract()
	contract := validSnapshotContract(t, version, result, columns)
	parameters[1].Sensitive = true
	parameters[1].DefaultValueJSON = model.JSONText(`"2026-08-01"`)
	if _, err := Compile(version, parameters, columns, grants, arguments, result, contract); !errors.Is(err, ErrInvalidContract) {
		t.Fatalf("sensitive default error = %v", err)
	}

	version, parameters, columns, grants, arguments, result = validContract()
	contract = validSnapshotContract(t, version, result, columns)
	grants[0].ActionsJSON = model.JSONText(`["READ","DELETE"]`)
	if _, err := Compile(version, parameters, columns, grants, arguments, result, contract); !errors.Is(err, ErrInvalidContract) {
		t.Fatalf("unknown grant action error = %v", err)
	}
}

func TestCompileRejectsUnsupportedParameterAutomation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func([]model.ReportParameter)
	}{
		{
			name: "system injection other than run id",
			mutate: func(parameters []model.ReportParameter) {
				parameters[1].SystemInjected = true
			},
		},
		{
			name: "normalizer on date",
			mutate: func(parameters []model.ReportParameter) {
				parameters[1].NormalizerJSON = model.JSONText(`{"trim":true}`)
			},
		},
		{
			name: "unsupported value source",
			mutate: func(parameters []model.ReportParameter) {
				parameters[1].ValueSourceJSON = model.JSONText(`{"source":"actor"}`)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			version, parameters, columns, grants, arguments, result := validContract()
			test.mutate(parameters)
			contract := validSnapshotContract(t, version, result, columns)
			if _, err := Compile(version, parameters, columns, grants, arguments, result, contract); !errors.Is(err, ErrInvalidContract) {
				t.Fatalf("Compile() error = %v, want ErrInvalidContract", err)
			}
		})
	}
}

func TestCompileAcceptsWhitelistedParameterAutomation(t *testing.T) {
	version, parameters, columns, grants, arguments, result := validContract()
	contract := validSnapshotContract(t, version, result, columns)
	parameters[0].ValueSourceJSON = model.JSONText(`{"source":"RUN_ID"}`)
	if _, err := Compile(version, parameters, columns, grants, arguments, result, contract); err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
}

func TestCompileRejectsLegacySensitiveSystemParameter(t *testing.T) {
	version, parameters, columns, grants, arguments, result := validContract()
	contract := validSnapshotContract(t, version, result, columns)
	parameters[0].Sensitive = true
	if _, err := Compile(version, parameters, columns, grants, arguments, result, contract); !errors.Is(err, ErrInvalidContract) {
		t.Fatalf("Compile() error = %v, want ErrInvalidContract", err)
	}
}

func TestCompileRejectsLogicalOracleTypeDrift(t *testing.T) {
	version, parameters, columns, grants, arguments, result := validContract()
	contract := validSnapshotContract(t, version, result, columns)
	parameters[1].OracleType = "NUMBER"
	arguments[1].DataType = "NUMBER"
	if _, err := Compile(version, parameters, columns, grants, arguments, result, contract); !errors.Is(err, ErrInvalidContract) {
		t.Fatalf("logical parameter type drift error = %v", err)
	}

	version, parameters, columns, grants, arguments, result = validContract()
	contract = validSnapshotContract(t, version, result, columns)
	columns[1].SourceOracleType = "VARCHAR2"
	result[3].DataType = "VARCHAR2"
	if _, err := Compile(version, parameters, columns, grants, arguments, result, contract); !errors.Is(err, ErrInvalidContract) {
		t.Fatalf("logical result type drift error = %v", err)
	}
}

func TestCompileRejectsParameterControlTypeDrift(t *testing.T) {
	version, parameters, columns, grants, arguments, result := validContract()
	parameters[1].ControlType = "CHECKBOX"
	contract := validSnapshotContract(t, version, result, columns)
	if _, err := Compile(version, parameters, columns, grants, arguments, result, contract); !errors.Is(err, ErrInvalidContract) {
		t.Fatalf("control type drift error = %v", err)
	}
}

func TestCompileRejectsInvalidParameterPresentation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func([]model.ReportParameter)
	}{
		{
			name: "control does not match logical type",
			mutate: func(parameters []model.ReportParameter) {
				parameters[1].ControlType = "CHECKBOX"
			},
		},
		{
			name: "collection encoding on scalar",
			mutate: func(parameters []model.ReportParameter) {
				parameters[1].CollectionEncoding = "JSON_CLOB"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			version, parameters, columns, grants, arguments, result := validContract()
			test.mutate(parameters)
			contract := validSnapshotContract(t, version, result, columns)
			if _, err := Compile(version, parameters, columns, grants, arguments, result, contract); !errors.Is(err, ErrInvalidContract) {
				t.Fatalf("Compile() error = %v, want ErrInvalidContract", err)
			}
		})
	}
}

func validSnapshotContract(
	t *testing.T,
	version model.ReportVersion,
	result []reportoracle.ResultColumn,
	columns []model.ReportColumn,
) reportoracle.ResultSnapshotContract {
	t.Helper()
	columnNames := make([]string, 0, len(columns))
	for _, column := range columns {
		columnNames = append(columnNames, column.DatabaseColumn)
	}
	contract, err := reportoracle.CompileResultSnapshotContract(reportoracle.SystemResultSnapshotRef(
		reportoracle.ResultTableRef{Owner: version.ResultTableOwner, Name: version.ResultTableName}, columnNames,
	), result, true)
	if err != nil {
		t.Fatalf("CompileResultSnapshotContract() error = %v", err)
	}
	return contract
}

func validContract() (
	model.ReportVersion,
	[]model.ReportParameter,
	[]model.ReportColumn,
	[]model.ReportGrant,
	[]reportoracle.ProcedureArgument,
	[]reportoracle.ResultColumn,
) {
	zero := int64(0)
	eighteen := int64(18)
	version := model.ReportVersion{
		DatasourceID:   3,
		ProcedureOwner: "report", PackageName: "pkg", ProcedureName: "sales",
		ResultTableOwner: "report", ResultTableName: "sales_result",
		ResultRunIDColumn: "run_id", ResultRowIDColumn: "id",
		CallTemplate: "BEGIN REPORT.PKG.SALES(P_RUN_ID => {{runId}}, P_FROM => {{from}}); END;",
	}
	parameters := []model.ReportParameter{
		{ParameterCode: "runId", Label: "运行编号", ControlType: "hidden", LogicalType: "string", Cardinality: "SINGLE", ProcedureArgName: "P_RUN_ID", Position: 1, Direction: "IN", OracleType: "VARCHAR2", Required: true, SystemInjected: true, NullPolicy: "TYPED_NULL"},
		{ParameterCode: "from", Label: "开始日期", ControlType: "date", LogicalType: "date", Cardinality: "SINGLE", ProcedureArgName: "P_FROM", Position: 2, Direction: "IN", OracleType: "DATE", Required: true, NullPolicy: "TYPED_NULL"},
	}
	columns := []model.ReportColumn{
		{FieldID: "field-store", LogicalCode: "storeCode", DatabaseColumn: "STORE_CODE", SourceOracleType: "VARCHAR2", ValueType: "string", PreviewHeader: "门店", ExcelHeader: "门店", DisplayOrder: 1, ExportOrder: 1, PreviewVisible: true, ExportVisible: true, ExportAllowed: true},
		{FieldID: "field-amount", LogicalCode: "amount", DatabaseColumn: "AMOUNT", SourceOracleType: "NUMBER", Nullable: true, ValueType: "decimal", PreviewHeader: "金额", ExcelHeader: "金额", DisplayOrder: 2, ExportOrder: 2, PreviewVisible: true, ExportVisible: true, ExportAllowed: true},
	}
	grants := []model.ReportGrant{
		{SubjectType: "ROLE", SubjectID: 2, ActionsJSON: model.JSONText(`["QUERY"]`)},
		{SubjectType: "USER", SubjectID: 8, ActionsJSON: model.JSONText(`["EXPORT"]`)},
	}
	arguments := []reportoracle.ProcedureArgument{
		{Name: "P_RUN_ID", Position: 1, Sequence: 1, Direction: "IN", DataType: "VARCHAR2"},
		{Name: "P_FROM", Position: 2, Sequence: 2, Direction: "IN", DataType: "DATE"},
	}
	result := []reportoracle.ResultColumn{
		{Name: "RUN_ID", Position: 1, DataType: "VARCHAR2", Nullable: false},
		{Name: "ID", Position: 2, DataType: "NUMBER", DataPrecision: &eighteen, DataScale: &zero, Nullable: false},
		{Name: "STORE_CODE", Position: 3, DataType: "VARCHAR2", Nullable: false},
		{Name: "AMOUNT", Position: 4, DataType: "NUMBER", Nullable: true},
	}
	return version, parameters, columns, grants, arguments, result
}
