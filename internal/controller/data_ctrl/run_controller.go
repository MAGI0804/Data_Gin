package data_ctrl

import (
	"strconv"

	"github.com/gin-gonic/gin"

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
	limit, err := parseRunLimit(c)
	if err != nil {
		c.JSON(400, msg.ErrResponse("无效的运行记录条数", err))
		return
	}

	runs, err := ctrl.service.ListPipelineRuns(c.Request.Context(), limit)
	if err != nil {
		c.JSON(500, msg.ErrResponse("查询运行记录失败", err))
		return
	}

	c.JSON(200, msg.SuccessResponse("查询运行记录成功", &map[string]any{
		"runs": runs,
	}))
}

func parseRunLimit(c *gin.Context) (int, error) {
	value := c.DefaultQuery("limit", "50")
	return strconv.Atoi(value)
}
