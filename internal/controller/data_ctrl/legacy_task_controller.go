package data_ctrl

import (
	"github.com/gin-gonic/gin"

	"gin-biz-web-api/internal/msg"
	"gin-biz-web-api/internal/service/data_svc"
)

type LegacyTaskController struct {
	service *data_svc.LegacyTaskService
}

func NewLegacyTaskController() *LegacyTaskController {
	return &LegacyTaskController{
		service: data_svc.NewLegacyTaskService(),
	}
}

func (ctrl *LegacyTaskController) List(c *gin.Context) {
	tasks := ctrl.service.ListDefinitions(c.Request.Context())
	c.JSON(200, msg.SuccessResponse("查询旧任务规则成功", &map[string]any{
		"tasks": tasks,
	}))
}

func (ctrl *LegacyTaskController) Run(c *gin.Context) {
	payload := map[string]interface{}{}
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&payload); err != nil {
			c.JSON(400, msg.ErrResponse("无效的任务参数", err))
			return
		}
	}

	result, err := ctrl.service.Enqueue(c.Request.Context(), c.Param("code"), payload)
	if err != nil {
		c.JSON(500, msg.ErrResponse("投递旧任务失败", err))
		return
	}

	c.JSON(200, msg.SuccessResponse("投递旧任务成功", &map[string]any{
		"result": result,
	}))
}
