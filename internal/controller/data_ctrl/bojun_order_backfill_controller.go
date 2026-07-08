package data_ctrl

import (
	"github.com/gin-gonic/gin"

	"gin-biz-web-api/internal/msg"
	"gin-biz-web-api/internal/service/data_svc"
)

type BojunOrderBackfillController struct {
	service *data_svc.BojunOrderService
}

type bojunOrderBackfillRequest struct {
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}

func NewBojunOrderBackfillController() *BojunOrderBackfillController {
	return &BojunOrderBackfillController{
		service: data_svc.NewBojunOrderService(),
	}
}

func (ctrl *BojunOrderBackfillController) Preview(c *gin.Context) {
	var req bojunOrderBackfillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, msg.ErrResponse("无效的伯俊补拉参数", err))
		return
	}

	result, err := ctrl.service.PreviewOrders(c.Request.Context(), req.StartTime, req.EndTime)
	if err != nil {
		c.JSON(500, msg.ErrResponse("伯俊补拉预览失败", err))
		return
	}

	c.JSON(200, msg.SuccessResponse("伯俊补拉预览成功", &map[string]any{
		"result": result,
	}))
}

func (ctrl *BojunOrderBackfillController) Confirm(c *gin.Context) {
	var req bojunOrderBackfillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, msg.ErrResponse("无效的伯俊补拉参数", err))
		return
	}

	result, err := ctrl.service.SyncOrders(c.Request.Context(), req.StartTime, req.EndTime)
	if err != nil {
		c.JSON(500, msg.ErrResponse("伯俊补拉写入失败", err))
		return
	}

	c.JSON(200, msg.SuccessResponse("伯俊补拉写入完成", &map[string]any{
		"result": result,
	}))
}
