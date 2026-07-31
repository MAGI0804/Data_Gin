package configsecret

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestRedactJSONHandlesCamelCaseAndInvalidURL(t *testing.T) {
	redacted, hasSecret := RedactJSON(`{"apiKey":"value","privateKey":"value","url":"https://user:password@example.test/path?accessToken=value"}`)
	if !hasSecret || strings.Contains(redacted, "value") || !strings.Contains(redacted, Placeholder) {
		t.Fatalf("redacted = %s, hasSecret = %t", redacted, hasSecret)
	}

	redacted, hasSecret = RedactJSON(`{"url":"http://[::1"}`)
	if !hasSecret || !strings.Contains(redacted, Placeholder) {
		t.Fatalf("invalid URL redacted = %s, hasSecret = %t", redacted, hasSecret)
	}
}

func TestMergeJSONPreservesMaskedSecretsAndAllowsRotation(t *testing.T) {
	existing := `{"apiKey":"old","headers":{"Authorization":"Bearer old"},"url":"https://user:password@example.test/path?accessToken=old&store=all"}`
	redacted, _ := RedactJSON(existing)
	merged, err := MergeJSON(existing, redacted)
	if err != nil {
		t.Fatalf("MergeJSON preserve returned error: %v", err)
	}
	var got, want interface{}
	if err := json.Unmarshal([]byte(merged), &got); err != nil {
		t.Fatalf("decode merged config: %v", err)
	}
	if err := json.Unmarshal([]byte(existing), &want); err != nil {
		t.Fatalf("decode existing config: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("merged = %s, want existing = %s", merged, existing)
	}

	rotated, err := MergeJSON(existing, `{"apiKey":"new","headers":{"Authorization":"Bearer new"},"url":"https://new-user:new-password@example.test/path?accessToken=new&store=all"}`)
	if err != nil {
		t.Fatalf("MergeJSON rotate returned error: %v", err)
	}
	for _, forbidden := range []string{"\"old\"", "Bearer old", "accessToken=old"} {
		if strings.Contains(rotated, forbidden) {
			t.Fatalf("rotated config retained %q: %s", forbidden, rotated)
		}
	}
}

func TestMergeJSONRejectsUnsafeMaskedSecretUpdates(t *testing.T) {
	existing := `{"token":"old","nested":[{"name":"api_key","value":"old"}]}`
	for _, submitted := range []string{
		`{"nested":[{"name":"api_key","value":"[已隐藏]"}]}`,
		`{"token":""}`,
	} {
		if _, err := MergeJSON(existing, submitted); err == nil {
			t.Fatalf("MergeJSON(%s) returned nil error", submitted)
		}
	}
	if _, err := NewJSON(`{"token":"[已隐藏]"}`, "{}"); err == nil {
		t.Fatal("NewJSON accepted a redacted placeholder")
	}
}
