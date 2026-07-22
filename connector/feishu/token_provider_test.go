package feishu

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestTenantTokenProviderCachesEncryptedTokenInMemoryAndRedis(t *testing.T) {
	now := time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC)
	source := &fakeTenantTokenSource{tokens: []TenantToken{{Value: "tenant-token-value-123", ExpiresIn: 2 * time.Hour}}}
	cache := newFakeTenantTokenCache()
	provider := testTenantTokenProvider(t, source, cache, now)

	first, err := provider.Token(t.Context())
	if err != nil {
		t.Fatalf("Token() error=%v", err)
	}
	second, err := provider.Token(t.Context())
	if err != nil || second != first || source.callCount() != 1 {
		t.Fatalf("second Token()=%q error=%v source calls=%d", second, err, source.callCount())
	}
	encoded, ttl := cache.stored(provider.cacheKey)
	if encoded == "" || strings.Contains(encoded, first) || ttl != 115*time.Minute {
		t.Fatalf("cached value=%q ttl=%v", encoded, ttl)
	}

	secondProcessSource := &fakeTenantTokenSource{err: errors.New("source must not be called")}
	secondProcess := testTenantTokenProvider(t, secondProcessSource, cache, now)
	fromRedis, err := secondProcess.Token(t.Context())
	if err != nil || fromRedis != first || secondProcessSource.callCount() != 0 {
		t.Fatalf("Redis Token()=%q error=%v source calls=%d", fromRedis, err, secondProcessSource.callCount())
	}
}

func TestTenantTokenProviderSingleflightsConcurrentRefresh(t *testing.T) {
	now := time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC)
	source := &fakeTenantTokenSource{
		tokens:  []TenantToken{{Value: "tenant-token-concurrent-123", ExpiresIn: 2 * time.Hour}},
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	provider := testTenantTokenProvider(t, source, newFakeTenantTokenCache(), now)

	const callers = 32
	results := make(chan string, callers)
	errorsFound := make(chan error, callers)
	var wait sync.WaitGroup
	wait.Add(callers)
	for range callers {
		go func() {
			defer wait.Done()
			value, err := provider.Token(context.Background())
			results <- value
			errorsFound <- err
		}()
	}
	<-source.started
	close(source.release)
	wait.Wait()
	close(results)
	close(errorsFound)
	for err := range errorsFound {
		if err != nil {
			t.Fatalf("Token() error=%v", err)
		}
	}
	for value := range results {
		if value != "tenant-token-concurrent-123" {
			t.Fatalf("Token() value=%q", value)
		}
	}
	if source.callCount() != 1 {
		t.Fatalf("source calls=%d", source.callCount())
	}
}

func TestTenantTokenProviderForceRefreshWaitsForNormalLookupThenRefreshes(t *testing.T) {
	now := time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC)
	source := &fakeTenantTokenSource{
		tokens: []TenantToken{
			{Value: "tenant-token-normal-value", ExpiresIn: 2 * time.Hour},
			{Value: "tenant-token-forced-value", ExpiresIn: 2 * time.Hour},
		},
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	provider := testTenantTokenProvider(t, source, newFakeTenantTokenCache(), now)
	normalResult := make(chan string, 1)
	normalError := make(chan error, 1)
	go func() {
		value, err := provider.Token(context.Background())
		normalResult <- value
		normalError <- err
	}()
	<-source.started
	forcedResult := make(chan string, 1)
	forcedError := make(chan error, 1)
	go func() {
		value, err := provider.Refresh(context.Background())
		forcedResult <- value
		forcedError <- err
	}()
	close(source.release)
	if err := <-normalError; err != nil || <-normalResult != "tenant-token-normal-value" {
		t.Fatalf("normal Token() error=%v", err)
	}
	if err := <-forcedError; err != nil || <-forcedResult != "tenant-token-forced-value" {
		t.Fatalf("forced Refresh() error=%v", err)
	}
	if source.callCount() != 2 {
		t.Fatalf("source calls=%d", source.callCount())
	}
}

func TestTenantTokenProviderDoesNotServeMemoryDuringForceRefresh(t *testing.T) {
	now := time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC)
	source := &fakeTenantTokenSource{tokens: []TenantToken{
		{Value: "tenant-token-memory-old", ExpiresIn: 2 * time.Hour},
		{Value: "tenant-token-memory-new", ExpiresIn: 2 * time.Hour},
	}}
	provider := testTenantTokenProvider(t, source, newFakeTenantTokenCache(), now)
	if value, err := provider.Token(t.Context()); err != nil || value != "tenant-token-memory-old" {
		t.Fatalf("initial Token()=%q error=%v", value, err)
	}
	source.started = make(chan struct{})
	source.release = make(chan struct{})
	refreshResult := make(chan string, 1)
	refreshError := make(chan error, 1)
	go func() {
		value, err := provider.Refresh(context.Background())
		refreshResult <- value
		refreshError <- err
	}()
	<-source.started
	tokenResult := make(chan string, 1)
	tokenError := make(chan error, 1)
	go func() {
		value, err := provider.Token(context.Background())
		tokenResult <- value
		tokenError <- err
	}()
	close(source.release)
	if err := <-refreshError; err != nil || <-refreshResult != "tenant-token-memory-new" {
		t.Fatalf("Refresh() error=%v", err)
	}
	if err := <-tokenError; err != nil || <-tokenResult != "tenant-token-memory-new" {
		t.Fatalf("Token() during refresh error=%v", err)
	}
}

func TestTenantTokenProviderRefreshInvalidatesBothCaches(t *testing.T) {
	now := time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC)
	source := &fakeTenantTokenSource{tokens: []TenantToken{
		{Value: "tenant-token-old-value", ExpiresIn: 2 * time.Hour},
		{Value: "tenant-token-new-value", ExpiresIn: 2 * time.Hour},
	}}
	cache := newFakeTenantTokenCache()
	provider := testTenantTokenProvider(t, source, cache, now)
	oldValue, err := provider.Token(t.Context())
	if err != nil {
		t.Fatalf("Token() error=%v", err)
	}
	newValue, err := provider.Refresh(t.Context())
	if err != nil || oldValue == newValue || newValue != "tenant-token-new-value" || cache.deleteCount() != 1 ||
		source.callCount() != 2 {
		t.Fatalf("Refresh() old=%q new=%q error=%v deletes=%d calls=%d", oldValue, newValue, err, cache.deleteCount(), source.callCount())
	}
}

func TestTenantTokenProviderDoesNotBypassRedisFailure(t *testing.T) {
	now := time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC)
	source := &fakeTenantTokenSource{tokens: []TenantToken{{Value: "tenant-token-value-123", ExpiresIn: 2 * time.Hour}}}
	cache := newFakeTenantTokenCache()
	cache.getErr = errors.New("redis unavailable")
	provider := testTenantTokenProvider(t, source, cache, now)
	if _, err := provider.Token(t.Context()); err == nil || source.callCount() != 0 {
		t.Fatalf("Token() error=%v source calls=%d", err, source.callCount())
	}
}

func TestTenantTokenProviderReplacesUnauthenticatedCacheEntry(t *testing.T) {
	now := time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC)
	source := &fakeTenantTokenSource{tokens: []TenantToken{{Value: "tenant-token-recovered", ExpiresIn: 2 * time.Hour}}}
	cache := newFakeTenantTokenCache()
	provider := testTenantTokenProvider(t, source, cache, now)
	cache.values[provider.cacheKey] = "not-authenticated-ciphertext"
	value, err := provider.Token(t.Context())
	if err != nil || value != "tenant-token-recovered" || cache.deleteCount() != 1 || source.callCount() != 1 {
		t.Fatalf("Token()=%q error=%v deletes=%d calls=%d", value, err, cache.deleteCount(), source.callCount())
	}
}

func TestTenantTokenCacheTTLRefreshesEarly(t *testing.T) {
	for _, test := range []struct {
		expires time.Duration
		want    time.Duration
		err     bool
	}{
		{expires: 2 * time.Hour, want: 115 * time.Minute},
		{expires: 5 * time.Minute, want: time.Minute},
		{expires: time.Minute, want: time.Minute},
		{expires: 30 * time.Second, err: true},
	} {
		got, err := tenantTokenCacheTTL(test.expires)
		if (err != nil) != test.err || got != test.want {
			t.Fatalf("tenantTokenCacheTTL(%v)=%v error=%v", test.expires, got, err)
		}
	}
}

func testTenantTokenProvider(
	t *testing.T,
	source tenantTokenSource,
	cache tenantTokenCache,
	now time.Time,
) *TenantTokenProvider {
	t.Helper()
	provider, err := newTenantTokenProvider(
		source,
		cache,
		"cli_app",
		"private-secret",
		func() time.Time { return now },
		bytes.NewReader(make([]byte, 4096)),
	)
	if err != nil {
		t.Fatalf("newTenantTokenProvider() error=%v", err)
	}
	return provider
}

type fakeTenantTokenSource struct {
	mu      sync.Mutex
	tokens  []TenantToken
	err     error
	calls   int
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (source *fakeTenantTokenSource) Fetch(ctx context.Context) (TenantToken, error) {
	source.mu.Lock()
	index := source.calls
	source.calls++
	var token TenantToken
	if len(source.tokens) > 0 {
		if index >= len(source.tokens) {
			index = len(source.tokens) - 1
		}
		token = source.tokens[index]
	}
	err := source.err
	source.mu.Unlock()
	if source.started != nil {
		source.once.Do(func() { close(source.started) })
	}
	if source.release != nil {
		select {
		case <-ctx.Done():
			return TenantToken{}, ctx.Err()
		case <-source.release:
		}
	}
	return token, err
}

func (source *fakeTenantTokenSource) callCount() int {
	source.mu.Lock()
	defer source.mu.Unlock()
	return source.calls
}

type fakeTenantTokenCache struct {
	mu      sync.Mutex
	values  map[string]string
	ttls    map[string]time.Duration
	getErr  error
	setErr  error
	delErr  error
	deletes int
}

func newFakeTenantTokenCache() *fakeTenantTokenCache {
	return &fakeTenantTokenCache{values: make(map[string]string), ttls: make(map[string]time.Duration)}
}

func (cache *fakeTenantTokenCache) Get(_ context.Context, key string) (string, error) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.getErr != nil {
		return "", cache.getErr
	}
	value, ok := cache.values[key]
	if !ok {
		return "", errTenantTokenCacheMiss
	}
	return value, nil
}

func (cache *fakeTenantTokenCache) Set(_ context.Context, key, value string, ttl time.Duration) error {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.setErr != nil {
		return cache.setErr
	}
	cache.values[key] = value
	cache.ttls[key] = ttl
	return nil
}

func (cache *fakeTenantTokenCache) Delete(_ context.Context, key string) error {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.delErr != nil {
		return cache.delErr
	}
	delete(cache.values, key)
	delete(cache.ttls, key)
	cache.deletes++
	return nil
}

func (cache *fakeTenantTokenCache) stored(key string) (string, time.Duration) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	return cache.values[key], cache.ttls[key]
}

func (cache *fakeTenantTokenCache) deleteCount() int {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	return cache.deletes
}
