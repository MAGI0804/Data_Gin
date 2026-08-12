package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestUserConsoleManagedIsNotSerialized(t *testing.T) {
	payload, err := json.Marshal(User{ConsoleManaged: true, Phone: "13800138000", Email: "secret@example.com", Password: "secret"})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	for _, forbidden := range []string{"console_managed", "ConsoleManaged", "13800138000", "secret@example.com", "secret"} {
		if strings.Contains(string(payload), forbidden) {
			t.Fatalf("private user field leaked through JSON: %s", payload)
		}
	}
}
