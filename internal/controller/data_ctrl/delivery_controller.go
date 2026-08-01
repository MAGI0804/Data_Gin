package data_ctrl

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"gin-biz-web-api/internal/dao/data_dao"
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
	values := c.Request.URL.Query()
	if !monitoringQueryKeysAllowed(values, "page", "page_size", "keyword", "enabled", "destination_type") {
		c.JSON(400, msg.ErrResponseStr("无效的推送目标查询参数"))
		return
	}
	if !monitoringHasAnyKey(values, "page", "page_size", "keyword", "enabled", "destination_type") {
		destinations, err := ctrl.service.ListDestinations(c.Request.Context())
		if err != nil {
			c.JSON(500, msg.ErrResponse("查询推送目标失败", err))
			return
		}
		c.JSON(200, msg.SuccessResponse("查询推送目标成功", &map[string]any{
			"destinations": safeDestinationDefinitions(destinations),
		}))
		return
	}
	page, pageSize, err := parseMonitoringPagination(values)
	if err != nil {
		c.JSON(400, msg.ErrResponseStr("无效的推送目标分页参数"))
		return
	}
	keyword, err := parseMonitoringText(values.Get("keyword"), 100)
	if err != nil {
		c.JSON(400, msg.ErrResponseStr("无效的推送目标查询参数"))
		return
	}
	enabled, err := parseMonitoringBool(values.Get("enabled"))
	if err != nil {
		c.JSON(400, msg.ErrResponseStr("无效的推送目标查询参数"))
		return
	}
	destinationType, err := parseMonitoringText(values.Get("destination_type"), 50)
	if err != nil {
		c.JSON(400, msg.ErrResponseStr("无效的推送目标查询参数"))
		return
	}
	result, err := ctrl.service.ListDestinationsPage(c.Request.Context(), data_dao.DestinationDefinitionListQuery{
		Page: page, PageSize: pageSize, Keyword: keyword, Enabled: enabled, DestinationType: destinationType,
	})
	if err != nil {
		c.JSON(500, msg.ErrResponse("查询推送目标失败", err))
		return
	}
	c.JSON(200, msg.SuccessResponse("查询推送目标成功", &map[string]any{
		"destinations": safeDestinationDefinitions(result.List),
		"pagination":   monitoringPaginationResponse(page, pageSize, result.Total),
	}))
}

func (ctrl *DeliveryController) GetDestination(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(400, msg.ErrResponse("无效的推送目标ID", err))
		return
	}

	destination, err := ctrl.service.GetDestination(c.Request.Context(), id)
	if err != nil {
		c.JSON(500, msg.ErrResponse("查询推送目标详情失败", err))
		return
	}

	c.JSON(200, msg.SuccessResponse("查询推送目标详情成功", &map[string]any{
		"destination": safeDestinationDefinition(*destination),
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
		"destination": safeDestinationDefinition(*destination),
	}))
}

func (ctrl *DeliveryController) UpdateDestination(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(400, msg.ErrResponse("无效的推送目标ID", err))
		return
	}

	var req requestbody.DestinationUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, msg.ErrResponse("无效的推送目标参数", err))
		return
	}

	destination, err := ctrl.service.UpdateDestination(c.Request.Context(), id, &req)
	if err != nil {
		c.JSON(500, msg.ErrResponse("更新推送目标失败", err))
		return
	}

	c.JSON(200, msg.SuccessResponse("更新推送目标成功", &map[string]any{
		"destination": safeDestinationDefinition(*destination),
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
	values := c.Request.URL.Query()
	if !monitoringQueryKeysAllowed(values, "page", "page_size", "keyword", "enabled", "destination_id") {
		c.JSON(400, msg.ErrResponseStr("无效的推送任务查询参数"))
		return
	}
	if !monitoringHasAnyKey(values, "page", "page_size", "keyword", "enabled", "destination_id") {
		tasks, err := ctrl.service.ListDeliveryTasks(c.Request.Context())
		if err != nil {
			c.JSON(500, msg.ErrResponse("查询推送任务失败", err))
			return
		}
		c.JSON(200, msg.SuccessResponse("查询推送任务成功", &map[string]any{
			"tasks": tasks,
		}))
		return
	}
	page, pageSize, err := parseMonitoringPagination(values)
	if err != nil {
		c.JSON(400, msg.ErrResponseStr("无效的推送任务分页参数"))
		return
	}
	keyword, err := parseMonitoringText(values.Get("keyword"), 100)
	if err != nil {
		c.JSON(400, msg.ErrResponseStr("无效的推送任务查询参数"))
		return
	}
	enabled, err := parseMonitoringBool(values.Get("enabled"))
	if err != nil {
		c.JSON(400, msg.ErrResponseStr("无效的推送任务查询参数"))
		return
	}
	destinationID, err := parseMonitoringUint(values.Get("destination_id"))
	if err != nil {
		c.JSON(400, msg.ErrResponseStr("无效的推送任务查询参数"))
		return
	}
	result, err := ctrl.service.ListDeliveryTasksPage(c.Request.Context(), data_dao.DeliveryTaskListQuery{
		Page: page, PageSize: pageSize, Keyword: keyword, Enabled: enabled, DestinationID: destinationID,
	})
	if err != nil {
		c.JSON(500, msg.ErrResponse("查询推送任务失败", err))
		return
	}
	c.JSON(200, msg.SuccessResponse("查询推送任务成功", &map[string]any{
		"tasks":      result.List,
		"pagination": monitoringPaginationResponse(page, pageSize, result.Total),
	}))
}

func (ctrl *DeliveryController) GetTask(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(400, msg.ErrResponse("无效的推送任务ID", err))
		return
	}

	task, err := ctrl.service.GetDeliveryTask(c.Request.Context(), id)
	if err != nil {
		c.JSON(500, msg.ErrResponse("查询推送任务详情失败", err))
		return
	}

	c.JSON(200, msg.SuccessResponse("查询推送任务详情成功", &map[string]any{
		"task": task,
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

func (ctrl *DeliveryController) UpdateTask(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(400, msg.ErrResponse("无效的推送任务ID", err))
		return
	}

	var req requestbody.DeliveryTaskUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, msg.ErrResponse("无效的推送任务参数", err))
		return
	}

	task, err := ctrl.service.UpdateDeliveryTask(c.Request.Context(), id, &req)
	if err != nil {
		c.JSON(500, msg.ErrResponse("更新推送任务失败", err))
		return
	}

	c.JSON(200, msg.SuccessResponse("更新推送任务成功", &map[string]any{
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
		if errors.Is(err, data_svc.ErrDeliveryTaskBusy) {
			c.JSON(409, msg.ErrResponseStr("推送任务正在执行，请等待当前任务完成"))
			return
		}
		c.JSON(500, msg.ErrResponse("执行推送任务失败", err))
		return
	}

	c.JSON(200, msg.SuccessResponse("执行推送任务完成", &map[string]any{
		"result": result,
	}))
}

func (ctrl *DeliveryController) ListLogs(c *gin.Context) {
	values := c.Request.URL.Query()
	if !monitoringQueryKeysAllowed(values, "limit", "page", "page_size", "destination", "source", "success", "business_key", "start_time", "end_time") {
		c.JSON(400, msg.ErrResponseStr("无效的推送日志查询参数"))
		return
	}
	page, pageSize, err := parseMonitoringPagination(values)
	if err != nil {
		c.JSON(400, msg.ErrResponseStr("无效的推送日志分页参数"))
		return
	}
	sentFrom, err := parseMonitoringTime(values.Get("start_time"))
	if err != nil {
		c.JSON(400, msg.ErrResponseStr("无效的推送日志时间范围"))
		return
	}
	sentTo, err := parseMonitoringTime(values.Get("end_time"))
	if err != nil || !monitoringTimeRangeValid(sentFrom, sentTo) {
		c.JSON(400, msg.ErrResponseStr("无效的推送日志时间范围"))
		return
	}
	success, err := parseDeliveryLogSuccess(values.Get("success"))
	if err != nil {
		c.JSON(400, msg.ErrResponseStr("无效的推送日志查询参数"))
		return
	}
	destination := strings.TrimSpace(values.Get("destination"))
	source := strings.TrimSpace(values.Get("source"))
	businessKey := strings.TrimSpace(values.Get("business_key"))
	if len(destination) > 100 || len(source) > 100 || len(businessKey) > 255 {
		c.JSON(400, msg.ErrResponseStr("无效的推送日志查询参数"))
		return
	}

	if isLegacyDeliveryLogQuery(values) {
		limit, limitErr := parseLimit(c)
		if limitErr != nil {
			c.JSON(400, msg.ErrResponseStr("无效的日志条数"))
			return
		}
		logs, serviceErr := ctrl.service.ListDeliveryLogs(c.Request.Context(), limit)
		if serviceErr != nil {
			c.JSON(500, msg.ErrResponseStr("查询推送日志失败"))
			return
		}
		c.JSON(200, msg.SuccessResponse("查询推送日志成功", &map[string]any{"logs": safeDeliveryLogs(logs)}))
		return
	}

	result, err := ctrl.service.ListDeliveryLogsPage(c.Request.Context(), data_dao.DeliveryLogListQuery{
		Page: page, PageSize: pageSize, DestinationCode: destination, SourceCode: source, Success: success,
		BusinessKey: businessKey, SentFrom: sentFrom, SentTo: sentTo,
	})
	if err != nil {
		c.JSON(500, msg.ErrResponseStr("查询推送日志失败"))
		return
	}
	pagination := monitoringPaginationResponse(page, pageSize, result.Total)
	c.JSON(200, msg.SuccessResponse("查询推送日志成功", &map[string]any{
		"logs": safeDeliveryLogs(result.List), "pagination": pagination,
	}))
}

func isLegacyDeliveryLogQuery(values map[string][]string) bool {
	for _, key := range []string{"page", "page_size", "destination", "source", "success", "business_key", "start_time", "end_time"} {
		if _, ok := values[key]; ok {
			return false
		}
	}
	return true
}

func parseDeliveryLogSuccess(value string) (*bool, error) {
	switch strings.TrimSpace(value) {
	case "":
		return nil, nil
	case "true":
		result := true
		return &result, nil
	case "false":
		result := false
		return &result, nil
	default:
		return nil, strconv.ErrSyntax
	}
}

func (ctrl *DeliveryController) RetryLog(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(400, msg.ErrResponse("无效的日志ID", err))
		return
	}

	result, err := ctrl.service.RetryDeliveryLog(c.Request.Context(), id)
	if err != nil {
		c.JSON(500, msg.ErrResponse("重试推送日志失败", err))
		return
	}

	c.JSON(200, msg.SuccessResponse("重试推送日志完成", &map[string]any{
		"result": result,
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
