package phonecode

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeStore struct {
	reserved   bool
	reserveErr error
	consumeErr error
	cleanupErr error
	keys       []string
	code       string
	cleanup    int
	ttl        time.Duration
	cooldown   time.Duration
}

func (s *fakeStore) Reserve(_ context.Context, key, code string, ttl, cooldown time.Duration) (bool, error) {
	s.keys = append(s.keys, key)
	s.code = code
	s.ttl = ttl
	s.cooldown = cooldown
	return s.reserved, s.reserveErr
}

func (s *fakeStore) Consume(_ context.Context, key, code string, _ int) error {
	s.keys = append(s.keys, key)
	s.code = code
	return s.consumeErr
}

func (s *fakeStore) Cleanup(_ context.Context, key, code string) error {
	s.keys = append(s.keys, key)
	s.code = code
	s.cleanup++
	return s.cleanupErr
}

type senderFunc func(context.Context, string, string) error

func (f senderFunc) SendCode(ctx context.Context, phoneNumber, code string) error {
	return f(ctx, phoneNumber, code)
}

func TestServiceIssue(t *testing.T) {
	sendErr := errors.New("provider unavailable")
	tests := []struct {
		name        string
		reserved    bool
		senderErr   error
		wantErr     error
		wantCleanup int
	}{
		{name: "issued", reserved: true},
		{name: "cooldown", wantErr: ErrCooldown},
		{name: "send failure cleans reservation", reserved: true, senderErr: sendErr, wantErr: sendErr, wantCleanup: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeStore{reserved: tt.reserved}
			senderCalls := 0
			service := New(store, senderFunc(func(_ context.Context, phoneNumber, code string) error {
				senderCalls++
				if phoneNumber != "13800138000" || code != "123456" {
					t.Fatalf("unexpected send args: phone=%q code=%q", phoneNumber, code)
				}
				return tt.senderErr
			}))
			service.generate = func() (string, error) { return "123456", nil }

			err := service.Issue(context.Background(), PurposeLogin, "13800138000")
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("Issue() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && err != nil {
				t.Fatalf("Issue() error = %v", err)
			}
			if store.cleanup != tt.wantCleanup {
				t.Fatalf("cleanup calls = %d, want %d", store.cleanup, tt.wantCleanup)
			}
			if tt.reserved && store.code == "123456" {
				t.Fatal("store received the plaintext verification code")
			}
			if store.ttl != CodeTTL || store.cooldown != ResendCooldown {
				t.Fatalf("durations = %v/%v, want %v/%v", store.ttl, store.cooldown, CodeTTL, ResendCooldown)
			}
			if !tt.reserved && senderCalls != 0 {
				t.Fatalf("sender calls = %d during cooldown", senderCalls)
			}
		})
	}
}

func TestServiceUsesPurposeSeparatedKeys(t *testing.T) {
	store := &fakeStore{reserved: true}
	service := New(store, senderFunc(func(context.Context, string, string) error { return nil }))
	service.generate = func() (string, error) { return "123456", nil }

	if err := service.Issue(context.Background(), PurposeLogin, "13800138000"); err != nil {
		t.Fatal(err)
	}
	if err := service.Issue(context.Background(), PurposePasswordReset, "13800138000"); err != nil {
		t.Fatal(err)
	}
	if len(store.keys) != 2 || store.keys[0] == store.keys[1] {
		t.Fatalf("purpose keys are not isolated: %#v", store.keys)
	}
}

func TestServiceVerifyAndConsume(t *testing.T) {
	store := &fakeStore{consumeErr: ErrAttemptsExceeded}
	service := New(store, senderFunc(func(context.Context, string, string) error { return nil }))

	err := service.VerifyAndConsume(context.Background(), PurposePasswordReset, "13800138000", "123456")
	if !errors.Is(err, ErrAttemptsExceeded) {
		t.Fatalf("VerifyAndConsume() error = %v, want ErrAttemptsExceeded", err)
	}
	if len(store.keys) != 1 || store.keys[0] != "sms:PASSWORD_RESET:13800138000" {
		t.Fatalf("consume keys = %#v", store.keys)
	}
	if store.code == "123456" {
		t.Fatal("store received the plaintext verification code")
	}
}

func TestServiceRejectsInvalidInput(t *testing.T) {
	service := New(&fakeStore{reserved: true}, senderFunc(func(context.Context, string, string) error { return nil }))
	if err := service.Issue(context.Background(), PurposeLogin, "123"); !errors.Is(err, ErrInvalidPhone) {
		t.Fatalf("invalid phone error = %v", err)
	}
	if err := service.Issue(context.Background(), Purpose("unknown"), "13800138000"); !errors.Is(err, ErrInvalidPurpose) {
		t.Fatalf("invalid purpose error = %v", err)
	}
}

func TestGenerateCode(t *testing.T) {
	for range 100 {
		code, err := generateCode()
		if err != nil {
			t.Fatal(err)
		}
		if len(code) != 6 {
			t.Fatalf("code length = %d", len(code))
		}
		for _, digit := range code {
			if digit < '0' || digit > '9' {
				t.Fatalf("code contains non-digit")
			}
		}
	}
}
