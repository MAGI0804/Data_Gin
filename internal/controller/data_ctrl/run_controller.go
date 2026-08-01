package data_ctrl

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/internal/msg"
	"gin-biz-web-api/internal/service/data_svc"
)

type RunController struct {
	service *data_svc.RunService
}

func NewRunController() *RunController {
	return &RunController{
		service: data_svc.NewRunService(),
	}
}

func (ctrl *RunController) ListRuns(c *gin.Context) {
	values := c.Request.URL.Query()
	if !monitoringQueryKeysAllowed(values, "limit", "page", "page_size", "status", "run_type", "trace_id", "start_time", "end_time") {
		c.JSON(400, msg.ErrResponseStr("无效的运行记录查询参数"))
		return
	}
	page, pageSize, err := parseMonitoringPagination(values)
	if err != nil {
		c.JSON(400, msg.ErrResponseStr("无效的运行记录分页参数"))
		return
	}
	startedAt, err := parseMonitoringTime(values.Get("start_time"))
	if err != nil {
		c.JSON(400, msg.ErrResponseStr("无效的运行记录时间范围"))
		return
	}
	endedAt, err := parseMonitoringTime(values.Get("end_time"))
	if err != nil || !monitoringTimeRangeValid(startedAt, endedAt) {
		c.JSON(400, msg.ErrResponseStr("无效的运行记录时间范围"))
		return
	}
	status := strings.TrimSpace(values.Get("status"))
	runType := strings.TrimSpace(values.Get("run_type"))
	traceID := strings.TrimSpace(values.Get("trace_id"))
	if !validRunStatus(status) || !validRunType(runType) || len(traceID) > 64 {
		c.JSON(400, msg.ErrResponseStr("无效的运行记录查询参数"))
		return
	}

	if isLegacyRunQuery(values) {
		limit, limitErr := parseRunLimit(c)
		if limitErr != nil {
			c.JSON(400, msg.ErrResponseStr("无效的运行记录条数"))
			return
		}
		runs, serviceErr := ctrl.service.ListPipelineRuns(c.Request.Context(), limit)
		if serviceErr != nil {
			c.JSON(500, msg.ErrResponseStr("查询运行记录失败"))
			return
		}
		c.JSON(200, msg.SuccessResponse("查询运行记录成功", &map[string]any{"runs": safePipelineRuns(runs)}))
		return
	}

	result, err := ctrl.service.ListPipelineRunsPage(c.Request.Context(), data_dao.PipelineRunListQuery{
		Page: page, PageSize: pageSize, Status: status, RunType: runType, TraceID: traceID, StartedAt: startedAt, EndedAt: endedAt,
	})
	if err != nil {
		c.JSON(500, msg.ErrResponseStr("查询运行记录失败"))
		return
	}
	pagination := monitoringPaginationResponse(page, pageSize, result.Total)
	c.JSON(200, msg.SuccessResponse("查询运行记录成功", &map[string]any{
		"runs": safePipelineRuns(result.List), "pagination": pagination,
	}))
}

func isLegacyRunQuery(values map[string][]string) bool {
	for _, key := range []string{"page", "page_size", "status", "run_type", "trace_id", "start_time", "end_time"} {
		if _, ok := values[key]; ok {
			return false
		}
	}
	return true
}

func validRunStatus(value string) bool {
	return value == "" || value == "running" || value == "success" || value == "failed" || value == "partial_success"
}

func validRunType(value string) bool {
	return value == "" || value == "fetch" || value == "ingest" || value == "transform" || value == "delivery"
}

func parseRunLimit(c *gin.Context) (int, error) {
	value := c.DefaultQuery("limit", "50")
	return strconv.Atoi(value)
}
