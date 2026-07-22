package feishu

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	redisv8 "github.com/go-redis/redis/v8"
)

const (
	tenantTokenRefreshBefore = 5 * time.Minute
	minimumTenantTokenTTL    = time.Minute
	maxCachedTokenBytes      = 16 * 1024
	tokenCacheVersion        = 1
)

var errTenantTokenCacheMiss = errors.New("feishu token cache: miss")

type tenantTokenSource interface {
	Fetch(context.Context) (TenantToken, error)
}

type tenantTokenCache interface {
	Get(context.Context, string) (string, error)
	Set(context.Context, string, string, time.Duration) error
	Delete(context.Context, string) error
}

type redisTenantTokenCache struct {
	client *redisv8.Client
}

func (cache redisTenantTokenCache) Get(ctx context.Context, key string) (string, error) {
	value, err := cache.client.Get(ctx, key).Result()
	if errors.Is(err, redisv8.Nil) {
		return "", errTenantTokenCacheMiss
	}
	if err != nil {
		return "", fmt.Errorf("feishu token cache: get: %w", err)
	}
	return value, nil
}

func (cache redisTenantTokenCache) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	if err := cache.client.Set(ctx, key, value, ttl).Err(); err != nil {
		return fmt.Errorf("feishu token cache: set: %w", err)
	}
	return nil
}

func (cache redisTenantTokenCache) Delete(ctx context.Context, key string) error {
	if err := cache.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("feishu token cache: delete: %w", err)
	}
	return nil
}

type cachedTenantToken struct {
	Value       string
	UsableUntil time.Time
}

type tenantTokenRefreshCall struct {
	done  chan struct{}
	force bool
	value string
	err   error
}

type TenantTokenProvider struct {
	source   tenantTokenSource
	cache    tenantTokenCache
	cacheKey string
	aead     cipher.AEAD
	now      func() time.Time
	random   io.Reader

	mu      sync.Mutex
	memory  cachedTenantToken
	refresh *tenantTokenRefreshCall
}

func NewTenantTokenProvider(
	appID string,
	appSecret string,
	httpClient *http.Client,
	redisClient *redisv8.Client,
) (*TenantTokenProvider, error) {
	if redisClient == nil {
		return nil, fmt.Errorf("feishu token provider: Redis is required")
	}
	source, err := NewTokenClient(appID, appSecret, httpClient)
	if err != nil {
		return nil, err
	}
	return newTenantTokenProvider(source, redisTenantTokenCache{client: redisClient}, appID, appSecret, time.Now, rand.Reader)
}

func newTenantTokenProvider(
	source tenantTokenSource,
	cache tenantTokenCache,
	appID string,
	appSecret string,
	now func() time.Time,
	random io.Reader,
) (*TenantTokenProvider, error) {
	if source == nil || cache == nil || !validTokenCredential(appID) || !validTokenCredential(appSecret) ||
		now == nil || random == nil {
		return nil, fmt.Errorf("feishu token provider: invalid configuration")
	}
	keyMaterial := sha256.Sum256([]byte("feishu-token-cache:v1:" + appSecret))
	block, err := aes.NewCipher(keyMaterial[:])
	if err != nil {
		return nil, fmt.Errorf("feishu token provider: create cache cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("feishu token provider: create cache AEAD: %w", err)
	}
	appIDHash := sha256.Sum256([]byte(appID))
	cacheKey := "feishu:tenant_access_token:" + hex.EncodeToString(appIDHash[:])
	return &TenantTokenProvider{
		source: source, cache: cache, cacheKey: cacheKey, aead: aead, now: now, random: random,
	}, nil
}

func (provider *TenantTokenProvider) Token(ctx context.Context) (string, error) {
	return provider.token(ctx, false)
}

func (provider *TenantTokenProvider) Refresh(ctx context.Context) (string, error) {
	return provider.token(ctx, true)
}

func (provider *TenantTokenProvider) token(ctx context.Context, forceRefresh bool) (string, error) {
	if provider == nil || provider.source == nil || provider.cache == nil || provider.aead == nil ||
		provider.now == nil || provider.random == nil || provider.cacheKey == "" || ctx == nil {
		return "", fmt.Errorf("feishu token provider: invalid request")
	}
	for {
		if !forceRefresh {
			if value, ok := provider.memoryToken(provider.now().UTC()); ok {
				return value, nil
			}
		}
		provider.mu.Lock()
		if call := provider.refresh; call != nil {
			joinCall := !forceRefresh || call.force
			provider.mu.Unlock()
			select {
			case <-ctx.Done():
				return "", fmt.Errorf("feishu token provider: wait for refresh: %w", ctx.Err())
			case <-call.done:
				if joinCall {
					return call.value, call.err
				}
				continue
			}
		}
		call := &tenantTokenRefreshCall{done: make(chan struct{}), force: forceRefresh}
		provider.refresh = call
		provider.mu.Unlock()

		call.value, call.err = provider.loadOrRefresh(ctx, forceRefresh)
		provider.mu.Lock()
		provider.refresh = nil
		close(call.done)
		provider.mu.Unlock()
		return call.value, call.err
	}
}

func (provider *TenantTokenProvider) loadOrRefresh(ctx context.Context, forceRefresh bool) (string, error) {
	now := provider.now().UTC()
	if now.IsZero() {
		return "", fmt.Errorf("feishu token provider: invalid clock")
	}
	if forceRefresh {
		provider.clearMemory()
		if err := provider.cache.Delete(ctx, provider.cacheKey); err != nil {
			return "", err
		}
	} else {
		if value, ok := provider.memoryToken(now); ok {
			return value, nil
		}
		entry, err := provider.loadRedis(ctx, now)
		if err == nil {
			provider.storeMemory(entry)
			return entry.Value, nil
		}
		if !errors.Is(err, errTenantTokenCacheMiss) {
			return "", err
		}
	}

	token, err := provider.source.Fetch(ctx)
	if err != nil {
		return "", err
	}
	if !validTenantToken(token.Value) {
		return "", fmt.Errorf("feishu token provider: source returned invalid token")
	}
	ttl, err := tenantTokenCacheTTL(token.ExpiresIn)
	if err != nil {
		return "", err
	}
	entry := cachedTenantToken{Value: token.Value, UsableUntil: now.Add(ttl)}
	encoded, err := provider.encrypt(entry)
	if err != nil {
		return "", err
	}
	if err := provider.cache.Set(ctx, provider.cacheKey, encoded, ttl); err != nil {
		return "", err
	}
	provider.storeMemory(entry)
	return token.Value, nil
}

func (provider *TenantTokenProvider) loadRedis(ctx context.Context, now time.Time) (cachedTenantToken, error) {
	encoded, err := provider.cache.Get(ctx, provider.cacheKey)
	if err != nil {
		return cachedTenantToken{}, err
	}
	entry, err := provider.decrypt(encoded)
	if err != nil || !entry.UsableUntil.After(now) || entry.UsableUntil.After(now.Add(maxTenantTokenLifetime)) {
		if deleteErr := provider.cache.Delete(ctx, provider.cacheKey); deleteErr != nil {
			return cachedTenantToken{}, errors.Join(err, deleteErr)
		}
		return cachedTenantToken{}, errTenantTokenCacheMiss
	}
	return entry, nil
}

func (provider *TenantTokenProvider) memoryToken(now time.Time) (string, bool) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	if provider.refresh != nil && provider.refresh.force {
		return "", false
	}
	if provider.memory.Value == "" || !provider.memory.UsableUntil.After(now) {
		provider.memory = cachedTenantToken{}
		return "", false
	}
	return provider.memory.Value, true
}

func (provider *TenantTokenProvider) storeMemory(entry cachedTenantToken) {
	provider.mu.Lock()
	provider.memory = entry
	provider.mu.Unlock()
}

func (provider *TenantTokenProvider) clearMemory() {
	provider.mu.Lock()
	provider.memory = cachedTenantToken{}
	provider.mu.Unlock()
}

func (provider *TenantTokenProvider) encrypt(entry cachedTenantToken) (string, error) {
	if !validTenantToken(entry.Value) || entry.UsableUntil.IsZero() {
		return "", fmt.Errorf("feishu token provider: invalid cache entry")
	}
	plaintext, err := json.Marshal(struct {
		Version     int    `json:"v"`
		Token       string `json:"token"`
		UsableUntil int64  `json:"usableUntil"`
	}{Version: tokenCacheVersion, Token: entry.Value, UsableUntil: entry.UsableUntil.Unix()})
	if err != nil {
		return "", fmt.Errorf("feishu token provider: encode cache entry: %w", err)
	}
	nonce := make([]byte, provider.aead.NonceSize())
	if _, err := io.ReadFull(provider.random, nonce); err != nil {
		return "", fmt.Errorf("feishu token provider: generate cache nonce: %w", err)
	}
	ciphertext := provider.aead.Seal(nil, nonce, plaintext, []byte(provider.cacheKey))
	encoded := base64.RawURLEncoding.EncodeToString(append(nonce, ciphertext...))
	if len(encoded) > maxCachedTokenBytes {
		return "", fmt.Errorf("feishu token provider: cache entry is too large")
	}
	return encoded, nil
}

func (provider *TenantTokenProvider) decrypt(encoded string) (cachedTenantToken, error) {
	if encoded == "" || len(encoded) > maxCachedTokenBytes {
		return cachedTenantToken{}, fmt.Errorf("feishu token provider: invalid cached token")
	}
	data, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(data) <= provider.aead.NonceSize() {
		return cachedTenantToken{}, fmt.Errorf("feishu token provider: decode cached token")
	}
	nonce, ciphertext := data[:provider.aead.NonceSize()], data[provider.aead.NonceSize():]
	plaintext, err := provider.aead.Open(nil, nonce, ciphertext, []byte(provider.cacheKey))
	if err != nil {
		return cachedTenantToken{}, fmt.Errorf("feishu token provider: authenticate cached token")
	}
	var decoded struct {
		Version     int    `json:"v"`
		Token       string `json:"token"`
		UsableUntil int64  `json:"usableUntil"`
	}
	decoder := json.NewDecoder(bytes.NewReader(plaintext))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil {
		return cachedTenantToken{}, fmt.Errorf("feishu token provider: decode cache payload: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) || decoded.Version != tokenCacheVersion ||
		!validTenantToken(decoded.Token) || decoded.UsableUntil <= 0 {
		return cachedTenantToken{}, fmt.Errorf("feishu token provider: invalid cache payload")
	}
	return cachedTenantToken{Value: decoded.Token, UsableUntil: time.Unix(decoded.UsableUntil, 0).UTC()}, nil
}

func tenantTokenCacheTTL(expiresIn time.Duration) (time.Duration, error) {
	if expiresIn < minimumTenantTokenTTL || expiresIn > maxTenantTokenLifetime {
		return 0, fmt.Errorf("feishu token provider: invalid token lifetime")
	}
	ttl := expiresIn - tenantTokenRefreshBefore
	if ttl < minimumTenantTokenTTL {
		ttl = minimumTenantTokenTTL
	}
	return ttl, nil
}
