package data_ctrl

import (
	"strings"
	"testing"
	"unicode/utf8"
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

func TestSafeDeliveryLogTextKeepsFrontendLengthLimit(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "ASCII at limit", value: strings.Repeat("a", 240), want: strings.Repeat("a", 240)},
		{name: "ASCII over limit", value: strings.Repeat("a", 241), want: strings.Repeat("a", 239) + "…"},
		{name: "Chinese over limit", value: strings.Repeat("数", 241), want: strings.Repeat("数", 239) + "…"},
		{name: "emoji at limit", value: strings.Repeat("😀", 120), want: strings.Repeat("😀", 120)},
		{name: "emoji over limit", value: strings.Repeat("😀", 121), want: strings.Repeat("😀", 119) + "…"},
		{name: "mixed BMP and non-BMP boundary", value: strings.Repeat("数", 237) + "😀ab", want: strings.Repeat("数", 237) + "😀…"},
		{name: "invalid UTF-8", value: strings.Repeat("a", 238) + string([]byte{0xff}) + "😀", want: strings.Repeat("a", 238) + "�…"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := safeDeliveryLogText(test.value)
			if got != test.want {
				t.Fatalf("safeDeliveryLogText() = %q, want %q", got, test.want)
			}
			if !utf8.ValidString(got) {
				t.Fatalf("safeDeliveryLogText() returned invalid UTF-8: %q", got)
			}
			if length := deliveryLogTextLength(got); length > 240 {
				t.Fatalf("safeDeliveryLogText() UTF-16 length = %d, want <= 240", length)
			}
		})
	}
}
