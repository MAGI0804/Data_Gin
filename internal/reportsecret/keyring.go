package reportsecret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

var ErrInvalidCredential = errors.New("invalid report credential")

const credentialPrefix = "v1:"

type Keyring struct {
	keys map[string][]byte
}

func ParseKeyring(raw string) (*Keyring, error) {
	var encoded map[string]string
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&encoded); err != nil || len(encoded) == 0 {
		return nil, credentialError("keyring must be a non-empty JSON object")
	}
	keys := make(map[string][]byte, len(encoded))
	for version, value := range encoded {
		version = strings.TrimSpace(version)
		key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(value))
		if version == "" || err != nil || len(key) != 32 {
			return nil, credentialError("keyring contains an invalid AES-256 key")
		}
		keys[version] = append([]byte(nil), key...)
	}
	return &Keyring{keys: keys}, nil
}

func (keyring *Keyring) Encrypt(version, plaintext string) (string, error) {
	aead, err := keyring.aead(version)
	if err != nil {
		return "", err
	}
	if plaintext == "" {
		return "", credentialError("plaintext is required")
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("encrypt report credential: generate nonce: %w", err)
	}
	sealed := aead.Seal(nonce, nonce, []byte(plaintext), credentialAAD(version))
	return credentialPrefix + base64.RawStdEncoding.EncodeToString(sealed), nil
}

func (keyring *Keyring) Decrypt(version, ciphertext string) (string, error) {
	aead, err := keyring.aead(version)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(ciphertext, credentialPrefix) {
		return "", credentialError("ciphertext format is invalid")
	}
	encoded := strings.TrimPrefix(ciphertext, credentialPrefix)
	sealed, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil || len(sealed) <= aead.NonceSize() {
		return "", credentialError("ciphertext format is invalid")
	}
	nonce, encrypted := sealed[:aead.NonceSize()], sealed[aead.NonceSize():]
	plaintext, err := aead.Open(nil, nonce, encrypted, credentialAAD(version))
	if err != nil {
		return "", credentialError("ciphertext authentication failed")
	}
	if len(plaintext) == 0 {
		return "", credentialError("decrypted credential is empty")
	}
	return string(plaintext), nil
}

func (keyring *Keyring) aead(version string) (cipher.AEAD, error) {
	if keyring == nil || len(keyring.keys) == 0 {
		return nil, credentialError("keyring is unavailable")
	}
	version = strings.TrimSpace(version)
	key, exists := keyring.keys[version]
	if !exists || len(key) != 32 {
		return nil, credentialError("credential key version is unavailable")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("initialize report credential cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("initialize report credential GCM: %w", err)
	}
	return aead, nil
}

func credentialAAD(version string) []byte {
	return []byte("Data_Gin/report-datasource/" + strings.TrimSpace(version))
}

func credentialError(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalidCredential, message)
}
