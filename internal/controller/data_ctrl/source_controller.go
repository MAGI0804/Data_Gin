package data_ctrl

import (
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
