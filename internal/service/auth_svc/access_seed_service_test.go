package auth_svc

import (
	"reflect"
	"sort"
	"testing"

	"gin-biz-web-api/model"
)

func TestPermissionCatalogIsUniqueAndComplete(t *testing.T) {
	catalog := PermissionCatalog()
	if len(catalog) < 20 {
		t.Fatalf("permission catalog count = %d", len(catalog))
	}
	seen := make(map[string]struct{}, len(catalog))
	for _, permission := range catalog {
		if permission.Code == "" || permission.Name == "" || permission.Module == "" || permission.Description == "" {
			t.Fatalf("incomplete permission: %+v", permission)
		}
		if _, exists := seen[permission.Code]; exists {
			t.Fatalf("duplicate permission %q", permission.Code)
		}
		seen[permission.Code] = struct{}{}
	}
	for _, required := range model.MallWeatherAdminPermissions() {
		if _, exists := seen[required]; !exists {
			t.Fatalf("missing existing permission %q", required)
		}
	}
	for _, required := range []string{
		model.PermissionReportRead,
		model.PermissionReportManage,
		model.PermissionReportExecute,
		model.PermissionReportExport,
	} {
		if _, exists := seen[required]; !exists {
			t.Fatalf("missing report permission %q", required)
		}
	}
}

func TestPermissionCodesReturnsSortedDefensiveCopy(t *testing.T) {
	first := PermissionCodes()
	if !sort.StringsAreSorted(first) {
		t.Fatalf("permission codes are not sorted: %v", first)
	}
	second := PermissionCodes()
	first[0] = "modified"
	if reflect.DeepEqual(first, second) {
		t.Fatal("PermissionCodes returned shared storage")
	}
}
