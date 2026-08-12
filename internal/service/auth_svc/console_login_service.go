package auth_svc

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"gin-biz-web-api/constant"
	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"
	"gin-biz-web-api/pkg/hash"
	"gin-biz-web-api/pkg/jwt"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const EnvConsoleAdminInitialPassword = "CONSOLE_ADMIN_INITIAL_PASSWORD"

type ConsoleLoginService struct {
	db *gorm.DB
}

func NewConsoleLoginService() *ConsoleLoginService {
	return &ConsoleLoginService{db: database.DB}
}

// SyncExistingConsoleAdminPermissions backfills the canonical permission set
// for an admin that already exists. Admin creation remains login-driven.
func SyncExistingConsoleAdminPermissions(ctx context.Context, db *gorm.DB) (bool, error) {
	if ctx == nil || db == nil {
		return false, fmt.Errorf("sync console admin permissions: invalid database")
	}

	synchronized := false
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user model.User
		err := tx.WithContext(ctx).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("account = ?", constant.ConsoleAdmin).
			First(&user).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("find admin: %w", err)
		}
		if !user.ConsoleManaged && !isLegacyConsoleAdmin(&user) {
			return nil
		}
		if !isLegacyConsoleAdmin(&user) {
			return fmt.Errorf("reserved admin identity is not console managed")
		}
		if normalizeConsoleAdmin(&user) {
			if user.CommonTimestampsField == nil {
				user.CommonTimestampsField = &model.CommonTimestampsField{}
			}
			user.UpdatedAt = int(time.Now().Unix())
			if err := tx.WithContext(ctx).Save(&user).Error; err != nil {
				return fmt.Errorf("normalize admin: %w", err)
			}
		}
		if err := grantConsoleAdminPermissions(ctx, tx, user.ID); err != nil {
			return err
		}
		synchronized = true
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("sync console admin permissions: %w", err)
	}
	return synchronized, nil
}

func (s *ConsoleLoginService) Login(ctx context.Context, username, password string) (string, *model.User, error) {
	if username != constant.ConsoleAdmin || strings.TrimSpace(password) == "" {
		return "", nil, fmt.Errorf("invalid console credentials")
	}

	user, err := s.ensureAdminUser(ctx, password)
	if err != nil {
		return "", nil, err
	}

	token := jwt.NewJWT().GenerateVersionedToken(user.GetStringID(), "refreshable", user.AuthVersion)
	if token == "" {
		return "", nil, fmt.Errorf("generate console token failed")
	}

	return token, user, nil
}

func (s *ConsoleLoginService) ensureAdminUser(ctx context.Context, password string) (*model.User, error) {
	if s == nil || s.db == nil || ctx == nil {
		return nil, fmt.Errorf("ensure console admin: invalid database")
	}

	var user *model.User
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		user, err = ensureAdminUserRecord(ctx, tx, password)
		if err != nil {
			return err
		}
		if err := grantConsoleAdminPermissions(ctx, tx, user.ID); err != nil {
			return err
		}
		if err := grantConsoleSuperAdminRole(ctx, tx, user.ID); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("ensure console admin: %w", err)
	}
	return user, nil
}

func grantConsoleSuperAdminRole(ctx context.Context, db *gorm.DB, userID uint) error {
	var role model.Role
	if err := db.WithContext(ctx).Where("code = ? AND is_super = ?", model.RoleCodeSuperAdmin, true).First(&role).Error; err != nil {
		return fmt.Errorf("grant console super admin role: find role: %w", err)
	}
	assignment := model.UserRole{UserID: userID, RoleID: role.ID, CreatedBy: userID}
	if err := db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&assignment).Error; err != nil {
		return fmt.Errorf("grant console super admin role: create assignment: %w", err)
	}
	return nil
}

func ensureAdminUserRecord(ctx context.Context, db *gorm.DB, password string) (*model.User, error) {
	now := int(time.Now().Unix())
	user := model.User{}
	err := db.WithContext(ctx).Where("account = ?", constant.ConsoleAdmin).First(&user).Error
	if err == nil {
		if !user.ConsoleManaged && !isLegacyConsoleAdmin(&user) {
			return nil, fmt.Errorf("reserved admin identity is not console managed")
		}
		if !hash.BcryptCheck(password, user.Password) {
			return nil, fmt.Errorf("invalid console credentials")
		}
		needsSave := normalizeConsoleAdmin(&user)
		if needsSave {
			if user.CommonTimestampsField == nil {
				user.CommonTimestampsField = &model.CommonTimestampsField{}
			}
			user.UpdatedAt = now
			if err := db.WithContext(ctx).Save(&user).Error; err != nil {
				return nil, fmt.Errorf("update console admin: %w", err)
			}
		}
		if !isTrustedConsoleAdmin(&user) {
			return nil, fmt.Errorf("reserved admin identity is not console managed")
		}
		return &user, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("find console admin: %w", err)
	}

	initialPassword, ok := configuredInitialAdminPassword(password)
	if !ok {
		return nil, fmt.Errorf("invalid console credentials")
	}
	user = model.User{
		BaseModel:             &model.BaseModel{},
		Account:               constant.ConsoleAdmin,
		Password:              initialPassword,
		Nickname:              constant.ConsoleAdminName,
		ConsoleManaged:        true,
		AccountType:           model.AccountTypeConsole,
		Status:                model.AccountStatusActive,
		AuthVersion:           1,
		MallScopeMode:         model.MallScopeAll,
		CommonTimestampsField: &model.CommonTimestampsField{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.WithContext(ctx).Create(&user).Error; err != nil {
		if isDuplicateEntry(err) {
			var existing model.User
			findErr := db.WithContext(ctx).
				Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("account = ?", constant.ConsoleAdmin).
				First(&existing).Error
			if findErr != nil {
				return nil, fmt.Errorf("read concurrently created console admin: %w", findErr)
			}
			if !isTrustedConsoleAdmin(&existing) {
				return nil, fmt.Errorf("reserved admin identity is not console managed")
			}
			return &existing, nil
		}
		return nil, fmt.Errorf("create console admin: %w", err)
	}

	return &user, nil
}

func normalizeConsoleAdmin(user *model.User) bool {
	if user == nil {
		return false
	}
	changed := !user.ConsoleManaged ||
		user.AccountType != model.AccountTypeConsole ||
		user.Status != model.AccountStatusActive ||
		user.MallScopeMode != model.MallScopeAll ||
		user.AuthVersion == 0
	user.ConsoleManaged = true
	user.AccountType = model.AccountTypeConsole
	user.Status = model.AccountStatusActive
	user.MallScopeMode = model.MallScopeAll
	if user.AuthVersion == 0 {
		user.AuthVersion = 1
	}
	return changed
}

func configuredInitialAdminPassword(candidate string) (string, bool) {
	initialPassword, configured := os.LookupEnv(EnvConsoleAdminInitialPassword)
	return initialPassword, configured && strings.TrimSpace(initialPassword) != "" && candidate == initialPassword
}

func isTrustedConsoleAdmin(user *model.User) bool {
	return user != nil &&
		user.ConsoleManaged &&
		isLegacyConsoleAdmin(user)
}

func isLegacyConsoleAdmin(user *model.User) bool {
	email := ""
	if user != nil && user.Email != nil {
		email = *user.Email
	}
	return user != nil &&
		user.Account == constant.ConsoleAdmin &&
		(email == "" || email == constant.ConsoleAdminMail) &&
		user.Nickname == constant.ConsoleAdminName
}

func isDuplicateEntry(err error) bool {
	var mysqlErr *mysqlDriver.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}

func grantConsoleAdminPermissions(ctx context.Context, db *gorm.DB, userID uint) error {
	if err := data_dao.NewMallWeatherPermissionDAO(db).GrantPermanentPermissions(
		ctx,
		userID,
		userID,
		model.MallWeatherAdminPermissions(),
	); err != nil {
		return fmt.Errorf("grant console admin permissions: %w", err)
	}
	return nil
}
