package reporting

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	LogicalTypeString    = "string"
	LogicalTypeInteger   = "integer"
	LogicalTypeDecimal   = "decimal"
	LogicalTypeBoolean   = "boolean"
	LogicalTypeDate      = "date"
	LogicalTypeDateTime  = "datetime"
	LogicalTypeEnum      = "enum"
	LogicalTypeMultiEnum = "multi_enum"
	LogicalTypeJSON      = "json"

	CardinalitySingle   = "SINGLE"
	CardinalityMultiple = "MULTIPLE"
	CardinalityRange    = "RANGE"

	CollectionEncodingJSONCLOB = "JSON_CLOB"
	NullPolicyTypedNull        = "TYPED_NULL"
)

// ValidateParameterDefinitions compiles the complete parameter schema,
// including defaults and validation rules, without requiring runtime input.
func ValidateParameterDefinitions(definitions []ParameterDefinition) error {
	if _, err := indexDefinitions(definitions); err != nil {
		return err
	}
	for _, definition := range definitions {
		if definition.Cardinality != "" && definition.Cardinality != CardinalitySingle &&
			definition.Cardinality != CardinalityMultiple {
			return contractError("parameter %q has unsupported cardinality %q", definition.Code, definition.Cardinality)
		}
		if definition.NullPolicy != "" && definition.NullPolicy != NullPolicyTypedNull {
			return contractError("parameter %q has unsupported null policy %q", definition.Code, definition.NullPolicy)
		}
		if definition.Sensitive && len(bytes.TrimSpace(definition.DefaultValue)) > 0 {
			return contractError("sensitive parameter %q cannot define a plaintext default", definition.Code)
		}
		if definition.SystemInjected && definition.Sensitive {
			return contractError("system parameter %q cannot be sensitive", definition.Code)
		}
		if definition.SystemInjected && len(bytes.TrimSpace(definition.Normalizer)) > 0 {
			return contractError("system parameter %q cannot define a normalizer", definition.Code)
		}
		if _, err := parseNormalizerRules(definition); err != nil {
			return err
		}
		if _, err := SystemValueSource(definition); err != nil {
			return err
		}
		if definition.LogicalType == LogicalTypeMultiEnum {
			if definition.Cardinality != CardinalityMultiple || definition.CollectionEncoding != CollectionEncodingJSONCLOB {
				return contractError("multi enum parameter %q must use MULTIPLE JSON_CLOB encoding", definition.Code)
			}
		} else if definition.Cardinality != "" && definition.Cardinality != CardinalitySingle {
			return contractError("parameter %q must use SINGLE cardinality", definition.Code)
		}
		if _, err := parseValidationRules(definition); err != nil {
			return err
		}
		if len(bytes.TrimSpace(definition.AllowedValues)) > 0 {
			var allowed []string
			if err := decodeStrictJSON(definition.AllowedValues, &allowed); err != nil || len(allowed) == 0 {
				return contractError("parameter %q has invalid allowed values", definition.Code)
			}
			if definition.LogicalType == LogicalTypeJSON {
				return contractError("JSON parameter %q cannot define allowed values", definition.Code)
			}
			for _, value := range allowed {
				if _, err := canonicalAllowedValue(definition, value); err != nil {
					return contractError("parameter %q has invalid allowed value %q", definition.Code, value)
				}
			}
		} else if definition.LogicalType == LogicalTypeEnum || definition.LogicalType == LogicalTypeMultiEnum {
			return contractError("enum parameter %q requires allowed values", definition.Code)
		}
		if len(bytes.TrimSpace(definition.DefaultValue)) > 0 {
			if _, _, err := normalizeValue(definition, definition.DefaultValue, true); err != nil {
				return contractError("parameter %q has invalid default value: %v", definition.Code, err)
			}
		}
	}
	return nil
}

// ValidateParameterPresentation keeps the configured input control aligned with
// the value shape accepted by the parameter normalization contract.
func ValidateParameterPresentation(controlType string, definition ParameterDefinition) error {
	controlType = strings.ToUpper(strings.TrimSpace(controlType))
	logicalType := strings.ToLower(strings.TrimSpace(definition.LogicalType))
	cardinality := strings.ToUpper(strings.TrimSpace(definition.Cardinality))
	collectionEncoding := strings.ToUpper(strings.TrimSpace(definition.CollectionEncoding))

	if logicalType == LogicalTypeMultiEnum {
		if controlType != "MULTI_SELECT" || cardinality != CardinalityMultiple ||
			collectionEncoding != CollectionEncodingJSONCLOB {
			return contractError("parameter %q must use MULTI_SELECT with MULTIPLE JSON_CLOB encoding", definition.Code)
		}
		return nil
	}
	if cardinality != CardinalitySingle || collectionEncoding != "" {
		return contractError("parameter %q must use SINGLE cardinality without collection encoding", definition.Code)
	}

	compatible := false
	switch logicalType {
	case LogicalTypeString:
		compatible = controlType == "TEXT" || controlType == "TEXTAREA" ||
			definition.SystemInjected && controlType == "HIDDEN"
	case LogicalTypeInteger, LogicalTypeDecimal:
		compatible = controlType == "NUMBER"
	case LogicalTypeBoolean:
		compatible = controlType == "CHECKBOX"
	case LogicalTypeDate:
		compatible = controlType == "DATE"
	case LogicalTypeDateTime:
		compatible = controlType == "DATETIME"
	case LogicalTypeEnum:
		compatible = controlType == "SELECT"
	case LogicalTypeJSON:
		compatible = controlType == "TEXTAREA"
	}
	if !compatible {
		return contractError("parameter %q control %q does not match logical type %q", definition.Code, controlType, logicalType)
	}
	return nil
}

func decodeStrictJSON(raw []byte, target interface{}) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	return requireJSONEOF(decoder)
}

var (
	parameterCodePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)
	placeholderPattern   = regexp.MustCompile(`\{\{([A-Za-z][A-Za-z0-9_]*)\}\}`)
	decimalPattern       = regexp.MustCompile(`^-?[0-9]+(?:\.([0-9]+))?$`)

	ErrInvalidParameterContract = errors.New("invalid report parameter contract")
	ErrInvalidParameterInput    = errors.New("invalid report parameter input")
)

type ParameterDefinition struct {
	Code               string
	ProcedureArgName   string
	Position           int
	Direction          string
	LogicalType        string
	OracleType         string
	Cardinality        string
	Required           bool
	Nullable           bool
	SystemInjected     bool
	Sensitive          bool
	DefaultValue       json.RawMessage
	AllowedValues      json.RawMessage
	Validation         json.RawMessage
	Normalizer         json.RawMessage
	ValueSource        json.RawMessage
	Timezone           string
	NullPolicy         string
	CollectionEncoding string
}

type ValidationRules struct {
	MinLength *int         `json:"minLength"`
	MaxLength *int         `json:"maxLength"`
	Pattern   string       `json:"pattern"`
	Min       *json.Number `json:"min"`
	Max       *json.Number `json:"max"`
	MinItems  *int         `json:"minItems"`
	MaxItems  *int         `json:"maxItems"`
}

type normalizerRules struct {
	Trim bool   `json:"trim"`
	Case string `json:"case"`
}

type valueSourceRules struct {
	Source string `json:"source"`
}

const (
	ValueSourceRunID   = "RUN_ID"
	ValueSourceActorID = "ACTOR_ID"
)

type BindSlot struct {
	Code             string
	BindName         string
	ProcedureArgName string
	Position         int
}

type CompiledCall struct {
	Statement string
	Slots     []BindSlot
}

// NormalizedParameters deliberately has no public JSON fields. Services must
// explicitly choose PublicJSON for MySQL and encrypt SensitiveJSON before use.
type NormalizedParameters struct {
	DatabaseValues map[string]interface{} `json:"-"`
	PublicJSON     []byte                 `json:"-"`
	SensitiveJSON  []byte                 `json:"-"`
	Fingerprint    string                 `json:"-"`
}

func CompileCallTemplate(template string, definitions []ParameterDefinition) (CompiledCall, error) {
	trimmed := strings.TrimSpace(template)
	if trimmed == "" {
		return CompiledCall{}, contractError("call template is required")
	}
	if strings.Contains(placeholderPattern.ReplaceAllString(trimmed, ""), "{{") ||
		strings.Contains(placeholderPattern.ReplaceAllString(trimmed, ""), "}}") {
		return CompiledCall{}, contractError("call template contains malformed placeholder")
	}

	byCode, err := indexDefinitions(definitions)
	if err != nil {
		return CompiledCall{}, err
	}

	seen := make(map[string]BindSlot, len(definitions))
	slots := make([]BindSlot, 0, len(definitions))
	statement := placeholderPattern.ReplaceAllStringFunc(trimmed, func(match string) string {
		code := placeholderPattern.FindStringSubmatch(match)[1]
		if existing, ok := seen[code]; ok {
			return ":" + existing.BindName
		}
		definition, ok := byCode[code]
		if !ok {
			return match
		}
		slot := BindSlot{
			Code:             code,
			BindName:         fmt.Sprintf("p%d", len(slots)+1),
			ProcedureArgName: definition.ProcedureArgName,
			Position:         definition.Position,
		}
		seen[code] = slot
		slots = append(slots, slot)
		return ":" + slot.BindName
	})

	if strings.Contains(statement, "{{") || strings.Contains(statement, "}}") {
		return CompiledCall{}, contractError("call template references undefined parameter")
	}
	for code := range byCode {
		if _, ok := seen[code]; !ok {
			return CompiledCall{}, contractError("parameter %q is not used by call template", code)
		}
	}

	return CompiledCall{Statement: statement, Slots: slots}, nil
}

func NormalizeParameters(
	definitions []ParameterDefinition,
	clientValues map[string]json.RawMessage,
	systemValues map[string]interface{},
) (NormalizedParameters, error) {
	byCode, err := indexDefinitions(definitions)
	if err != nil {
		return NormalizedParameters{}, err
	}

	for code := range clientValues {
		definition, ok := byCode[code]
		if !ok {
			return NormalizedParameters{}, inputError(code, "parameter is not declared")
		}
		if definition.SystemInjected {
			return NormalizedParameters{}, inputError(code, "system parameter cannot be supplied by client")
		}
	}
	for code := range systemValues {
		definition, ok := byCode[code]
		if !ok || !definition.SystemInjected {
			return NormalizedParameters{}, contractError("system value %q has no system-owned definition", code)
		}
	}

	databaseValues := make(map[string]interface{}, len(definitions))
	publicValues := make(map[string]interface{}, len(definitions))
	sensitiveValues := make(map[string]interface{})
	fingerprintValues := make(map[string]interface{}, len(definitions))

	ordered := append([]ParameterDefinition(nil), definitions...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Position == ordered[j].Position {
			return ordered[i].Code < ordered[j].Code
		}
		return ordered[i].Position < ordered[j].Position
	})

	for _, definition := range ordered {
		raw, present := clientValues[definition.Code]
		if definition.SystemInjected {
			value, exists := systemValues[definition.Code]
			if !exists {
				return NormalizedParameters{}, inputError(definition.Code, "system parameter is unavailable")
			}
			raw, err = json.Marshal(value)
			if err != nil {
				return NormalizedParameters{}, fmt.Errorf("%w: encode system parameter %q: %v", ErrInvalidParameterInput, definition.Code, err)
			}
			present = true
		}
		if !present && len(bytes.TrimSpace(definition.DefaultValue)) > 0 {
			raw = definition.DefaultValue
			present = true
		}

		value, dbValue, err := normalizeValue(definition, raw, present)
		if err != nil {
			return NormalizedParameters{}, err
		}
		databaseValues[definition.Code] = dbValue
		if !definition.SystemInjected {
			fingerprintValues[definition.Code] = value
		}
		if definition.Sensitive {
			sensitiveValues[definition.Code] = value
		} else {
			publicValues[definition.Code] = value
		}
	}

	publicJSON, err := json.Marshal(publicValues)
	if err != nil {
		return NormalizedParameters{}, fmt.Errorf("encode public report parameters: %w", err)
	}
	sensitiveJSON, err := json.Marshal(sensitiveValues)
	if err != nil {
		return NormalizedParameters{}, fmt.Errorf("encode sensitive report parameters: %w", err)
	}
	fingerprintJSON, err := json.Marshal(fingerprintValues)
	if err != nil {
		return NormalizedParameters{}, fmt.Errorf("encode report parameter fingerprint: %w", err)
	}
	sum := sha256.Sum256(fingerprintJSON)

	return NormalizedParameters{
		DatabaseValues: databaseValues,
		PublicJSON:     publicJSON,
		SensitiveJSON:  sensitiveJSON,
		Fingerprint:    hex.EncodeToString(sum[:]),
	}, nil
}

func indexDefinitions(definitions []ParameterDefinition) (map[string]ParameterDefinition, error) {
	if len(definitions) == 0 {
		return nil, contractError("at least one parameter is required")
	}
	byCode := make(map[string]ParameterDefinition, len(definitions))
	positions := make(map[int]string, len(definitions))
	for _, definition := range definitions {
		if !parameterCodePattern.MatchString(definition.Code) {
			return nil, contractError("parameter code %q is invalid", definition.Code)
		}
		if strings.TrimSpace(definition.ProcedureArgName) == "" {
			return nil, contractError("parameter %q has no procedure argument", definition.Code)
		}
		if definition.Position <= 0 {
			return nil, contractError("parameter %q has invalid position", definition.Code)
		}
		if previous, ok := positions[definition.Position]; ok {
			return nil, contractError("parameters %q and %q share position %d", previous, definition.Code, definition.Position)
		}
		if _, exists := byCode[definition.Code]; exists {
			return nil, contractError("parameter code %q is duplicated", definition.Code)
		}
		if !supportedLogicalType(definition.LogicalType) {
			return nil, contractError("parameter %q has unsupported logical type %q", definition.Code, definition.LogicalType)
		}
		if definition.Cardinality == CardinalityMultiple && definition.CollectionEncoding != CollectionEncodingJSONCLOB {
			return nil, contractError("multiple parameter %q must use JSON_CLOB encoding", definition.Code)
		}
		byCode[definition.Code] = definition
		positions[definition.Position] = definition.Code
	}
	return byCode, nil
}

func normalizeValue(definition ParameterDefinition, raw json.RawMessage, present bool) (interface{}, interface{}, error) {
	if !present || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		if definition.Required || !definition.Nullable {
			return nil, nil, inputError(definition.Code, "parameter is required")
		}
		return nil, nil, nil
	}

	var decoded interface{}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&decoded); err != nil {
		return nil, nil, inputError(definition.Code, "value is not valid JSON")
	}
	if err := requireJSONEOF(decoder); err != nil {
		return nil, nil, inputError(definition.Code, "value must contain one JSON value")
	}
	normalizer, err := parseNormalizerRules(definition)
	if err != nil {
		return nil, nil, err
	}
	decoded = applyNormalizer(decoded, normalizer)
	rules, err := parseValidationRules(definition)
	if err != nil {
		return nil, nil, err
	}

	switch definition.LogicalType {
	case LogicalTypeString:
		value, ok := decoded.(string)
		if !ok {
			return nil, nil, inputError(definition.Code, "value must be a string")
		}
		if err := validateString(definition.Code, value, rules); err != nil {
			return nil, nil, err
		}
		if err := validateAllowedValue(definition, value); err != nil {
			return nil, nil, err
		}
		return value, value, nil
	case LogicalTypeInteger:
		value, err := normalizeInteger(decoded)
		if err != nil {
			return nil, nil, inputError(definition.Code, "value must be an integer")
		}
		if err := validateNumber(definition.Code, strconv.FormatInt(value, 10), rules); err != nil {
			return nil, nil, err
		}
		if err := validateAllowedValue(definition, strconv.FormatInt(value, 10)); err != nil {
			return nil, nil, err
		}
		return value, value, nil
	case LogicalTypeDecimal:
		value, err := normalizeDecimal(decoded)
		if err != nil {
			return nil, nil, inputError(definition.Code, "value must be a decimal")
		}
		if err := validateNumber(definition.Code, value, rules); err != nil {
			return nil, nil, err
		}
		if err := validateAllowedValue(definition, value); err != nil {
			return nil, nil, err
		}
		return value, value, nil
	case LogicalTypeBoolean:
		value, ok := decoded.(bool)
		if !ok {
			return nil, nil, inputError(definition.Code, "value must be a boolean")
		}
		if err := validateAllowedValue(definition, strconv.FormatBool(value)); err != nil {
			return nil, nil, err
		}
		return value, value, nil
	case LogicalTypeDate:
		value, ok := decoded.(string)
		if !ok {
			return nil, nil, inputError(definition.Code, "value must be a date string")
		}
		parsed, err := time.Parse("2006-01-02", value)
		if err != nil {
			return nil, nil, inputError(definition.Code, "value must use YYYY-MM-DD")
		}
		if err := validateAllowedValue(definition, value); err != nil {
			return nil, nil, err
		}
		return value, parsed, nil
	case LogicalTypeDateTime:
		value, ok := decoded.(string)
		if !ok {
			return nil, nil, inputError(definition.Code, "value must be a datetime string")
		}
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return nil, nil, inputError(definition.Code, "value must use RFC3339 with timezone")
		}
		canonical := parsed.UTC().Format(time.RFC3339Nano)
		if err := validateAllowedValue(definition, canonical); err != nil {
			return nil, nil, err
		}
		return canonical, parsed, nil
	case LogicalTypeEnum:
		value, ok := decoded.(string)
		if !ok {
			return nil, nil, inputError(definition.Code, "value must be an enum string")
		}
		if err := validateAllowedValues(definition, []string{value}); err != nil {
			return nil, nil, err
		}
		return value, value, nil
	case LogicalTypeMultiEnum:
		values, err := normalizeStringArray(decoded)
		if err != nil {
			return nil, nil, inputError(definition.Code, "value must be a string array")
		}
		if rules.MinItems != nil && len(values) < *rules.MinItems {
			return nil, nil, inputError(definition.Code, "value contains too few items")
		}
		if rules.MaxItems != nil && len(values) > *rules.MaxItems {
			return nil, nil, inputError(definition.Code, "value contains too many items")
		}
		if err := validateAllowedValues(definition, values); err != nil {
			return nil, nil, err
		}
		encoded, err := json.Marshal(values)
		if err != nil {
			return nil, nil, fmt.Errorf("encode parameter %q collection: %w", definition.Code, err)
		}
		return values, string(encoded), nil
	case LogicalTypeJSON:
		encoded, err := json.Marshal(decoded)
		if err != nil {
			return nil, nil, inputError(definition.Code, "value must be valid JSON")
		}
		return decoded, string(encoded), nil
	default:
		return nil, nil, contractError("parameter %q has unsupported logical type", definition.Code)
	}
}

func parseNormalizerRules(definition ParameterDefinition) (normalizerRules, error) {
	if len(bytes.TrimSpace(definition.Normalizer)) == 0 {
		return normalizerRules{}, nil
	}
	var rules normalizerRules
	if err := decodeStrictJSON(definition.Normalizer, &rules); err != nil {
		return normalizerRules{}, contractError("parameter %q has invalid normalizer", definition.Code)
	}
	rules.Case = strings.ToUpper(strings.TrimSpace(rules.Case))
	if rules.Case != "" && rules.Case != "UPPER" && rules.Case != "LOWER" {
		return normalizerRules{}, contractError("parameter %q has unsupported normalizer case", definition.Code)
	}
	if (rules.Trim || rules.Case != "") && definition.LogicalType != LogicalTypeString && definition.LogicalType != LogicalTypeEnum && definition.LogicalType != LogicalTypeMultiEnum {
		return normalizerRules{}, contractError("parameter %q normalizer requires a string or enum type", definition.Code)
	}
	return rules, nil
}

func applyNormalizer(value interface{}, rules normalizerRules) interface{} {
	normalize := func(value string) string {
		if rules.Trim {
			value = strings.TrimSpace(value)
		}
		switch rules.Case {
		case "UPPER":
			return strings.ToUpper(value)
		case "LOWER":
			return strings.ToLower(value)
		default:
			return value
		}
	}
	switch typed := value.(type) {
	case string:
		return normalize(typed)
	case []interface{}:
		result := append([]interface{}(nil), typed...)
		for index, item := range result {
			if text, ok := item.(string); ok {
				result[index] = normalize(text)
			}
		}
		return result
	default:
		return value
	}
}

// SystemValueSource returns the bounded runtime source for a system-owned
// parameter. Empty valueSource remains compatible with the required runId.
func SystemValueSource(definition ParameterDefinition) (string, error) {
	if len(bytes.TrimSpace(definition.ValueSource)) == 0 {
		if definition.SystemInjected && definition.Code == "runId" {
			return ValueSourceRunID, nil
		}
		if definition.SystemInjected {
			return "", contractError("system parameter %q requires a value source", definition.Code)
		}
		return "", nil
	}
	var rules valueSourceRules
	if err := decodeStrictJSON(definition.ValueSource, &rules); err != nil {
		return "", contractError("parameter %q has invalid value source", definition.Code)
	}
	rules.Source = strings.ToUpper(strings.TrimSpace(rules.Source))
	if !definition.SystemInjected {
		return "", contractError("client parameter %q cannot define a value source", definition.Code)
	}
	if rules.Source != ValueSourceRunID && rules.Source != ValueSourceActorID {
		return "", contractError("system parameter %q has unsupported value source", definition.Code)
	}
	if rules.Source == ValueSourceRunID && definition.LogicalType != LogicalTypeString {
		return "", contractError("system parameter %q run id source requires string type", definition.Code)
	}
	if rules.Source == ValueSourceActorID && definition.LogicalType != LogicalTypeInteger {
		return "", contractError("system parameter %q actor source requires integer type", definition.Code)
	}
	return rules.Source, nil
}

func parseValidationRules(definition ParameterDefinition) (ValidationRules, error) {
	if len(bytes.TrimSpace(definition.Validation)) == 0 {
		return ValidationRules{}, nil
	}
	var rules ValidationRules
	decoder := json.NewDecoder(bytes.NewReader(definition.Validation))
	decoder.UseNumber()
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&rules); err != nil {
		return ValidationRules{}, contractError("parameter %q has invalid validation rules", definition.Code)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return ValidationRules{}, contractError("parameter %q validation rules contain trailing JSON", definition.Code)
	}
	if rules.Pattern != "" {
		if _, err := regexp.Compile(rules.Pattern); err != nil {
			return ValidationRules{}, contractError("parameter %q has invalid validation pattern", definition.Code)
		}
	}
	return rules, nil
}

func validateString(code, value string, rules ValidationRules) error {
	length := len([]rune(value))
	if rules.MinLength != nil && length < *rules.MinLength {
		return inputError(code, "value is shorter than allowed")
	}
	if rules.MaxLength != nil && length > *rules.MaxLength {
		return inputError(code, "value is longer than allowed")
	}
	if rules.Pattern != "" && !regexp.MustCompile(rules.Pattern).MatchString(value) {
		return inputError(code, "value does not match required format")
	}
	return nil
}

func validateNumber(code, value string, rules ValidationRules) error {
	parsedValue, ok := new(big.Rat).SetString(value)
	if !ok {
		return inputError(code, "value is not a decimal")
	}
	if rules.Min != nil {
		minValue, valid := new(big.Rat).SetString(rules.Min.String())
		if !valid {
			return contractError("parameter %q has invalid minimum", code)
		}
		if parsedValue.Cmp(minValue) < 0 {
			return inputError(code, "value is smaller than allowed")
		}
	}
	if rules.Max != nil {
		maxValue, valid := new(big.Rat).SetString(rules.Max.String())
		if !valid {
			return contractError("parameter %q has invalid maximum", code)
		}
		if parsedValue.Cmp(maxValue) > 0 {
			return inputError(code, "value is larger than allowed")
		}
	}
	return nil
}

func requireJSONEOF(decoder *json.Decoder) error {
	var trailing interface{}
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func validateAllowedValues(definition ParameterDefinition, values []string) error {
	if len(bytes.TrimSpace(definition.AllowedValues)) == 0 {
		return contractError("enum parameter %q has no allowed values", definition.Code)
	}
	var allowed []string
	if err := json.Unmarshal(definition.AllowedValues, &allowed); err != nil || len(allowed) == 0 {
		return contractError("enum parameter %q has invalid allowed values", definition.Code)
	}
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, value := range allowed {
		canonical, err := canonicalAllowedValue(definition, value)
		if err != nil {
			return contractError("parameter %q has invalid allowed value", definition.Code)
		}
		allowedSet[canonical] = struct{}{}
	}
	for _, value := range values {
		canonical, err := canonicalAllowedValue(definition, value)
		if err != nil {
			return inputError(definition.Code, "value is not allowed")
		}
		if _, ok := allowedSet[canonical]; !ok {
			return inputError(definition.Code, "value is not allowed")
		}
	}
	return nil
}

func validateAllowedValue(definition ParameterDefinition, value string) error {
	if len(bytes.TrimSpace(definition.AllowedValues)) == 0 {
		return nil
	}
	return validateAllowedValues(definition, []string{value})
}

func canonicalAllowedValue(definition ParameterDefinition, value string) (string, error) {
	switch definition.LogicalType {
	case LogicalTypeString, LogicalTypeEnum, LogicalTypeMultiEnum:
		rules, err := parseNormalizerRules(definition)
		if err != nil {
			return "", err
		}
		return applyNormalizer(value, rules).(string), nil
	case LogicalTypeInteger:
		parsed, err := normalizeInteger(value)
		if err != nil {
			return "", err
		}
		return strconv.FormatInt(parsed, 10), nil
	case LogicalTypeDecimal:
		return normalizeDecimal(value)
	case LogicalTypeBoolean:
		if value != "true" && value != "false" {
			return "", errors.New("not a boolean")
		}
		return value, nil
	case LogicalTypeDate:
		if _, err := time.Parse("2006-01-02", value); err != nil {
			return "", err
		}
		return value, nil
	case LogicalTypeDateTime:
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return "", err
		}
		return parsed.UTC().Format(time.RFC3339Nano), nil
	default:
		return "", errors.New("unsupported allowed value type")
	}
}

func normalizeInteger(value interface{}) (int64, error) {
	var raw string
	switch typed := value.(type) {
	case json.Number:
		raw = typed.String()
	case string:
		raw = typed
	default:
		return 0, errors.New("not an integer")
	}
	if !regexp.MustCompile(`^-?\d+$`).MatchString(raw) {
		return 0, errors.New("not an integer")
	}
	return strconv.ParseInt(raw, 10, 64)
}

func normalizeDecimal(value interface{}) (string, error) {
	var raw string
	switch typed := value.(type) {
	case json.Number:
		raw = typed.String()
	case string:
		raw = typed
	default:
		return "", errors.New("not a decimal")
	}
	if !decimalPattern.MatchString(raw) {
		return "", errors.New("not a decimal")
	}
	negative := strings.HasPrefix(raw, "-")
	unsigned := strings.TrimPrefix(raw, "-")
	parts := strings.SplitN(unsigned, ".", 2)
	integerPart := strings.TrimLeft(parts[0], "0")
	if integerPart == "" {
		integerPart = "0"
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = strings.TrimRight(parts[1], "0")
	}
	result := integerPart
	if fraction != "" {
		result += "." + fraction
	}
	if negative && result != "0" {
		result = "-" + result
	}
	return result, nil
}

func normalizeStringArray(value interface{}) ([]string, error) {
	items, ok := value.([]interface{})
	if !ok {
		return nil, errors.New("not an array")
	}
	values := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		value, ok := item.(string)
		if !ok {
			return nil, errors.New("array item is not a string")
		}
		if _, duplicate := seen[value]; duplicate {
			continue
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}
	return values, nil
}

func supportedLogicalType(value string) bool {
	switch value {
	case LogicalTypeString, LogicalTypeInteger, LogicalTypeDecimal, LogicalTypeBoolean,
		LogicalTypeDate, LogicalTypeDateTime, LogicalTypeEnum, LogicalTypeMultiEnum, LogicalTypeJSON:
		return true
	default:
		return false
	}
}

func contractError(format string, args ...interface{}) error {
	return fmt.Errorf("%w: %s", ErrInvalidParameterContract, fmt.Sprintf(format, args...))
}

func inputError(code, message string) error {
	return fmt.Errorf("%w: parameter %q %s", ErrInvalidParameterInput, code, message)
}
