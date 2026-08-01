package data_dao

import "testing"

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
