package auth_svc

import (
	"encoding/json"
	"strings"
	"testing"

	"gin-biz-web-api/model"
)

func TestValidAccessWriteRejectsMissingOrUnsafeMetadata(t *testing.T) {
	if !validAccessWrite("request-123", "业务需要") {
		t.Fatal("valid access write was rejected")
	}
	for _, test := range []struct{ key, reason string }{
		{"short", "业务需要"},
		{"request-123", ""},
		{"request-123", "line one\nline two"},
	} {
		if validAccessWrite(test.key, test.reason) {
			t.Fatalf("unsafe write metadata accepted: %#v", test)
		}
	}
}

func TestAccountDTOUsesEmptyArraysForAllMallScope(t *testing.T) {
	dto := buildAccessAccountDTO(&model.User{BaseModel: &model.BaseModel{ID: 1}, MallScopeMode: model.MallScopeAll}, nil, nil)
	payload, err := json.Marshal(dto)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	serialized := string(payload)
	if !strings.Contains(serialized, `"roles":[]`) || !strings.Contains(serialized, `"mallIds":[]`) {
		t.Fatalf("account DTO arrays serialized as null: %s", serialized)
	}
}

func TestBuildAccessMallQueryResult(t *testing.T) {
	result := buildAccessMallQueryResult([]model.Mall{
		{BaseModel: model.BaseModel{ID: 3}, MallCode: "SH-003", NameCN: "徐汇商场"},
		{BaseModel: model.BaseModel{ID: 8}, MallCode: "SH-008", NameCN: "浦东商场"},
	})
	if len(result.Items) != 2 || result.Items[0].ID != 3 || result.Items[0].NameCN != "徐汇商场" ||
		result.Items[1].MallCode != "SH-008" || result.NextAfterID != 8 {
		t.Fatalf("buildAccessMallQueryResult() = %#v", result)
	}
	empty := buildAccessMallQueryResult(nil)
	if empty.Items == nil || len(empty.Items) != 0 || empty.NextAfterID != 0 {
		t.Fatalf("empty buildAccessMallQueryResult() = %#v", empty)
	}
}

func TestUniqueIDsDeduplicatesAndDropsZero(t *testing.T) {
	got := uniqueIDs([]uint{3, 0, 3, 2})
	if len(got) != 2 || got[0] != 3 || got[1] != 2 {
		t.Fatalf("uniqueIDs() = %#v", got)
	}
}

func TestAccessAccountKeyHashDoesNotExposeIdempotencyKey(t *testing.T) {
	key := "account-request-123"
	got := accessAccountKeyHash(key)
	if len(got) != 64 || strings.Contains(got, key) || got != accessAccountKeyHash(key) {
		t.Fatalf("accessAccountKeyHash() returned unsafe or unstable value %q", got)
	}
}
