package auth_svc

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

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

// ConstrainMallIDs intersects an explicit filter with the user's data scope.
// An empty requested set means all malls in the user's scope, never all malls
// globally for SELECTED accounts.
func (service *MallScopeService) ConstrainMallIDs(ctx context.Context, userID uint, requested []uint) ([]uint, error) {
	if service == nil || service.db == nil || ctx == nil || userID == 0 {
		return nil, fmt.Errorf("mall scope: invalid constraint")
	}
	var user model.User
	if err := service.db.WithContext(ctx).Select("id", "status", "mall_scope_mode").First(&user, userID).Error; err != nil {
		return nil, fmt.Errorf("mall scope: read user: %w", err)
	}
	if user.Status != model.AccountStatusActive {
		return nil, ErrMallScopeForbidden
	}
	if user.MallScopeMode == model.MallScopeAll {
		return uniqueScopeIDs(requested), nil
	}
	if user.MallScopeMode != model.MallScopeSelected {
		return nil, ErrMallScopeForbidden
	}
	var allowed []uint
	if err := service.db.WithContext(ctx).Model(&model.UserMallScope{}).Where("user_id = ?", userID).Pluck("mall_id", &allowed).Error; err != nil {
		return nil, fmt.Errorf("mall scope: read selected malls: %w", err)
	}
	allowed = uniqueScopeIDs(allowed)
	if len(allowed) == 0 {
		return nil, ErrMallScopeForbidden
	}
	if len(requested) == 0 {
		return allowed, nil
	}
	set := make(map[uint]struct{}, len(allowed))
	for _, id := range allowed {
		set[id] = struct{}{}
	}
	result := uniqueScopeIDs(requested)
	for _, id := range result {
		if _, ok := set[id]; !ok {
			return nil, ErrMallScopeForbidden
		}
	}
	return result, nil
}

func uniqueScopeIDs(values []uint) []uint {
	set := make(map[uint]struct{}, len(values))
	for _, id := range values {
		if id > 0 {
			set[id] = struct{}{}
		}
	}
	result := make([]uint, 0, len(set))
	for id := range set {
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

// ConstrainMallCodes applies the same scope policy to APIs whose stable mall
// identifier is a business code instead of a numeric ID.
func (service *MallScopeService) ConstrainMallCodes(ctx context.Context, userID uint, requested []string) ([]string, error) {
	if service == nil || service.db == nil || ctx == nil || userID == 0 {
		return nil, fmt.Errorf("mall scope: invalid code constraint")
	}
	var user model.User
	if err := service.db.WithContext(ctx).Select("id", "status", "mall_scope_mode").First(&user, userID).Error; err != nil {
		return nil, fmt.Errorf("mall scope: read user: %w", err)
	}
	requested = uniqueScopeCodes(requested)
	if user.Status != model.AccountStatusActive {
		return nil, ErrMallScopeForbidden
	}
	if user.MallScopeMode == model.MallScopeAll {
		return requested, nil
	}
	if user.MallScopeMode != model.MallScopeSelected {
		return nil, ErrMallScopeForbidden
	}
	var allowed []string
	if err := service.db.WithContext(ctx).Model(&model.Mall{}).
		Joins("JOIN user_mall_scopes ON user_mall_scopes.mall_id = malls.id").
		Where("user_mall_scopes.user_id = ?", userID).Pluck("malls.mall_code", &allowed).Error; err != nil {
		return nil, fmt.Errorf("mall scope: read selected mall codes: %w", err)
	}
	allowed = uniqueScopeCodes(allowed)
	if len(allowed) == 0 {
		return nil, ErrMallScopeForbidden
	}
	if len(requested) == 0 {
		return allowed, nil
	}
	set := make(map[string]struct{}, len(allowed))
	for _, code := range allowed {
		set[code] = struct{}{}
	}
	for _, code := range requested {
		if _, ok := set[code]; !ok {
			return nil, ErrMallScopeForbidden
		}
	}
	return requested, nil
}

func uniqueScopeCodes(values []string) []string {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		if code := strings.ToUpper(strings.TrimSpace(value)); code != "" {
			set[code] = struct{}{}
		}
	}
	result := make([]string, 0, len(set))
	for code := range set {
		result = append(result, code)
	}
	sort.Strings(result)
	return result
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
