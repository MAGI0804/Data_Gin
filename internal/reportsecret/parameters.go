package reportsecret

import (
	"fmt"
	"os"
	"strings"
)

const reportParameterPurpose = "report-run-parameters"

type EnvironmentParameterCipher struct {
	KeyringVariable string
	VersionVariable string
}

func (environment EnvironmentParameterCipher) Encrypt(plaintext []byte) (string, string, error) {
	keyringVariable := strings.TrimSpace(environment.KeyringVariable)
	if keyringVariable == "" {
		keyringVariable = "REPORT_PARAMETER_KEYS_JSON"
	}
	versionVariable := strings.TrimSpace(environment.VersionVariable)
	if versionVariable == "" {
		versionVariable = "REPORT_PARAMETER_KEY_VERSION"
	}
	version := strings.TrimSpace(os.Getenv(versionVariable))
	if version == "" {
		return "", "", credentialError("parameter key version is unavailable")
	}
	keyring, err := ParseKeyring(os.Getenv(keyringVariable))
	if err != nil {
		return "", "", fmt.Errorf("load report parameter keyring: %w", err)
	}
	ciphertext, err := keyring.EncryptScoped(version, reportParameterPurpose, string(plaintext))
	if err != nil {
		return "", "", err
	}
	return version, ciphertext, nil
}

func (environment EnvironmentParameterCipher) Decrypt(version, ciphertext string) ([]byte, error) {
	keyringVariable := strings.TrimSpace(environment.KeyringVariable)
	if keyringVariable == "" {
		keyringVariable = "REPORT_PARAMETER_KEYS_JSON"
	}
	keyring, err := ParseKeyring(os.Getenv(keyringVariable))
	if err != nil {
		return nil, fmt.Errorf("load report parameter keyring: %w", err)
	}
	plaintext, err := keyring.DecryptScoped(version, reportParameterPurpose, ciphertext)
	if err != nil {
		return nil, err
	}
	return []byte(plaintext), nil
}
