package data_dao

import "testing"

func TestProcessedDataTypeCondition(t *testing.T) {
	if condition, args := processedDataTypeCondition(""); condition != "" || len(args) != 0 {
		t.Fatalf("empty data type condition = %q, %#v; want no condition", condition, args)
	}
	if condition, args := processedDataTypeCondition("order"); condition != "data_type = ?" || len(args) != 1 || args[0] != "order" {
		t.Fatalf("data type condition = %q, %#v", condition, args)
	}
}
