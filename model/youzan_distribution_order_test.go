package model

import (
	"sync"
	"testing"

	"gorm.io/gorm/schema"
)

func TestYouzanDistributionOrderSchema(t *testing.T) {
	order := YouzanDistributionOrder{}
	if got := order.TableName(); got != "youzan_distribution_orders" {
		t.Fatalf("TableName() = %q, want %q", got, "youzan_distribution_orders")
	}

	parsed, err := schema.Parse(&order, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	tid := parsed.LookUpField("TID")
	if tid == nil || !tid.Unique {
		t.Fatalf("TID field must have a unique index")
	}
	if parsed.LookUpField("FansNickname") == nil || parsed.LookUpField("FansNicknameEncrypted") == nil {
		t.Fatalf("decrypted and encrypted nickname fields must both exist")
	}
}
