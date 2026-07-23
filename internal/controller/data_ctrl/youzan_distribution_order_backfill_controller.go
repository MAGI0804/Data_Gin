package data_ctrl

import (
	"github.com/gin-gonic/gin"

	"gin-biz-web-api/internal/msg"
	"gin-biz-web-api/internal/service/data_svc"
	"gin-biz-web-api/pkg/youzan"
)

type YouzanDistributionOrderBackfillController struct {
	service *data_svc.YouzanDistributionOrderService
}

type youzanDistributionOrderBackfillRequest struct {
	TimeFilter string `json:"time_filter" binding:"omitempty,oneof=created success"`
	StartTime  string `json:"start_time"`
	EndTime    string `json:"end_time"`
}

func NewYouzanDistributionOrderBackfillController() *YouzanDistributionOrderBackfillController {
	return &YouzanDistributionOrderBackfillController{service: data_svc.NewYouzanDistributionOrderService()}
}

func (ctrl *YouzanDistributionOrderBackfillController) Preview(c *gin.Context) {
	var req youzanDistributionOrderBackfillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, msg.ErrResponse("无效的有赞分销补拉参数", err))
		return
	}

	timeFilter, err := youzan.ParseOrderTimeFilter(req.TimeFilter)
	if err != nil {
		c.JSON(400, msg.ErrResponse("无效的有赞分销时间筛选方式", err))
		return
	}

	result, err := ctrl.service.PreviewRange(c.Request.Context(), timeFilter, req.StartTime, req.EndTime)
	if err != nil {
		c.JSON(500, msg.ErrResponse("有赞分销补拉预览失败", err))
		return
	}

	c.JSON(200, msg.SuccessResponse("有赞分销补拉预览成功", &map[string]any{"result": result}))
}

func (ctrl *YouzanDistributionOrderBackfillController) Confirm(c *gin.Context) {
	var req youzanDistributionOrderBackfillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, msg.ErrResponse("无效的有赞分销补拉参数", err))
		return
	}

	timeFilter, err := youzan.ParseOrderTimeFilter(req.TimeFilter)
	if err != nil {
		c.JSON(400, msg.ErrResponse("无效的有赞分销时间筛选方式", err))
		return
	}

	result, err := ctrl.service.SyncRange(c.Request.Context(), timeFilter, req.StartTime, req.EndTime)
	if err != nil {
		c.JSON(500, msg.ErrResponse("有赞分销补拉写入失败", err))
		return
	}

	c.JSON(200, msg.SuccessResponse("有赞分销补拉写入完成", &map[string]any{"result": result}))
}
