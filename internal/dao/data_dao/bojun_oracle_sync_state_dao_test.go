package data_dao

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestBojunOracleSyncStateDAORejectsInvalidLeaseInput(t *testing.T) {
	dao := &BojunOracleSyncStateDAO{}
	now := time.Now()
	if _, _, err := dao.AcquireLease(context.Background(), "etl01", "token", now, time.Minute); !errors.Is(err, gorm.ErrInvalidData) {
		t.Fatalf("AcquireLease() error = %v", err)
	}
	if err := dao.Advance(context.Background(), "etl01", "token", 10, 9, now, time.Minute); !errors.Is(err, gorm.ErrInvalidData) {
		t.Fatalf("Advance() error = %v", err)
	}
	if err := dao.ReleaseLease(context.Background(), "", "token", now); !errors.Is(err, gorm.ErrInvalidData) {
		t.Fatalf("ReleaseLease() error = %v", err)
	}
}

func TestBojunOracleSyncStateErrorsAreDistinct(t *testing.T) {
	if errors.Is(ErrBojunOracleSyncStateNotInitialized, ErrBojunOracleSyncLeaseLost) {
		t.Fatal("state and lease errors must remain distinguishable")
	}
}
