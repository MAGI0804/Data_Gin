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

func TestRedactConfigJSONRedactsDataSourceDSNAndURLCredentials(t *testing.T) {
	value, hasSecret := redactConfigJSON(`{"dsn":"mysql://admin:db-password@db.internal/orders","url":"https://api-user:api-password@example.test/orders?access_token=query-token&store=all"}`)
	if !hasSecret {
		t.Fatal("hasSecret = false, want true")
	}
	for _, secret := range []string{"db-password", "api-user", "api-password", "query-token"} {
		if containsSensitiveValue(value, secret) {
			t.Fatalf("redacted source config leaked %q: %s", secret, value)
		}
	}
	if !strings.Contains(value, "store=all") {
		t.Fatalf("redacted source config lost non-sensitive URL query: %s", value)
	}
}

func containsSensitiveValue(value, secret string) bool { return strings.Contains(value, secret) }
