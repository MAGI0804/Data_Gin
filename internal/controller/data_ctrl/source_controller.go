package data_ctrl

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"

	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/internal/msg"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/internal/service/data_svc"
	"gorm.io/gorm"
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
	values := c.Request.URL.Query()
	if !monitoringQueryKeysAllowed(values, "page", "page_size", "keyword", "enabled", "source_type") {
		c.JSON(400, msg.ErrResponseStr("无效的数据源查询参数"))
		return
	}
	if !monitoringHasAnyKey(values, "page", "page_size", "keyword", "enabled", "source_type") {
		sources, err := ctrl.service.ListSourceDefinitions(c.Request.Context())
		if err != nil {
			c.JSON(500, msg.ErrResponse("查询数据源失败", err))
			return
		}

		c.JSON(200, msg.SuccessResponse("查询数据源成功", &map[string]any{
			"sources": safeSourceDefinitions(sources),
		}))
		return
	}
	page, pageSize, err := parseMonitoringPagination(values)
	if err != nil {
		c.JSON(400, msg.ErrResponseStr("无效的数据源分页参数"))
		return
	}
	keyword, err := parseMonitoringText(values.Get("keyword"), 100)
	if err != nil {
		c.JSON(400, msg.ErrResponseStr("无效的数据源查询参数"))
		return
	}
	enabled, err := parseMonitoringBool(values.Get("enabled"))
	if err != nil {
		c.JSON(400, msg.ErrResponseStr("无效的数据源查询参数"))
		return
	}
	sourceType, err := parseMonitoringText(values.Get("source_type"), 50)
	if err != nil {
		c.JSON(400, msg.ErrResponseStr("无效的数据源查询参数"))
		return
	}

	result, err := ctrl.service.ListSourceDefinitionsPage(c.Request.Context(), data_dao.SourceDefinitionListQuery{
		Page: page, PageSize: pageSize, Keyword: keyword, Enabled: enabled, SourceType: sourceType,
	})
	if err != nil {
		c.JSON(500, msg.ErrResponse("查询数据源失败", err))
		return
	}
	c.JSON(200, msg.SuccessResponse("查询数据源成功", &map[string]any{
		"sources":    safeSourceDefinitions(result.List),
		"pagination": monitoringPaginationResponse(page, pageSize, result.Total),
	}))
}

func (ctrl *SourceController) GetSource(c *gin.Context) {
	sourceID, err := parseSourceID(c)
	if err != nil {
		c.JSON(400, msg.ErrResponse("无效的数据源ID", err))
		return
	}

	source, err := ctrl.service.GetSourceDefinition(c.Request.Context(), sourceID)
	if err != nil {
		c.JSON(500, msg.ErrResponse("查询数据源详情失败", err))
		return
	}

	c.JSON(200, msg.SuccessResponse("查询数据源详情成功", &map[string]any{
		"source": safeSourceDefinition(*source),
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
		"source": safeSourceDefinition(*source),
	}))
}

func (ctrl *SourceController) UpdateSource(c *gin.Context) {
	sourceID, err := parseSourceID(c)
	if err != nil {
		c.JSON(400, msg.ErrResponse("无效的数据源ID", err))
		return
	}

	var req requestbody.SourceDefinitionUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, msg.ErrResponse("无效的数据源参数", err))
		return
	}

	source, err := ctrl.service.UpdateSourceDefinition(c.Request.Context(), sourceID, &req)
	if err != nil {
		c.JSON(500, msg.ErrResponse("更新数据源失败", err))
		return
	}

	c.JSON(200, msg.SuccessResponse("更新数据源成功", &map[string]any{
		"source": safeSourceDefinition(*source),
	}))
}

func (ctrl *SourceController) UpdateSourceEnabled(c *gin.Context) {
	sourceID, err := parseSourceID(c)
	if err != nil {
		c.JSON(400, msg.ErrResponse("无效的数据源ID", err))
		return
	}
	var req requestbody.EnabledUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		c.JSON(400, msg.ErrResponse("无效的数据源状态参数", err))
		return
	}
	source, err := ctrl.service.SetSourceDefinitionEnabled(c.Request.Context(), sourceID, *req.Enabled)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(404, msg.ErrResponseStr("数据源不存在"))
		return
	}
	if err != nil {
		c.JSON(500, msg.ErrResponse("更新数据源状态失败", err))
		return
	}
	c.JSON(200, msg.SuccessResponse("更新数据源状态成功", &map[string]any{"source": safeSourceDefinition(*source)}))
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
