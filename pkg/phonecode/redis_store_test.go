package phonecode

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

type evalCall struct {
	script string
	keys   []string
	args   []interface{}
}

type fakeEvaluator struct {
	result interface{}
	err    error
	calls  []evalCall
}

func (e *fakeEvaluator) Eval(_ context.Context, script string, keys []string, args ...interface{}) (interface{}, error) {
	e.calls = append(e.calls, evalCall{script: script, keys: keys, args: args})
	return e.result, e.err
}

func TestRedisStoreReserveUsesAtomicScriptAndNamespacedKeys(t *testing.T) {
	evaluator := &fakeEvaluator{result: int64(1)}
	store := newRedisStore(evaluator, "app:")

	reserved, err := store.Reserve(context.Background(), "sms:LOGIN:13800138000", "123456", 5*time.Minute, time.Minute)
	if err != nil || !reserved {
		t.Fatalf("Reserve() = %v, %v", reserved, err)
	}
	wantKeys := []string{
		"app:verify_code:sms:LOGIN:13800138000:code",
		"app:verify_code:sms:LOGIN:13800138000:cooldown",
		"app:verify_code:sms:LOGIN:13800138000:attempts",
	}
	if len(evaluator.calls) != 1 || evaluator.calls[0].script != reserveScript || !reflect.DeepEqual(evaluator.calls[0].keys, wantKeys) {
		t.Fatalf("unexpected eval call: %#v", evaluator.calls)
	}
}

func TestRedisStoreConsumeMapsScriptResults(t *testing.T) {
	tests := []struct {
		name    string
		result  int64
		wantErr error
	}{
		{name: "consumed", result: 1},
		{name: "expired", result: 0, wantErr: ErrExpired},
		{name: "mismatch", result: -1, wantErr: ErrMismatch},
		{name: "attempts exceeded", result: -2, wantErr: ErrAttemptsExceeded},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluator := &fakeEvaluator{result: tt.result}
			store := newRedisStore(evaluator, "")
			err := store.Consume(context.Background(), "sms:PASSWORD_RESET:13800138000", "123456", MaxVerifyErrors)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Consume() error = %v, want %v", err, tt.wantErr)
			}
			if evaluator.calls[0].script != consumeScript {
				t.Fatalf("Consume() did not use consume Lua script")
			}
		})
	}
}

func TestRedisStorePropagatesRedisFailure(t *testing.T) {
	redisErr := errors.New("redis unavailable")
	store := newRedisStore(&fakeEvaluator{err: redisErr}, "")
	if _, err := store.Reserve(context.Background(), "key", "123456", CodeTTL, ResendCooldown); !errors.Is(err, redisErr) {
		t.Fatalf("Reserve() error = %v, want wrapped Redis error", err)
	}
	if err := store.Consume(context.Background(), "key", "123456", MaxVerifyErrors); !errors.Is(err, redisErr) {
		t.Fatalf("Consume() error = %v, want wrapped Redis error", err)
	}
}

func TestRedisStoreCleanupUsesCompareAndDeleteScript(t *testing.T) {
	evaluator := &fakeEvaluator{result: int64(1)}
	store := newRedisStore(evaluator, "")
	if err := store.Cleanup(context.Background(), "key", "123456"); err != nil {
		t.Fatal(err)
	}
	if evaluator.calls[0].script != cleanupScript {
		t.Fatalf("Cleanup() did not use compare-and-delete Lua script")
	}
}
