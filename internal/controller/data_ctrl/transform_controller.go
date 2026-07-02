package data_ctrl

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"gin-biz-web-api/internal/msg"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/internal/service/data_svc"
)

type TransformController struct {
	service *data_svc.TransformService
}

func NewTransformController() *TransformController {
	return &TransformController{
		service: data_svc.NewTransformService(),
	}
}

func (ctrl *TransformController) ListRules(c *gin.Context) {
	rules, err := ctrl.service.ListTransformRules(c.Request.Context())
	if err != nil {
		c.JSON(500, msg.ErrResponse("查询清洗规则失败", err))
		return
	}

	c.JSON(200, msg.SuccessResponse("查询清洗规则成功", &map[string]any{
		"rules": rules,
	}))
}

func (ctrl *TransformController) CreateRule(c *gin.Context) {
	var req requestbody.TransformRuleCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, msg.ErrResponse("无效的清洗规则参数", err))
		return
	}

	rule, err := ctrl.service.CreateTransformRule(c.Request.Context(), &req)
	if err != nil {
		c.JSON(500, msg.ErrResponse("创建清洗规则失败", err))
		return
	}

	c.JSON(200, msg.SuccessResponse("创建清洗规则成功", &map[string]any{
		"rule": rule,
	}))
}

func (ctrl *TransformController) TestRule(c *gin.Context) {
	var req requestbody.TransformRuleTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, msg.ErrResponse("无效的清洗规则测试参数", err))
		return
	}

	clean, err := ctrl.service.TestMappingRule(c.Request.Context(), &req)
	if err != nil {
		c.JSON(500, msg.ErrResponse("清洗规则测试失败", err))
		return
	}

	c.JSON(200, msg.SuccessResponse("清洗规则测试成功", &map[string]any{
		"clean_content": clean,
	}))
}

func (ctrl *TransformController) RetransformRawRecord(c *gin.Context) {
	rawRecordID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(400, msg.ErrResponse("无效的原始记录ID", err))
		return
	}

	result, err := ctrl.service.TransformRawRecord(c.Request.Context(), uint(rawRecordID))
	if err != nil {
		c.JSON(500, msg.ErrResponse("重新清洗失败", err))
		return
	}

	c.JSON(200, msg.SuccessResponse("重新清洗成功", &map[string]any{
		"result": result,
	}))
}
