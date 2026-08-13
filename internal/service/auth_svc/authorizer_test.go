package auth_svc

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"gin-biz-web-api/model"
)

type fakeAuthorizationRepository struct {
	roles              []authorizationRole
	console            bool
	openAPI            bool
	consolePermissions map[string]bool
	openPermissions    map[string]bool
	permissionErrors   map[string]error
	err                error
	roleCalls          int
	consoleCodes       []string
	openCodes          []string
	now                time.Time
}

func (f *fakeAuthorizationRepository) ActiveConsoleRoles(context.Context, uint) ([]authorizationRole, error) {
	f.roleCalls++
	return f.roles, f.err
}
func (f *fakeAuthorizationRepository) ConsoleRoleHasPermission(_ context.Context, _ uint, code string) (bool, error) {
	f.consoleCodes = append(f.consoleCodes, code)
	if err := f.permissionErrors[code]; err != nil {
		return false, err
	}
	if f.consolePermissions != nil {
		return f.consolePermissions[code], nil
	}
	return f.console, f.err
}
func (f *fakeAuthorizationRepository) OpenAPIHasPermission(_ context.Context, _ uint, code string, now time.Time) (bool, error) {
	f.openCodes, f.now = append(f.openCodes, code), now
	if err := f.permissionErrors[code]; err != nil {
		return false, err
	}
	if f.openPermissions != nil {
		return f.openPermissions[code], nil
	}
	return f.openAPI, f.err
}

func TestAuthorizerDeniesInvalidUsersWithoutRepository(t *testing.T) {
	repository := &fakeAuthorizationRepository{}
	authorizer := newAuthorizer(repository)
	tests := []model.User{
		{AccountType: model.AccountTypeConsole, Status: model.AccountStatusActive},
		{BaseModel: &model.BaseModel{ID: 1}, AccountType: model.AccountTypeConsole, Status: model.AccountStatusDisabled},
		{BaseModel: &model.BaseModel{ID: 1}, AccountType: "UNKNOWN", Status: model.AccountStatusActive},
	}
	for _, user := range tests {
		allowed, err := authorizer.HasPermission(context.Background(), user, "data.read")
		if err != nil || allowed {
			t.Fatalf("HasPermission() = %t, %v", allowed, err)
		}
	}
	if repository.roleCalls != 0 {
		t.Fatal("invalid users reached repository")
	}
}

func TestAuthorizerConsoleRoles(t *testing.T) {
	user := model.User{BaseModel: &model.BaseModel{ID: 7}, AccountType: model.AccountTypeConsole, Status: model.AccountStatusActive}
	tests := []struct {
		name    string
		roles   []authorizationRole
		granted bool
		want    bool
	}{
		{name: "permission union", roles: []authorizationRole{{Code: model.RoleCodeViewer}, {Code: model.RoleCodeOperator}}, granted: true, want: true},
		{name: "super flag", roles: []authorizationRole{{Code: "custom", IsSuper: true}}, want: true},
		{name: "super code", roles: []authorizationRole{{Code: model.RoleCodeSuperAdmin}}, want: true},
		{name: "no roles"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := &fakeAuthorizationRepository{roles: tt.roles, console: tt.granted}
			allowed, err := newAuthorizer(repository).HasPermission(context.Background(), user, model.PermissionDataRead)
			if err != nil || allowed != tt.want {
				t.Fatalf("HasPermission() = %t, %v, want %t", allowed, err, tt.want)
			}
		})
	}
}

func TestAuthorizerOpenAPIUsesUTCAndPropagatesErrors(t *testing.T) {
	repository := &fakeAuthorizationRepository{openAPI: true}
	authorizer := newAuthorizer(repository)
	authorizer.now = func() time.Time { return time.Date(2026, 8, 12, 12, 0, 0, 0, time.FixedZone("CST", 8*60*60)) }
	user := model.User{BaseModel: &model.BaseModel{ID: 9}, AccountType: model.AccountTypeOpenAPI, Status: model.AccountStatusActive}
	allowed, err := authorizer.HasPermission(context.Background(), user, model.PermissionWeatherRead)
	if err != nil || !allowed || repository.now.Location() != time.UTC {
		t.Fatalf("HasPermission() = %t, %v, now=%v", allowed, err, repository.now)
	}
	repository.err = errors.New("database offline")
	if allowed, err = authorizer.HasPermission(context.Background(), user, model.PermissionWeatherRead); err == nil || allowed {
		t.Fatalf("HasPermission() = %t, %v", allowed, err)
	}
}

func TestAuthorizerRejectsInvalidPermissionCodes(t *testing.T) {
	user := model.User{BaseModel: &model.BaseModel{ID: 1}, AccountType: model.AccountTypeConsole, Status: model.AccountStatusActive}
	for _, code := range []string{"", "   ", "unknown.permission", string(make([]byte, maximumPermissionCodeBytes+1))} {
		allowed, err := newAuthorizer(&fakeAuthorizationRepository{}).HasPermission(context.Background(), user, code)
		if err != nil || allowed {
			t.Fatalf("HasPermission(%q) = %t, %v", code, allowed, err)
		}
	}
}

func TestAuthorizerUsesExplicitManageToReadImplications(t *testing.T) {
	user := model.User{BaseModel: &model.BaseModel{ID: 7}, AccountType: model.AccountTypeConsole, Status: model.AccountStatusActive}
	tests := []struct {
		name        string
		requested   string
		permissions map[string]bool
		want        bool
		wantCodes   []string
	}{
		{name: "source manage can read source", requested: model.PermissionSourceRead, permissions: map[string]bool{model.PermissionSourceManage: true}, want: true, wantCodes: []string{model.PermissionSourceRead, model.PermissionSourceManage}},
		{name: "pipeline manage can read pipeline", requested: model.PermissionPipelineRead, permissions: map[string]bool{model.PermissionPipelineManage: true}, want: true, wantCodes: []string{model.PermissionPipelineRead, model.PermissionPipelineManage}},
		{name: "delivery manage can read delivery", requested: model.PermissionDeliveryRead, permissions: map[string]bool{model.PermissionDeliveryManage: true}, want: true, wantCodes: []string{model.PermissionDeliveryRead, model.PermissionDeliveryManage}},
		{name: "direct read stops candidate search", requested: model.PermissionPipelineRead, permissions: map[string]bool{model.PermissionPipelineRead: true}, want: true, wantCodes: []string{model.PermissionPipelineRead}},
		{name: "read cannot manage", requested: model.PermissionPipelineManage, permissions: map[string]bool{model.PermissionPipelineRead: true}, wantCodes: []string{model.PermissionPipelineManage}},
		{name: "execute cannot read", requested: model.PermissionPipelineRead, permissions: map[string]bool{model.PermissionPipelineExecute: true}, wantCodes: []string{model.PermissionPipelineRead, model.PermissionPipelineManage}},
		{name: "manage does not cross modules", requested: model.PermissionDeliveryRead, permissions: map[string]bool{model.PermissionSourceManage: true}, wantCodes: []string{model.PermissionDeliveryRead, model.PermissionDeliveryManage}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := &fakeAuthorizationRepository{roles: []authorizationRole{{Code: "custom"}}, consolePermissions: tt.permissions}
			allowed, err := newAuthorizer(repository).HasPermission(context.Background(), user, tt.requested)
			if err != nil || allowed != tt.want {
				t.Fatalf("HasPermission() = %t, %v, want %t", allowed, err, tt.want)
			}
			if !reflect.DeepEqual(repository.consoleCodes, tt.wantCodes) {
				t.Fatalf("permission checks = %v, want %v", repository.consoleCodes, tt.wantCodes)
			}
		})
	}
}

func TestAuthorizerFailsClosedWhenImpliedPermissionCheckFails(t *testing.T) {
	providerError := errors.New("permission database unavailable")
	repository := &fakeAuthorizationRepository{
		roles:            []authorizationRole{{Code: "custom"}},
		permissionErrors: map[string]error{model.PermissionSourceManage: providerError},
	}
	user := model.User{BaseModel: &model.BaseModel{ID: 7}, AccountType: model.AccountTypeConsole, Status: model.AccountStatusActive}
	allowed, err := newAuthorizer(repository).HasPermission(context.Background(), user, model.PermissionSourceRead)
	if allowed || !errors.Is(err, providerError) {
		t.Fatalf("HasPermission() = %t, %v", allowed, err)
	}
}

func TestAuthorizerAppliesReadImplicationsToOpenAPI(t *testing.T) {
	repository := &fakeAuthorizationRepository{openPermissions: map[string]bool{model.PermissionDeliveryManage: true}}
	user := model.User{BaseModel: &model.BaseModel{ID: 9}, AccountType: model.AccountTypeOpenAPI, Status: model.AccountStatusActive}
	allowed, err := newAuthorizer(repository).HasPermission(context.Background(), user, model.PermissionDeliveryRead)
	if err != nil || !allowed {
		t.Fatalf("HasPermission() = %t, %v", allowed, err)
	}
	want := []string{model.PermissionDeliveryRead, model.PermissionDeliveryManage}
	if !reflect.DeepEqual(repository.openCodes, want) {
		t.Fatalf("permission checks = %v, want %v", repository.openCodes, want)
	}
}
