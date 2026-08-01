package data_dao

import (
	"reflect"
	"testing"
)

func TestRawDataOriginCondition(t *testing.T) {
	tests := []struct {
		origin    string
		condition string
		argument  string
	}{
		{origin: "pull", condition: "metadata LIKE ?", argument: "%\"format\":\"fetch\"%"},
		{origin: "receive", condition: "(metadata IS NULL OR metadata NOT LIKE ?)", argument: "%\"format\":\"fetch\"%"},
		{origin: "", condition: "", argument: ""},
		{origin: "unknown", condition: "", argument: ""},
	}

	for _, tt := range tests {
		t.Run(tt.origin, func(t *testing.T) {
			condition, args := rawDataOriginCondition(tt.origin)
			if condition != tt.condition {
				t.Fatalf("condition = %q, want %q", condition, tt.condition)
			}
			if tt.argument == "" {
				if len(args) != 0 {
					t.Fatalf("args = %#v, want none", args)
				}
				return
			}
			if len(args) != 1 || args[0] != tt.argument {
				t.Fatalf("args = %#v, want %q", args, tt.argument)
			}
		})
	}
}

func TestRawDataQueryConditionsUseExactParameterizedFilters(t *testing.T) {
	params := RawDataQueryParams{
		Source:      "youzan",
		DataType:    "order",
		Status:      "processed",
		BusinessKey: "order-1001",
		StartTime:   "2026-08-01 00:00:00",
		EndTime:     "2026-08-01 23:59:59",
		Origin:      "pull",
	}

	got := rawDataQueryConditions(params)
	want := []rawDataQueryCondition{
		{query: "source = ?", args: []interface{}{"youzan"}},
		{query: "data_type = ?", args: []interface{}{"order"}},
		{query: "status = ?", args: []interface{}{"processed"}},
		{query: "external_id = ?", args: []interface{}{"order-1001"}},
		{query: "ingested_at >= ?", args: []interface{}{"2026-08-01 00:00:00"}},
		{query: "ingested_at <= ?", args: []interface{}{"2026-08-01 23:59:59"}},
		{query: "metadata LIKE ?", args: []interface{}{"%\"format\":\"fetch\"%"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rawDataQueryConditions() = %#v, want %#v", got, want)
	}
}
