package auth_svc

import (
	"context"
	"fmt"

	"gin-biz-web-api/model"

	"gorm.io/gorm"
)

func reportCategoryActionForPermission(code string) string {
	switch code {
	case model.PermissionReportRead, model.PermissionReportExecute:
		return "QUERY"
	case model.PermissionReportExport:
		return "EXPORT"
	default:
		return ""
	}
}

func reportRuntimePermissions(actions []string) []string {
	hasQuery, hasExport := false, false
	for _, action := range actions {
		switch action {
		case "QUERY":
			hasQuery = true
		case "EXPORT":
			hasExport = true
		}
	}
	permissions := make([]string, 0, 3)
	if hasQuery || hasExport {
		permissions = append(permissions, model.PermissionReportRead, model.PermissionReportExecute)
	}
	if hasExport {
		permissions = append(permissions, model.PermissionReportExport)
	}
	return permissions
}

func loadConsoleReportCategoryActions(ctx context.Context, db *gorm.DB, userID uint) ([]string, error) {
	if ctx == nil || db == nil || userID == 0 {
		return nil, fmt.Errorf("report category runtime permissions: invalid input")
	}
	var rows []struct {
		ActionsJSON model.JSONText `gorm:"column:actions_json"`
	}
	err := db.WithContext(ctx).
		Table("report_category_grants AS grants").
		Select("grants.actions_json").
		Joins("JOIN report_category_access AS access ON access.id = grants.category_access_id").
		Where(`
			(grants.subject_type = ? AND grants.subject_id = ?)
			OR (
				grants.subject_type = ?
				AND EXISTS (
					SELECT 1 FROM user_roles
					JOIN roles ON roles.id = user_roles.role_id AND roles.status = ?
					WHERE user_roles.user_id = ? AND user_roles.role_id = grants.subject_id
				)
			)`, reportCategorySubjectUser, userID, reportCategorySubjectRole, model.RoleStatusActive, userID).
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("report category runtime permissions: load grants: %w", err)
	}
	actions := make([]string, 0, 2)
	for _, row := range rows {
		decoded, decodeErr := decodeAccessReportActions(row.ActionsJSON)
		if decodeErr != nil {
			return nil, decodeErr
		}
		actions = mergeReportActions(actions, decoded)
	}
	return actions, nil
}
