package reportquery

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestNormalizeUsesFrozenFieldContract(t *testing.T) {
	columns := []Column{{FieldID: "amount-id", LogicalCode: "amount", DatabaseColumn: "AMOUNT", ValueType: "decimal", SourceOracleType: "NUMBER", Filterable: true, Sortable: true, AllowedOperators: []string{"EQ", "BETWEEN"}}}
	query, err := Normalize(Input{
		Filters: []FilterInput{{Field: "amount-id", Operator: "between", Value: json.RawMessage(`["1.25",10]`)}},
		Sort:    []SortInput{{Field: "amount-id", Direction: "desc"}},
	}, columns)
	if err != nil || len(query.Filters) != 1 || query.Filters[0].Column != "AMOUNT" || query.Filters[0].OracleType != "NUMBER" || query.Filters[0].Values[0].Text != "1.25" || query.Sort[0].Direction != "DESC" {
		t.Fatalf("Normalize() = %#v, %v", query, err)
	}
	first, _ := Fingerprint(query)
	second, _ := Fingerprint(query)
	if first == "" || first != second {
		t.Fatalf("fingerprints = %q, %q", first, second)
	}
}

func TestNormalizeRejectsUnpublishedCapabilitiesAndExcessiveSets(t *testing.T) {
	columns := []Column{{FieldID: "name-id", LogicalCode: "name", DatabaseColumn: "NAME", ValueType: "string", SourceOracleType: "VARCHAR2", Filterable: true, AllowedOperators: []string{"EQ"}}}
	for _, input := range []Input{
		{Filters: []FilterInput{{Field: "name-id", Operator: "CONTAINS", Value: json.RawMessage(`"x"`)}}},
		{Sort: []SortInput{{Field: "name-id", Direction: "ASC"}}},
		{Filters: []FilterInput{{Field: "missing", Operator: "EQ", Value: json.RawMessage(`"x"`)}}},
	} {
		if _, err := Normalize(input, columns); !errors.Is(err, ErrInvalid) {
			t.Fatalf("Normalize(%#v) error = %v", input, err)
		}
	}
	values := `[` + strings.Repeat(`"x",`, MaxSetValues) + `"x"]`
	if _, err := Normalize(Input{Filters: []FilterInput{{Field: "name-id", Operator: "IN", Value: json.RawMessage(values)}}}, []Column{{FieldID: "name-id", LogicalCode: "name", DatabaseColumn: "NAME", ValueType: "string", Filterable: true, AllowedOperators: []string{"IN"}}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("excessive IN error = %v", err)
	}
}

func TestNormalizeNullOperatorRejectsValues(t *testing.T) {
	column := Column{FieldID: "name-id", LogicalCode: "name", DatabaseColumn: "NAME", ValueType: "string", Filterable: true, AllowedOperators: []string{"IS_NULL"}}
	if _, err := Normalize(Input{Filters: []FilterInput{{Field: "name-id", Operator: "IS_NULL", Value: json.RawMessage(`"ignored"`)}}}, []Column{column}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("IS_NULL value error = %v", err)
	}
}
