// jwt 认证
package jwt

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/gin-gonic/gin"
	jwtPkg "github.com/golang-jwt/jwt"

	"gin-biz-web-api/pkg/app"
	"gin-biz-web-api/pkg/config"
	"gin-biz-web-api/pkg/logger"
)

var (
	ErrTokenExpired           = errors.New("令牌已过期")
	ErrTokenExpiredMaxRefresh = errors.New("令牌已过最大刷新时间")
	ErrTokenMalformed         = errors.New("请求令牌格式有误")
	ErrTokenInvalid           = errors.New("请求令牌无效")
	ErrTokenNotFound          = errors.New("无法找到令牌")
)

// JWT 定义一个 jwt 对象
type JWT struct {

	// 密钥，用以加密 JWT，从配置文件 config/jwt.go 中读取
	Key []byte

	// 刷新 token 的最大过期时间
	MaxRefresh time.Duration

	// 过期时间（分钟），用于测试
	ExpireTime int64
}

// JWTCustomClaims 自定义载荷
type JWTCustomClaims struct {
	U string `json:"u"` // 用户ID
	T string `json:"t"` // 令牌类型: "r"(refreshable) 或 "p"(permanent)
	E int64  `json:"e"` // 过期时间
	I int64  `json:"i"` // 签发时间
}

// Valid 实现jwt.Claims接口的方法
func (c *JWTCustomClaims) Valid() error {
	// 手动验证时间
	now := time.Now().Unix()
	if c.E < now {
		return ErrTokenExpired
	}
	return nil
}

func NewJWT() *JWT {
	return &JWT{
		Key:        []byte(config.GetString("cfg.jwt.key")),                                  // 密钥
		MaxRefresh: time.Duration(config.GetInt64("cfg.jwt.max_refresh_time")) * time.Minute, // 允许刷新时间
		ExpireTime: config.GetInt64("cfg.jwt.expire_time"),                                   // 过期时间（分钟）
	}
}

// ParseToken 解析 token
func (j *JWT) ParseToken(c *gin.Context, userToken ...string) (*JWTCustomClaims, error) {
	var (
		tokenStr string
		err      error
	)

	if len(userToken) > 0 {
		tokenStr = userToken[0]
	} else {
		// 获取 token
		tokenStr, err = j.GetToken(c)
		if err != nil {
			return nil, err
		}
	}

	// 解析用户 token
	token, err := j.parseTokenString(tokenStr)

	// 解析出错时
	if err != nil {
		validationErr, ok := err.(*jwtPkg.ValidationError)
		if ok {
			switch validationErr.Errors {
			case jwtPkg.ValidationErrorMalformed:
				return nil, ErrTokenMalformed
			case jwtPkg.ValidationErrorExpired:
				return nil, ErrTokenExpired
			}
		}
		return nil, ErrTokenInvalid
	}

	// 将 token 中的 claims 信息解析出来和 JWTCustomClaims 数据结构进行校验
	if claims, ok := token.Claims.(*JWTCustomClaims); ok {
		// 手动验证时间
		now := time.Now().Unix()
		if claims.E < now {
			return nil, ErrTokenExpired
		}
		return claims, nil
	}

	return nil, ErrTokenInvalid
}

// GetTTL 计算出 token 还剩多少秒后过期
func (j *JWT) GetTTL(c *gin.Context, userToken ...string) (int64, error) {

	claims, err := j.ParseToken(c, userToken...)

	if err != nil {
		// 此时已经过期，或者出现 token 解析失败
		return 0, err
	}

	// 此时的 token 一定是没有过期的，否则上一步 ParseToken 就已经报错了
	ttl := claims.E - app.TimeNowInTimezone().Unix()

	return ttl, nil
}

// RefreshToken 刷新 token
func (j *JWT) RefreshToken(c *gin.Context) (string, error) {
	// 获取 token
	tokenStr, err := j.GetToken(c)
	if err != nil {
		return "", err
	}

	// 解析用户 token
	token, err := j.parseTokenString(tokenStr)

	// 解析出错时（未报错证明是合法的 token 或者未到过期时间）
	if err != nil {
		validationErr, ok := err.(*jwtPkg.ValidationError)
		// 如果满足刷新 token 的条件，就继续往下走下一步（只要是单一的 ValidationErrorExpired 报错就认为是）
		if !ok || validationErr.Errors != jwtPkg.ValidationErrorExpired {
			return "", err
		}
	}

	// 解析出自定义的载荷信息 JWTCustomClaims
	claims := token.Claims.(*JWTCustomClaims)

	// 检查令牌类型
	if claims.T == "p" {
		// 永久令牌不需要刷新，直接返回原令牌
		return tokenStr, nil
	}

	// 检查是否过了【最大允许刷新的时间】
	// 首次签名时间 + 最大允许刷新时间区间 > 当前时间 ====> 首次签名时间 > 当前时间 - 最大允许刷新时间区间
	if claims.I > time.Now().Add(-j.MaxRefresh).Unix() {
		// 此时并没有过最大允许刷新时间，因此可以重新颁发 token
		// 创建新的 claims 对象，使用简化的结构
		newClaims := &JWTCustomClaims{
			U: claims.U,
			T: claims.T,
			E: j.expireAtTime(), // 更新过期时间
			I: claims.I,         // 保持原有的签名时间
		}
		return j.createToken(newClaims)
	}

	// 当前时间过了最大允许刷新的时间
	return "", ErrTokenExpiredMaxRefresh
}

// GenerateToken 生成 token
func (j *JWT) GenerateToken(userId string, tokenType string) string {
	// 构造用户 claims 信息（负荷）
	var expireAtTime int64
	if tokenType == "p" || tokenType == "permanent" {
		// 永久令牌，设置一个非常大的过期时间（100年）
		expireAtTime = time.Now().Add(100 * 365 * 24 * time.Hour).Unix()
		tokenType = "p"
	} else {
		// 可刷新令牌，使用配置的过期时间
		expireAtTime = j.expireAtTime()
		tokenType = "r" // 默认类型
	}

	now := time.Now().Unix()
	claims := &JWTCustomClaims{
		U: userId,
		T: tokenType,
		E: expireAtTime,
		I: now,
	}

	// 根据 claims 生成 token
	token, err := j.createToken(claims)
	if err != nil {
		logger.LogErrorIf(err)
		return ""
	}

	return token
}

// createToken 创建 token，用于内部调用
func (j *JWT) createToken(claims *JWTCustomClaims) (string, error) {
	// 自定义token生成方式，使用更紧凑的编码
	// 格式：用户ID-令牌类型-过期时间-签发时间-签名
	// 然后使用Base64URL编码

	// 为了控制长度，我们只保留必要的信息
	// 并使用更短的格式
	tokenData := fmt.Sprintf("%s:%s:%d:%d", claims.U, claims.T, claims.E, claims.I)

	// 生成签名
	h := hmac.New(sha256.New, j.Key)
	h.Write([]byte(tokenData))
	signature := h.Sum(nil)

	// 取签名的前8个字节
	if len(signature) > 8 {
		signature = signature[:8]
	}

	// 组合token
	fullToken := fmt.Sprintf("%s:%x", tokenData, signature)

	// 使用Base64URL编码
	token := base64.RawURLEncoding.EncodeToString([]byte(fullToken))

	// 确保token长度不超过50个字符
	if len(token) > 50 {
		// 如果超过50个字符，使用更紧凑的格式
		// 只保留用户ID、过期时间和简短签名
		compactData := fmt.Sprintf("%s:%d", claims.U, claims.E)
		h.Reset()
		h.Write([]byte(compactData))
		compactSignature := h.Sum(nil)
		if len(compactSignature) > 4 {
			compactSignature = compactSignature[:4]
		}
		compactToken := fmt.Sprintf("%s:%x", compactData, compactSignature)
		token = base64.RawURLEncoding.EncodeToString([]byte(compactToken))
	}

	// 确保token长度不超过50个字符
	if len(token) > 50 {
		token = token[:50]
	}

	return token, nil
}

// expireAtTime 获取过期时间点
func (j *JWT) expireAtTime() int64 {
	timeNow := time.Now() // 获取当前时间
	expireTime := j.ExpireTime
	if expireTime == 0 {
		expireTime = config.GetInt64("cfg.jwt.expire_time")
	}
	expire := time.Duration(expireTime) * time.Minute
	// 返回加过期时间区间后的时间点
	return timeNow.Add(expire).Unix()
}

// parseTokenString 解析 Token
func (j *JWT) parseTokenString(tokenStr string) (*jwtPkg.Token, error) {
	// 尝试解析自定义格式的token
	// 解码Base64URL
	tokenBytes, err := base64.RawURLEncoding.DecodeString(tokenStr)
	if err == nil {
		tokenData := string(tokenBytes)
		// 尝试解析紧凑格式
		parts := strings.Split(tokenData, ":")
		if len(parts) >= 3 {
			// 紧凑格式: 用户ID:过期时间:签名
			uid := parts[0]
			expireStr := parts[1]
			signature := parts[2]

			expire, err := strconv.ParseInt(expireStr, 10, 64)
			if err == nil {
				// 验证签名
				compactData := fmt.Sprintf("%s:%d", uid, expire)
				h := hmac.New(sha256.New, j.Key)
				h.Write([]byte(compactData))
				expectedSignature := h.Sum(nil)
				if len(expectedSignature) > 4 {
					expectedSignature = expectedSignature[:4]
				}
				expectedSignatureHex := fmt.Sprintf("%x", expectedSignature)

				if signature == expectedSignatureHex {
					// 签名验证通过，创建claims
					claims := &JWTCustomClaims{
						U: uid,
						T: "r", // 默认类型
						E: expire,
						I: time.Now().Unix(), // 使用当前时间作为签发时间
					}
					// 检查是否过期
					if claims.E < time.Now().Unix() {
						// 创建一个token对象
						token := jwtPkg.NewWithClaims(jwtPkg.SigningMethodHS256, claims)
						// 返回过期错误
						return token, &jwtPkg.ValidationError{Errors: jwtPkg.ValidationErrorExpired}
					}
					// 创建一个token对象
					token := jwtPkg.NewWithClaims(jwtPkg.SigningMethodHS256, claims)
					return token, nil
				}
			}
		}
		// 尝试解析完整格式
		if len(parts) >= 5 {
			// 完整格式: 用户ID:令牌类型:过期时间:签发时间:签名
			uid := parts[0]
			tokenType := parts[1]
			expireStr := parts[2]
			issuedStr := parts[3]
			signature := parts[4]

			expire, err := strconv.ParseInt(expireStr, 10, 64)
			if err != nil {
				return nil, ErrTokenInvalid
			}

			issued, err := strconv.ParseInt(issuedStr, 10, 64)
			if err != nil {
				return nil, ErrTokenInvalid
			}

			// 验证签名
			tokenDataWithoutSig := fmt.Sprintf("%s:%s:%d:%d", uid, tokenType, expire, issued)
			h := hmac.New(sha256.New, j.Key)
			h.Write([]byte(tokenDataWithoutSig))
			expectedSignature := h.Sum(nil)
			if len(expectedSignature) > 8 {
				expectedSignature = expectedSignature[:8]
			}
			expectedSignatureHex := fmt.Sprintf("%x", expectedSignature)

			if signature == expectedSignatureHex {
				// 签名验证通过，创建claims
				claims := &JWTCustomClaims{
					U: uid,
					T: tokenType,
					E: expire,
					I: issued,
				}
				// 检查是否过期
				if claims.E < time.Now().Unix() {
					// 创建一个token对象
					token := jwtPkg.NewWithClaims(jwtPkg.SigningMethodHS256, claims)
					// 返回过期错误
					return token, &jwtPkg.ValidationError{Errors: jwtPkg.ValidationErrorExpired}
				}
				// 创建一个token对象
				token := jwtPkg.NewWithClaims(jwtPkg.SigningMethodHS256, claims)
				return token, nil
			}
		}
	}

	// 如果解析失败，尝试使用原始的JWT解析
	token, err := jwtPkg.ParseWithClaims(tokenStr, &JWTCustomClaims{}, func(token *jwtPkg.Token) (interface{}, error) {
		return j.Key, nil
	})

	// 如果是过期错误，仍然返回token，让调用者处理
	if err != nil {
		validationErr, ok := err.(*jwtPkg.ValidationError)
		if ok && validationErr.Errors == jwtPkg.ValidationErrorExpired {
			return token, nil
		}
	}

	return token, err
}

// GetToken 获取请求中的 token 参数
func (j *JWT) GetToken(c *gin.Context) (string, error) {
	var token string

	if query, exists := c.GetQuery("token"); exists && "" != query {
		token = query
	} else if post, exists := c.GetPostForm("token"); exists && "" != post {
		token = post
	} else {
		token = c.GetHeader("token")
	}

	if "" == token {
		return "", ErrTokenNotFound
	}

	return token, nil
}

// GetUserIDFromToken 从令牌中获取用户 ID，即使令牌过期
func (j *JWT) GetUserIDFromToken(tokenStr string) (string, error) {
	// 解析令牌，忽略过期错误
	token, err := j.parseTokenString(tokenStr)

	if err != nil {
		// 检查是否是令牌过期错误
		validationErr, ok := err.(*jwtPkg.ValidationError)
		if !ok || validationErr.Errors != jwtPkg.ValidationErrorExpired {
			return "", ErrTokenInvalid
		}
	}

	// 解析出 claims
	if claims, ok := token.Claims.(*JWTCustomClaims); ok {
		return claims.U, nil
	}

	return "", ErrTokenInvalid
}

// TestTokenLength 测试token长度
func TestTokenLength() {
	// 创建一个临时的JWT实例
	j := &JWT{
		Key:        []byte("test_key"),
		MaxRefresh: 86400 * time.Minute,
		ExpireTime: 120, // 2小时
	}

	// 生成token
	token := j.GenerateToken("123", "r")
	println("Token:", token)
	println("Token length:", len(token))

	// 测试不同用户ID长度的token
	for _, uid := range []string{"1", "12345", "1234567890"} {
		token := j.GenerateToken(uid, "r")
		println("UID:", uid, "Token length:", len(token))
	}

	// 测试永久令牌
	permanentToken := j.GenerateToken("123", "p")
	println("Permanent token:", permanentToken)
	println("Permanent token length:", len(permanentToken))
}
