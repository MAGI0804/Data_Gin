package transform

import "testing"

func TestApplyMappingMapsConvertsAndTransformsFields(t *testing.T) {
	raw := map[string]interface{}{
		"params": map[string]interface{}{
			"orderNo": "ORDER-1",
		},
		"data": map[string]interface{}{
			"amount": "12345",
			"status": "TRADE_SUCCESS",
		},
	}

	clean, err := ApplyMapping(raw, MappingConfig{
		Fields: []FieldRule{
			{Name: "order_no", SourcePath: "$.params.orderNo", Type: "string", Required: true},
			{Name: "actual_amount", SourcePath: "$.data.amount", Type: "decimal", Transform: "divide:100"},
			{Name: "status", SourcePath: "$.data.status", Type: "string", Enum: []interface{}{"TRADE_SUCCESS"}},
		},
	})
	if err != nil {
		t.Fatalf("ApplyMapping returned error: %v", err)
	}

	if clean["order_no"] != "ORDER-1" {
		t.Fatalf("order_no = %v, want ORDER-1", clean["order_no"])
	}
	if clean["actual_amount"] != 123.45 {
		t.Fatalf("actual_amount = %v, want 123.45", clean["actual_amount"])
	}
}

func TestApplyMappingRejectsMissingRequiredField(t *testing.T) {
	_, err := ApplyMapping(map[string]interface{}{}, MappingConfig{
		Fields: []FieldRule{
			{Name: "order_no", SourcePath: "$.params.orderNo", Required: true},
		},
	})
	if err == nil {
		t.Fatal("ApplyMapping returned nil error, want required field error")
	}
}

func TestApplyMappingRejectsEnumMismatch(t *testing.T) {
	raw := map[string]interface{}{
		"status": "WAIT_BUYER_PAY",
	}

	_, err := ApplyMapping(raw, MappingConfig{
		Fields: []FieldRule{
			{Name: "status", SourcePath: "$.status", Type: "string", Enum: []interface{}{"TRADE_SUCCESS"}},
		},
	})
	if err == nil {
		t.Fatal("ApplyMapping returned nil error, want enum error")
	}
}
