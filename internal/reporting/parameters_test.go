package reporting

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestCompileCallTemplate(t *testing.T) {
	definitions := []ParameterDefinition{
		{Code: "runId", ProcedureArgName: "P_RUN_ID", Position: 1, LogicalType: LogicalTypeString},
		{Code: "orgCodes", ProcedureArgName: "P_ORG_CODES", Position: 2, LogicalType: LogicalTypeMultiEnum, Cardinality: CardinalityMultiple, CollectionEncoding: CollectionEncodingJSONCLOB},
	}

	compiled, err := CompileCallTemplate(
		"BEGIN REPORT_PKG.RUN(P_RUN_ID => {{runId}}, P_ORG_CODES => {{orgCodes}}, P_AGAIN => {{runId}}); END;",
		definitions,
	)
	if err != nil {
		t.Fatalf("CompileCallTemplate() error = %v", err)
	}
	if compiled.Statement != "BEGIN REPORT_PKG.RUN(P_RUN_ID => :p1, P_ORG_CODES => :p2, P_AGAIN => :p1); END;" {
		t.Fatalf("Statement = %q", compiled.Statement)
	}
	if len(compiled.Slots) != 2 || compiled.Slots[0].Code != "runId" || compiled.Slots[1].Code != "orgCodes" {
		t.Fatalf("Slots = %#v", compiled.Slots)
	}
}

func TestCompileCallTemplateRejectsInvalidContracts(t *testing.T) {
	tests := []struct {
		name        string
		template    string
		definitions []ParameterDefinition
	}{
		{
			name:     "undefined placeholder",
			template: "BEGIN P({{missing}}); END;",
			definitions: []ParameterDefinition{
				{Code: "known", ProcedureArgName: "P_KNOWN", Position: 1, LogicalType: LogicalTypeString},
			},
		},
		{
			name:     "unused definition",
			template: "BEGIN P({{known}}); END;",
			definitions: []ParameterDefinition{
				{Code: "known", ProcedureArgName: "P_KNOWN", Position: 1, LogicalType: LogicalTypeString},
				{Code: "extra", ProcedureArgName: "P_EXTRA", Position: 2, LogicalType: LogicalTypeString},
			},
		},
		{
			name:     "malformed placeholder",
			template: "BEGIN P({{not-valid}}); END;",
			definitions: []ParameterDefinition{
				{Code: "known", ProcedureArgName: "P_KNOWN", Position: 1, LogicalType: LogicalTypeString},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := CompileCallTemplate(test.template, test.definitions)
			if !errors.Is(err, ErrInvalidParameterContract) {
				t.Fatalf("error = %v, want ErrInvalidParameterContract", err)
			}
		})
	}
}

func TestValidateParameterDefinitionsRejectsUnusableEnumContracts(t *testing.T) {
	tests := []struct {
		name       string
		definition ParameterDefinition
	}{
		{name: "enum without allowed values", definition: ParameterDefinition{Code: "status", ProcedureArgName: "P_STATUS", Position: 1, LogicalType: LogicalTypeEnum, Cardinality: CardinalitySingle}},
		{name: "multi enum with single cardinality", definition: ParameterDefinition{Code: "orgs", ProcedureArgName: "P_ORGS", Position: 1, LogicalType: LogicalTypeMultiEnum, Cardinality: CardinalitySingle, AllowedValues: json.RawMessage(`["A"]`)}},
		{name: "string with multiple cardinality", definition: ParameterDefinition{Code: "name", ProcedureArgName: "P_NAME", Position: 1, LogicalType: LogicalTypeString, Cardinality: CardinalityMultiple, CollectionEncoding: CollectionEncodingJSONCLOB}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := ValidateParameterDefinitions([]ParameterDefinition{test.definition}); !errors.Is(err, ErrInvalidParameterContract) {
				t.Fatalf("ValidateParameterDefinitions() error = %v, want ErrInvalidParameterContract", err)
			}
		})
	}
}

func TestValidateParameterPresentation(t *testing.T) {
	tests := []struct {
		name        string
		controlType string
		definition  ParameterDefinition
		wantError   bool
	}{
		{name: "string text", controlType: "TEXT", definition: ParameterDefinition{Code: "name", LogicalType: LogicalTypeString, Cardinality: CardinalitySingle}},
		{name: "system string hidden", controlType: "hidden", definition: ParameterDefinition{Code: "runId", LogicalType: LogicalTypeString, Cardinality: CardinalitySingle, SystemInjected: true}},
		{name: "multi enum", controlType: "MULTI_SELECT", definition: ParameterDefinition{Code: "stores", LogicalType: LogicalTypeMultiEnum, Cardinality: CardinalityMultiple, CollectionEncoding: CollectionEncodingJSONCLOB}},
		{name: "wrong boolean control", controlType: "TEXT", definition: ParameterDefinition{Code: "enabled", LogicalType: LogicalTypeBoolean, Cardinality: CardinalitySingle}, wantError: true},
		{name: "multiple scalar", controlType: "TEXT", definition: ParameterDefinition{Code: "name", LogicalType: LogicalTypeString, Cardinality: CardinalityMultiple, CollectionEncoding: CollectionEncodingJSONCLOB}, wantError: true},
		{name: "scalar collection encoding", controlType: "DATE", definition: ParameterDefinition{Code: "from", LogicalType: LogicalTypeDate, Cardinality: CardinalitySingle, CollectionEncoding: CollectionEncodingJSONCLOB}, wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateParameterPresentation(test.controlType, test.definition)
			if test.wantError && !errors.Is(err, ErrInvalidParameterContract) {
				t.Fatalf("ValidateParameterPresentation() error = %v, want ErrInvalidParameterContract", err)
			}
			if !test.wantError && err != nil {
				t.Fatalf("ValidateParameterPresentation() error = %v", err)
			}
		})
	}
}

func TestNormalizeParameters(t *testing.T) {
	definitions := []ParameterDefinition{
		{
			Code: "runId", ProcedureArgName: "P_RUN_ID", Position: 1,
			LogicalType: LogicalTypeString, Required: true, SystemInjected: true,
		},
		{
			Code: "amount", ProcedureArgName: "P_AMOUNT", Position: 2,
			LogicalType: LogicalTypeDecimal, Required: true,
			Validation: json.RawMessage(`{"min":0,"max":1000}`),
		},
		{
			Code: "status", ProcedureArgName: "P_STATUS", Position: 3,
			LogicalType: LogicalTypeEnum, Required: true,
			AllowedValues: json.RawMessage(`["PAID","CLOSED"]`),
		},
		{
			Code: "orgCodes", ProcedureArgName: "P_ORG_CODES", Position: 4,
			LogicalType: LogicalTypeMultiEnum, Cardinality: CardinalityMultiple,
			CollectionEncoding: CollectionEncodingJSONCLOB,
			AllowedValues:      json.RawMessage(`["A","B","C"]`),
			Validation:         json.RawMessage(`{"minItems":1,"maxItems":3}`),
		},
		{
			Code: "secret", ProcedureArgName: "P_SECRET", Position: 5,
			LogicalType: LogicalTypeString, Required: true, Sensitive: true,
		},
		{
			Code: "requestedAt", ProcedureArgName: "P_REQUESTED_AT", Position: 6,
			LogicalType: LogicalTypeDateTime, Required: true,
		},
	}

	normalized, err := NormalizeParameters(definitions, map[string]json.RawMessage{
		"amount":      json.RawMessage(`"0012.3400"`),
		"status":      json.RawMessage(`"PAID"`),
		"orgCodes":    json.RawMessage(`["A","B","A"]`),
		"secret":      json.RawMessage(`"hidden"`),
		"requestedAt": json.RawMessage(`"2026-08-12T09:30:00+08:00"`),
	}, map[string]interface{}{
		"runId": "run-1",
	})
	if err != nil {
		t.Fatalf("NormalizeParameters() error = %v", err)
	}
	if normalized.DatabaseValues["amount"] != "12.34" {
		t.Fatalf("amount = %#v", normalized.DatabaseValues["amount"])
	}
	if normalized.DatabaseValues["orgCodes"] != `["A","B"]` {
		t.Fatalf("orgCodes = %#v", normalized.DatabaseValues["orgCodes"])
	}
	requestedAt, ok := normalized.DatabaseValues["requestedAt"].(time.Time)
	if !ok || requestedAt.Format(time.RFC3339) != "2026-08-12T09:30:00+08:00" {
		t.Fatalf("requestedAt = %#v", normalized.DatabaseValues["requestedAt"])
	}
	if strings.Contains(string(normalized.PublicJSON), "hidden") {
		t.Fatalf("PublicJSON leaked sensitive value: %s", normalized.PublicJSON)
	}
	if !strings.Contains(string(normalized.SensitiveJSON), "hidden") {
		t.Fatalf("SensitiveJSON = %s", normalized.SensitiveJSON)
	}
	if len(normalized.Fingerprint) != 64 {
		t.Fatalf("Fingerprint = %q", normalized.Fingerprint)
	}
}

func TestNormalizeParametersFingerprintExcludesSystemValues(t *testing.T) {
	definitions := []ParameterDefinition{
		{Code: "runId", ProcedureArgName: "P_RUN_ID", Position: 1, LogicalType: LogicalTypeString, Required: true, SystemInjected: true},
		{Code: "store", ProcedureArgName: "P_STORE", Position: 2, LogicalType: LogicalTypeString, Required: true},
	}
	values := map[string]json.RawMessage{"store": json.RawMessage(`"S001"`)}
	first, err := NormalizeParameters(definitions, values, map[string]interface{}{"runId": "run-1"})
	if err != nil {
		t.Fatalf("first NormalizeParameters() error = %v", err)
	}
	second, err := NormalizeParameters(definitions, values, map[string]interface{}{"runId": "run-2"})
	if err != nil {
		t.Fatalf("second NormalizeParameters() error = %v", err)
	}
	if first.Fingerprint != second.Fingerprint {
		t.Fatalf("business fingerprint changed with run id: %q != %q", first.Fingerprint, second.Fingerprint)
	}
	if first.DatabaseValues["runId"] == second.DatabaseValues["runId"] {
		t.Fatalf("database run ids were not independently normalized: %#v %#v", first.DatabaseValues, second.DatabaseValues)
	}
}

func TestNormalizeParametersAppliesBoundedStringNormalizer(t *testing.T) {
	definitions := []ParameterDefinition{{
		Code: "store", ProcedureArgName: "P_STORE", Position: 1, LogicalType: LogicalTypeEnum, Required: true,
		AllowedValues: json.RawMessage(`["S001"]`), Normalizer: json.RawMessage(`{"trim":true,"case":"UPPER"}`),
	}}
	normalized, err := NormalizeParameters(definitions, map[string]json.RawMessage{"store": json.RawMessage(`" s001 "`)}, nil)
	if err != nil {
		t.Fatalf("NormalizeParameters() error = %v", err)
	}
	if normalized.DatabaseValues["store"] != "S001" {
		t.Fatalf("normalized store = %#v", normalized.DatabaseValues["store"])
	}
	for _, invalid := range []json.RawMessage{json.RawMessage(`{"unknown":true}`), json.RawMessage(`{"case":"TITLE"}`)} {
		definitions[0].Normalizer = invalid
		if err := ValidateParameterDefinitions(definitions); !errors.Is(err, ErrInvalidParameterContract) {
			t.Fatalf("normalizer %s error = %v", invalid, err)
		}
	}
}

func TestSystemValueSourceUsesExplicitWhitelist(t *testing.T) {
	definition := ParameterDefinition{Code: "actorId", ProcedureArgName: "P_ACTOR_ID", Position: 1, LogicalType: LogicalTypeInteger, SystemInjected: true, ValueSource: json.RawMessage(`{"source":"ACTOR_ID"}`)}
	if source, err := SystemValueSource(definition); err != nil || source != ValueSourceActorID {
		t.Fatalf("SystemValueSource() = %q, %v", source, err)
	}
	definition.ValueSource = json.RawMessage(`{"source":"REQUEST_HEADER"}`)
	if _, err := SystemValueSource(definition); !errors.Is(err, ErrInvalidParameterContract) {
		t.Fatalf("unsupported SystemValueSource() error = %v", err)
	}
	definition.LogicalType = LogicalTypeString
	definition.ValueSource = json.RawMessage(`{"source":"ACTOR_ID"}`)
	if _, err := SystemValueSource(definition); !errors.Is(err, ErrInvalidParameterContract) {
		t.Fatalf("incompatible SystemValueSource() error = %v", err)
	}
	definition.LogicalType = LogicalTypeInteger
	definition.ValueSource = json.RawMessage(`{"source":"RUN_ID"}`)
	if _, err := SystemValueSource(definition); !errors.Is(err, ErrInvalidParameterContract) {
		t.Fatalf("incompatible run id SystemValueSource() error = %v", err)
	}
}

func TestValidateParameterDefinitionsRejectsSensitiveSystemParameter(t *testing.T) {
	definitions := []ParameterDefinition{{
		Code: "runId", ProcedureArgName: "P_RUN_ID", Position: 1, LogicalType: LogicalTypeString,
		SystemInjected: true, Sensitive: true,
	}}
	if err := ValidateParameterDefinitions(definitions); !errors.Is(err, ErrInvalidParameterContract) {
		t.Fatalf("ValidateParameterDefinitions() error = %v", err)
	}
}

func TestNormalizeParametersRejectsClientSystemParameterAndUnknownParameter(t *testing.T) {
	definitions := []ParameterDefinition{
		{Code: "runId", ProcedureArgName: "P_RUN_ID", Position: 1, LogicalType: LogicalTypeString, SystemInjected: true},
	}

	tests := []struct {
		name   string
		values map[string]json.RawMessage
	}{
		{name: "system override", values: map[string]json.RawMessage{"runId": json.RawMessage(`"forged"`)}},
		{name: "unknown", values: map[string]json.RawMessage{"where": json.RawMessage(`"1=1"`)}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NormalizeParameters(definitions, test.values, map[string]interface{}{"runId": "run-1"})
			if !errors.Is(err, ErrInvalidParameterInput) {
				t.Fatalf("error = %v, want ErrInvalidParameterInput", err)
			}
		})
	}
}

func TestNormalizeParametersRejectsMissingAndDisallowedValues(t *testing.T) {
	tests := []struct {
		name       string
		definition ParameterDefinition
		value      json.RawMessage
	}{
		{
			name: "missing required",
			definition: ParameterDefinition{Code: "name", ProcedureArgName: "P_NAME", Position: 1,
				LogicalType: LogicalTypeString, Required: true},
		},
		{
			name: "enum outside allowlist",
			definition: ParameterDefinition{Code: "status", ProcedureArgName: "P_STATUS", Position: 1,
				LogicalType: LogicalTypeEnum, AllowedValues: json.RawMessage(`["PAID"]`)},
			value: json.RawMessage(`"DROPPED"`),
		},
		{
			name: "number outside range",
			definition: ParameterDefinition{Code: "count", ProcedureArgName: "P_COUNT", Position: 1,
				LogicalType: LogicalTypeInteger, Validation: json.RawMessage(`{"min":1,"max":10}`)},
			value: json.RawMessage(`11`),
		},
		{
			name: "string outside allowlist",
			definition: ParameterDefinition{Code: "store", ProcedureArgName: "P_STORE", Position: 1,
				LogicalType: LogicalTypeString, AllowedValues: json.RawMessage(`["S001","S002"]`)},
			value: json.RawMessage(`"S003"`),
		},
		{
			name: "integer outside allowlist",
			definition: ParameterDefinition{Code: "level", ProcedureArgName: "P_LEVEL", Position: 1,
				LogicalType: LogicalTypeInteger, AllowedValues: json.RawMessage(`["1","2"]`)},
			value: json.RawMessage(`3`),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			values := map[string]json.RawMessage{}
			if len(test.value) > 0 {
				values[test.definition.Code] = test.value
			}
			_, err := NormalizeParameters([]ParameterDefinition{test.definition}, values, nil)
			if !errors.Is(err, ErrInvalidParameterInput) {
				t.Fatalf("error = %v, want ErrInvalidParameterInput", err)
			}
		})
	}
}

func TestNormalizeParametersAppliesTypedAllowedValues(t *testing.T) {
	definitions := []ParameterDefinition{
		{Code: "store", ProcedureArgName: "P_STORE", Position: 1, LogicalType: LogicalTypeString, AllowedValues: json.RawMessage(`["S001"]`), Normalizer: json.RawMessage(`{"trim":true,"case":"UPPER"}`)},
		{Code: "level", ProcedureArgName: "P_LEVEL", Position: 2, LogicalType: LogicalTypeInteger, AllowedValues: json.RawMessage(`["01","2"]`)},
		{Code: "enabled", ProcedureArgName: "P_ENABLED", Position: 3, LogicalType: LogicalTypeBoolean, AllowedValues: json.RawMessage(`["true"]`)},
		{Code: "day", ProcedureArgName: "P_DAY", Position: 4, LogicalType: LogicalTypeDate, AllowedValues: json.RawMessage(`["2026-08-13"]`)},
	}
	values := map[string]json.RawMessage{
		"store": json.RawMessage(`" s001 "`), "level": json.RawMessage(`1`),
		"enabled": json.RawMessage(`true`), "day": json.RawMessage(`"2026-08-13"`),
	}
	if _, err := NormalizeParameters(definitions, values, nil); err != nil {
		t.Fatalf("NormalizeParameters() error = %v", err)
	}
}

func TestValidateParameterDefinitionsRejectsInvalidTypedAllowedValues(t *testing.T) {
	definitions := []ParameterDefinition{{
		Code: "level", ProcedureArgName: "P_LEVEL", Position: 1,
		LogicalType: LogicalTypeInteger, AllowedValues: json.RawMessage(`["one"]`),
	}}
	if err := ValidateParameterDefinitions(definitions); !errors.Is(err, ErrInvalidParameterContract) {
		t.Fatalf("ValidateParameterDefinitions() error = %v", err)
	}
}

func TestNormalizeParametersAcceptsExactInt64String(t *testing.T) {
	definitions := []ParameterDefinition{{
		Code: "amount", ProcedureArgName: "P_AMOUNT", Position: 1,
		LogicalType: LogicalTypeInteger, Cardinality: CardinalitySingle,
		Required: true, NullPolicy: NullPolicyTypedNull,
	}}
	normalized, err := NormalizeParameters(definitions, map[string]json.RawMessage{"amount": json.RawMessage(`"9223372036854775807"`)}, nil)
	if err != nil {
		t.Fatalf("NormalizeParameters() error = %v", err)
	}
	if normalized.DatabaseValues["amount"] != int64(9223372036854775807) {
		t.Fatalf("database integer = %#v", normalized.DatabaseValues["amount"])
	}
}

func TestNormalizeParametersRejectsTrailingJSON(t *testing.T) {
	definition := ParameterDefinition{
		Code: "count", ProcedureArgName: "P_COUNT", Position: 1,
		LogicalType: LogicalTypeInteger,
	}
	_, err := NormalizeParameters([]ParameterDefinition{definition}, map[string]json.RawMessage{
		"count": json.RawMessage(`1 2`),
	}, nil)
	if !errors.Is(err, ErrInvalidParameterInput) {
		t.Fatalf("error = %v, want ErrInvalidParameterInput", err)
	}
}

func TestNormalizeParametersComparesHighPrecisionDecimalExactly(t *testing.T) {
	definition := ParameterDefinition{
		Code: "amount", ProcedureArgName: "P_AMOUNT", Position: 1,
		LogicalType: LogicalTypeDecimal,
		Validation:  json.RawMessage(`{"min":99999999999999999999.99999999999999999998,"max":99999999999999999999.99999999999999999999}`),
	}

	normalized, err := NormalizeParameters([]ParameterDefinition{definition}, map[string]json.RawMessage{
		"amount": json.RawMessage(`"99999999999999999999.999999999999999999985"`),
	}, nil)
	if err != nil {
		t.Fatalf("NormalizeParameters() error = %v", err)
	}
	if normalized.DatabaseValues["amount"] != "99999999999999999999.999999999999999999985" {
		t.Fatalf("amount = %#v", normalized.DatabaseValues["amount"])
	}

	_, err = NormalizeParameters([]ParameterDefinition{definition}, map[string]json.RawMessage{
		"amount": json.RawMessage(`"100000000000000000000"`),
	}, nil)
	if !errors.Is(err, ErrInvalidParameterInput) {
		t.Fatalf("error = %v, want ErrInvalidParameterInput", err)
	}
}

func TestNormalizeParametersRejectsValidationRulesWithTrailingJSON(t *testing.T) {
	definition := ParameterDefinition{
		Code: "count", ProcedureArgName: "P_COUNT", Position: 1,
		LogicalType: LogicalTypeInteger,
		Validation:  json.RawMessage(`{"min":1}{"max":2}`),
	}
	_, err := NormalizeParameters([]ParameterDefinition{definition}, map[string]json.RawMessage{
		"count": json.RawMessage(`1`),
	}, nil)
	if !errors.Is(err, ErrInvalidParameterContract) {
		t.Fatalf("error = %v, want ErrInvalidParameterContract", err)
	}
}
