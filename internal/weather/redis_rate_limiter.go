package weather

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	redisv8 "github.com/go-redis/redis/v8"
)

const redisTokenBucketScript = `
local rate = tonumber(ARGV[1])
local capacity = tonumber(ARGV[2])
local requested = tonumber(ARGV[3])
local ttl_ms = tonumber(ARGV[4])
local redis_time = redis.call("TIME")
local now_ms = redis_time[1] * 1000 + math.floor(redis_time[2] / 1000)
local state = redis.call("HMGET", KEYS[1], "tokens", "updated_ms")
local tokens = tonumber(state[1])
local updated_ms = tonumber(state[2])
if tokens == nil or updated_ms == nil then
  tokens = capacity
  updated_ms = now_ms
else
  local elapsed_ms = math.max(0, now_ms - updated_ms)
  tokens = math.min(capacity, tokens + elapsed_ms * rate / 1000)
end
local allowed = 0
local wait_ms = 0
if tokens >= requested then
  tokens = tokens - requested
  allowed = 1
else
  wait_ms = math.ceil((requested - tokens) * 1000 / rate)
end
redis.call("HMSET", KEYS[1], "tokens", tokens, "updated_ms", now_ms)
redis.call("PEXPIRE", KEYS[1], ttl_ms)
return {allowed, wait_ms}
`

type ProviderRateLimiter interface {
	Wait(ctx context.Context) error
}

type redisRateLimitClient interface {
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redisv8.Cmd
}

type RedisTokenBucketLimiter struct {
	client     redisRateLimitClient
	key        string
	rate       float64
	capacity   int
	stateTTLMS int64
}

func NewRedisTokenBucketLimiter(client *redisv8.Client, key string, rate float64, capacity int) (*RedisTokenBucketLimiter, error) {
	return newRedisTokenBucketLimiter(client, key, rate, capacity)
}

func newRedisTokenBucketLimiter(client redisRateLimitClient, key string, rate float64, capacity int) (*RedisTokenBucketLimiter, error) {
	if client == nil || key == "" || len(key) > 512 || math.IsNaN(rate) || math.IsInf(rate, 0) || rate <= 0 || capacity < 1 {
		return nil, fmt.Errorf("weather rate limiter: invalid configuration")
	}
	stateTTL := 2 * time.Duration(math.Ceil(float64(capacity)/rate*float64(time.Second)))
	if stateTTL < time.Minute {
		stateTTL = time.Minute
	}
	return &RedisTokenBucketLimiter{
		client: client, key: key, rate: rate, capacity: capacity, stateTTLMS: stateTTL.Milliseconds(),
	}, nil
}

func (limiter *RedisTokenBucketLimiter) Wait(ctx context.Context) error {
	if limiter == nil || limiter.client == nil || ctx == nil {
		return fmt.Errorf("weather rate limiter: invalid wait")
	}
	for {
		wait, err := limiter.take(ctx)
		if err != nil {
			return err
		}
		if wait <= 0 {
			return nil
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (limiter *RedisTokenBucketLimiter) take(ctx context.Context) (time.Duration, error) {
	result, err := limiter.client.Eval(
		ctx,
		redisTokenBucketScript,
		[]string{limiter.key},
		strconv.FormatFloat(limiter.rate, 'g', -1, 64),
		limiter.capacity,
		1,
		limiter.stateTTLMS,
	).Result()
	if err != nil {
		return 0, fmt.Errorf("weather rate limiter: reserve token: %w", err)
	}
	values, ok := result.([]interface{})
	if !ok || len(values) != 2 {
		return 0, fmt.Errorf("weather rate limiter: invalid redis response")
	}
	allowed, ok := redisInteger(values[0])
	if !ok || (allowed != 0 && allowed != 1) {
		return 0, fmt.Errorf("weather rate limiter: invalid allow result")
	}
	waitMS, ok := redisInteger(values[1])
	if !ok || waitMS < 0 || waitMS > int64((time.Hour/time.Millisecond)) {
		return 0, fmt.Errorf("weather rate limiter: invalid wait result")
	}
	if allowed == 1 {
		return 0, nil
	}
	if waitMS == 0 {
		waitMS = 1
	}
	return time.Duration(waitMS) * time.Millisecond, nil
}

func redisInteger(value interface{}) (int64, bool) {
	switch typed := value.(type) {
	case int64:
		return typed, true
	case int:
		return int64(typed), true
	case string:
		parsed, err := strconv.ParseInt(typed, 10, 64)
		return parsed, err == nil
	case []byte:
		parsed, err := strconv.ParseInt(string(typed), 10, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}
