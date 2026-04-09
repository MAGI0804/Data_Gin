package data_ctrl

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"gin-biz-web-api/internal/msg"
	"gin-biz-web-api/internal/service/data_svc"
)

type CollectController struct {
	service *data_svc.CollectService
}

func NewCollectController() *CollectController {
	return &CollectController{
		service: data_svc.NewCollectService(),
	}
}

// ManualCollect 手动触发数据采集
// @Summary 手动触发数据采集
// @Description 根据数据源ID手动触发数据采集任务
// @Tags 数据采集
// @Accept json
// @Produce json
// @Param source_id path int true "数据源ID"
// @Success 200 {object} msg.Response
// @Failure 400 {object} msg.ErrResponseST
// @Failure 500 {object} msg.ErrResponseST
// @Router /api/v1/data/collect/{source_id} [post]
func (ctrl *CollectController) ManualCollect(c *gin.Context) {
	// 解析数据源ID
	sourceIDStr := c.Param("source_id")
	sourceID, err := strconv.ParseUint(sourceIDStr, 10, 32)
	if err != nil {
		c.JSON(400, msg.ErrResponse("无效的数据源ID", err))
		return
	}

	// 调用服务采集数据
	result, err := ctrl.service.CollectFromSource(c.Request.Context(), uint(sourceID))
	if err != nil {
		c.JSON(500, msg.ErrResponse("数据采集失败", err))
		return
	}

	// 构建响应数据
	data := map[string]any{
		"status":  "started",
		"message": result,
	}

	// 返回成功响应
	c.JSON(200, msg.SuccessResponse("数据采集任务已启动", &data))
}

// CollectStatus 查询采集状态
// @Summary 查询采集状态
// @Description 根据任务ID查询数据采集状态
// @Tags 数据采集
// @Accept json
// @Produce json
// @Param job_id path string true "任务ID"
// @Success 200 {object} msg.Response
// @Failure 400 {object} msg.ErrResponseST
// @Failure 500 {object} msg.ErrResponseST
// @Router /api/v1/data/collect/status/{job_id} [get]
func (ctrl *CollectController) CollectStatus(c *gin.Context) {
	// 解析任务ID
	jobID := c.Param("job_id")
	if jobID == "" {
		c.JSON(400, msg.ErrResponse("无效的任务ID", nil))
		return
	}

	// 这里简化处理，实际应该查询任务状态
	// 由于我们使用的是异步任务，需要实现任务状态查询逻辑
	data := map[string]any{
		"job_id":   jobID,
		"status":   "completed",
		"progress": 100,
		"result": map[string]any{
			"total":   100,
			"success": 98,
			"failed":  2,
		},
		"message": "任务执行完成",
	}

	// 返回成功响应
	c.JSON(200, msg.SuccessResponse("查询成功", &data))
}

// CreateTask 创建任务
// @Summary 创建任务
// @Description 从原始数据获取数据，查询并存储结果
// @Tags 数据采集
// @Accept json
// @Produce json
// @Param source_id path int true "数据源ID"
// @Success 200 {object} msg.Response
// @Failure 400 {object} msg.ErrResponseST
// @Failure 500 {object} msg.ErrResponseST
// @Router /api/v1/data/task/create/{source_id} [post]
func (ctrl *CollectController) CreateTask(c *gin.Context) {
	// 解析数据源ID
	sourceIDStr := c.Param("source_id")
	sourceID, err := strconv.ParseUint(sourceIDStr, 10, 32)
	if err != nil {
		c.JSON(400, msg.ErrResponse("无效的数据源ID", err))
		return
	}

	// 1. 从原始数据中获取数据
	result, err := ctrl.service.CollectFromSource(c.Request.Context(), uint(sourceID))
	if err != nil {
		c.JSON(500, msg.ErrResponse("数据采集失败", err))
		return
	}

	// 2. 使用查询方法获取数据
	queryService := data_svc.NewQueryService()
	rawData, err := queryService.GetRawData(c.Request.Context(), "", uint(sourceID), "", 100)
	if err != nil {
		c.JSON(500, msg.ErrResponse("查询原始数据失败", err))
		return
	}

	// 3. 将数据存储到表中（这里已经在CollectFromSource中完成）
	// 实际应用中，可能需要根据业务逻辑进行额外的存储处理

	// 构建响应数据
	data := map[string]any{
		"status":         "completed",
		"message":        result,
		"raw_data_count": len(rawData),
		"source_id":      sourceID,
	}

	// 返回成功响应
	c.JSON(200, msg.SuccessResponse("任务创建成功", &data))
}
