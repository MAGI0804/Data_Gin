package auth_ctrl

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"gin-biz-web-api/constant"
	"gin-biz-web-api/internal/requests/auth_request"
	"gin-biz-web-api/internal/service/auth_svc"
	"gin-biz-web-api/model"
)

type fakeAccessRoleService struct {
	createKey string
	create    auth_request.AccessRoleCreateRequest
	result    *auth_svc.AccessRoleMutationResult
	err       error
}

func (f *fakeAccessRoleService) PermissionCatalog(context.Context, uint) ([]model.Permission, error) {
	return nil, f.err
}
func (f *fakeAccessRoleService) ListRoles(context.Context, uint) ([]auth_svc.AccessRoleDTO, error) {
	return nil, f.err
}
func (f *fakeAccessRoleService) CreateRole(_ context.Context, _ uint, key string, r auth_request.AccessRoleCreateRequest) (*auth_svc.AccessRoleMutationResult, error) {
	f.createKey, f.create = key, r
	return f.result, f.err
}
func (f *fakeAccessRoleService) UpdateRole(context.Context, uint, uint, string, auth_request.AccessRoleUpdateRequest) (*auth_svc.AccessRoleMutationResult, error) {
	return f.result, f.err
}
func (f *fakeAccessRoleService) SetRoleStatus(context.Context, uint, uint, string, auth_request.AccessRoleStatusRequest) (*auth_svc.AccessRoleMutationResult, error) {
	return f.result, f.err
}
func (f *fakeAccessRoleService) ReplacePermissions(context.Context, uint, uint, string, auth_request.AccessRolePermissionsRequest) (*auth_svc.AccessRoleMutationResult, error) {
	return f.result, f.err
}
func (f *fakeAccessRoleService) DeleteRole(context.Context, uint, uint, string, auth_request.AccessRoleDeleteRequest) (*auth_svc.AccessRoleMutationResult, error) {
	return f.result, f.err
}
func (f *fakeAccessRoleService) QueryAudits(context.Context, uint, auth_request.AccessAuditQueryRequest) (*auth_svc.AccessAuditQueryResult, error) {
	return nil, f.err
}

func TestAccessRoleControllerCreateRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &fakeAccessRoleService{result: &auth_svc.AccessRoleMutationResult{Role: &auth_svc.AccessRoleDTO{ID: 7, Code: "custom"}}}
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set(constant.CurrentUserID, "9") })
	router.POST("/roles", NewAccessRoleControllerWithService(service).CreateRole)
	request := httptest.NewRequest(http.MethodPost, "/roles", bytes.NewBufferString(`{"code":"custom","name":"角色","reason":"业务需要"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "request-123")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated || service.createKey != "request-123" || service.create.Code != "custom" {
		t.Fatalf("status=%d key=%q request=%#v", recorder.Code, service.createKey, service.create)
	}
}

func TestAccessRoleControllerRejectsUnknownJSONField(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := new(fakeAccessRoleService)
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set(constant.CurrentUserID, "9") })
	router.POST("/roles", NewAccessRoleControllerWithService(service).CreateRole)
	request := httptest.NewRequest(http.MethodPost, "/roles", bytes.NewBufferString(`{"code":"custom","unknown":true}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
