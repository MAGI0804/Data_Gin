package data_ctrl

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"gin-biz-web-api/internal/msg"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/internal/service/data_svc"
)

type PipelineController struct {
	service *data_svc.PipelineService
}

func NewPipelineController() *PipelineController {
	return &PipelineController{service: data_svc.NewPipelineService()}
}

func (ctrl *PipelineController) ListPipelines(c *gin.Context) {
	pipelines, err := ctrl.service.ListPipelines(c.Request.Context())
	if err != nil {
		c.JSON(500, msg.ErrResponse("查询流水线失败", err))
		return
	}
	c.JSON(200, msg.SuccessResponse("查询流水线成功", &map[string]any{"pipelines": pipelines}))
}

func (ctrl *PipelineController) GetPipeline(c *gin.Context) {
	id, err := parsePipelineID(c)
	if err != nil {
		c.JSON(400, msg.ErrResponse("无效的流水线ID", err))
		return
	}
	pipeline, err := ctrl.service.GetPipeline(c.Request.Context(), id)
	if err != nil {
		c.JSON(500, msg.ErrResponse("查询流水线详情失败", err))
		return
	}
	c.JSON(200, msg.SuccessResponse("查询流水线详情成功", &map[string]any{"pipeline": pipeline}))
}

func (ctrl *PipelineController) CreatePipeline(c *gin.Context) {
	var req requestbody.PipelineCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, msg.ErrResponse("无效的流水线参数", err))
		return
	}
	pipeline, err := ctrl.service.CreatePipeline(c.Request.Context(), &req)
	if err != nil {
		c.JSON(500, msg.ErrResponse("创建流水线失败", err))
		return
	}
	c.JSON(200, msg.SuccessResponse("创建流水线成功", &map[string]any{"pipeline": pipeline}))
}

func (ctrl *PipelineController) UpdatePipeline(c *gin.Context) {
	id, err := parsePipelineID(c)
	if err != nil {
		c.JSON(400, msg.ErrResponse("无效的流水线ID", err))
		return
	}
	var req requestbody.PipelineUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, msg.ErrResponse("无效的流水线参数", err))
		return
	}
	pipeline, err := ctrl.service.UpdatePipeline(c.Request.Context(), id, &req)
	if err != nil {
		c.JSON(500, msg.ErrResponse("更新流水线失败", err))
		return
	}
	c.JSON(200, msg.SuccessResponse("更新流水线成功", &map[string]any{"pipeline": pipeline}))
}

func (ctrl *PipelineController) ListSteps(c *gin.Context) {
	id, err := parsePipelineID(c)
	if err != nil {
		c.JSON(400, msg.ErrResponse("无效的流水线ID", err))
		return
	}
	steps, err := ctrl.service.GetPipelineSteps(c.Request.Context(), id)
	if err != nil {
		c.JSON(500, msg.ErrResponse("查询方法步骤失败", err))
		return
	}
	c.JSON(200, msg.SuccessResponse("查询方法步骤成功", &map[string]any{"steps": steps}))
}

func (ctrl *PipelineController) CreateStep(c *gin.Context) {
	id, err := parsePipelineID(c)
	if err != nil {
		c.JSON(400, msg.ErrResponse("无效的流水线ID", err))
		return
	}
	var req requestbody.MethodStepCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, msg.ErrResponse("无效的方法步骤参数", err))
		return
	}
	step, err := ctrl.service.CreateStep(c.Request.Context(), id, &req)
	if err != nil {
		c.JSON(500, msg.ErrResponse("创建方法步骤失败", err))
		return
	}
	c.JSON(200, msg.SuccessResponse("创建方法步骤成功", &map[string]any{"step": step}))
}

func (ctrl *PipelineController) UpdateStep(c *gin.Context) {
	stepID, err := parseStepID(c)
	if err != nil {
		c.JSON(400, msg.ErrResponse("无效的方法步骤ID", err))
		return
	}
	var req requestbody.MethodStepUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, msg.ErrResponse("无效的方法步骤参数", err))
		return
	}
	step, err := ctrl.service.UpdateStep(c.Request.Context(), stepID, &req)
	if err != nil {
		c.JSON(500, msg.ErrResponse("更新方法步骤失败", err))
		return
	}
	c.JSON(200, msg.SuccessResponse("更新方法步骤成功", &map[string]any{"step": step}))
}

func (ctrl *PipelineController) PreviewJSON(c *gin.Context) {
	id, err := parsePipelineID(c)
	if err != nil {
		c.JSON(400, msg.ErrResponse("无效的流水线ID", err))
		return
	}
	preview, err := ctrl.service.PreviewPipelineJSON(c.Request.Context(), id)
	if err != nil {
		c.JSON(500, msg.ErrResponse("生成流水线 JSON 失败", err))
		return
	}
	c.JSON(200, msg.SuccessResponse("生成流水线 JSON 成功", &map[string]any{"preview": preview}))
}

func (ctrl *PipelineController) RunPipeline(c *gin.Context) {
	id, err := parsePipelineID(c)
	if err != nil {
		c.JSON(400, msg.ErrResponse("无效的流水线ID", err))
		return
	}
	result, err := ctrl.service.RunPipeline(c.Request.Context(), id)
	if err != nil {
		c.JSON(500, msg.ErrResponse("执行流水线失败", err))
		return
	}
	c.JSON(200, msg.SuccessResponse("执行流水线完成", &map[string]any{"result": result}))
}

func (ctrl *PipelineController) ListStepRuns(c *gin.Context) {
	runID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(400, msg.ErrResponse("无效的运行记录ID", err))
		return
	}
	stepRuns, err := ctrl.service.ListStepRuns(c.Request.Context(), uint(runID))
	if err != nil {
		c.JSON(500, msg.ErrResponse("查询步骤运行明细失败", err))
		return
	}
	c.JSON(200, msg.SuccessResponse("查询步骤运行明细成功", &map[string]any{"step_runs": stepRuns}))
}

func parsePipelineID(c *gin.Context) (uint, error) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	return uint(id), err
}

func parseStepID(c *gin.Context) (uint, error) {
	id, err := strconv.ParseUint(c.Param("step_id"), 10, 32)
	return uint(id), err
}
