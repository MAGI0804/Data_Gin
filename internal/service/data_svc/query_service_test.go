package data_svc

import (
	"reflect"
	"testing"
)

func TestDecodeRawJSONPreservesEveryValidJSONShape(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  interface{}
	}{
		{name: "object", input: `{"order":"A-1"}`, want: map[string]any{"order": "A-1"}},
		{name: "array", input: `[{"order":"A-1"},2]`, want: []any{map[string]any{"order": "A-1"}, float64(2)}},
		{name: "string", input: `"raw-value"`, want: "raw-value"},
		{name: "number", input: `42`, want: float64(42)},
		{name: "empty", input: ``, want: nil},
		{name: "invalid", input: `{`, want: nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := decodeRawJSON(test.input); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("decodeRawJSON(%q) = %#v, want %#v", test.input, got, test.want)
			}
		})
	}
}
