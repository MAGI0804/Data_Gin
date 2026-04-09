package auth_svc

import (
	"context"

	"gin-biz-web-api/internal/dao/auth_dao"
	"gin-biz-web-api/model"
)

// TokenDataService token_data服务
type TokenDataService struct {
	tokenDataDAO *auth_dao.TokenDataDAO
}

// NewTokenDataService 创建TokenDataService实例
func NewTokenDataService() *TokenDataService {
	return &TokenDataService{
		tokenDataDAO: auth_dao.NewTokenDataDAO(),
	}
}

// CreateTokenData 创建token数据
func (s *TokenDataService) CreateTokenData(ctx context.Context, tokenData *model.TokenData) (uint, error) {
	return s.tokenDataDAO.Create(ctx, tokenData)
}

// UpdateTokenData 更新token数据
func (s *TokenDataService) UpdateTokenData(ctx context.Context, tokenData *model.TokenData) error {
	return s.tokenDataDAO.Update(ctx, tokenData)
}

// GetTokenDataByID 根据ID获取token数据
func (s *TokenDataService) GetTokenDataByID(ctx context.Context, id uint) (*model.TokenData, error) {
	return s.tokenDataDAO.FindByID(ctx, id)
}

// GetAllTokenData 获取所有token数据
func (s *TokenDataService) GetAllTokenData(ctx context.Context) ([]model.TokenData, error) {
	return s.tokenDataDAO.FindAll(ctx)
}

// DeleteTokenData 删除token数据
func (s *TokenDataService) DeleteTokenData(ctx context.Context, id uint) error {
	return s.tokenDataDAO.Delete(ctx, id)
}
