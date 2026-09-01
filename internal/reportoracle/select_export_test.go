package reportoracle

import "testing"

func TestValidateSelect(t *testing.T) {
	tests := []struct {
		name      string
		statement string
		want      bool
	}{
		{name: "named date bind", statement: "SELECT order_no FROM sales WHERE bill_date = :bill_date", want: true},
		{name: "multiple binds", statement: "SELECT * FROM sales WHERE shop = :shop AND amount >= :amount", want: true},
		{name: "update", statement: "UPDATE sales SET amount = 0"},
		{name: "semicolon", statement: "SELECT * FROM sales; DELETE FROM sales"},
		{name: "comment", statement: "SELECT * FROM sales -- unsafe"},
		{name: "locking read", statement: "SELECT * FROM sales FOR UPDATE"},
		{name: "multiline locking read", statement: "SELECT * FROM sales FOR\nUPDATE"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ValidateSelect(test.statement); got != test.want {
				t.Fatalf("ValidateSelect() = %t, want %t", got, test.want)
			}
		})
	}
}
