package reportsecret

import (
	"encoding/base64"
	"testing"
)

func TestEnvironmentParameterCipherUsesVersionedDedicatedKeyring(t *testing.T) {
	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	t.Setenv("TEST_PARAMETER_KEYS", `{"parameter-v1":"`+key+`"}`)
	t.Setenv("TEST_PARAMETER_VERSION", "parameter-v1")
	cipher := EnvironmentParameterCipher{KeyringVariable: "TEST_PARAMETER_KEYS", VersionVariable: "TEST_PARAMETER_VERSION"}
	version, ciphertext, err := cipher.Encrypt([]byte(`{"token":"secret"}`))
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	plaintext, err := cipher.Decrypt(version, ciphertext)
	if err != nil || string(plaintext) != `{"token":"secret"}` || version != "parameter-v1" {
		t.Fatalf("Decrypt() = %q, version=%q, err=%v", plaintext, version, err)
	}
}
