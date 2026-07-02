package auth_ctrl

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"gin-biz-web-api/internal/msg"
	"gin-biz-web-api/internal/requests/auth_request"
	"gin-biz-web-api/internal/service/auth_svc"
)

type LoginController struct {
	service *auth_svc.ConsoleLoginService
}

func NewLoginController() *LoginController {
	return &LoginController{
		service: auth_svc.NewConsoleLoginService(),
	}
}

func (ctrl *LoginController) ConsoleLogin(c *gin.Context) {
	var req auth_request.ConsoleLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, msg.ErrResponse("无效的登录参数", err))
		return
	}

	token, user, err := ctrl.service.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, msg.ErrResponse("账号或密码不正确", err))
		return
	}

	c.JSON(http.StatusOK, msg.SuccessResponse("登录成功", &map[string]any{
		"token": token,
		"user": gin.H{
			"id":       user.ID,
			"account":  user.Account,
			"nickname": user.Nickname,
		},
	}))
}
