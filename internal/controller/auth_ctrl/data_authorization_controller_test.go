package auth_ctrl

import "testing"

func TestParseDataAuthorizationID(t *testing.T) {
	for _, value := range []string{"", "0", "-1", "abc"} {
		if _, err := parseDataAuthorizationID(value); err == nil {
			t.Fatalf("parseDataAuthorizationID(%q) error = nil", value)
		}
	}
	if got, err := parseDataAuthorizationID("17"); err != nil || got != 17 {
		t.Fatalf("parseDataAuthorizationID(17) = %d, %v", got, err)
	}
}
