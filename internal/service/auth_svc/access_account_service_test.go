package auth_svc

import (
	"testing"
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

func TestUniqueIDsDeduplicatesAndDropsZero(t *testing.T) {
	got := uniqueIDs([]uint{3, 0, 3, 2})
	if len(got) != 2 || got[0] != 3 || got[1] != 2 {
		t.Fatalf("uniqueIDs() = %#v", got)
	}
}
