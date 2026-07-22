package weather

import (
	"context"
	"errors"
	"testing"
	"time"

	redisv8 "github.com/go-redis/redis/v8"
)

func TestRedisTokenBucketLimiterWaitsForAtomicReservation(t *testing.T) {
	client := &fakeRedisRateLimitClient{results: []interface{}{
		[]interface{}{int64(0), int64(1)},
		[]interface{}{int64(1), int64(0)},
	}}
	limiter, err := newRedisTokenBucketLimiter(client, "app:rate:caiyun", 2.5, 3)
	if err != nil {
		t.Fatalf("newRedisTokenBucketLimiter() error=%v", err)
	}
	if err := limiter.Wait(context.Background()); err != nil {
		t.Fatalf("Wait() error=%v", err)
	}
	if client.calls != 2 || client.keys[0] != "app:rate:caiyun" || client.args[0] != "2.5" {
		t.Fatalf("Eval call=%+v", client)
	}
}

func TestRedisTokenBucketLimiterPropagatesRedisFailure(t *testing.T) {
	client := &fakeRedisRateLimitClient{errors: []error{errors.New("redis unavailable")}}
	limiter, err := newRedisTokenBucketLimiter(client, "app:rate:caiyun", 1, 1)
	if err != nil {
		t.Fatalf("newRedisTokenBucketLimiter() error=%v", err)
	}
	if err := limiter.Wait(context.Background()); err == nil {
		t.Fatal("Wait() error=nil")
	}
}

func TestRedisTokenBucketLimiterHonorsCancellation(t *testing.T) {
	client := &fakeRedisRateLimitClient{results: []interface{}{
		[]interface{}{int64(0), int64(1000)},
	}}
	limiter, err := newRedisTokenBucketLimiter(client, "app:rate:caiyun", 1, 1)
	if err != nil {
		t.Fatalf("newRedisTokenBucketLimiter() error=%v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	if err := limiter.Wait(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Wait() error=%v", err)
	}
	if time.Since(started) > 100*time.Millisecond {
		t.Fatal("Wait() did not stop promptly")
	}
}

type fakeRedisRateLimitClient struct {
	results []interface{}
	errors  []error
	calls   int
	keys    []string
	args    []interface{}
}

func (client *fakeRedisRateLimitClient) Eval(_ context.Context, _ string, keys []string, args ...interface{}) *redisv8.Cmd {
	index := client.calls
	client.calls++
	client.keys = append([]string(nil), keys...)
	client.args = append([]interface{}(nil), args...)
	command := redisv8.NewCmd(context.Background())
	if index < len(client.results) {
		command.SetVal(client.results[index])
	}
	if index < len(client.errors) {
		command.SetErr(client.errors[index])
	}
	return command
}
