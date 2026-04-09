package auth_dao

import (
	"context"
	"encoding/json"

	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"

	"gorm.io/gorm"
)

// TokenDataDAO token_data表的DAO
type TokenDataDAO struct {
	db *gorm.DB
}

// NewTokenDataDAO 创建TokenDataDAO实例
func NewTokenDataDAO() *TokenDataDAO {
	return &TokenDataDAO{
		db: database.DB,
	}
}

// Create 创建token数据
func (dao *TokenDataDAO) Create(ctx context.Context, tokenData *model.TokenData) (uint, error) {
	// 序列化VerificationInfo为JSON字符串
	if tokenData.VerificationInfo != nil {
		verificationInfoJSON, err := json.Marshal(tokenData.VerificationInfo)
		if err != nil {
			return 0, err
		}
		tokenData.VerificationInfo = string(verificationInfoJSON)
	}
	result := dao.db.WithContext(ctx).Create(tokenData)
	return tokenData.ID, result.Error
}

// Update 更新token数据
func (dao *TokenDataDAO) Update(ctx context.Context, tokenData *model.TokenData) error {
	// 序列化VerificationInfo为JSON字符串
	if tokenData.VerificationInfo != nil {
		verificationInfoJSON, err := json.Marshal(tokenData.VerificationInfo)
		if err != nil {
			return err
		}
		tokenData.VerificationInfo = string(verificationInfoJSON)
	}
	return dao.db.WithContext(ctx).Save(tokenData).Error
}

// FindByID 根据ID查询token数据
func (dao *TokenDataDAO) FindByID(ctx context.Context, id uint) (*model.TokenData, error) {
	var tokenData model.TokenData
	err := dao.db.WithContext(ctx).First(&tokenData, id).Error
	if err != nil {
		return nil, err
	}
	// 反序列化VerificationInfo为JSON对象
	if tokenData.VerificationInfo != nil {
		if jsonStr, ok := tokenData.VerificationInfo.(string); ok {
			var verificationInfo interface{}
			err := json.Unmarshal([]byte(jsonStr), &verificationInfo)
			if err != nil {
				return nil, err
			}
			tokenData.VerificationInfo = verificationInfo
		}
	}
	return &tokenData, nil
}

// FindAll 查询所有token数据
func (dao *TokenDataDAO) FindAll(ctx context.Context) ([]model.TokenData, error) {
	var tokenDataList []model.TokenData
	err := dao.db.WithContext(ctx).Find(&tokenDataList).Error
	if err != nil {
		return nil, err
	}
	// 反序列化VerificationInfo为JSON对象
	for i := range tokenDataList {
		if tokenDataList[i].VerificationInfo != nil {
			if jsonStr, ok := tokenDataList[i].VerificationInfo.(string); ok {
				var verificationInfo interface{}
				err := json.Unmarshal([]byte(jsonStr), &verificationInfo)
				if err != nil {
					return nil, err
				}
				tokenDataList[i].VerificationInfo = verificationInfo
			}
		}
	}
	return tokenDataList, nil
}

// Delete 删除token数据
func (dao *TokenDataDAO) Delete(ctx context.Context, id uint) error {
	return dao.db.WithContext(ctx).Delete(&model.TokenData{}, id).Error
}
