package data_ctrl

import (
	"strings"
	"testing"
)

func TestRedactConfigJSON(t *testing.T) {
	value, hasSecret := redactConfigJSON(`{"endpoint":"https://example.test","headers":{"Authorization":"Bearer top-secret","Content-Type":"application/json"},"token":"abc"}`)
	if !hasSecret {
		t.Fatal("hasSecret = false, want true")
	}
	if value == "" || containsSensitiveValue(value, "top-secret") || containsSensitiveValue(value, "abc") {
		t.Fatalf("redacted config leaked a secret: %s", value)
	}
}

func containsSensitiveValue(value, secret string) bool { return strings.Contains(value, secret) }
