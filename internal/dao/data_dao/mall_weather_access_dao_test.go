package data_dao

import (
	"context"
	"testing"
	"time"

	"gin-biz-web-api/model"
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
