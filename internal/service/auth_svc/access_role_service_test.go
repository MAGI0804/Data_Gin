package auth_svc

import (
	"errors"
	"strings"
	"testing"
	"time"

	"gin-biz-web-api/internal/requests/auth_request"
	"gin-biz-web-api/model"
)

func TestNormalizeRoleCreate(t *testing.T) {
	code, name, description, permissions, reason, err := normalizeRoleCreate(auth_request.AccessRoleCreateRequest{
		Code: " Custom_Operator ", Name: " 自定义操作员 ", Description: " 日常操作 ", Permissions: []string{"data.read", "data.read", "source.read"}, Reason: "业务需要",
	})
	if err != nil || code != "custom_operator" || name != "自定义操作员" || description != "日常操作" || reason != "业务需要" {
		t.Fatalf("normalizeRoleCreate() = %q %q %q %#v %q %v", code, name, description, permissions, reason, err)
	}
	if len(permissions) != 2 || permissions[0] != "data.read" || permissions[1] != "source.read" {
		t.Fatalf("permissions = %#v", permissions)
	}
}

func TestNormalizeRoleCreateRejectsSystemAndBadInput(t *testing.T) {
	tests := []auth_request.AccessRoleCreateRequest{
		{Code: model.RoleCodeAdmin, Name: "管理员", Reason: "尝试覆盖"},
		{Code: "A", Name: "短编码", Reason: "创建角色"},
		{Code: "custom", Name: "", Reason: "创建角色"},
		{Code: "custom", Name: "角色", Reason: "line one\nline two"},
		{Code: "custom", Name: "角色", Reason: "创建角色", Permissions: []string{strings.Repeat("x", 65)}},
	}
	for _, request := range tests {
		if _, _, _, _, _, err := normalizeRoleCreate(request); !errors.Is(err, ErrAccessRoleInvalidInput) {
			t.Fatalf("request %#v error = %v", request, err)
		}
	}
}

func TestNormalizeAccessTimes(t *testing.T) {
	start, end, err := normalizeAccessTimes("2026-08-12T08:00:00+08:00", "2026-08-12T01:00:00Z")
	if err != nil || start.Location() != time.UTC || end.Location() != time.UTC || !start.Before(*end) {
		t.Fatalf("normalizeAccessTimes() = %v %v %v", start, end, err)
	}
	if _, _, err := normalizeAccessTimes("2026-08-13T00:00:00Z", "2026-08-12T00:00:00Z"); !errors.Is(err, ErrAccessRoleInvalidInput) {
		t.Fatalf("reversed range error = %v", err)
	}
}
