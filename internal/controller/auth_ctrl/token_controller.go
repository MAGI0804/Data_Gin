package auth_ctrl

import (
	"github.com/gin-gonic/gin"

	"gin-biz-web-api/internal/requests/auth_request"
	"gin-biz-web-api/pkg/errcode"
	"gin-biz-web-api/pkg/jwt"
	"gin-biz-web-api/pkg/responses"
	"gin-biz-web-api/pkg/validator"
)

type TokenController struct {
}

// RefreshToken 刷新令牌
// curl --location --request POST '0.0.0.0:3000/api/auth/token/refresh'
// --header 'Content-Type: application/json'
// --header 'token: {your_token}'
// --data-raw '{"token_type": "refreshable"}'
func (ctrl *TokenController) RefreshToken(c *gin.Context) {
	response := responses.New(c)

	// 表单验证
	request := auth_request.RefreshTokenRequest{}
	if ok := validator.BindAndValidate(c, &request, auth_request.RefreshToken); !ok {
		return
	}

	// 获取令牌
	tokenStr, err := jwt.NewJWT().GetToken(c)
	if err != nil {
		response.ToErrorResponse(errcode.Unauthorized.WithDetails(err.Error()), err.Error())
		return
	}

	// 从令牌中获取用户 ID，即使令牌过期
	userID, err := jwt.NewJWT().GetUserIDFromToken(tokenStr)
	if err != nil {
		response.ToErrorResponse(errcode.Unauthorized.WithDetails(err.Error()), err.Error())
		return
	}

	// 检查是否提供了 token_type 参数
	if request.TokenType != "" {
		// 生成新令牌
		tokenType := request.TokenType
		newToken := jwt.NewJWT().GenerateToken(userID, tokenType)
		if newToken == "" {
			response.ToErrorResponse(errcode.InternalServerError, "生成令牌失败")
			return
		}

		response.ToResponse(gin.H{
			"token_type": tokenType,
			"token":      newToken,
		})
		return
	}

	// 如果没有提供 token_type，则使用 JWT 包的 RefreshToken 函数刷新令牌
	newToken, err := jwt.NewJWT().RefreshToken(c)
	if err != nil {
		response.ToErrorResponse(errcode.Unauthorized.WithDetails(err.Error()), err.Error())
		return
	}

	// 获取令牌类型
	newClaims, err := jwt.NewJWT().ParseToken(c, newToken)
	if err != nil {
		response.ToErrorResponse(errcode.Unauthorized.WithDetails(err.Error()), err.Error())
		return
	}

	response.ToResponse(gin.H{
		"token_type": newClaims.T,
		"token":      newToken,
	})
}

// GetTokenInfo 根据token查询当前的信息
// curl --location --request GET '0.0.0.0:3000/api/auth/token/info'
// --header 'token: {your_token}'
func (ctrl *TokenController) GetTokenInfo(c *gin.Context) {
	response := responses.New(c)

	// 获取令牌
	tokenStr, err := jwt.NewJWT().GetToken(c)
	if err != nil {
		response.ToErrorResponse(errcode.Unauthorized.WithDetails(err.Error()), err.Error())
		return
	}

	// 解析令牌
	claims, err := jwt.NewJWT().ParseToken(c, tokenStr)
	if err != nil {
		response.ToErrorResponse(errcode.Unauthorized.WithDetails(err.Error()), err.Error())
		return
	}

	// 计算令牌剩余过期时间
	ttl, err := jwt.NewJWT().GetTTL(c, tokenStr)
	if err != nil {
		ttl = 0
	}

	// 返回令牌信息
	response.ToResponse(gin.H{
		"user_id":     claims.U,
		"token_type":  claims.T,
		"expire_time": claims.E,
		"issued_time": claims.I,
		"ttl":         ttl,
	})
}
