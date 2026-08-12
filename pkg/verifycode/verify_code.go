// 数字验证码处理逻辑
package verifycode

import (
	"fmt"
	"sync"

	"gin-biz-web-api/pkg/config"
	"gin-biz-web-api/pkg/email"
	"gin-biz-web-api/pkg/helper/strx"
)

type VerifyCode struct {
	Driver Driver
}

var (
	once               sync.Once // once 确保 internalVerifyCode 对象只初始化一次
	internalVerifyCode *VerifyCode
)

// NewVerifyCode 单例模式获取
func NewVerifyCode() *VerifyCode {
	once.Do(func() {

		// 这里的 driver 也可以绑定其他的驱动
		internalVerifyCode = &VerifyCode{Driver: &RedisDriver{
			KeyPrefix: "verify_code:",
			Group:     "default", // 使用默认的 config/redis.go 中的 default 配置连接
		}}

	})

	return internalVerifyCode
}

// GenerateVerifyCode 生成随机数字验证码
func (v *VerifyCode) GenerateVerifyCode(key string) string {

	// 生成指定长度的数字随机码作为验证码
	// code := strx.StrRandomNumber(config.GetInt("cfg.verify_code.length"))
	code := strx.StrRandomOptionalString(config.GetInt("cfg.verify_code.length"), strx.Numeric)

	// 将 key 和 code 进行保存，方便后续验证
	if !v.Driver.Set(key, code) {
		return ""
	}

	return code
}

// CheckVerifyCode 检查验证码是否正确
func (v *VerifyCode) CheckVerifyCode(key, answer string) bool {
	return v.Driver.Verify(key, answer, true)
}

// SendEmailVerifyCode 发送邮件验证码
func (v *VerifyCode) SendEmailVerifyCode(mail string) error {

	// 生成验证码
	code := v.GenerateVerifyCode(mail)
	if code == "" {
		return fmt.Errorf("verify code storage unavailable")
	}
	// 验证码的有效期
	expire := config.GetInt64("cfg.verify_code.expire_time")

	content := fmt.Sprintf("<h1> 尊敬的用户：您的验证码为 %v 有效期为 %v 分钟。</h1>", code, expire)

	// 发送邮件
	err := email.NewMailer().SendMail([]string{mail}, "Email 验证码", content)
	if err != nil {
		return err
	}

	return nil
}
