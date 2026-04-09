package auth_ctrl

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"gin-biz-web-api/internal/msg"
	"gin-biz-web-api/internal/service/auth_svc"
	"gin-biz-web-api/model"
)

// TokenDataController token_data控制器
type TokenDataController struct {
	service *auth_svc.TokenDataService
}

// NewTokenDataController 创建TokenDataController实例
func NewTokenDataController() *TokenDataController {
	return &TokenDataController{
		service: auth_svc.NewTokenDataService(),
	}
}

// CreateTokenData 创建token数据
// @Summary 创建token数据
// @Description 创建各平台的token信息
// @Tags token管理
// @Accept json
// @Produce json
// @Param token_data body model.TokenData true "token数据"
// @Success 200 {object} msg.Response
// @Failure 400 {object} msg.ErrResponseST
// @Failure 500 {object} msg.ErrResponseST
// @Router /api/auth/token-data [post]
func (ctrl *TokenDataController) CreateTokenData(c *gin.Context) {
	var tokenData model.TokenData
	if err := c.ShouldBindJSON(&tokenData); err != nil {
		c.JSON(http.StatusBadRequest, msg.ErrResponse("无效的请求数据", err))
		return
	}

	id, err := ctrl.service.CreateTokenData(c.Request.Context(), &tokenData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, msg.ErrResponse("创建token数据失败", err))
		return
	}

	data := map[string]any{
		"id": id,
		"message": "token数据创建成功",
	}

	c.JSON(http.StatusOK, msg.SuccessResponse("创建成功", &data))
}

// UpdateTokenData 更新token数据
// @Summary 更新token数据
// @Description 更新各平台的token信息
// @Tags token管理
// @Accept json
// @Produce json
// @Param id path int true "token数据ID"
// @Param token_data body model.TokenData true "token数据"
// @Success 200 {object} msg.Response
// @Failure 400 {object} msg.ErrResponseST
// @Failure 500 {object} msg.ErrResponseST
// @Router /api/auth/token-data/update/{id} [post]
func (ctrl *TokenDataController) UpdateTokenData(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, msg.ErrResponse("无效的ID", err))
		return
	}

	var tokenData model.TokenData
	if err := c.ShouldBindJSON(&tokenData); err != nil {
		c.JSON(http.StatusBadRequest, msg.ErrResponse("无效的请求数据", err))
		return
	}

	tokenData.ID = uint(id)

	err = ctrl.service.UpdateTokenData(c.Request.Context(), &tokenData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, msg.ErrResponse("更新token数据失败", err))
		return
	}

	data := map[string]any{
		"id": id,
		"message": "token数据更新成功",
	}

	c.JSON(http.StatusOK, msg.SuccessResponse("更新成功", &data))
}

// GetTokenDataByID 根据ID获取token数据
// @Summary 根据ID获取token数据
// @Description 根据ID获取各平台的token信息
// @Tags token管理
// @Accept json
// @Produce json
// @Param id path int true "token数据ID"
// @Success 200 {object} msg.Response
// @Failure 400 {object} msg.ErrResponseST
// @Failure 500 {object} msg.ErrResponseST
// @Router /api/auth/token-data/{id} [get]
func (ctrl *TokenDataController) GetTokenDataByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, msg.ErrResponse("无效的ID", err))
		return
	}

	tokenData, err := ctrl.service.GetTokenDataByID(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, msg.ErrResponse("获取token数据失败", err))
		return
	}

	data := map[string]any{
		"token_data": tokenData,
	}
	c.JSON(http.StatusOK, msg.SuccessResponse("获取成功", &data))
}

// GetAllTokenData 获取所有token数据
// @Summary 获取所有token数据
// @Description 获取所有平台的token信息
// @Tags token管理
// @Accept json
// @Produce json
// @Success 200 {object} msg.Response
// @Failure 500 {object} msg.ErrResponseST
// @Router /api/auth/token-data [get]
func (ctrl *TokenDataController) GetAllTokenData(c *gin.Context) {
	tokenDataList, err := ctrl.service.GetAllTokenData(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, msg.ErrResponse("获取token数据失败", err))
		return
	}

	data := map[string]any{
		"token_data_list": tokenDataList,
	}
	c.JSON(http.StatusOK, msg.SuccessResponse("获取成功", &data))
}

// DeleteTokenData 删除token数据
// @Summary 删除token数据
// @Description 删除各平台的token信息
// @Tags token管理
// @Accept json
// @Produce json
// @Param id path int true "token数据ID"
// @Success 200 {object} msg.Response
// @Failure 400 {object} msg.ErrResponseST
// @Failure 500 {object} msg.ErrResponseST
// @Router /api/auth/token-data/delete/{id} [post]
func (ctrl *TokenDataController) DeleteTokenData(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, msg.ErrResponse("无效的ID", err))
		return
	}

	err = ctrl.service.DeleteTokenData(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, msg.ErrResponse("删除token数据失败", err))
		return
	}

	data := map[string]any{
		"id": id,
		"message": "token数据删除成功",
	}

	c.JSON(http.StatusOK, msg.SuccessResponse("删除成功", &data))
}
