package data_ctrl

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"gin-biz-web-api/internal/msg"
	"gin-biz-web-api/internal/service/data_svc"
)

const bojunOrderBackfillTaskCode = "bojun_order_fetch"

type bojunOrderPreviewService interface {
	PreviewOrders(ctx context.Context, startTime, endTime string) (*data_svc.BojunOrderSyncResult, error)
}

type bojunOrderTaskService interface {
	Enqueue(ctx context.Context, code string, payload map[string]interface{}) (*data_svc.LegacyTaskRunResult, error)
}

type BojunOrderBackfillController struct {
	previewService bojunOrderPreviewService
	taskService    bojunOrderTaskService
}

type bojunOrderBackfillRequest struct {
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}

func NewBojunOrderBackfillController() *BojunOrderBackfillController {
	return newBojunOrderBackfillController(
		data_svc.NewBojunOrderService(),
		data_svc.NewLegacyTaskService(),
	)
}

func newBojunOrderBackfillController(
	previewService bojunOrderPreviewService,
	taskService bojunOrderTaskService,
) *BojunOrderBackfillController {
	return &BojunOrderBackfillController{previewService: previewService, taskService: taskService}
}

func (ctrl *BojunOrderBackfillController) Preview(c *gin.Context) {
	var req bojunOrderBackfillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, msg.ErrResponse("无效的伯俊补拉参数", err))
		return
	}

	result, err := ctrl.previewService.PreviewOrders(c.Request.Context(), req.StartTime, req.EndTime)
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
	startTime, endTime, err := data_svc.NormalizeBojunOrderTimeRange(req.StartTime, req.EndTime)
	if err != nil {
		c.JSON(http.StatusBadRequest, msg.ErrResponse("无效的伯俊补拉时间范围", err))
		return
	}

	result, err := ctrl.taskService.Enqueue(c.Request.Context(), bojunOrderBackfillTaskCode, map[string]interface{}{
		"start_time": startTime,
		"end_time":   endTime,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, msg.ErrResponse("伯俊补拉任务投递失败", err))
		return
	}

	c.JSON(http.StatusAccepted, msg.SuccessResponse("伯俊补拉任务已投递", &map[string]any{
		"result": result,
	}))
}
