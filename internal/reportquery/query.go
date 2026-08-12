package reportquery

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

const (
	MaxFilters        = 8
	MaxSorts          = 1
	MaxSetValues      = 100
	MaxTextValueBytes = 256
)

var ErrInvalid = errors.New("report query: invalid input")

type Input struct {
	Filters []FilterInput `json:"filters"`
	Sort    []SortInput   `json:"sort"`
}

type FilterInput struct {
	Field    string          `json:"field"`
	Operator string          `json:"operator"`
	Value    json.RawMessage `json:"value,omitempty"`
}

type SortInput struct {
	Field     string `json:"field"`
	Direction string `json:"direction"`
}

type Column struct {
	FieldID          string
	LogicalCode      string
	DatabaseColumn   string
	ValueType        string
	SourceOracleType string
	Nullable         bool
	Filterable       bool
	Sortable         bool
	AllowedOperators []string
}

type Value struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

type Filter struct {
	Field      string  `json:"field"`
	Column     string  `json:"column"`
	OracleType string  `json:"oracleType"`
	Operator   string  `json:"operator"`
	Values     []Value `json:"values"`
}

type Sort struct {
	Field     string `json:"field"`
	Column    string `json:"column"`
	Direction string `json:"direction"`
	Kind      string `json:"kind"`
}

type Query struct {
	Filters []Filter `json:"filters"`
	Sort    []Sort   `json:"sort"`
}

func Normalize(input Input, columns []Column) (Query, error) {
	if len(input.Filters) > MaxFilters || len(input.Sort) > MaxSorts {
		return Query{}, ErrInvalid
	}
	byField := make(map[string]Column, len(columns))
	for _, column := range columns {
		key := strings.ToLower(strings.TrimSpace(column.FieldID))
		if key == "" || strings.TrimSpace(column.DatabaseColumn) == "" {
			return Query{}, ErrInvalid
		}
		if _, exists := byField[key]; exists {
			return Query{}, ErrInvalid
		}
		byField[key] = column
	}
	query := Query{Filters: make([]Filter, 0, len(input.Filters)), Sort: make([]Sort, 0, len(input.Sort))}
	for _, requested := range input.Filters {
		field := strings.TrimSpace(requested.Field)
		column, exists := byField[strings.ToLower(field)]
		operator := strings.ToUpper(strings.TrimSpace(requested.Operator))
		if !exists || field != column.FieldID || !column.Filterable || !containsOperator(column.AllowedOperators, operator) {
			return Query{}, ErrInvalid
		}
		if !column.Nullable && (operator == "IS_NULL" || operator == "IS_NOT_NULL") {
			return Query{}, ErrInvalid
		}
		values, err := normalizeFilterValues(operator, requested.Value, column.ValueType)
		if err != nil {
			return Query{}, err
		}
		query.Filters = append(query.Filters, Filter{
			Field: column.FieldID, Column: strings.ToUpper(strings.TrimSpace(column.DatabaseColumn)), OracleType: strings.ToUpper(strings.TrimSpace(column.SourceOracleType)), Operator: operator, Values: values,
		})
	}
	for _, requested := range input.Sort {
		field := strings.TrimSpace(requested.Field)
		column, exists := byField[strings.ToLower(field)]
		direction := strings.ToUpper(strings.TrimSpace(requested.Direction))
		kind := normalizeKind(column.ValueType)
		if !exists || field != column.FieldID || !column.Sortable || (direction != "ASC" && direction != "DESC") || !sortableKind(kind) || strings.EqualFold(strings.TrimSpace(column.SourceOracleType), "CLOB") {
			return Query{}, ErrInvalid
		}
		query.Sort = append(query.Sort, Sort{Field: column.FieldID, Column: strings.ToUpper(strings.TrimSpace(column.DatabaseColumn)), Direction: direction, Kind: kind})
	}
	return query, nil
}

func Encode(query Query) ([]byte, []byte, error) {
	filters, err := json.Marshal(query.Filters)
	if err != nil {
		return nil, nil, fmt.Errorf("report query: encode filters: %w", err)
	}
	sort, err := json.Marshal(query.Sort)
	if err != nil {
		return nil, nil, fmt.Errorf("report query: encode sort: %w", err)
	}
	return filters, sort, nil
}

func Decode(filtersJSON, sortJSON []byte) (Query, error) {
	query := Query{}
	if bytes.Equal(bytes.TrimSpace(filtersJSON), []byte("{}")) {
		filtersJSON = []byte("[]")
	}
	if err := strictDecode(filtersJSON, &query.Filters); err != nil {
		return Query{}, ErrInvalid
	}
	if err := strictDecode(sortJSON, &query.Sort); err != nil {
		return Query{}, ErrInvalid
	}
	if len(query.Filters) > MaxFilters || len(query.Sort) > MaxSorts {
		return Query{}, ErrInvalid
	}
	return query, nil
}

func ValidateCompiled(query Query, columns []Column) error {
	input := Input{Filters: make([]FilterInput, 0, len(query.Filters)), Sort: make([]SortInput, 0, len(query.Sort))}
	for _, filter := range query.Filters {
		var value json.RawMessage
		if filter.Operator != "IS_NULL" && filter.Operator != "IS_NOT_NULL" {
			encodedValues := make([]interface{}, len(filter.Values))
			for index, item := range filter.Values {
				encodedValues[index] = valueJSON(item)
			}
			var encoded interface{} = encodedValues
			if len(encodedValues) == 1 && filter.Operator != "IN" && filter.Operator != "NOT_IN" && filter.Operator != "BETWEEN" {
				encoded = encodedValues[0]
			}
			var err error
			value, err = json.Marshal(encoded)
			if err != nil {
				return ErrInvalid
			}
		}
		input.Filters = append(input.Filters, FilterInput{Field: filter.Field, Operator: filter.Operator, Value: value})
	}
	for _, sort := range query.Sort {
		input.Sort = append(input.Sort, SortInput{Field: sort.Field, Direction: sort.Direction})
	}
	normalized, err := Normalize(input, columns)
	if err != nil {
		return err
	}
	expected, err := json.Marshal(query)
	if err != nil {
		return ErrInvalid
	}
	actual, err := json.Marshal(normalized)
	if err != nil || !bytes.Equal(expected, actual) {
		return ErrInvalid
	}
	return nil
}

func valueJSON(value Value) interface{} {
	switch value.Kind {
	case "integer", "decimal":
		return json.Number(value.Text)
	case "boolean":
		return value.Text == "true"
	default:
		return value.Text
	}
}

func Fingerprint(query Query) (string, error) {
	encoded, err := json.Marshal(query)
	if err != nil {
		return "", fmt.Errorf("report query: encode fingerprint: %w", err)
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func ValueFromDatabase(kind string, raw interface{}) (Value, error) {
	kind = normalizeKind(kind)
	if raw == nil || !sortableKind(kind) {
		return Value{}, ErrInvalid
	}
	switch typed := raw.(type) {
	case []byte:
		raw = string(typed)
	case time.Time:
		if kind == "date" {
			return Value{Kind: "datetime", Text: typed.UTC().Format(time.RFC3339Nano)}, nil
		}
		return Value{Kind: kind, Text: typed.UTC().Format(time.RFC3339Nano)}, nil
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return Value{}, ErrInvalid
	}
	return normalizeScalar(json.RawMessage(encoded), kind)
}

func normalizeFilterValues(operator string, raw json.RawMessage, valueType string) ([]Value, error) {
	kind := normalizeKind(valueType)
	switch operator {
	case "IS_NULL", "IS_NOT_NULL":
		if len(bytes.TrimSpace(raw)) != 0 && !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return nil, ErrInvalid
		}
		return []Value{}, nil
	case "IN", "NOT_IN", "BETWEEN":
		var values []json.RawMessage
		if strictDecode(raw, &values) != nil || len(values) == 0 || len(values) > MaxSetValues || operator == "BETWEEN" && len(values) != 2 {
			return nil, ErrInvalid
		}
		result := make([]Value, 0, len(values))
		for _, value := range values {
			normalized, err := normalizeScalar(value, kind)
			if err != nil {
				return nil, err
			}
			result = append(result, normalized)
		}
		return result, nil
	case "EQ", "NE", "GT", "GTE", "LT", "LTE", "CONTAINS", "STARTS_WITH":
		if (operator == "CONTAINS" || operator == "STARTS_WITH") && kind != "string" {
			return nil, ErrInvalid
		}
		normalized, err := normalizeScalar(raw, kind)
		if err != nil {
			return nil, err
		}
		return []Value{normalized}, nil
	default:
		return nil, ErrInvalid
	}
}

func normalizeScalar(raw json.RawMessage, kind string) (Value, error) {
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return Value{}, ErrInvalid
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var decoded interface{}
	if err := decoder.Decode(&decoded); err != nil {
		return Value{}, ErrInvalid
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Value{}, ErrInvalid
	}
	kind = normalizeKind(kind)
	value := Value{Kind: kind}
	switch kind {
	case "integer":
		value.Text = scalarText(decoded)
		if _, err := strconv.ParseInt(value.Text, 10, 64); err != nil {
			return Value{}, ErrInvalid
		}
	case "decimal":
		value.Text = scalarText(decoded)
		if !validDecimal(value.Text) {
			return Value{}, ErrInvalid
		}
	case "boolean":
		typed, ok := decoded.(bool)
		if !ok {
			return Value{}, ErrInvalid
		}
		value.Text = strconv.FormatBool(typed)
	case "date":
		text, ok := decoded.(string)
		if !ok {
			return Value{}, ErrInvalid
		}
		parsed, err := time.Parse("2006-01-02", text)
		if err != nil {
			return Value{}, ErrInvalid
		}
		value.Text = parsed.Format("2006-01-02")
	case "datetime":
		text, ok := decoded.(string)
		if !ok {
			return Value{}, ErrInvalid
		}
		parsed, err := time.Parse(time.RFC3339Nano, text)
		if err != nil {
			return Value{}, ErrInvalid
		}
		value.Text = parsed.UTC().Format(time.RFC3339Nano)
	case "string":
		text, ok := decoded.(string)
		if !ok || text == "" || len([]byte(text)) > MaxTextValueBytes {
			return Value{}, ErrInvalid
		}
		value.Text = text
	default:
		return Value{}, ErrInvalid
	}
	return value, nil
}

func containsOperator(configured []string, requested string) bool {
	for _, operator := range configured {
		if strings.EqualFold(strings.TrimSpace(operator), requested) {
			return true
		}
	}
	return false
}

func normalizeKind(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "integer":
		return "integer"
	case "decimal", "number":
		return "decimal"
	case "boolean", "bool":
		return "boolean"
	case "date":
		return "date"
	case "datetime", "timestamp":
		return "datetime"
	case "string", "enum":
		return "string"
	default:
		return ""
	}
}

func sortableKind(kind string) bool { return kind != "" && kind != "boolean" }

func scalarText(value interface{}) string {
	switch typed := value.(type) {
	case json.Number:
		return string(typed)
	case string:
		return strings.TrimSpace(typed)
	default:
		return ""
	}
}

func validDecimal(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	if value[0] == '-' || value[0] == '+' {
		value = value[1:]
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" {
		return false
	}
	for _, part := range parts {
		if part == "" {
			return false
		}
		for _, char := range part {
			if char < '0' || char > '9' {
				return false
			}
		}
	}
	return true
}

func strictDecode(raw []byte, destination interface{}) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		return ErrInvalid
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return ErrInvalid
	}
	return nil
}
