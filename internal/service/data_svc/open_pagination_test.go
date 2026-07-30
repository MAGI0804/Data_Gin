package data_svc

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNewOpenPaginationCalculatesTotals(t *testing.T) {
	pagination := newOpenPagination(2, 50, 101)
	if pagination.Page != 2 || pagination.PageSize != 50 || pagination.TotalItems != 101 || pagination.TotalPages != 3 {
		t.Fatalf("pagination=%+v", pagination)
	}
	raw, err := json.Marshal(OpenBojunOrderPagination{OpenPagination: pagination, CurrentItems: 50})
	if err != nil {
		t.Fatalf("json.Marshal() error=%v", err)
	}
	for _, field := range []string{`"page":2`, `"pageSize":50`, `"totalItems":101`, `"totalPages":3`, `"currentItems":50`} {
		if !strings.Contains(string(raw), field) {
			t.Fatalf("response=%s missing %s", raw, field)
		}
	}
}

func TestOpenCursorPageSupportsLegacyCursor(t *testing.T) {
	if page := openCursorPage(0, true); page != 2 {
		t.Fatalf("legacy cursor page=%d", page)
	}
	if !invalidOpenCursorPage(1) || invalidOpenCursorPage(0) || invalidOpenCursorPage(2) {
		t.Fatal("cursor page validation mismatch")
	}
	if !invalidOpenCursorPage(maxOpenCursorPage + 1) {
		t.Fatal("cursor page above limit accepted")
	}
	if _, err := nextOpenCursorPage(maxOpenCursorPage); err == nil {
		t.Fatal("nextOpenCursorPage() accepted page at limit")
	}
}
