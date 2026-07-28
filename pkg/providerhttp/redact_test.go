package providerhttp

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestRedactURLRemovesCredentialsAndFragment(t *testing.T) {
	const secret = "path-secret-value"
	redacted := RedactURL(
		"https://user:password@example.com/v2/"+secret+"?address=mall&key=query-secret&token=token-secret#fragment-secret",
		secret,
	)
	for _, forbidden := range []string{"user", "password", secret, "query-secret", "token-secret", "fragment-secret"} {
		if strings.Contains(redacted, forbidden) {
			t.Fatalf("RedactURL() contains %q: %s", forbidden, redacted)
		}
	}
	parsed, err := url.Parse(redacted)
	if err != nil {
		t.Fatalf("parse redacted URL: %v", err)
	}
	if parsed.Query().Get("key") != redactedValue || parsed.Query().Get("token") != redactedValue {
		t.Fatalf("redacted query = %s", parsed.RawQuery)
	}
	if parsed.Query().Get("address") != "mall" {
		t.Fatalf("non-sensitive query changed: %s", parsed.RawQuery)
	}
}

func TestRedactURLFailsClosedForMalformedURL(t *testing.T) {
	if got := RedactURL("not a URL?key=secret"); got != redactedValue {
		t.Fatalf("RedactURL() = %q, want %q", got, redactedValue)
	}
}

func TestRedactHeadersReturnsSanitizedClone(t *testing.T) {
	headers := http.Header{
		"Authorization":  []string{"Bearer access-token"},
		"X-Cy-Signature": []string{"signature-value"},
		"X-Request-ID":   []string{"trace-1"},
	}
	redacted := RedactHeaders(headers)
	if redacted.Get("Authorization") != redactedValue || redacted.Get("X-Cy-Signature") != redactedValue {
		t.Fatalf("RedactHeaders() = %#v", redacted)
	}
	if redacted.Get("X-Request-ID") != "trace-1" {
		t.Fatalf("X-Request-ID = %q", redacted.Get("X-Request-ID"))
	}
	if headers.Get("Authorization") != "Bearer access-token" {
		t.Fatal("RedactHeaders mutated the source")
	}
}

func TestRedactTextSanitizesQueryHeadersAndExactValues(t *testing.T) {
	message := "Get \"https://example.com/v2/path-secret?key=query-secret\"\nAuthorization: Bearer access-token\nx-cy-signature=signature-value database password=database-secret DB_PASSWORD=db-secret CAIYUN_APP_SECRET=caiyun-secret"
	redacted := RedactText(message, "path-secret")
	for _, forbidden := range []string{
		"path-secret", "query-secret", "access-token", "signature-value",
		"database-secret", "db-secret", "caiyun-secret",
	} {
		if strings.Contains(redacted, forbidden) {
			t.Fatalf("RedactText() contains %q: %s", forbidden, redacted)
		}
	}
	if !strings.Contains(redacted, "key="+redactedValue) {
		t.Fatalf("RedactText() did not preserve safe context: %s", redacted)
	}
}

func TestRedactJSONPreservesValidityForEscapedCredential(t *testing.T) {
	credential := `secret\"with\\escapes`
	data, err := json.Marshal(map[string]interface{}{
		"request_url": "https://example.com/path/" + credential,
		"nested":      []interface{}{map[string]interface{}{"echo": credential}},
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	redacted, err := RedactJSON(data, credential)
	if err != nil {
		t.Fatalf("RedactJSON() error = %v", err)
	}
	if !json.Valid(redacted) || strings.Contains(string(redacted), credential) {
		t.Fatalf("RedactJSON() = %s", redacted)
	}
}
