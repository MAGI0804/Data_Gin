package data_ctrl

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"gin-biz-web-api/internal/msg"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/internal/service/data_svc"
)

type SourceController struct {
	service *data_svc.SourceService
}

func NewSourceController() *SourceController {
	return &SourceController{
		service: data_svc.NewSourceService(),
	}
}

func (ctrl *SourceController) ListSources(c *gin.Context) {
	sources, err := ctrl.service.ListSourceDefinitions(c.Request.Context())
	if err != nil {
		c.JSON(500, msg.ErrResponse("查询数据源失败", err))
		return
	}

	c.JSON(200, msg.SuccessResponse("查询数据源成功", &map[string]any{
		"sources": sources,
	}))
}

func (ctrl *SourceController) CreateSource(c *gin.Context) {
	var req requestbody.SourceDefinitionCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, msg.ErrResponse("无效的数据源参数", err))
		return
	}

	source, err := ctrl.service.CreateSourceDefinition(c.Request.Context(), &req)
	if err != nil {
		c.JSON(500, msg.ErrResponse("创建数据源失败", err))
		return
	}

	c.JSON(200, msg.SuccessResponse("创建数据源成功", &map[string]any{
		"source": source,
	}))
}

func (ctrl *SourceController) TestSource(c *gin.Context) {
	sourceID, err := parseSourceID(c)
	if err != nil {
		c.JSON(400, msg.ErrResponse("无效的数据源ID", err))
		return
	}

	if err := ctrl.service.TestSourceDefinition(c.Request.Context(), sourceID); err != nil {
		c.JSON(500, msg.ErrResponse("数据源测试失败", err))
		return
	}

	c.JSON(200, msg.SuccessResponse("数据源测试成功", &map[string]any{
		"source_id": sourceID,
		"status":    "success",
	}))
}

func (ctrl *SourceController) FetchSource(c *gin.Context) {
	sourceID, err := parseSourceID(c)
	if err != nil {
		c.JSON(400, msg.ErrResponse("无效的数据源ID", err))
		return
	}

	result, err := ctrl.service.FetchSourceDefinition(c.Request.Context(), sourceID)
	if err != nil {
		c.JSON(500, msg.ErrResponse("数据源拉取失败", err))
		return
	}

	c.JSON(200, msg.SuccessResponse("数据源拉取完成", &map[string]any{
		"result": result,
	}))
}

func parseSourceID(c *gin.Context) (uint, error) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return 0, err
	}
	return uint(id), nil
}
