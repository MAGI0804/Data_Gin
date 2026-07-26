package model

import "testing"

func TestMallWeatherAdminPermissions(t *testing.T) {
	permissions := MallWeatherAdminPermissions()
	if len(permissions) != 9 {
		t.Fatalf("MallWeatherAdminPermissions() len = %d, want 9", len(permissions))
	}

	seen := make(map[string]struct{}, len(permissions))
	for _, permission := range permissions {
		if permission == "" || len(permission) > 64 {
			t.Fatalf("invalid permission %q", permission)
		}
		if _, exists := seen[permission]; exists {
			t.Fatalf("duplicate permission %q", permission)
		}
		seen[permission] = struct{}{}
	}
	if _, exists := seen[PermissionWeatherRawRead]; !exists {
		t.Fatalf("missing reserved permission %q", PermissionWeatherRawRead)
	}

	permissions[0] = "modified"
	if MallWeatherAdminPermissions()[0] == "modified" {
		t.Fatal("MallWeatherAdminPermissions() returned shared mutable state")
	}
}
