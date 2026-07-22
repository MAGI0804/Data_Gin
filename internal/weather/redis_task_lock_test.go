package weather

import (
	"context"
	"errors"
	"testing"
	"time"

	redisv8 "github.com/go-redis/redis/v8"
)

func TestRedisTaskLockerAcquiresAndReleasesOwnedLock(t *testing.T) {
	client := &fakeRedisTaskLockClient{setNXResult: true, evalResult: int64(1)}
	locker, err := newRedisTaskLocker(client, "app:lock:mall_weather:", 10*time.Minute, func() (string, error) {
		return "token-1", nil
	})
	if err != nil {
		t.Fatalf("newRedisTaskLocker() error=%v", err)
	}
	lock, acquired, err := locker.Acquire(context.Background(), "7:full:full:7:2026072203")
	if err != nil || !acquired || lock == nil {
		t.Fatalf("Acquire() lock=%v acquired=%t error=%v", lock, acquired, err)
	}
	if client.setNXKey != "app:lock:mall_weather:7:full:full:7:2026072203" || client.setNXValue != "token-1" || client.setNXTTL != 10*time.Minute {
		t.Fatalf("SetNX call=%+v", client)
	}
	if err := lock.Release(context.Background()); err != nil {
		t.Fatalf("Release() error=%v", err)
	}
	if client.evalCalls != 1 || client.evalKeys[0] != client.setNXKey || client.evalArgs[0] != "token-1" {
		t.Fatalf("Eval call=%+v", client)
	}
	if err := lock.Release(context.Background()); err != nil || client.evalCalls != 1 {
		t.Fatalf("second Release() error=%v calls=%d", err, client.evalCalls)
	}
}

func TestRedisTaskLockerDoesNotBypassContentionOrRedisFailure(t *testing.T) {
	tests := []struct {
		name       string
		client     *fakeRedisTaskLockClient
		wantLocked bool
		wantError  bool
	}{
		{name: "already owned", client: &fakeRedisTaskLockClient{}, wantLocked: false},
		{name: "redis unavailable", client: &fakeRedisTaskLockClient{setNXErr: errors.New("redis unavailable")}, wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			locker, err := newRedisTaskLocker(test.client, "app:lock:", time.Minute, func() (string, error) {
				return "token", nil
			})
			if err != nil {
				t.Fatalf("newRedisTaskLocker() error=%v", err)
			}
			_, acquired, err := locker.Acquire(context.Background(), "7:full:window")
			if acquired != test.wantLocked || (err != nil) != test.wantError {
				t.Fatalf("Acquire() acquired=%t error=%v", acquired, err)
			}
		})
	}
}

func TestRedisTaskLockerRejectsUnsafeKey(t *testing.T) {
	locker, err := newRedisTaskLocker(&fakeRedisTaskLockClient{}, "app:lock:", time.Minute, func() (string, error) {
		return "token", nil
	})
	if err != nil {
		t.Fatalf("newRedisTaskLocker() error=%v", err)
	}
	if _, _, err := locker.Acquire(context.Background(), "../unsafe"); err == nil {
		t.Fatal("Acquire() accepted unsafe key")
	}
}

func TestRedisTaskLockAllowsReleaseRetryAfterRedisFailure(t *testing.T) {
	client := &fakeRedisTaskLockClient{setNXResult: true, evalErr: errors.New("redis unavailable")}
	locker, err := newRedisTaskLocker(client, "app:lock:", time.Minute, func() (string, error) {
		return "token", nil
	})
	if err != nil {
		t.Fatalf("newRedisTaskLocker() error=%v", err)
	}
	lock, acquired, err := locker.Acquire(context.Background(), "7:full:window")
	if err != nil || !acquired {
		t.Fatalf("Acquire() acquired=%t error=%v", acquired, err)
	}
	if err := lock.Release(context.Background()); err == nil {
		t.Fatal("Release() error=nil")
	}
	client.evalErr = nil
	client.evalResult = int64(1)
	if err := lock.Release(context.Background()); err != nil || client.evalCalls != 2 {
		t.Fatalf("retry Release() error=%v calls=%d", err, client.evalCalls)
	}
}

type fakeRedisTaskLockClient struct {
	setNXResult bool
	setNXErr    error
	setNXKey    string
	setNXValue  interface{}
	setNXTTL    time.Duration
	evalResult  interface{}
	evalErr     error
	evalCalls   int
	evalKeys    []string
	evalArgs    []interface{}
}

func (client *fakeRedisTaskLockClient) SetNX(_ context.Context, key string, value interface{}, ttl time.Duration) *redisv8.BoolCmd {
	client.setNXKey = key
	client.setNXValue = value
	client.setNXTTL = ttl
	command := redisv8.NewBoolCmd(context.Background())
	command.SetVal(client.setNXResult)
	command.SetErr(client.setNXErr)
	return command
}

func (client *fakeRedisTaskLockClient) Eval(_ context.Context, _ string, keys []string, args ...interface{}) *redisv8.Cmd {
	client.evalCalls++
	client.evalKeys = append([]string(nil), keys...)
	client.evalArgs = append([]interface{}(nil), args...)
	command := redisv8.NewCmd(context.Background())
	command.SetVal(client.evalResult)
	command.SetErr(client.evalErr)
	return command
}
