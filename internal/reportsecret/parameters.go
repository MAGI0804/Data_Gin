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

// Validate verifies the dedicated key used for sensitive report parameters.
func (environment EnvironmentParameterCipher) Validate() error {
	keyringVariable, versionVariable := environment.variables()
	version := strings.TrimSpace(os.Getenv(versionVariable))
	if version == "" {
		return credentialError("parameter key version is unavailable")
	}
	keyring, err := ParseKeyring(os.Getenv(keyringVariable))
	if err != nil {
		return fmt.Errorf("load report parameter keyring: %w", err)
	}
	if _, err := keyring.aead(version); err != nil {
		return err
	}
	return nil
}

func (environment EnvironmentParameterCipher) Encrypt(plaintext []byte) (string, string, error) {
	keyringVariable, versionVariable := environment.variables()
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
	keyringVariable, _ := environment.variables()
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

func (environment EnvironmentParameterCipher) variables() (string, string) {
	keyringVariable := strings.TrimSpace(environment.KeyringVariable)
	if keyringVariable == "" {
		keyringVariable = "REPORT_PARAMETER_KEYS_JSON"
	}
	versionVariable := strings.TrimSpace(environment.VersionVariable)
	if versionVariable == "" {
		versionVariable = "REPORT_PARAMETER_KEY_VERSION"
	}
	return keyringVariable, versionVariable
}
