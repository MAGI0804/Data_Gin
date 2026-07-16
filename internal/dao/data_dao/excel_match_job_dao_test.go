package data_dao

import "testing"

func TestIsSafeExcelSQLIdentifier(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "table", value: "bojun_retail_orders", want: true},
		{name: "column", value: "matched_docno", want: true},
		{name: "starts with number", value: "1_orders", want: false},
		{name: "contains dot", value: "warehouse.orders", want: false},
		{name: "contains sql", value: "orders` WHERE 1=1 --", want: false},
		{name: "empty", value: "", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSafeExcelSQLIdentifier(tt.value); got != tt.want {
				t.Fatalf("isSafeExcelSQLIdentifier(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}
