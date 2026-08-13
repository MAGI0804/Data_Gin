package reportcontract

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"gin-biz-web-api/internal/reporting"
	"gin-biz-web-api/internal/reportoracle"
	"gin-biz-web-api/model"
)

var ErrInvalidContract = errors.New("invalid report publication contract")

type Hashes struct {
	Contract           string `json:"contract"`
	ParameterSchema    string `json:"parameterSchema"`
	ProcedureSignature string `json:"procedureSignature"`
	ResultSchema       string `json:"resultSchema"`
	Permission         string `json:"permission"`
	ExportSchema       string `json:"exportSchema"`
}

type Compiled struct {
	SpecJSON []byte
	Hashes   Hashes
}

// VerifyRuntimeMetadata proves that the live Oracle procedure and result table
// still match the immutable publication contract. Runtime execution must stop
// before invoking the procedure when either schema has drifted.
func VerifyRuntimeMetadata(specJSON []byte, contractHash, procedureHash, resultHash string, procedure []reportoracle.ProcedureArgument, result []reportoracle.ResultColumn) error {
	spec, err := decodeVerifiedSpec(specJSON, contractHash)
	if err != nil {
		return err
	}
	normalizedProcedure := normalizeProcedureArguments(procedure)
	actualProcedureHash, err := hashJSON(normalizedProcedure)
	if err != nil {
		return err
	}
	actualResultHash, err := resultSchemaHash(result)
	if err != nil {
		return err
	}
	if actualProcedureHash != procedureHash || actualResultHash != resultHash {
		return contractError("live Oracle metadata does not match the published contract")
	}
	storedProcedureHash, err := hashJSON(spec.Procedure)
	if err != nil {
		return err
	}
	storedResultHash, err := hashJSON(spec.Result)
	if err != nil {
		return err
	}
	if storedProcedureHash != procedureHash || storedResultHash != resultHash {
		return contractError("stored Oracle metadata hashes do not match")
	}
	return nil
}

// VerifyRuntimeResultMetadata is used before every preview and export read.
// It prevents a changed Oracle result table from being interpreted through an
// older frozen presentation contract even when the original run succeeded.
func VerifyRuntimeResultMetadata(specJSON []byte, contractHash, resultHash string, result []reportoracle.ResultColumn) error {
	spec, err := decodeVerifiedSpec(specJSON, contractHash)
	if err != nil {
		return err
	}
	actualResultHash, err := resultSchemaHash(result)
	if err != nil {
		return err
	}
	storedResultHash, err := hashJSON(spec.Result)
	if err != nil {
		return err
	}
	if actualResultHash != resultHash || storedResultHash != resultHash {
		return contractError("live Oracle result metadata does not match the published contract")
	}
	return nil
}

func decodeVerifiedSpec(specJSON []byte, contractHash string) (contractSpec, error) {
	if len(bytes.TrimSpace(specJSON)) == 0 {
		return contractSpec{}, contractError("stored contract payload is empty")
	}
	var spec contractSpec
	decoder := json.NewDecoder(bytes.NewReader(specJSON))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&spec); err != nil {
		return contractSpec{}, contractError("stored contract payload is invalid")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return contractSpec{}, contractError("stored contract payload contains trailing data")
	}
	canonicalSpec, err := json.Marshal(spec)
	if err != nil {
		return contractSpec{}, fmt.Errorf("canonicalize stored report contract: %w", err)
	}
	if hashBytes(canonicalSpec) != contractHash {
		return contractSpec{}, contractError("stored contract payload hash does not match")
	}
	return spec, nil
}

func resultSchemaHash(result []reportoracle.ResultColumn) (string, error) {
	normalized := append([]reportoracle.ResultColumn(nil), result...)
	sort.Slice(normalized, func(i, j int) bool { return normalized[i].Position < normalized[j].Position })
	for index := range normalized {
		normalized[index].Name = strings.ToUpper(strings.TrimSpace(normalized[index].Name))
		normalized[index].DataType = normalizeOracleType(normalized[index].DataType)
	}
	return hashJSON(normalized)
}

type contractSpec struct {
	Version    versionSpec                      `json:"version"`
	Parameters []parameterSpec                  `json:"parameters"`
	Procedure  []reportoracle.ProcedureArgument `json:"procedure"`
	Columns    []columnSpec                     `json:"columns"`
	Result     []reportoracle.ResultColumn      `json:"result"`
	Grants     []grantSpec                      `json:"grants"`
}

type versionSpec struct {
	DatasourceID      uint   `json:"datasourceId"`
	ProcedureOwner    string `json:"procedureOwner"`
	PackageName       string `json:"packageName,omitempty"`
	ProcedureName     string `json:"procedureName"`
	ProcedureOverload string `json:"procedureOverload,omitempty"`
	ResultTableOwner  string `json:"resultTableOwner"`
	ResultTableName   string `json:"resultTableName"`
	ResultRunIDColumn string `json:"resultRunIdColumn"`
	ResultRowIDColumn string `json:"resultRowIdColumn"`
	CallStatement     string `json:"callStatement"`
}

type parameterSpec struct {
	Code               string          `json:"code"`
	Label              string          `json:"label"`
	DisplayOrder       int             `json:"displayOrder"`
	ControlType        string          `json:"controlType"`
	LogicalType        string          `json:"logicalType"`
	Cardinality        string          `json:"cardinality"`
	ProcedureArgName   string          `json:"procedureArgName"`
	Position           int             `json:"position"`
	Direction          string          `json:"direction"`
	OracleType         string          `json:"oracleType"`
	Precision          *int            `json:"precision,omitempty"`
	Scale              *int            `json:"scale,omitempty"`
	MaxLength          *int            `json:"maxLength,omitempty"`
	Required           bool            `json:"required"`
	Nullable           bool            `json:"nullable"`
	SystemInjected     bool            `json:"systemInjected"`
	Sensitive          bool            `json:"sensitive"`
	DefaultValue       json.RawMessage `json:"defaultValue,omitempty"`
	AllowedValues      json.RawMessage `json:"allowedValues,omitempty"`
	Validation         json.RawMessage `json:"validation,omitempty"`
	Normalizer         json.RawMessage `json:"normalizer,omitempty"`
	ValueSource        json.RawMessage `json:"valueSource,omitempty"`
	Timezone           string          `json:"timezone,omitempty"`
	NullPolicy         string          `json:"nullPolicy"`
	CollectionEncoding string          `json:"collectionEncoding,omitempty"`
	ErrorMessage       string          `json:"errorMessage,omitempty"`
}

type columnSpec struct {
	FieldID          string          `json:"fieldId"`
	LogicalCode      string          `json:"logicalCode"`
	DatabaseColumn   string          `json:"databaseColumn"`
	SourceOracleType string          `json:"sourceOracleType"`
	Precision        *int            `json:"precision,omitempty"`
	Scale            *int            `json:"scale,omitempty"`
	Nullable         bool            `json:"nullable"`
	ValueType        string          `json:"valueType"`
	PreviewHeader    string          `json:"previewHeader"`
	ExcelHeader      string          `json:"excelHeader"`
	DisplayOrder     int             `json:"displayOrder"`
	ExportOrder      int             `json:"exportOrder"`
	PreviewVisible   bool            `json:"previewVisible"`
	ExportVisible    bool            `json:"exportVisible"`
	Filterable       bool            `json:"filterable"`
	Sortable         bool            `json:"sortable"`
	ExportAllowed    bool            `json:"exportAllowed"`
	AllowedOperators json.RawMessage `json:"allowedOperators,omitempty"`
	Format           json.RawMessage `json:"format,omitempty"`
	MaskingPolicy    json.RawMessage `json:"maskingPolicy,omitempty"`
	Dictionary       json.RawMessage `json:"dictionaryVersion,omitempty"`
	ExcelWidth       float64         `json:"excelWidth"`
	NullDisplay      string          `json:"nullDisplay,omitempty"`
}

type grantSpec struct {
	SubjectType string          `json:"subjectType"`
	SubjectID   uint            `json:"subjectId"`
	Actions     json.RawMessage `json:"actions"`
}

func Compile(
	version model.ReportVersion,
	parameters []model.ReportParameter,
	columns []model.ReportColumn,
	grants []model.ReportGrant,
	procedureArguments []reportoracle.ProcedureArgument,
	resultColumns []reportoracle.ResultColumn,
	snapshotContract reportoracle.ResultSnapshotContract,
) (Compiled, error) {
	if version.DatasourceID == 0 {
		return Compiled{}, contractError("datasource id is required")
	}
	if err := validateProcedureSequences(procedureArguments); err != nil {
		return Compiled{}, err
	}
	parameterSpecs, definitions, err := compileParameters(parameters, procedureArguments)
	if err != nil {
		return Compiled{}, err
	}
	callPlan, err := reportoracle.BuildCallPlan(reportoracle.ProcedureRef{
		Owner: version.ProcedureOwner, Package: version.PackageName,
		Name: version.ProcedureName, Overload: version.ProcedureOverload,
	}, definitions)
	if err != nil {
		return Compiled{}, contractError("compile procedure call: %v", err)
	}
	if err := validateConfiguredTemplate(version.CallTemplate, callPlan.Statement(), definitions); err != nil {
		return Compiled{}, err
	}
	configuredResultColumns := make([]string, 0, len(columns))
	for _, column := range columns {
		configuredResultColumns = append(configuredResultColumns, column.DatabaseColumn)
	}
	if err := reportoracle.ValidateResultSnapshotContract(snapshotContract, reportoracle.ResultSnapshotRef{
		Table:       reportoracle.ResultTableRef{Owner: version.ResultTableOwner, Name: version.ResultTableName},
		RunIDColumn: version.ResultRunIDColumn, RowIDColumn: version.ResultRowIDColumn,
		Columns: configuredResultColumns,
	}); err != nil {
		return Compiled{}, contractError("result snapshot contract is invalid: %v", err)
	}
	columnSpecs, normalizedResult, err := compileColumns(version, columns, resultColumns)
	if err != nil {
		return Compiled{}, err
	}
	grantSpecs, err := compileGrants(grants)
	if err != nil {
		return Compiled{}, err
	}
	spec := contractSpec{
		Version: versionSpec{
			DatasourceID:      version.DatasourceID,
			ProcedureOwner:    strings.ToUpper(strings.TrimSpace(version.ProcedureOwner)),
			PackageName:       strings.ToUpper(strings.TrimSpace(version.PackageName)),
			ProcedureName:     strings.ToUpper(strings.TrimSpace(version.ProcedureName)),
			ProcedureOverload: strings.TrimSpace(version.ProcedureOverload),
			ResultTableOwner:  strings.ToUpper(strings.TrimSpace(version.ResultTableOwner)),
			ResultTableName:   strings.ToUpper(strings.TrimSpace(version.ResultTableName)),
			ResultRunIDColumn: strings.ToUpper(strings.TrimSpace(version.ResultRunIDColumn)),
			ResultRowIDColumn: strings.ToUpper(strings.TrimSpace(version.ResultRowIDColumn)),
			CallStatement:     callPlan.Statement(),
		},
		Parameters: parameterSpecs, Procedure: normalizeProcedureArguments(procedureArguments),
		Columns: columnSpecs, Result: normalizedResult, Grants: grantSpecs,
	}
	parameterHash, err := hashJSON(spec.Parameters)
	if err != nil {
		return Compiled{}, err
	}
	procedureHash, err := hashJSON(spec.Procedure)
	if err != nil {
		return Compiled{}, err
	}
	resultHash, err := hashJSON(spec.Result)
	if err != nil {
		return Compiled{}, err
	}
	permissionHash, err := hashJSON(spec.Grants)
	if err != nil {
		return Compiled{}, err
	}
	exportHash, err := hashJSON(spec.Columns)
	if err != nil {
		return Compiled{}, err
	}
	specJSON, err := json.Marshal(spec)
	if err != nil {
		return Compiled{}, fmt.Errorf("encode report publication contract: %w", err)
	}
	return Compiled{SpecJSON: specJSON, Hashes: Hashes{
		Contract: hashBytes(specJSON), ParameterSchema: parameterHash,
		ProcedureSignature: procedureHash, ResultSchema: resultHash,
		Permission: permissionHash, ExportSchema: exportHash,
	}}, nil
}

func compileParameters(
	parameters []model.ReportParameter,
	actual []reportoracle.ProcedureArgument,
) ([]parameterSpec, []reporting.ParameterDefinition, error) {
	if len(parameters) != len(actual) {
		return nil, nil, contractError("configured and actual procedure parameter counts differ")
	}
	configured := append([]model.ReportParameter(nil), parameters...)
	sort.Slice(configured, func(i, j int) bool { return configured[i].Position < configured[j].Position })
	actualByPosition := make(map[int]reportoracle.ProcedureArgument, len(actual))
	for _, argument := range actual {
		if argument.Name == "" || argument.Position <= 0 || argument.Direction == "" || argument.DataType == "" {
			return nil, nil, contractError("actual procedure signature is incomplete")
		}
		if _, exists := actualByPosition[argument.Position]; exists {
			return nil, nil, contractError("actual procedure signature has duplicate positions")
		}
		actualByPosition[argument.Position] = argument
	}
	specs := make([]parameterSpec, 0, len(configured))
	definitions := make([]reporting.ParameterDefinition, 0, len(configured))
	for _, parameter := range configured {
		argument, exists := actualByPosition[parameter.Position]
		if !exists || !sameIdentifier(parameter.ProcedureArgName, argument.Name) ||
			!sameOracleType(parameter.OracleType, argument.DataType) ||
			!logicalOracleCompatible(parameter.LogicalType, argument.DataType) ||
			!sameParameterShape(parameter, argument) ||
			strings.ToUpper(strings.TrimSpace(parameter.Direction)) != strings.ToUpper(strings.TrimSpace(argument.Direction)) {
			return nil, nil, contractError("parameter %q does not match Oracle procedure signature", parameter.ParameterCode)
		}
		if strings.ToUpper(strings.TrimSpace(argument.Direction)) != "IN" {
			return nil, nil, contractError("parameter %q is not an IN parameter", parameter.ParameterCode)
		}
		if !parameterControlCompatible(parameter.ControlType, parameter.LogicalType, parameter.Cardinality) {
			return nil, nil, contractError("parameter %q control type does not match its logical type and cardinality", parameter.ParameterCode)
		}
		definition := reporting.ParameterDefinition{
			Code: parameter.ParameterCode, ProcedureArgName: parameter.ProcedureArgName,
			Position: parameter.Position, Direction: parameter.Direction,
			LogicalType: parameter.LogicalType, OracleType: parameter.OracleType,
			Cardinality: parameter.Cardinality, Required: parameter.Required,
			Nullable: parameter.Nullable, SystemInjected: parameter.SystemInjected,
			Sensitive: parameter.Sensitive, DefaultValue: json.RawMessage(parameter.DefaultValueJSON),
			AllowedValues: json.RawMessage(parameter.AllowedValuesJSON),
			Validation:    json.RawMessage(parameter.ValidationJSON), Normalizer: json.RawMessage(parameter.NormalizerJSON),
			ValueSource: json.RawMessage(parameter.ValueSourceJSON), Timezone: parameter.Timezone,
			NullPolicy: parameter.NullPolicy, CollectionEncoding: parameter.CollectionEncoding,
		}
		if err := reporting.ValidateParameterPresentation(parameter.ControlType, definition); err != nil {
			return nil, nil, contractError("parameter presentation is invalid: %v", err)
		}
		definitions = append(definitions, definition)
		specs = append(specs, parameterSpec{
			Code: parameter.ParameterCode, Label: parameter.Label, DisplayOrder: parameter.DisplayOrder,
			ControlType: parameter.ControlType,
			LogicalType: parameter.LogicalType, Cardinality: parameter.Cardinality,
			ProcedureArgName: strings.ToUpper(strings.TrimSpace(parameter.ProcedureArgName)),
			Position:         parameter.Position, Direction: strings.ToUpper(strings.TrimSpace(parameter.Direction)),
			OracleType: normalizeOracleType(parameter.OracleType), Precision: parameter.PrecisionValue,
			Scale: parameter.ScaleValue, MaxLength: parameter.MaxLength, Required: parameter.Required,
			Nullable: parameter.Nullable, SystemInjected: parameter.SystemInjected, Sensitive: parameter.Sensitive,
			DefaultValue: canonicalJSON(parameter.DefaultValueJSON), AllowedValues: canonicalJSON(parameter.AllowedValuesJSON),
			Validation: canonicalJSON(parameter.ValidationJSON), Normalizer: canonicalJSON(parameter.NormalizerJSON),
			ValueSource: canonicalJSON(parameter.ValueSourceJSON), Timezone: parameter.Timezone,
			NullPolicy: parameter.NullPolicy, CollectionEncoding: parameter.CollectionEncoding,
			ErrorMessage: parameter.ErrorMessage,
		})
	}
	if err := reporting.ValidateParameterDefinitions(definitions); err != nil {
		return nil, nil, contractError("parameter schema is invalid: %v", err)
	}
	runParameterCount := 0
	for _, definition := range definitions {
		source, sourceErr := reporting.SystemValueSource(definition)
		if sourceErr != nil {
			return nil, nil, contractError("parameter schema is invalid: %v", sourceErr)
		}
		if source == reporting.ValueSourceRunID && definition.SystemInjected && definition.LogicalType == reporting.LogicalTypeString {
			if !characterOracleType(definition.OracleType) {
				return nil, nil, contractError("system-injected runId parameter must bind a character Oracle type")
			}
			runParameterCount++
		}
	}
	if runParameterCount != 1 {
		return nil, nil, contractError("exactly one system-injected string runId parameter is required")
	}
	return specs, definitions, nil
}

func compileColumns(
	version model.ReportVersion,
	columns []model.ReportColumn,
	actual []reportoracle.ResultColumn,
) ([]columnSpec, []reportoracle.ResultColumn, error) {
	if len(columns) == 0 || len(actual) == 0 {
		return nil, nil, contractError("result columns are required")
	}
	actualByName := make(map[string]reportoracle.ResultColumn, len(actual))
	actualPositions := make(map[int]struct{}, len(actual))
	for _, column := range actual {
		name := strings.ToUpper(column.Name)
		if _, exists := actualByName[name]; exists {
			return nil, nil, contractError("Oracle result schema has duplicate columns")
		}
		if column.Position <= 0 {
			return nil, nil, contractError("Oracle result schema has invalid positions")
		}
		if _, exists := actualPositions[column.Position]; exists {
			return nil, nil, contractError("Oracle result schema has duplicate positions")
		}
		actualByName[name] = column
		actualPositions[column.Position] = struct{}{}
	}
	for _, key := range []string{version.ResultRunIDColumn, version.ResultRowIDColumn} {
		if _, exists := actualByName[strings.ToUpper(strings.TrimSpace(key))]; !exists {
			return nil, nil, contractError("result key column %q does not exist", key)
		}
	}
	configured := append([]model.ReportColumn(nil), columns...)
	sort.Slice(configured, func(i, j int) bool {
		if configured[i].DisplayOrder == configured[j].DisplayOrder {
			return configured[i].LogicalCode < configured[j].LogicalCode
		}
		return configured[i].DisplayOrder < configured[j].DisplayOrder
	})
	logicalCodes := make(map[string]struct{}, len(configured))
	excelHeaders := make(map[string]struct{}, len(configured))
	specs := make([]columnSpec, 0, len(configured))
	exportableColumns := 0
	for _, column := range configured {
		actualColumn, exists := actualByName[strings.ToUpper(strings.TrimSpace(column.DatabaseColumn))]
		if !exists || !sameOracleType(column.SourceOracleType, actualColumn.DataType) ||
			!logicalOracleCompatible(column.ValueType, actualColumn.DataType) || !sameColumnShape(column, actualColumn) {
			return nil, nil, contractError("result column %q does not match Oracle result table", column.LogicalCode)
		}
		logicalKey := strings.ToUpper(strings.TrimSpace(column.LogicalCode))
		if logicalKey == "" {
			return nil, nil, contractError("result logical column is required")
		}
		if _, exists := logicalCodes[logicalKey]; exists {
			return nil, nil, contractError("result logical column %q is duplicated", column.LogicalCode)
		}
		logicalCodes[logicalKey] = struct{}{}
		exportable := column.ExportVisible && column.ExportAllowed
		if exportable {
			header := strings.TrimSpace(column.ExcelHeader)
			if header == "" {
				return nil, nil, contractError("export column %q requires an Excel header", column.LogicalCode)
			}
			if _, exists := excelHeaders[header]; exists {
				return nil, nil, contractError("Excel header %q is duplicated", header)
			}
			excelHeaders[header] = struct{}{}
		}
		if exportable {
			exportableColumns++
		}
		specs = append(specs, columnSpec{
			FieldID: column.FieldID, LogicalCode: column.LogicalCode,
			DatabaseColumn:   strings.ToUpper(strings.TrimSpace(column.DatabaseColumn)),
			SourceOracleType: normalizeOracleType(column.SourceOracleType), ValueType: column.ValueType,
			Precision: column.PrecisionValue, Scale: column.ScaleValue, Nullable: column.Nullable,
			PreviewHeader: column.PreviewHeader, ExcelHeader: column.ExcelHeader,
			DisplayOrder: column.DisplayOrder, ExportOrder: column.ExportOrder,
			PreviewVisible: column.PreviewVisible, ExportVisible: column.ExportVisible,
			Filterable: column.Filterable, Sortable: column.Sortable, ExportAllowed: column.ExportAllowed,
			AllowedOperators: canonicalJSON(column.AllowedOperatorsJSON), Format: canonicalJSON(column.FormatJSON),
			MaskingPolicy: canonicalJSON(column.MaskingPolicyJSON), Dictionary: canonicalJSON(column.DictionaryVersionJSON),
			ExcelWidth:  column.ExcelWidth,
			NullDisplay: column.NullDisplay,
		})
	}
	if exportableColumns == 0 {
		return nil, nil, contractError("at least one exportable result column is required")
	}
	normalized := append([]reportoracle.ResultColumn(nil), actual...)
	sort.Slice(normalized, func(i, j int) bool { return normalized[i].Position < normalized[j].Position })
	for index := range normalized {
		normalized[index].Name = strings.ToUpper(normalized[index].Name)
		normalized[index].DataType = normalizeOracleType(normalized[index].DataType)
	}
	return specs, normalized, nil
}

func compileGrants(grants []model.ReportGrant) ([]grantSpec, error) {
	result := make([]grantSpec, 0, len(grants))
	seen := make(map[string]struct{}, len(grants))
	for _, grant := range grants {
		subjectType := strings.ToUpper(strings.TrimSpace(grant.SubjectType))
		if (subjectType != "USER" && subjectType != "ROLE") || grant.SubjectID == 0 {
			return nil, contractError("report grant is invalid")
		}
		var actions []string
		if err := json.Unmarshal([]byte(grant.ActionsJSON), &actions); err != nil || len(actions) == 0 {
			return nil, contractError("report grant actions are invalid")
		}
		actionSet := make(map[string]struct{}, len(actions))
		normalizedActions := make([]string, 0, len(actions))
		for _, action := range actions {
			action = strings.ToUpper(strings.TrimSpace(action))
			if !allowedReportAction(action) {
				return nil, contractError("report grant action %q is invalid", action)
			}
			if _, exists := actionSet[action]; exists {
				continue
			}
			actionSet[action] = struct{}{}
			normalizedActions = append(normalizedActions, action)
		}
		sort.Strings(normalizedActions)
		actionsJSON, err := json.Marshal(normalizedActions)
		if err != nil {
			return nil, fmt.Errorf("encode report grant actions: %w", err)
		}
		key := fmt.Sprintf("%s:%d", subjectType, grant.SubjectID)
		if _, exists := seen[key]; exists {
			return nil, contractError("report grant %q is duplicated", key)
		}
		seen[key] = struct{}{}
		result = append(result, grantSpec{SubjectType: subjectType, SubjectID: grant.SubjectID, Actions: actionsJSON})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].SubjectType == result[j].SubjectType {
			return result[i].SubjectID < result[j].SubjectID
		}
		return result[i].SubjectType < result[j].SubjectType
	})
	return result, nil
}

func validateConfiguredTemplate(template, canonical string, definitions []reporting.ParameterDefinition) error {
	compiled, err := reporting.CompileCallTemplate(template, definitions)
	if err != nil {
		return contractError("configured call template is invalid: %v", err)
	}
	if len(compiled.Slots) != len(definitions) {
		return contractError("configured call template does not bind every parameter")
	}
	if normalizePLSQL(compiled.Statement) != normalizePLSQL(canonical) {
		return contractError("configured call template differs from the canonical procedure call")
	}
	return nil
}

func normalizeProcedureArguments(arguments []reportoracle.ProcedureArgument) []reportoracle.ProcedureArgument {
	result := append([]reportoracle.ProcedureArgument(nil), arguments...)
	sort.Slice(result, func(i, j int) bool { return result[i].Sequence < result[j].Sequence })
	for index := range result {
		result[index].Name = strings.ToUpper(result[index].Name)
		result[index].Direction = strings.ToUpper(result[index].Direction)
		result[index].DataType = normalizeOracleType(result[index].DataType)
		result[index].TypeOwner = strings.ToUpper(result[index].TypeOwner)
		result[index].TypeName = strings.ToUpper(result[index].TypeName)
	}
	return result
}

func validateProcedureSequences(arguments []reportoracle.ProcedureArgument) error {
	seen := make(map[int]struct{}, len(arguments))
	for _, argument := range arguments {
		if argument.Sequence <= 0 {
			return contractError("Oracle procedure signature has invalid sequences")
		}
		if _, exists := seen[argument.Sequence]; exists {
			return contractError("Oracle procedure signature has duplicate sequences")
		}
		seen[argument.Sequence] = struct{}{}
	}
	return nil
}

func hashJSON(value interface{}) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("hash report publication contract: %w", err)
	}
	return hashBytes(encoded), nil
}

func hashBytes(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func canonicalJSON(value model.JSONText) json.RawMessage {
	if strings.TrimSpace(string(value)) == "" {
		return nil
	}
	var decoded interface{}
	decoder := json.NewDecoder(bytes.NewReader([]byte(value)))
	decoder.UseNumber()
	if err := decoder.Decode(&decoded); err != nil {
		return append(json.RawMessage(nil), []byte(value)...)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return append(json.RawMessage(nil), []byte(value)...)
	}
	encoded, err := json.Marshal(decoded)
	if err != nil {
		return append(json.RawMessage(nil), []byte(value)...)
	}
	return encoded
}

func sameIdentifier(left, right string) bool {
	return strings.EqualFold(strings.TrimSpace(left), strings.TrimSpace(right))
}

func sameOracleType(left, right string) bool {
	return normalizeOracleType(left) == normalizeOracleType(right)
}

func normalizeOracleType(value string) string {
	return strings.ToUpper(strings.Join(strings.Fields(value), " "))
}

func sameParameterShape(configured model.ReportParameter, actual reportoracle.ProcedureArgument) bool {
	if configured.MaxLength != nil && (actual.DataLength == nil || int64(*configured.MaxLength) != *actual.DataLength) {
		return false
	}
	if configured.PrecisionValue != nil &&
		(actual.DataPrecision == nil || int64(*configured.PrecisionValue) != *actual.DataPrecision) {
		return false
	}
	return configured.ScaleValue == nil || actual.DataScale != nil && int64(*configured.ScaleValue) == *actual.DataScale
}

func sameColumnShape(configured model.ReportColumn, actual reportoracle.ResultColumn) bool {
	if configured.Nullable != actual.Nullable {
		return false
	}
	if configured.PrecisionValue != nil &&
		(actual.DataPrecision == nil || int64(*configured.PrecisionValue) != *actual.DataPrecision) {
		return false
	}
	return configured.ScaleValue == nil || actual.DataScale != nil && int64(*configured.ScaleValue) == *actual.DataScale
}

func allowedReportAction(action string) bool {
	switch action {
	case "QUERY", "EXPORT":
		return true
	default:
		return false
	}
}

func normalizePLSQL(value string) string {
	return strings.ToUpper(strings.Join(strings.Fields(value), " "))
}

func logicalOracleCompatible(logicalType, oracleType string) bool {
	logicalType = strings.ToLower(strings.TrimSpace(logicalType))
	oracleType = normalizeOracleType(oracleType)
	switch logicalType {
	case reporting.LogicalTypeString, reporting.LogicalTypeEnum:
		return characterOracleType(oracleType) || oracleType == "CLOB" || oracleType == "NCLOB"
	case reporting.LogicalTypeInteger, reporting.LogicalTypeDecimal:
		return oracleType == "NUMBER" || oracleType == "BINARY_FLOAT" || oracleType == "BINARY_DOUBLE"
	case reporting.LogicalTypeBoolean:
		return oracleType == "BOOLEAN" || oracleType == "NUMBER" || characterOracleType(oracleType)
	case reporting.LogicalTypeDate:
		return oracleType == "DATE"
	case reporting.LogicalTypeDateTime:
		return oracleType == "DATE" || strings.HasPrefix(oracleType, "TIMESTAMP")
	case reporting.LogicalTypeMultiEnum, reporting.LogicalTypeJSON:
		return oracleType == "CLOB" || oracleType == "NCLOB"
	default:
		return false
	}
}

func parameterControlCompatible(controlType, logicalType, cardinality string) bool {
	controlType = strings.ToUpper(strings.TrimSpace(controlType))
	logicalType = strings.ToLower(strings.TrimSpace(logicalType))
	cardinality = strings.ToUpper(strings.TrimSpace(cardinality))
	if logicalType == reporting.LogicalTypeMultiEnum {
		return controlType == "MULTI_SELECT" && cardinality == reporting.CardinalityMultiple
	}
	if cardinality != reporting.CardinalitySingle {
		return false
	}
	switch logicalType {
	case reporting.LogicalTypeBoolean:
		return controlType == "CHECKBOX"
	case reporting.LogicalTypeDate:
		return controlType == "DATE"
	case reporting.LogicalTypeDateTime:
		return controlType == "DATETIME"
	case reporting.LogicalTypeInteger, reporting.LogicalTypeDecimal:
		return controlType == "NUMBER"
	case reporting.LogicalTypeEnum:
		return controlType == "SELECT"
	case reporting.LogicalTypeJSON:
		return controlType == "TEXTAREA"
	case reporting.LogicalTypeString:
		return controlType == "TEXT" || controlType == "TEXTAREA" || controlType == "HIDDEN"
	default:
		return false
	}
}

func characterOracleType(oracleType string) bool {
	switch normalizeOracleType(oracleType) {
	case "CHAR", "NCHAR", "VARCHAR2", "NVARCHAR2":
		return true
	default:
		return false
	}
}

func contractError(format string, arguments ...interface{}) error {
	return fmt.Errorf("%w: %s", ErrInvalidContract, fmt.Sprintf(format, arguments...))
}
