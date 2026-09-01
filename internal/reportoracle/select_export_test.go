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
		{name: "line comments", statement: "\tselect *\n\tfrom BJ_REPORT_RETAIL_DAY_SF a\n\twhere a.billdate=20260901  --单据日期\n\tand a.c_store_id=23        --店仓\n\tand a.c_payway_id=1", want: true},
		{name: "block comment", statement: "SELECT /* report columns */ * FROM sales", want: true},
		{name: "semicolon in comment", statement: "SELECT * FROM sales -- ; DELETE FROM sales", want: true},
		{name: "semicolon in string", statement: "SELECT ';' AS separator FROM sales", want: true},
		{name: "update", statement: "UPDATE sales SET amount = 0"},
		{name: "semicolon", statement: "SELECT * FROM sales; DELETE FROM sales"},
		{name: "locking read", statement: "SELECT * FROM sales FOR UPDATE"},
		{name: "multiline locking read", statement: "SELECT * FROM sales FOR\nUPDATE"},
		{name: "comment separated locking read", statement: "SELECT * FROM sales FOR/**/UPDATE"},
		{name: "unterminated block comment", statement: "SELECT * FROM sales /*"},
		{name: "unterminated string", statement: "SELECT 'value FROM sales"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ValidateSelect(test.statement); got != test.want {
				t.Fatalf("ValidateSelect() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestAnalyzeSelectIgnoresBindsInCommentsAndQuotedText(t *testing.T) {
	analysis, valid := AnalyzeSelect("SELECT ':literal' FROM sales -- :comment\nWHERE shop = :shop /* :ignored */")
	if !valid || analysis.HasPositionalBind || len(analysis.NamedBinds) != 1 {
		t.Fatalf("AnalyzeSelect() analysis=%#v valid=%t", analysis, valid)
	}
	if _, exists := analysis.NamedBinds["shop"]; !exists {
		t.Fatalf("AnalyzeSelect() binds=%#v", analysis.NamedBinds)
	}
}
