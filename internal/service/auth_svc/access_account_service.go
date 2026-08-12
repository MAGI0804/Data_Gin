package auth_svc

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"gin-biz-web-api/internal/requests/auth_request"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrAccessAccountInvalid   = errors.New("access account: invalid input")
	ErrAccessAccountForbidden = errors.New("access account: forbidden")
	ErrAccessAccountNotFound  = errors.New("access account: not found")
	ErrAccessAccountConflict  = errors.New("access account: conflict")
)

var accessAccountPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{2,39}$`)
var accessPhonePattern = regexp.MustCompile(`^1[3-9]\d{9}$`)

type AccessAccountService struct {
	db  *gorm.DB
	now func() time.Time
}

type AccessAccountDTO struct {
	ID            uint             `json:"id"`
	Account       string           `json:"account"`
	Phone         string           `json:"phone"`
	Nickname      string           `json:"nickname"`
	Status        string           `json:"status"`
	MallScopeMode string           `json:"mallScopeMode"`
	Roles         []ConsoleRoleDTO `json:"roles"`
	MallIDs       []uint           `json:"mallIds"`
	LastLoginAt   *time.Time       `json:"lastLoginAt,omitempty"`
}

type AccessAccountQueryResult struct {
	Accounts     []AccessAccountDTO `json:"accounts"`
	NextBeforeID uint               `json:"nextBeforeId,omitempty"`
	HasMore      bool               `json:"hasMore"`
}

func NewAccessAccountService() *AccessAccountService {
	return &AccessAccountService{db: database.DB, now: time.Now}
}

func (service *AccessAccountService) Query(ctx context.Context, actorID uint, request auth_request.AccessAccountQueryRequest) (*AccessAccountQueryResult, error) {
	if err := service.requireActorPermission(ctx, actorID, model.PermissionSystemAccountRead); err != nil {
		return nil, err
	}
	pageSize := request.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	query := service.db.WithContext(ctx).Where("account_type = ?", model.AccountTypeConsole)
	if request.BeforeID > 0 {
		query = query.Where("id < ?", request.BeforeID)
	}
	if status := strings.ToUpper(strings.TrimSpace(request.Status)); status != "" {
		if status != model.AccountStatusActive && status != model.AccountStatusDisabled {
			return nil, ErrAccessAccountInvalid
		}
		query = query.Where("status = ?", status)
	}
	if keyword := strings.TrimSpace(request.Keyword); keyword != "" {
		if utf8.RuneCountInString(keyword) > 64 {
			return nil, ErrAccessAccountInvalid
		}
		like := "%" + keyword + "%"
		query = query.Where("account LIKE ? OR nickname LIKE ? OR phone LIKE ?", like, like, like)
	}
	var users []model.User
	if err := query.Order("id DESC").Limit(pageSize + 1).Find(&users).Error; err != nil {
		return nil, fmt.Errorf("access account: query: %w", err)
	}
	hasMore := len(users) > pageSize
	if hasMore {
		users = users[:pageSize]
	}
	result := &AccessAccountQueryResult{Accounts: make([]AccessAccountDTO, 0, len(users)), HasMore: hasMore}
	for i := range users {
		dto, err := service.accountDTO(ctx, &users[i])
		if err != nil {
			return nil, err
		}
		result.Accounts = append(result.Accounts, dto)
	}
	if hasMore && len(users) > 0 {
		result.NextBeforeID = users[len(users)-1].ID
	}
	return result, nil
}

func (service *AccessAccountService) Create(ctx context.Context, actorID uint, key string, request auth_request.AccessAccountCreateRequest) (*AccessAccountDTO, error) {
	request.Account = strings.TrimSpace(request.Account)
	request.Phone = strings.TrimSpace(request.Phone)
	request.Nickname = strings.TrimSpace(request.Nickname)
	if !validAccessWrite(key, request.Reason) || !accessAccountPattern.MatchString(request.Account) || !accessPhonePattern.MatchString(request.Phone) || utf8.RuneCountInString(request.Nickname) < 1 || utf8.RuneCountInString(request.Nickname) > 64 || validateNewPassword(request.Password) != nil {
		return nil, ErrAccessAccountInvalid
	}
	if request.MallScopeMode != model.MallScopeAll && request.MallScopeMode != model.MallScopeSelected || request.MallScopeMode == model.MallScopeSelected && len(request.MallIDs) == 0 {
		return nil, ErrAccessAccountInvalid
	}
	var created model.User
	err := service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := service.requireActorPermissionTx(ctx, tx, actorID, model.PermissionSystemAccountManage); err != nil {
			return err
		}
		if err := service.validateGrants(ctx, tx, actorID, request.RoleIDs, request.MallScopeMode, request.MallIDs); err != nil {
			return err
		}
		now := service.now().UTC()
		phone := request.Phone
		created = model.User{BaseModel: &model.BaseModel{}, CommonTimestampsField: &model.CommonTimestampsField{CreatedAt: int(now.Unix()), UpdatedAt: int(now.Unix())}, Account: request.Account, Phone: &phone, Password: request.Password, Nickname: request.Nickname, ConsoleManaged: true, AccountType: model.AccountTypeConsole, Status: model.AccountStatusActive, AuthVersion: 1, MallScopeMode: request.MallScopeMode, PasswordChangedAt: &now}
		if err := tx.Create(&created).Error; err != nil {
			return ErrAccessAccountConflict
		}
		if err := replaceAccountRoles(ctx, tx, actorID, created.ID, request.RoleIDs); err != nil {
			return err
		}
		if err := replaceAccountMalls(ctx, tx, actorID, created.ID, request.MallScopeMode, request.MallIDs); err != nil {
			return err
		}
		return createAccessAudit(tx, actorID, "ACCOUNT_CREATE", "ACCOUNT", created.ID, request.Reason, key)
	})
	if err != nil {
		return nil, err
	}
	dto, err := service.accountDTO(ctx, &created)
	return &dto, err
}

func (service *AccessAccountService) Update(ctx context.Context, actorID, targetID uint, key string, request auth_request.AccessAccountUpdateRequest) (*AccessAccountDTO, error) {
	request.Phone, request.Nickname = strings.TrimSpace(request.Phone), strings.TrimSpace(request.Nickname)
	if targetID == 0 || !validAccessWrite(key, request.Reason) || !accessPhonePattern.MatchString(request.Phone) || utf8.RuneCountInString(request.Nickname) < 1 || utf8.RuneCountInString(request.Nickname) > 64 {
		return nil, ErrAccessAccountInvalid
	}
	err := service.mutateAccount(ctx, actorID, targetID, func(tx *gorm.DB, user *model.User) error {
		phone := request.Phone
		if err := tx.Model(user).Updates(map[string]interface{}{"phone": &phone, "nickname": request.Nickname, "auth_version": gorm.Expr("auth_version + 1")}).Error; err != nil {
			return ErrAccessAccountConflict
		}
		return createAccessAudit(tx, actorID, "ACCOUNT_UPDATE", "ACCOUNT", targetID, request.Reason, key)
	})
	if err != nil {
		return nil, err
	}
	var user model.User
	if err := service.db.WithContext(ctx).First(&user, targetID).Error; err != nil {
		return nil, err
	}
	dto, err := service.accountDTO(ctx, &user)
	return &dto, err
}

func (service *AccessAccountService) SetStatus(ctx context.Context, actorID, targetID uint, key string, request auth_request.AccessAccountStatusRequest) error {
	request.Status = strings.ToUpper(strings.TrimSpace(request.Status))
	if targetID == 0 || !validAccessWrite(key, request.Reason) || request.Status != model.AccountStatusActive && request.Status != model.AccountStatusDisabled {
		return ErrAccessAccountInvalid
	}
	return service.mutateAccount(ctx, actorID, targetID, func(tx *gorm.DB, user *model.User) error {
		if request.Status == model.AccountStatusDisabled {
			var count int64
			if err := tx.Table("user_roles").Joins("JOIN roles ON roles.id = user_roles.role_id").Joins("JOIN users ON users.id = user_roles.user_id").Where("roles.is_super = ? AND roles.status = ? AND users.status = ? AND users.id <> ?", true, model.RoleStatusActive, model.AccountStatusActive, targetID).Count(&count).Error; err != nil {
				return err
			}
			var targetSuper int64
			if err := tx.Table("user_roles").Joins("JOIN roles ON roles.id = user_roles.role_id").Where("user_roles.user_id = ? AND roles.is_super = ?", targetID, true).Count(&targetSuper).Error; err != nil {
				return err
			}
			if targetSuper > 0 && count == 0 {
				return ErrAccessAccountConflict
			}
		}
		if err := tx.Model(user).Updates(map[string]interface{}{"status": request.Status, "auth_version": gorm.Expr("auth_version + 1")}).Error; err != nil {
			return err
		}
		return createAccessAudit(tx, actorID, "ACCOUNT_STATUS", "ACCOUNT", targetID, request.Reason, key)
	})
}

func (service *AccessAccountService) ResetPassword(ctx context.Context, actorID, targetID uint, key string, request auth_request.AccessAccountPasswordResetRequest) error {
	if targetID == 0 || !validAccessWrite(key, request.Reason) || validateNewPassword(request.Password) != nil {
		return ErrAccessAccountInvalid
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(request.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return service.mutateAccount(ctx, actorID, targetID, func(tx *gorm.DB, user *model.User) error {
		now := service.now().UTC()
		if err := tx.Model(user).Updates(map[string]interface{}{"password": string(hash), "password_changed_at": now, "auth_version": gorm.Expr("auth_version + 1")}).Error; err != nil {
			return err
		}
		return createAccessAudit(tx, actorID, "PASSWORD_RESET", "ACCOUNT", targetID, request.Reason, key)
	})
}

func (service *AccessAccountService) ReplaceRoles(ctx context.Context, actorID, targetID uint, key string, request auth_request.AccessAccountRolesRequest) error {
	if targetID == 0 || !validAccessWrite(key, request.Reason) {
		return ErrAccessAccountInvalid
	}
	return service.mutateAccount(ctx, actorID, targetID, func(tx *gorm.DB, user *model.User) error {
		if err := service.validateGrants(ctx, tx, actorID, request.RoleIDs, user.MallScopeMode, nil); err != nil {
			return err
		}
		if err := ensureSuperAdminRemains(tx, targetID, request.RoleIDs); err != nil {
			return err
		}
		if err := replaceAccountRoles(ctx, tx, actorID, targetID, request.RoleIDs); err != nil {
			return err
		}
		if err := tx.Model(user).Update("auth_version", gorm.Expr("auth_version + 1")).Error; err != nil {
			return err
		}
		return createAccessAudit(tx, actorID, "ACCOUNT_ROLES", "ACCOUNT", targetID, request.Reason, key)
	})
}

func ensureSuperAdminRemains(tx *gorm.DB, targetID uint, replacementRoleIDs []uint) error {
	var currentlySuper int64
	if err := tx.Table("user_roles").Joins("JOIN roles ON roles.id = user_roles.role_id").Where("user_roles.user_id = ? AND roles.is_super = ?", targetID, true).Count(&currentlySuper).Error; err != nil {
		return err
	}
	if currentlySuper == 0 {
		return nil
	}
	var replacementSuper int64
	if len(replacementRoleIDs) > 0 {
		if err := tx.Model(&model.Role{}).Where("id IN ? AND is_super = ?", replacementRoleIDs, true).Count(&replacementSuper).Error; err != nil {
			return err
		}
	}
	if replacementSuper > 0 {
		return nil
	}
	var otherActiveSuper int64
	if err := tx.Table("user_roles").Joins("JOIN roles ON roles.id = user_roles.role_id").Joins("JOIN users ON users.id = user_roles.user_id").Where("roles.is_super = ? AND roles.status = ? AND users.status = ? AND users.id <> ?", true, model.RoleStatusActive, model.AccountStatusActive, targetID).Count(&otherActiveSuper).Error; err != nil {
		return err
	}
	if otherActiveSuper == 0 {
		return ErrAccessAccountConflict
	}
	return nil
}

func (service *AccessAccountService) ReplaceMallScope(ctx context.Context, actorID, targetID uint, key string, request auth_request.AccessAccountMallScopeRequest) error {
	if targetID == 0 || !validAccessWrite(key, request.Reason) || request.MallScopeMode != model.MallScopeAll && request.MallScopeMode != model.MallScopeSelected || request.MallScopeMode == model.MallScopeSelected && len(request.MallIDs) == 0 {
		return ErrAccessAccountInvalid
	}
	return service.mutateAccount(ctx, actorID, targetID, func(tx *gorm.DB, user *model.User) error {
		if err := service.validateGrants(ctx, tx, actorID, nil, request.MallScopeMode, request.MallIDs); err != nil {
			return err
		}
		if err := replaceAccountMalls(ctx, tx, actorID, targetID, request.MallScopeMode, request.MallIDs); err != nil {
			return err
		}
		if err := tx.Model(user).Updates(map[string]interface{}{"mall_scope_mode": request.MallScopeMode, "auth_version": gorm.Expr("auth_version + 1")}).Error; err != nil {
			return err
		}
		return createAccessAudit(tx, actorID, "ACCOUNT_MALL_SCOPE", "ACCOUNT", targetID, request.Reason, key)
	})
}

func (service *AccessAccountService) mutateAccount(ctx context.Context, actorID, targetID uint, mutation func(*gorm.DB, *model.User) error) error {
	return service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := service.requireActorPermissionTx(ctx, tx, actorID, model.PermissionSystemAccountManage); err != nil {
			return err
		}
		var user model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND account_type = ?", targetID, model.AccountTypeConsole).First(&user).Error; err != nil {
			return ErrAccessAccountNotFound
		}
		return mutation(tx, &user)
	})
}

func (service *AccessAccountService) requireActorPermission(ctx context.Context, actorID uint, code string) error {
	return service.requireActorPermissionTx(ctx, service.db, actorID, code)
}

func (service *AccessAccountService) requireActorPermissionTx(ctx context.Context, db *gorm.DB, actorID uint, code string) error {
	var actor model.User
	if err := db.WithContext(ctx).First(&actor, actorID).Error; err != nil {
		return ErrAccessAccountForbidden
	}
	allowed, err := NewAuthorizer(db).HasPermission(ctx, actor, code)
	if err != nil || !allowed {
		return ErrAccessAccountForbidden
	}
	return nil
}

func (service *AccessAccountService) validateGrants(ctx context.Context, tx *gorm.DB, actorID uint, roleIDs []uint, scopeMode string, mallIDs []uint) error {
	var actor model.User
	if err := tx.WithContext(ctx).First(&actor, actorID).Error; err != nil {
		return ErrAccessAccountForbidden
	}
	var superCount int64
	if err := tx.Table("user_roles").Joins("JOIN roles ON roles.id = user_roles.role_id").Where("user_roles.user_id = ? AND roles.is_super = ? AND roles.status = ?", actorID, true, model.RoleStatusActive).Count(&superCount).Error; err != nil {
		return err
	}
	if superCount > 0 {
		return validateRoleAndMallExistence(tx, roleIDs, mallIDs)
	}
	if scopeMode == model.MallScopeAll && actor.MallScopeMode != model.MallScopeAll {
		return ErrAccessAccountForbidden
	}
	if len(mallIDs) > 0 && actor.MallScopeMode != model.MallScopeAll {
		var count int64
		if err := tx.Model(&model.UserMallScope{}).Where("user_id = ? AND mall_id IN ?", actorID, uniqueIDs(mallIDs)).Count(&count).Error; err != nil || count != int64(len(uniqueIDs(mallIDs))) {
			return ErrAccessAccountForbidden
		}
	}
	if len(roleIDs) > 0 {
		var high int64
		if err := tx.Model(&model.Role{}).Where("id IN ? AND (is_super = ? OR code = ?)", roleIDs, true, model.RoleCodeAdmin).Count(&high).Error; err != nil || high > 0 {
			return ErrAccessAccountForbidden
		}
		var actorPermissions []string
		if err := tx.Table("role_permissions").Distinct("role_permissions.permission_code").Joins("JOIN user_roles ON user_roles.role_id = role_permissions.role_id").Joins("JOIN roles ON roles.id = user_roles.role_id").Where("user_roles.user_id = ? AND roles.status = ?", actorID, model.RoleStatusActive).Pluck("role_permissions.permission_code", &actorPermissions).Error; err != nil {
			return err
		}
		var requestedPermissions []string
		if err := tx.Model(&model.RolePermission{}).Distinct("permission_code").Where("role_id IN ?", roleIDs).Pluck("permission_code", &requestedPermissions).Error; err != nil {
			return err
		}
		owned := make(map[string]struct{}, len(actorPermissions))
		for _, permission := range actorPermissions {
			owned[permission] = struct{}{}
		}
		for _, permission := range requestedPermissions {
			if _, ok := owned[permission]; !ok {
				return ErrAccessAccountForbidden
			}
		}
	}
	return validateRoleAndMallExistence(tx, roleIDs, mallIDs)
}

func validateRoleAndMallExistence(tx *gorm.DB, roleIDs, mallIDs []uint) error {
	roles := uniqueIDs(roleIDs)
	if len(roles) > 0 {
		var count int64
		if err := tx.Model(&model.Role{}).Where("id IN ? AND status = ?", roles, model.RoleStatusActive).Count(&count).Error; err != nil || count != int64(len(roles)) {
			return ErrAccessAccountInvalid
		}
	}
	malls := uniqueIDs(mallIDs)
	if len(malls) > 0 {
		var count int64
		if err := tx.Model(&model.Mall{}).Where("id IN ?", malls).Count(&count).Error; err != nil || count != int64(len(malls)) {
			return ErrAccessAccountInvalid
		}
	}
	return nil
}

func replaceAccountRoles(ctx context.Context, tx *gorm.DB, actorID, targetID uint, roleIDs []uint) error {
	if err := tx.WithContext(ctx).Where("user_id = ?", targetID).Delete(&model.UserRole{}).Error; err != nil {
		return err
	}
	for _, roleID := range uniqueIDs(roleIDs) {
		if err := tx.Create(&model.UserRole{UserID: targetID, RoleID: roleID, CreatedBy: actorID}).Error; err != nil {
			return err
		}
	}
	return nil
}

func replaceAccountMalls(ctx context.Context, tx *gorm.DB, actorID, targetID uint, mode string, mallIDs []uint) error {
	if err := tx.WithContext(ctx).Where("user_id = ?", targetID).Delete(&model.UserMallScope{}).Error; err != nil {
		return err
	}
	if mode != model.MallScopeSelected {
		return nil
	}
	for _, mallID := range uniqueIDs(mallIDs) {
		if err := tx.Create(&model.UserMallScope{UserID: targetID, MallID: mallID, CreatedBy: actorID}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (service *AccessAccountService) accountDTO(ctx context.Context, user *model.User) (AccessAccountDTO, error) {
	var roles []ConsoleRoleDTO
	if err := service.db.WithContext(ctx).Table("roles").Select("roles.code, roles.name").Joins("JOIN user_roles ON user_roles.role_id = roles.id").Where("user_roles.user_id = ?", user.ID).Scan(&roles).Error; err != nil {
		return AccessAccountDTO{}, err
	}
	var mallIDs []uint
	if user.MallScopeMode == model.MallScopeSelected {
		if err := service.db.WithContext(ctx).Model(&model.UserMallScope{}).Where("user_id = ?", user.ID).Pluck("mall_id", &mallIDs).Error; err != nil {
			return AccessAccountDTO{}, err
		}
	}
	sort.Slice(roles, func(i, j int) bool { return roles[i].Code < roles[j].Code })
	sort.Slice(mallIDs, func(i, j int) bool { return mallIDs[i] < mallIDs[j] })
	phone := ""
	if user.Phone != nil {
		phone = maskPhone(*user.Phone)
	}
	return AccessAccountDTO{ID: user.ID, Account: user.Account, Phone: phone, Nickname: user.Nickname, Status: user.Status, MallScopeMode: user.MallScopeMode, Roles: roles, MallIDs: mallIDs, LastLoginAt: user.LastLoginAt}, nil
}

func validAccessWrite(key, reason string) bool {
	return dataAuthorizationKeyPattern.MatchString(strings.TrimSpace(key)) && strings.TrimSpace(reason) != "" && utf8.RuneCountInString(reason) <= 500 && !strings.ContainsAny(reason, "\r\n\x00")
}

func uniqueIDs(values []uint) []uint {
	seen := map[uint]struct{}{}
	result := make([]uint, 0, len(values))
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func createAccessAudit(tx *gorm.DB, actorID uint, action, targetType string, targetID uint, reason, requestID string) error {
	audit := model.AuthAudit{ActorUserID: actorID, Action: action, TargetType: targetType, TargetID: targetID, BeforeJSON: model.JSONText(`{}`), AfterJSON: model.JSONText(`{}`), Reason: strings.TrimSpace(reason), RequestID: strings.TrimSpace(requestID)}
	return tx.Create(&audit).Error
}
