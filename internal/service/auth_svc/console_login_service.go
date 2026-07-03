package auth_svc

import (
	"context"
	"fmt"
	"time"

	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"
	"gin-biz-web-api/pkg/hash"
	"gin-biz-web-api/pkg/jwt"

	"gorm.io/gorm"
)

const (
	consoleAdminAccount  = "admin"
	consoleAdminPassword = "youlan123"
)

type ConsoleLoginService struct {
	db *gorm.DB
}

func NewConsoleLoginService() *ConsoleLoginService {
	return &ConsoleLoginService{db: database.DB}
}

func (s *ConsoleLoginService) Login(ctx context.Context, username, password string) (string, *model.User, error) {
	if username != consoleAdminAccount || password != consoleAdminPassword {
		return "", nil, fmt.Errorf("invalid console credentials")
	}

	user, err := s.ensureAdminUser(ctx)
	if err != nil {
		return "", nil, err
	}

	token := jwt.NewJWT().GenerateToken(user.GetStringID(), "refreshable")
	if token == "" {
		return "", nil, fmt.Errorf("generate console token failed")
	}

	return token, user, nil
}

func (s *ConsoleLoginService) ensureAdminUser(ctx context.Context) (*model.User, error) {
	now := int(time.Now().Unix())
	user := model.User{}
	err := s.db.WithContext(ctx).Where("account = ?", consoleAdminAccount).First(&user).Error
	if err == nil {
		if !hash.BcryptCheck(consoleAdminPassword, user.Password) {
			if user.CommonTimestampsField == nil {
				user.CommonTimestampsField = &model.CommonTimestampsField{}
			}
			user.Password = consoleAdminPassword
			user.UpdatedAt = now
			if err := s.db.WithContext(ctx).Save(&user).Error; err != nil {
				return nil, err
			}
		}
		return &user, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	user = model.User{
		BaseModel:             &model.BaseModel{},
		Account:               consoleAdminAccount,
		Email:                 "admin@warehouse.local",
		Password:              consoleAdminPassword,
		Nickname:              "管理员",
		CommonTimestampsField: &model.CommonTimestampsField{CreatedAt: now, UpdatedAt: now},
	}
	if err := s.db.WithContext(ctx).Create(&user).Error; err != nil {
		return nil, err
	}

	return &user, nil
}
