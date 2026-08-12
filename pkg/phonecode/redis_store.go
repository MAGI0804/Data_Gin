package phonecode

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	goredis "github.com/go-redis/redis/v8"
)

const reserveScript = `
if redis.call("EXISTS", KEYS[2]) == 1 then
  return 0
end
redis.call("SET", KEYS[1], ARGV[1], "PX", ARGV[2])
redis.call("SET", KEYS[3], "0", "PX", ARGV[2])
redis.call("SET", KEYS[2], "1", "PX", ARGV[3])
return 1
`

const consumeScript = `
local stored = redis.call("GET", KEYS[1])
if not stored then
  return 0
end
local attempts = tonumber(redis.call("GET", KEYS[2]) or "0")
if attempts >= tonumber(ARGV[2]) then
  redis.call("DEL", KEYS[1])
  return -2
end
if stored == ARGV[1] then
  redis.call("DEL", KEYS[1], KEYS[2])
  return 1
end
attempts = redis.call("INCR", KEYS[2])
if attempts >= tonumber(ARGV[2]) then
  redis.call("DEL", KEYS[1])
  return -2
end
return -1
`

const cleanupScript = `
if redis.call("GET", KEYS[1]) ~= ARGV[1] then
  return 0
end
redis.call("DEL", KEYS[1], KEYS[2], KEYS[3])
return 1
`

type evaluator interface {
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error)
}

type redisEvaluator struct {
	client *goredis.Client
}

func (e redisEvaluator) Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error) {
	if e.client == nil {
		return nil, fmt.Errorf("phonecode: nil redis client")
	}
	return e.client.Eval(ctx, script, keys, args...).Result()
}

type RedisStore struct {
	evaluator evaluator
	prefix    string
}

func NewRedisStore(client *goredis.Client, namespace string) *RedisStore {
	return &RedisStore{evaluator: redisEvaluator{client: client}, prefix: namespace + "verify_code:"}
}

func newRedisStore(evaluator evaluator, namespace string) *RedisStore {
	return &RedisStore{evaluator: evaluator, prefix: namespace + "verify_code:"}
}

func (s *RedisStore) Reserve(ctx context.Context, key, code string, ttl, cooldown time.Duration) (bool, error) {
	if err := s.validate(ctx); err != nil {
		return false, err
	}
	result, err := s.evaluator.Eval(ctx, reserveScript, []string{s.codeKey(key), s.cooldownKey(key), s.attemptsKey(key)}, code, ttl.Milliseconds(), cooldown.Milliseconds())
	if err != nil {
		return false, fmt.Errorf("reserve verification code: %w", err)
	}
	value, err := integerResult(result)
	if err != nil {
		return false, err
	}
	return value == 1, nil
}

func (s *RedisStore) Consume(ctx context.Context, key, code string, maxErrors int) error {
	if err := s.validate(ctx); err != nil {
		return err
	}
	result, err := s.evaluator.Eval(ctx, consumeScript, []string{s.codeKey(key), s.attemptsKey(key)}, code, maxErrors)
	if err != nil {
		return fmt.Errorf("consume verification code: %w", err)
	}
	value, err := integerResult(result)
	if err != nil {
		return err
	}
	switch value {
	case 1:
		return nil
	case 0:
		return ErrExpired
	case -1:
		return ErrMismatch
	case -2:
		return ErrAttemptsExceeded
	default:
		return fmt.Errorf("consume verification code: unexpected result")
	}
}

func (s *RedisStore) Cleanup(ctx context.Context, key, code string) error {
	if err := s.validate(ctx); err != nil {
		return err
	}
	_, err := s.evaluator.Eval(ctx, cleanupScript, []string{s.codeKey(key), s.attemptsKey(key), s.cooldownKey(key)}, code)
	if err != nil {
		return fmt.Errorf("cleanup verification code: %w", err)
	}
	return nil
}

func (s *RedisStore) validate(ctx context.Context) error {
	if s == nil || s.evaluator == nil || ctx == nil {
		return fmt.Errorf("phonecode: invalid redis store")
	}
	return nil
}

func (s *RedisStore) codeKey(key string) string     { return s.prefix + key + ":code" }
func (s *RedisStore) attemptsKey(key string) string { return s.prefix + key + ":attempts" }
func (s *RedisStore) cooldownKey(key string) string { return s.prefix + key + ":cooldown" }

func integerResult(result interface{}) (int64, error) {
	switch value := result.(type) {
	case int64:
		return value, nil
	case string:
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err == nil {
			return parsed, nil
		}
	}
	return 0, errors.New("phonecode: redis script returned an invalid result")
}
