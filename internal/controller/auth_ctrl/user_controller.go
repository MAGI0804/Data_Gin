package auth_ctrl

import (
	"github.com/gin-gonic/gin"

	"gin-biz-web-api/pkg/auth"
	"gin-biz-web-api/pkg/responses"
)

type UserController struct {
}

// Profile 用户个人信息
// curl --location --request GET '0.0.0.0:3000/api/auth/me' \
// --header 'token: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoiMjQiLCJleHBpcmVfdGltZSI6MTY1MzEzMDM0MCwiZXhwIjoxNjUzMTMwMzQwLCJpYXQiOjE2NDc5NDYzNDAsImlzcyI6Imdpbi1iaXotd2ViLWFwaSIsIm5iZiI6MTY0Nzk0NjM0MH0.3Rzl8PmE519qWVmNziJ6ovH6Bwq5hnqmelkMUxfYsXc'
func (ctrl *UserController) Profile(c *gin.Context) {
	profile := auth.CurrentUser(c)
	responses.New(c).ToResponse(gin.H{
		"id": profile.ID, "account": profile.Account, "email": profile.Email,
		"nickname": profile.Nickname, "consoleManaged": profile.ConsoleManaged,
	})
}
