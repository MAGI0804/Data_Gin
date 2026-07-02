package data_ctrl

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"gin-biz-web-api/internal/msg"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/internal/service/data_svc"
)

type DeliveryController struct {
	service *data_svc.DeliveryService
}

func NewDeliveryController() *DeliveryController {
	return &DeliveryController{
		service: data_svc.NewDeliveryService(),
	}
}

func (ctrl *DeliveryController) ListDestinations(c *gin.Context) {
	destinations, err := ctrl.service.ListDestinations(c.Request.Context())
	if err != nil {
		c.JSON(500, msg.ErrResponse("查询推送目标失败", err))
		return
	}

	c.JSON(200, msg.SuccessResponse("查询推送目标成功", &map[string]any{
		"destinations": destinations,
	}))
}

func (ctrl *DeliveryController) CreateDestination(c *gin.Context) {
	var req requestbody.DestinationCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, msg.ErrResponse("无效的推送目标参数", err))
		return
	}

	destination, err := ctrl.service.CreateDestination(c.Request.Context(), &req)
	if err != nil {
		c.JSON(500, msg.ErrResponse("创建推送目标失败", err))
		return
	}

	c.JSON(200, msg.SuccessResponse("创建推送目标成功", &map[string]any{
		"destination": destination,
	}))
}

func (ctrl *DeliveryController) TestDestination(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(400, msg.ErrResponse("无效的推送目标ID", err))
		return
	}

	if err := ctrl.service.TestDestination(c.Request.Context(), id); err != nil {
		c.JSON(500, msg.ErrResponse("推送目标测试失败", err))
		return
	}

	c.JSON(200, msg.SuccessResponse("推送目标测试成功", &map[string]any{
		"destination_id": id,
		"status":         "success",
	}))
}

func (ctrl *DeliveryController) ListTasks(c *gin.Context) {
	tasks, err := ctrl.service.ListDeliveryTasks(c.Request.Context())
	if err != nil {
		c.JSON(500, msg.ErrResponse("查询推送任务失败", err))
		return
	}

	c.JSON(200, msg.SuccessResponse("查询推送任务成功", &map[string]any{
		"tasks": tasks,
	}))
}

func (ctrl *DeliveryController) CreateTask(c *gin.Context) {
	var req requestbody.DeliveryTaskCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, msg.ErrResponse("无效的推送任务参数", err))
		return
	}

	task, err := ctrl.service.CreateDeliveryTask(c.Request.Context(), &req)
	if err != nil {
		c.JSON(500, msg.ErrResponse("创建推送任务失败", err))
		return
	}

	c.JSON(200, msg.SuccessResponse("创建推送任务成功", &map[string]any{
		"task": task,
	}))
}

func (ctrl *DeliveryController) RunTask(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(400, msg.ErrResponse("无效的推送任务ID", err))
		return
	}

	result, err := ctrl.service.RunDeliveryTask(c.Request.Context(), id)
	if err != nil {
		c.JSON(500, msg.ErrResponse("执行推送任务失败", err))
		return
	}

	c.JSON(200, msg.SuccessResponse("执行推送任务完成", &map[string]any{
		"result": result,
	}))
}

func (ctrl *DeliveryController) ListLogs(c *gin.Context) {
	limit, err := parseLimit(c)
	if err != nil {
		c.JSON(400, msg.ErrResponse("无效的日志条数", err))
		return
	}

	logs, err := ctrl.service.ListDeliveryLogs(c.Request.Context(), limit)
	if err != nil {
		c.JSON(500, msg.ErrResponse("查询推送日志失败", err))
		return
	}

	c.JSON(200, msg.SuccessResponse("查询推送日志成功", &map[string]any{
		"logs": logs,
	}))
}

func parseUintParam(c *gin.Context, name string) (uint, error) {
	id, err := strconv.ParseUint(c.Param(name), 10, 32)
	if err != nil {
		return 0, err
	}
	return uint(id), nil
}

func parseLimit(c *gin.Context) (int, error) {
	value := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	return limit, nil
}
