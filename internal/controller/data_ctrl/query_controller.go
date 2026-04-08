package data_ctrl

import (
	"github.com/gin-gonic/gin"

	"gin-biz-web-api/internal/msg"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/internal/service/data_svc"
)

type QueryController struct {
	service *data_svc.QueryService
}

func NewQueryController() *QueryController {
	return &QueryController{
		service: data_svc.NewQueryService(),
	}
}

// GetRawData 查询原始数据
// @Summary 查询原始数据
// @Description 查询原始数据列表
// @Tags 数据查询
// @Accept json
// @Produce json
// @Param data_type query string false "数据类型"
// @Param data_source_id query int false "数据源ID"
// @Param status query string false "状态"
// @Param limit query int false "限制数量"
// @Success 200 {object} msg.Response
// @Failure 400 {object} msg.ErrResponseST
// @Failure 500 {object} msg.ErrResponseST
// @Router /api/v1/data/raw [get]
func (ctrl *QueryController) GetRawData(c *gin.Context) {
	// 解析查询参数
	var req requestbody.RawDataQueryRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(400, msg.ErrResponse("无效的请求参数", err))
		return
	}

	// 设置默认值
	if req.Limit <= 0 {
		req.Limit = 20
	}

	// 调用服务查询数据
	rawDataList, err := ctrl.service.GetRawData(c.Request.Context(), req.DataType, req.DataSourceID, req.Status, req.Limit)
	if err != nil {
		c.JSON(500, msg.ErrResponse("查询原始数据失败", err))
		return
	}

	// 构建响应数据
	data := map[string]any{
		"data": rawDataList,
		"meta": map[string]any{
			"total": len(rawDataList),
			"limit": req.Limit,
		},
	}

	// 返回成功响应
	c.JSON(200, msg.SuccessResponse("查询成功", &data))
}

// GetProcessedData 查询处理后的数据
// @Summary 查询处理后的数据
// @Description 查询处理后的数据列表
// @Tags 数据查询
// @Accept json
// @Produce json
// @Param data_type query string false "数据类型"
// @Param limit query int false "限制数量"
// @Success 200 {object} msg.Response
// @Failure 400 {object} msg.ErrResponseST
// @Failure 500 {object} msg.ErrResponseST
// @Router /api/v1/data/processed [get]
func (ctrl *QueryController) GetProcessedData(c *gin.Context) {
	// 解析查询参数
	var req requestbody.ProcessedDataQueryRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(400, msg.ErrResponse("无效的请求参数", err))
		return
	}

	// 设置默认值
	if req.Limit <= 0 {
		req.Limit = 20
	}

	// 调用服务查询数据
	processedDataList, err := ctrl.service.GetProcessedData(c.Request.Context(), req.DataType, req.Limit)
	if err != nil {
		c.JSON(500, msg.ErrResponse("查询处理后的数据失败", err))
		return
	}

	// 计算平均质量分
	var totalQualityScore float64
	for _, data := range processedDataList {
		totalQualityScore += data.QualityScore
	}

	avgQuality := 0.0
	if len(processedDataList) > 0 {
		avgQuality = totalQualityScore / float64(len(processedDataList))
	}

	// 构建响应数据
	data := map[string]any{
		"data": processedDataList,
		"summary": map[string]any{
			"total_count": len(processedDataList),
			"avg_quality": avgQuality,
		},
	}

	// 返回成功响应
	c.JSON(200, msg.SuccessResponse("查询成功", &data))
}

// GetStatistics 查询统计数据
// @Summary 查询统计数据
// @Description 查询数据统计信息
// @Tags 数据查询
// @Accept json
// @Produce json
// @Param start_date query string false "开始日期"
// @Param end_date query string false "结束日期"
// @Param data_type query string false "数据类型"
// @Success 200 {object} msg.Response
// @Failure 400 {object} msg.ErrResponseST
// @Failure 500 {object} msg.ErrResponseST
// @Router /api/v1/data/statistics [get]
func (ctrl *QueryController) GetStatistics(c *gin.Context) {
	// 解析查询参数
	var req requestbody.StatisticsQueryRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(400, msg.ErrResponse("无效的请求参数", err))
		return
	}

	// 调用服务查询数据
	statsList, err := ctrl.service.GetStatistics(c.Request.Context(), req.StartDate, req.EndDate, req.DataType)
	if err != nil {
		c.JSON(500, msg.ErrResponse("查询统计数据失败", err))
		return
	}

	// 构建响应数据
	data := map[string]any{
		"data": statsList,
		"meta": map[string]any{
			"total": len(statsList),
		},
	}

	// 返回成功响应
	c.JSON(200, msg.SuccessResponse("查询成功", &data))
}
