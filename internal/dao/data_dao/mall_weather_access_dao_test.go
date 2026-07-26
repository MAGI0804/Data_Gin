package data_dao

import (
	"context"
	"strings"
	"testing"
	"time"

	"gin-biz-web-api/model"

	"gorm.io/gorm"
)

func TestMallWeatherPermissionDAORejectsInvalidLookupBeforeDatabase(t *testing.T) {
	dao := &MallWeatherPermissionDAO{}
	tests := []struct {
		name       string
		userID     uint
		permission string
	}{
		{"missing user", 0, "mall.read"},
		{"missing permission", 1, ""},
		{"oversized permission", 1, string(make([]byte, 65))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := dao.HasPermission(context.Background(), tt.userID, tt.permission, time.Now()); err == nil {
				t.Fatal("HasPermission() error = nil")
			}
		})
	}
}

func TestMallWeatherPermissionDAORejectsInvalidPermanentGrantBeforeDatabase(t *testing.T) {
	dao := NewMallWeatherPermissionDAO(dryRunWeatherDAOTestDB(t).Session(&gorm.Session{SkipDefaultTransaction: true}))
	tests := []struct {
		name        string
		userID      uint
		grantedBy   uint
		permissions []string
	}{
		{name: "missing user", grantedBy: 1, permissions: []string{"mall.read"}},
		{name: "missing grantor", userID: 1, permissions: []string{"mall.read"}},
		{name: "empty permission set", userID: 1, grantedBy: 1},
		{name: "blank permission", userID: 1, grantedBy: 1, permissions: []string{" "}},
		{name: "oversized permission", userID: 1, grantedBy: 1, permissions: []string{strings.Repeat("x", 65)}},
		{name: "duplicate permission", userID: 1, grantedBy: 1, permissions: []string{"mall.read", "mall.read"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := dao.GrantPermanentPermissions(context.Background(), tt.userID, tt.grantedBy, tt.permissions); err == nil {
				t.Fatal("GrantPermanentPermissions() error = nil")
			}
		})
	}
}

func TestMallWeatherPermissionDAOGrantPermanentPermissionsUsesConflictUpdate(t *testing.T) {
	db := dryRunWeatherDAOTestDB(t).Session(&gorm.Session{SkipDefaultTransaction: true})
	var generatedSQL string
	if err := db.Callback().Create().After("gorm:create").Register("test:capture_sql", func(tx *gorm.DB) {
		generatedSQL = tx.Statement.SQL.String()
	}); err != nil {
		t.Fatalf("register SQL capture callback: %v", err)
	}

	dao := NewMallWeatherPermissionDAO(db)
	if err := dao.GrantPermanentPermissions(context.Background(), 7, 7, []string{"mall.read", "weather.read"}); err != nil {
		t.Fatalf("GrantPermanentPermissions() error = %v", err)
	}
	for _, fragment := range []string{
		"ON DUPLICATE KEY UPDATE",
		"`granted_by`=VALUES(`granted_by`)",
		"`expires_at`=VALUES(`expires_at`)",
		"`updated_at`=VALUES(`updated_at`)",
	} {
		if !strings.Contains(generatedSQL, fragment) {
			t.Fatalf("generated SQL missing %q: %s", fragment, generatedSQL)
		}
	}
}

func TestAPIIdempotencyDAORejectsInvalidStateBeforeDatabase(t *testing.T) {
	dao := &APIIdempotencyDAO{}
	if _, err := dao.Reserve(context.Background(), nil); err == nil {
		t.Fatal("Reserve(nil) error = nil")
	}
	if _, err := dao.Reserve(context.Background(), &model.APIIdempotencyRecord{
		OperationScope: "mall.create",
		ActorUserID:    1,
		KeyHash:        "short",
		RequestHash:    "short",
	}); err == nil {
		t.Fatal("Reserve(invalid hashes) error = nil")
	}
	if err := dao.Complete(context.Background(), 0, 0, 500, ""); err == nil {
		t.Fatal("Complete(invalid state) error = nil")
	}
}
