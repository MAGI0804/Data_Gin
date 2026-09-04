package auth_svc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"gin-biz-web-api/internal/reportrepo"
	"gin-biz-web-api/internal/requests/auth_request"
	"gin-biz-web-api/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	reportCategorySubjectUser = "USER"
	reportCategorySubjectRole = "ROLE"
)

type AccessAccountReportCategoryDTO struct {
	Category         string   `json:"category"`
	ReportCount      int64    `json:"reportCount"`
	Configured       bool     `json:"configured"`
	LockVersion      uint64   `json:"lockVersion"`
	DirectActions    []string `json:"directActions"`
	InheritedActions []string `json:"inheritedActions"`
	EffectiveActions []string `json:"effectiveActions"`
}

type AccessAccountReportCategoryResult struct {
	Items []AccessAccountReportCategoryDTO `json:"items"`
}

type accessAccountReportCategoryRecord struct {
	Category         string `gorm:"column:category"`
	ReportCount      int64  `gorm:"column:report_count"`
	CategoryAccessID uint   `gorm:"column:category_access_id"`
	LockVersion      uint64 `gorm:"column:lock_version"`
}

func (service *AccessAccountService) ListReportCategories(ctx context.Context, actorID, targetID uint) (*AccessAccountReportCategoryResult, error) {
	if service == nil || service.db == nil || ctx == nil || actorID == 0 || targetID == 0 {
		return nil, ErrAccessAccountInvalid
	}
	if err := service.requireActorPermission(ctx, actorID, model.PermissionSystemAccountManage); err != nil {
		return nil, err
	}
	if err := service.requireActorPermission(ctx, actorID, model.PermissionReportManage); err != nil {
		return nil, err
	}
	var target model.User
	if err := service.db.WithContext(ctx).Where("id = ? AND account_type = ?", targetID, model.AccountTypeConsole).First(&target).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAccessAccountNotFound
		}
		return nil, fmt.Errorf("access account: load report category target: %w", err)
	}
	var records []accessAccountReportCategoryRecord
	if err := accessAccountReportCategoryQuery(service.db.WithContext(ctx)).Scan(&records).Error; err != nil {
		return nil, fmt.Errorf("access account: list report categories: %w", err)
	}
	return service.buildAccountReportCategoryResult(ctx, targetID, records)
}

func accessAccountReportCategoryQuery(db *gorm.DB) *gorm.DB {
	return db.Table(`(
			SELECT category FROM report_definitions WHERE TRIM(category) <> ''
			UNION
			SELECT category FROM report_category_access
		) AS categories`).
		Select(`categories.category, COUNT(definitions.id) AS report_count,
			COALESCE(category_access.id, 0) AS category_access_id,
			COALESCE(category_access.lock_version, 0) AS lock_version`).
		Joins("LEFT JOIN report_definitions AS definitions ON definitions.category = categories.category").
		Joins("LEFT JOIN report_category_access AS category_access ON category_access.category = categories.category").
		Group("categories.category, category_access.id, category_access.lock_version").
		Order("categories.category ASC")
}

func (service *AccessAccountService) buildAccountReportCategoryResult(ctx context.Context, targetID uint, records []accessAccountReportCategoryRecord) (*AccessAccountReportCategoryResult, error) {
	result := &AccessAccountReportCategoryResult{Items: make([]AccessAccountReportCategoryDTO, 0, len(records))}
	accessIDs := make([]uint, 0, len(records))
	for _, record := range records {
		if record.CategoryAccessID > 0 {
			accessIDs = append(accessIDs, record.CategoryAccessID)
		}
	}
	grantsByAccessID := make(map[uint][]model.ReportCategoryGrant, len(accessIDs))
	if len(accessIDs) > 0 {
		var roleIDs []uint
		if err := service.db.WithContext(ctx).Table("user_roles").
			Select("user_roles.role_id").
			Joins("JOIN roles ON roles.id = user_roles.role_id").
			Where("user_roles.user_id = ? AND roles.status = ?", targetID, model.RoleStatusActive).
			Pluck("user_roles.role_id", &roleIDs).Error; err != nil {
			return nil, fmt.Errorf("access account: list active report roles: %w", err)
		}
		query := service.db.WithContext(ctx).Where("category_access_id IN ?", accessIDs).
			Where("subject_type = ? AND subject_id = ?", reportCategorySubjectUser, targetID)
		if len(roleIDs) > 0 {
			query = service.db.WithContext(ctx).Where("category_access_id IN ?", accessIDs).
				Where("(subject_type = ? AND subject_id = ?) OR (subject_type = ? AND subject_id IN ?)", reportCategorySubjectUser, targetID, reportCategorySubjectRole, roleIDs)
		}
		var grants []model.ReportCategoryGrant
		if err := query.Order("category_access_id ASC, subject_type ASC, subject_id ASC").Find(&grants).Error; err != nil {
			return nil, fmt.Errorf("access account: list report category grants: %w", err)
		}
		for _, grant := range grants {
			grantsByAccessID[grant.CategoryAccessID] = append(grantsByAccessID[grant.CategoryAccessID], grant)
		}
	}
	for _, record := range records {
		direct, inherited, err := splitAccountReportCategoryActions(grantsByAccessID[record.CategoryAccessID])
		if err != nil {
			return nil, err
		}
		result.Items = append(result.Items, AccessAccountReportCategoryDTO{
			Category: record.Category, ReportCount: record.ReportCount, Configured: record.CategoryAccessID > 0,
			LockVersion: record.LockVersion, DirectActions: direct, InheritedActions: inherited,
			EffectiveActions: mergeReportActions(direct, inherited),
		})
	}
	return result, nil
}

func (service *AccessAccountService) ReplaceReportCategory(ctx context.Context, actorID, targetID uint, key string, request auth_request.AccessAccountReportCategoryRequest) error {
	request.Category = strings.TrimSpace(request.Category)
	actions, err := normalizeAccessReportActions(request.Actions)
	if err != nil || targetID == 0 || request.Category == "" || utf8.RuneCountInString(request.Category) > 64 || !validAccessWrite(key, request.Reason) {
		return ErrAccessAccountInvalid
	}
	request.Actions = actions
	_, err = service.mutateAccount(ctx, actorID, targetID, key, "access.account.report_category", request, func(tx *gorm.DB, user *model.User) error {
		if err := service.requireActorPermissionTx(ctx, tx, actorID, model.PermissionReportManage); err != nil {
			return err
		}
		var policy model.ReportCategoryAccess
		findErr := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("category = ?", request.Category).First(&policy).Error
		if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return fmt.Errorf("access account: lock report category: %w", findErr)
		}
		if policy.ID == 0 {
			if request.ExpectedLockVersion != 0 {
				return ErrAccessAccountConflict
			}
			var reportCount int64
			if err := tx.WithContext(ctx).Model(&model.ReportDefinition{}).Where("category = ?", request.Category).Count(&reportCount).Error; err != nil {
				return err
			}
			if reportCount == 0 {
				return ErrAccessAccountInvalid
			}
			if len(actions) == 0 {
				return nil
			}
			policy = model.ReportCategoryAccess{Category: request.Category, LockVersion: 1, UpdatedBy: actorID}
			if err := tx.WithContext(ctx).Create(&policy).Error; err != nil {
				return ErrAccessAccountConflict
			}
		} else {
			if request.ExpectedLockVersion == 0 || request.ExpectedLockVersion != policy.LockVersion {
				return ErrAccessAccountConflict
			}
			policy.LockVersion++
			if err := tx.WithContext(ctx).Model(&policy).Updates(map[string]interface{}{"lock_version": policy.LockVersion, "updated_by": actorID}).Error; err != nil {
				return err
			}
		}
		var previous model.ReportCategoryGrant
		previousActions := make([]string, 0)
		findPrevious := tx.WithContext(ctx).Where("category_access_id = ? AND subject_type = ? AND subject_id = ?", policy.ID, reportCategorySubjectUser, targetID).First(&previous).Error
		if findPrevious == nil {
			decoded, decodeErr := decodeAccessReportActions(previous.ActionsJSON)
			if decodeErr != nil {
				return decodeErr
			}
			previousActions = decoded
		} else if !errors.Is(findPrevious, gorm.ErrRecordNotFound) {
			return findPrevious
		}
		if err := tx.WithContext(ctx).Where("category_access_id = ? AND subject_type = ? AND subject_id = ?", policy.ID, reportCategorySubjectUser, targetID).Delete(&model.ReportCategoryGrant{}).Error; err != nil {
			return err
		}
		if len(actions) > 0 {
			raw, encodeErr := json.Marshal(actions)
			if encodeErr != nil {
				return encodeErr
			}
			grant := model.ReportCategoryGrant{CategoryAccessID: policy.ID, SubjectType: reportCategorySubjectUser, SubjectID: targetID, ActionsJSON: model.JSONText(raw), CreatedBy: actorID, UpdatedBy: actorID}
			if err := tx.WithContext(ctx).Create(&grant).Error; err != nil {
				return err
			}
		}
		if err := tx.WithContext(ctx).Model(user).Update("auth_version", gorm.Expr("auth_version + 1")).Error; err != nil {
			return err
		}
		return createAccessReportCategoryAudit(tx, actorID, targetID, request.Category, previousActions, actions, request.Reason, key)
	})
	return err
}

func normalizeAccessReportActions(actions []string) ([]string, error) {
	seen := make(map[string]struct{}, len(actions))
	for _, action := range actions {
		action = strings.ToUpper(strings.TrimSpace(action))
		if action != reportrepo.ReportActionQuery && action != reportrepo.ReportActionExport {
			return nil, ErrAccessAccountInvalid
		}
		seen[action] = struct{}{}
	}
	if _, export := seen[reportrepo.ReportActionExport]; export {
		if _, query := seen[reportrepo.ReportActionQuery]; !query {
			return nil, ErrAccessAccountInvalid
		}
	}
	result := make([]string, 0, len(seen))
	for _, action := range []string{reportrepo.ReportActionQuery, reportrepo.ReportActionExport} {
		if _, ok := seen[action]; ok {
			result = append(result, action)
		}
	}
	return result, nil
}

func decodeAccessReportActions(raw model.JSONText) ([]string, error) {
	var actions []string
	if err := json.Unmarshal([]byte(raw), &actions); err != nil {
		return nil, fmt.Errorf("access account: decode report category actions: %w", err)
	}
	return normalizeAccessReportActions(actions)
}

func splitAccountReportCategoryActions(grants []model.ReportCategoryGrant) ([]string, []string, error) {
	direct := make([]string, 0)
	inherited := make([]string, 0)
	for _, grant := range grants {
		actions, err := decodeAccessReportActions(grant.ActionsJSON)
		if err != nil {
			return nil, nil, err
		}
		if grant.SubjectType == reportCategorySubjectUser {
			direct = mergeReportActions(direct, actions)
		} else if grant.SubjectType == reportCategorySubjectRole {
			inherited = mergeReportActions(inherited, actions)
		}
	}
	return direct, inherited, nil
}

func mergeReportActions(groups ...[]string) []string {
	seen := make(map[string]struct{})
	for _, actions := range groups {
		for _, action := range actions {
			seen[action] = struct{}{}
		}
	}
	result := make([]string, 0, len(seen))
	for _, action := range []string{reportrepo.ReportActionQuery, reportrepo.ReportActionExport} {
		if _, ok := seen[action]; ok {
			result = append(result, action)
		}
	}
	return result
}

func createAccessReportCategoryAudit(tx *gorm.DB, actorID, targetID uint, category string, before, after []string, reason, requestID string) error {
	beforeJSON, err := json.Marshal(map[string]interface{}{"category": category, "actions": before})
	if err != nil {
		return err
	}
	afterJSON, err := json.Marshal(map[string]interface{}{"category": category, "actions": after})
	if err != nil {
		return err
	}
	audit := model.AuthAudit{ActorUserID: actorID, Action: "ACCOUNT_REPORT_CATEGORY", TargetType: "ACCOUNT", TargetID: targetID, BeforeJSON: model.JSONText(beforeJSON), AfterJSON: model.JSONText(afterJSON), Reason: strings.TrimSpace(reason), RequestID: accessAccountKeyHash(requestID)}
	return tx.Create(&audit).Error
}
