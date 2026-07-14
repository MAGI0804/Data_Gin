package data_ctrl

import (
	"github.com/gin-gonic/gin"

	"gin-biz-web-api/internal/msg"
	"gin-biz-web-api/internal/service/config_svc"
	"gin-biz-web-api/pkg/orderpush"
)

type OrderPushConfigController struct {
	service *config_svc.OrderPushSkipConfigService
}

func NewOrderPushConfigController() *OrderPushConfigController {
	return &OrderPushConfigController{
		service: config_svc.NewOrderPushSkipConfigService(),
	}
}

func (ctrl *OrderPushConfigController) GetSkipPolicy(c *gin.Context) {
	config, err := ctrl.service.Get(c.Request.Context())
	if err != nil {
		c.JSON(500, msg.ErrResponse("查询订单少推送配置失败", err))
		return
	}

	c.JSON(200, msg.SuccessResponse("查询订单少推送配置成功", &map[string]any{
		"config": config,
	}))
}

func (ctrl *OrderPushConfigController) SaveSkipPolicy(c *gin.Context) {
	var req orderpush.SkipConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, msg.ErrResponse("无效的订单少推送配置", err))
		return
	}

	config, err := ctrl.service.Save(c.Request.Context(), req)
	if err != nil {
		c.JSON(400, msg.ErrResponse("保存订单少推送配置失败", err))
		return
	}

	c.JSON(200, msg.SuccessResponse("保存订单少推送配置成功", &map[string]any{
		"config": config,
	}))
}
