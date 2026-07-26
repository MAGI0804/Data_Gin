package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestUserConsoleManagedIsNotSerialized(t *testing.T) {
	payload, err := json.Marshal(User{ConsoleManaged: true})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if strings.Contains(string(payload), "console_managed") || strings.Contains(string(payload), "ConsoleManaged") {
		t.Fatalf("console-managed marker leaked in JSON: %s", payload)
	}
}
