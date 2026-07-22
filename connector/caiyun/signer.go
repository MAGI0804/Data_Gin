// Package caiyun implements credential-safe request construction for Caiyun
// weather APIs. Provider response parsing is kept behind separate boundaries.
package caiyun

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

const (
	minimumNonceLength = 16
	maximumNonceLength = 40
)

var (
	appKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
	noncePattern  = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
)

// Signer implements the shared v2.6/v3 HMAC-SHA256 contract. Credentials are
// private and all diagnostic/serialization surfaces are explicitly redacted.
type Signer struct {
	appKey    string
	appSecret string
}

func NewSigner(appKey, appSecret string) (*Signer, error) {
	if !appKeyPattern.MatchString(appKey) || strings.TrimSpace(appSecret) == "" {
		return nil, fmt.Errorf("caiyun signer: credentials are required")
	}
	return &Signer{appKey: appKey, appSecret: appSecret}, nil
}

func (Signer) String() string   { return "caiyun.Signer{redacted}" }
func (Signer) GoString() string { return "caiyun.Signer{redacted}" }
func (Signer) MarshalJSON() ([]byte, error) {
	return []byte("{}"), nil
}

// Sign returns padded URL-safe Base64 over the canonical request components.
// The caller must send exactly the same escaped path and encoded query.
func (signer *Signer) Sign(method, escapedPath, nonce string, timestamp int64, query url.Values) (string, error) {
	if signer == nil || !appKeyPattern.MatchString(signer.appKey) || strings.TrimSpace(signer.appSecret) == "" {
		return "", fmt.Errorf("caiyun signer: not configured")
	}
	method = strings.ToUpper(strings.TrimSpace(method))
	if method != http.MethodGet || !validEscapedPath(escapedPath) || !validNonce(nonce) || timestamp <= 0 {
		return "", fmt.Errorf("caiyun signer: invalid signing input")
	}
	canonicalQuery := cloneValues(query).Encode()
	stringToSign := strings.Join([]string{
		method,
		escapedPath,
		canonicalQuery,
		signer.appKey,
		nonce,
		strconv.FormatInt(timestamp, 10),
	}, ":")

	mac := hmac.New(sha256.New, []byte(signer.appSecret))
	if _, err := mac.Write([]byte(stringToSign)); err != nil {
		return "", fmt.Errorf("caiyun signer: calculate signature")
	}
	return base64.URLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func validEscapedPath(path string) bool {
	if path == "" || path[0] != '/' || strings.ContainsAny(path, "?#\r\n") {
		return false
	}
	unescaped, err := url.PathUnescape(path)
	return err == nil && !strings.ContainsAny(unescaped, "?#\r\n")
}

func validNonce(nonce string) bool {
	return len(nonce) >= minimumNonceLength && len(nonce) <= maximumNonceLength && noncePattern.MatchString(nonce)
}

func cloneValues(source url.Values) url.Values {
	cloned := make(url.Values, len(source))
	for key, values := range source {
		cloned[key] = append([]string(nil), values...)
	}
	return cloned
}
