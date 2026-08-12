package phonecode

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"
)

const (
	CodeTTL         = 5 * time.Minute
	ResendCooldown  = time.Minute
	MaxVerifyErrors = 5
	cleanupTimeout  = 2 * time.Second
)

var (
	ErrInvalidPhone     = errors.New("phonecode: invalid phone number")
	ErrInvalidPurpose   = errors.New("phonecode: invalid purpose")
	ErrCooldown         = errors.New("phonecode: resend cooldown is active")
	ErrExpired          = errors.New("phonecode: code expired or not found")
	ErrMismatch         = errors.New("phonecode: code does not match")
	ErrAttemptsExceeded = errors.New("phonecode: verification attempts exceeded")
)

var mainlandPhonePattern = regexp.MustCompile(`^1[3-9]\d{9}$`)

type Purpose string

const (
	PurposeLogin         Purpose = "LOGIN"
	PurposePasswordReset Purpose = "PASSWORD_RESET"
)

func (p Purpose) valid() bool {
	switch p {
	case PurposeLogin, PurposePasswordReset:
		return true
	default:
		return false
	}
}

type Sender interface {
	SendCode(ctx context.Context, phoneNumber, code string) error
}

type Store interface {
	Reserve(ctx context.Context, key, code string, ttl, cooldown time.Duration) (bool, error)
	Consume(ctx context.Context, key, code string, maxErrors int) error
	Cleanup(ctx context.Context, key, code string) error
}

type Generator func() (string, error)

type Service struct {
	store     Store
	sender    Sender
	generate  Generator
	ttl       time.Duration
	cooldown  time.Duration
	maxErrors int
}

func New(store Store, sender Sender) *Service {
	return &Service{
		store:     store,
		sender:    sender,
		generate:  generateCode,
		ttl:       CodeTTL,
		cooldown:  ResendCooldown,
		maxErrors: MaxVerifyErrors,
	}
}

func (s *Service) Issue(ctx context.Context, purpose Purpose, phoneNumber string) error {
	key, err := verificationKey(purpose, phoneNumber)
	if err != nil {
		return err
	}
	if s == nil || s.store == nil || s.sender == nil || s.generate == nil || ctx == nil {
		return fmt.Errorf("phonecode: invalid service")
	}

	code, err := s.generate()
	if err != nil {
		return fmt.Errorf("phonecode: generate code: %w", err)
	}
	digest := codeDigest(code)
	reserved, err := s.store.Reserve(ctx, key, digest, s.ttl, s.cooldown)
	if err != nil {
		return fmt.Errorf("phonecode: reserve code: %w", err)
	}
	if !reserved {
		return ErrCooldown
	}

	if err := s.sender.SendCode(ctx, phoneNumber, code); err != nil {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
		defer cancel()
		cleanupErr := s.store.Cleanup(cleanupCtx, key, digest)
		if cleanupErr != nil {
			return errors.Join(fmt.Errorf("phonecode: send code: %w", err), fmt.Errorf("phonecode: cleanup failed code: %w", cleanupErr))
		}
		return fmt.Errorf("phonecode: send code: %w", err)
	}
	return nil
}

func (s *Service) VerifyAndConsume(ctx context.Context, purpose Purpose, phoneNumber, code string) error {
	key, err := verificationKey(purpose, phoneNumber)
	if err != nil {
		return err
	}
	if s == nil || s.store == nil || ctx == nil {
		return fmt.Errorf("phonecode: invalid service")
	}
	if err := s.store.Consume(ctx, key, codeDigest(code), s.maxErrors); err != nil {
		return fmt.Errorf("phonecode: verify code: %w", err)
	}
	return nil
}

func codeDigest(code string) string {
	digest := sha256.Sum256([]byte(code))
	return hex.EncodeToString(digest[:])
}

func verificationKey(purpose Purpose, phoneNumber string) (string, error) {
	phoneNumber = strings.TrimSpace(phoneNumber)
	if !purpose.valid() {
		return "", ErrInvalidPurpose
	}
	if !mainlandPhonePattern.MatchString(phoneNumber) {
		return "", ErrInvalidPhone
	}
	return "sms:" + string(purpose) + ":" + phoneNumber, nil
}

func generateCode() (string, error) {
	digits := make([]byte, 6)
	for i := range digits {
		number, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		digits[i] = byte('0' + number.Int64())
	}
	return string(digits), nil
}
