package routers

import (
	"gin-biz-web-api/internal/controller/auth_ctrl"
	"gin-biz-web-api/internal/middleware"
	"gin-biz-web-api/model"

	"github.com/gin-gonic/gin"
)

func registerAccessRoutes(api *gin.RouterGroup) {
	group := api.Group("/v1/access")
	group.Use(middleware.AuthInternalBearerJWT())

	accounts := auth_ctrl.NewAccessAccountController()
	group.POST("/accounts/query", middleware.RequirePermission(model.PermissionSystemAccountRead), accounts.Query)
	group.POST("/accounts", middleware.RequirePermission(model.PermissionSystemAccountManage), accounts.Create)
	group.PUT("/accounts/:id", middleware.RequirePermission(model.PermissionSystemAccountManage), accounts.Update)
	group.PUT("/accounts/:id/status", middleware.RequirePermission(model.PermissionSystemAccountManage), accounts.SetStatus)
	group.PUT("/accounts/:id/password", middleware.RequirePermission(model.PermissionSystemAccountManage), accounts.ResetPassword)
	group.PUT("/accounts/:id/roles", middleware.RequirePermission(model.PermissionSystemAccountManage), accounts.ReplaceRoles)
	group.PUT("/accounts/:id/mall-scope", middleware.RequirePermission(model.PermissionSystemAccountManage), accounts.ReplaceMallScope)

	roles := auth_ctrl.NewAccessRoleController()
	group.GET("/permissions", middleware.RequirePermission(model.PermissionSystemRoleRead), roles.PermissionCatalog)
	group.GET("/roles", middleware.RequirePermission(model.PermissionSystemRoleRead), roles.ListRoles)
	group.POST("/roles", middleware.RequirePermission(model.PermissionSystemRoleManage), roles.CreateRole)
	group.PUT("/roles/:id", middleware.RequirePermission(model.PermissionSystemRoleManage), roles.UpdateRole)
	group.PUT("/roles/:id/status", middleware.RequirePermission(model.PermissionSystemRoleManage), roles.SetRoleStatus)
	group.PUT("/roles/:id/permissions", middleware.RequirePermission(model.PermissionSystemRoleManage), roles.ReplacePermissions)
	group.DELETE("/roles/:id", middleware.RequirePermission(model.PermissionSystemRoleManage), roles.DeleteRole)
	group.GET("/audits", middleware.RequirePermission(model.PermissionSystemAuditRead), roles.QueryAudits)
}
