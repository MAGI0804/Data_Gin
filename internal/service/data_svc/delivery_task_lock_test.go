package data_svc

import (
	"context"
	"errors"
	"testing"

	weatherdomain "gin-biz-web-api/internal/weather"
)

type fakeDeliveryTaskLock struct{}

func (fakeDeliveryTaskLock) Release(context.Context) error { return nil }

type fakeDeliveryTaskLocker struct {
	key      string
	acquired bool
	err      error
}

func (locker *fakeDeliveryTaskLocker) Acquire(_ context.Context, key string) (weatherdomain.TaskLock, bool, error) {
	locker.key = key
	if locker.err != nil {
		return nil, false, locker.err
	}
	if !locker.acquired {
		return nil, false, nil
	}
	return fakeDeliveryTaskLock{}, true, nil
}

func TestAcquireDeliveryTaskLockScopesToTaskAndRejectsBusy(t *testing.T) {
	locker := &fakeDeliveryTaskLocker{acquired: true}
	service := &DeliveryService{taskLocker: locker}
	if _, err := service.acquireDeliveryTaskLock(context.Background(), 42); err != nil {
		t.Fatalf("acquireDeliveryTaskLock() error = %v", err)
	}
	if locker.key != "task:42" {
		t.Fatalf("lock key = %q, want task:42", locker.key)
	}

	locker.acquired = false
	if _, err := service.acquireDeliveryTaskLock(context.Background(), 42); !errors.Is(err, ErrDeliveryTaskBusy) {
		t.Fatalf("busy error = %v, want ErrDeliveryTaskBusy", err)
	}
}

func TestAcquireDeliveryTaskLockFailsClosed(t *testing.T) {
	service := &DeliveryService{taskLocker: &fakeDeliveryTaskLocker{err: errors.New("redis unavailable")}}
	if _, err := service.acquireDeliveryTaskLock(context.Background(), 1); err == nil || errors.Is(err, ErrDeliveryTaskBusy) {
		t.Fatalf("acquire error = %v, want unavailable lock error", err)
	}
}
