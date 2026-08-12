package model

import (
	"time"

	"gorm.io/gorm"

	"gin-biz-web-api/pkg/hash"
)

type User struct {
	*BaseModel

	// 账号   UNI varchar(255) is_nullable: NO
	Account string `gorm:"column:account;unique;" json:"account"`
	// 邮箱   UNI varchar(80) is_nullable: YES
	Email string `gorm:"column:email;unique;" json:"-"`
	// 手机号   UNI varchar(40) is_nullable: YES
	Phone string `gorm:"column:phone;unique;" json:"-"`
	// 密码    varchar(255) is_nullable: NO
	Password string `gorm:"column:password;" json:"-"`
	// 昵称   MUL varchar(255) is_nullable: NO
	Nickname string `gorm:"column:nickname;" json:"nickname"`
	// 自我简介    text is_nullable: YES
	Introduction string `gorm:"column:introduction;" json:"introduction"`
	// 头像地址    varchar(255) is_nullable: NO
	Avatar string `gorm:"column:avatar;" json:"avatar"`
	// 控制台管理员身份只能由内部控制台登录流程设置，公开注册不可写入。
	ConsoleManaged    bool       `gorm:"column:console_managed;not null;default:false" json:"-"`
	AccountType       string     `gorm:"column:account_type;size:16;not null;default:'CONSOLE';index" json:"accountType"`
	Status            string     `gorm:"column:status;size:16;not null;default:'ACTIVE';index" json:"status"`
	AuthVersion       uint64     `gorm:"column:auth_version;not null;default:1" json:"-"`
	MallScopeMode     string     `gorm:"column:mall_scope_mode;size:16;not null;default:'SELECTED'" json:"mallScopeMode"`
	LastLoginAt       *time.Time `gorm:"column:last_login_at;type:datetime(3)" json:"lastLoginAt,omitempty"`
	PasswordChangedAt *time.Time `gorm:"column:password_changed_at;type:datetime(3)" json:"-"`

	*CommonTimestampsField
}

func (u User) TableName() string {
	return "users"
}

// 模型钩子
// 参见： [gorm document link](https://gorm.io/zh_CN/docs/hooks.html)
// func (u *User) BeforeSave(tx *gorm.DB) (err error) {}
// func (u *User) BeforeCreate(tx *gorm.DB) (err error) {}
// func (u *User) AfterCreate(tx *gorm.DB) (err error) {}
// func (u *User) BeforeUpdate(tx *gorm.DB) (err error) {}
// func (u *User) AfterUpdate(tx *gorm.DB) (err error) {}
// func (u *User) AfterSave(tx *gorm.DB) (err error) {}
// func (u *User) BeforeDelete(tx *gorm.DB) (err error) {}
// func (u *User) AfterDelete(tx *gorm.DB) (err error) {}
// func (u *User) AfterFind(tx *gorm.DB) (err error) {}

// BeforeSave GORM 的模型钩子，在创建和更新模型前调用
func (u *User) BeforeSave(tx *gorm.DB) (err error) {

	// 先判断用户密码是否已经经过 hash 加密
	if !hash.BcryptIsHashed(u.Password) {
		// 如果没有经过 hash 加密，则触发加密
		u.Password = hash.BcryptHash(u.Password)
	}

	return
}
