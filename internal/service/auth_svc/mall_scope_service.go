package auth_svc

import (
	"context"
	"errors"
	"fmt"

	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"

	"gorm.io/gorm"
)

var ErrMallScopeForbidden = errors.New("mall scope: forbidden")

// MallScopeService is the shared data-scope boundary for both console and
// Open API accounts. ALL bypasses the relation table; SELECTED fails closed.
type MallScopeService struct {
	db *gorm.DB
}

func NewMallScopeService(databases ...*gorm.DB) *MallScopeService {
	db := database.DB
	if len(databases) > 0 && databases[0] != nil {
		db = databases[0]
	}
	return &MallScopeService{db: db}
}

func (service *MallScopeService) CanAccess(ctx context.Context, userID, mallID uint) (bool, error) {
	if service == nil || service.db == nil || ctx == nil || userID == 0 || mallID == 0 {
		return false, fmt.Errorf("mall scope: invalid request")
	}
	var user model.User
	if err := service.db.WithContext(ctx).Select("id", "status", "mall_scope_mode").First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("mall scope: read user: %w", err)
	}
	if user.Status != model.AccountStatusActive {
		return false, nil
	}
	if user.MallScopeMode == model.MallScopeAll {
		return true, nil
	}
	if user.MallScopeMode != model.MallScopeSelected {
		return false, nil
	}
	var count int64
	if err := service.db.WithContext(ctx).Model(&model.UserMallScope{}).
		Where("user_id = ? AND mall_id = ?", userID, mallID).Count(&count).Error; err != nil {
		return false, fmt.Errorf("mall scope: read selected mall: %w", err)
	}
	return count == 1, nil
}

func (service *MallScopeService) Require(ctx context.Context, userID, mallID uint) error {
	allowed, err := service.CanAccess(ctx, userID, mallID)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrMallScopeForbidden
	}
	return nil
}

// Apply filters a mall query at the DAO boundary. Unknown scope modes produce
// an empty result instead of accidentally widening access.
func (service *MallScopeService) Apply(ctx context.Context, query *gorm.DB, userID uint, mallIDColumn string) (*gorm.DB, error) {
	if service == nil || service.db == nil || ctx == nil || query == nil || userID == 0 {
		return nil, fmt.Errorf("mall scope: invalid query")
	}
	if mallIDColumn == "" {
		mallIDColumn = "id"
	}
	var user model.User
	if err := service.db.WithContext(ctx).Select("id", "status", "mall_scope_mode").First(&user, userID).Error; err != nil {
		return nil, fmt.Errorf("mall scope: read user: %w", err)
	}
	if user.Status != model.AccountStatusActive {
		return query.Where("1 = 0"), nil
	}
	switch user.MallScopeMode {
	case model.MallScopeAll:
		return query, nil
	case model.MallScopeSelected:
		return query.Where(
			mallIDColumn+" IN (?)",
			service.db.WithContext(ctx).Model(&model.UserMallScope{}).Select("mall_id").Where("user_id = ?", userID),
		), nil
	default:
		return query.Where("1 = 0"), nil
	}
}
