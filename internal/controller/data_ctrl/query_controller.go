package data_ctrl

import (
	"github.com/gin-gonic/gin"

	"gin-biz-web-api/internal/dao/data_dao"
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

// GetProcessedDataList 查询处理结果（分页）。业务键并不属于 legacy
// processed_data，须使用 clean_records 的独立查询接口。
func (ctrl *QueryController) GetProcessedDataList(c *gin.Context) {
	var req requestbody.ProcessedDataListQueryRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(400, msg.ErrResponse("无效的请求参数", err))
		return
	}
	if !processedDataListQueryValid(req) {
		c.JSON(400, msg.ErrResponse("无效的筛选范围", nil))
		return
	}
	if req.Page == 0 {
		req.Page = 1
	}
	if req.PageSize == 0 {
		req.PageSize = 20
	}
	result, err := ctrl.service.GetProcessedDataList(c.Request.Context(), req.Page, req.PageSize, req.DataType, req.MinQuality, req.MaxQuality, req.CreatedFrom, req.CreatedTo)
	if err != nil {
		c.JSON(500, msg.ErrResponse("查询处理后的数据失败", err))
		return
	}
	c.JSON(200, msg.SuccessResponse("查询成功", &map[string]any{
		"list": result.List, "total": result.Total, "page": result.Page, "page_size": result.PageSize, "total_pages": result.TotalPages,
		"summary": map[string]any{"total_count": result.Total, "avg_quality": result.AverageQuality},
	}))
}

func processedDataListQueryValid(req requestbody.ProcessedDataListQueryRequest) bool {
	if req.MinQuality != nil && req.MaxQuality != nil && *req.MinQuality > *req.MaxQuality {
		return false
	}
	return req.CreatedFrom == 0 || req.CreatedTo == 0 || req.CreatedFrom <= req.CreatedTo
}

// GetCleanRecordList queries clean_records independently from legacy
// processed_data so source, business key and delivery status remain truthful.
func (ctrl *QueryController) GetCleanRecordList(c *gin.Context) {
	var req requestbody.CleanRecordListQueryRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(400, msg.ErrResponse("无效的请求参数", err))
		return
	}
	if !cleanRecordListQueryValid(req) {
		c.JSON(400, msg.ErrResponse("无效的筛选范围", nil))
		return
	}
	if req.Page == 0 {
		req.Page = 1
	}
	if req.PageSize == 0 {
		req.PageSize = 20
	}
	result, err := ctrl.service.GetCleanRecordList(c.Request.Context(), data_dao.CleanRecordListQuery{
		Page: req.Page, PageSize: req.PageSize, SourceID: req.SourceID, TableName: req.TableName, BusinessKey: req.BusinessKey,
		Status: req.Status, MinQuality: req.MinQuality, MaxQuality: req.MaxQuality, CreatedFrom: req.CreatedFrom, CreatedTo: req.CreatedTo,
	})
	if err != nil {
		c.JSON(500, msg.ErrResponse("查询清洗记录失败", err))
		return
	}
	c.JSON(200, msg.SuccessResponse("查询成功", &map[string]any{
		"list": result.List, "total": result.Total, "page": result.Page, "page_size": result.PageSize, "total_pages": result.TotalPages,
		"summary": map[string]any{"total_count": result.Total, "avg_quality": result.AverageQuality},
	}))
}

func cleanRecordListQueryValid(req requestbody.CleanRecordListQueryRequest) bool {
	if req.MinQuality != nil && req.MaxQuality != nil && *req.MinQuality > *req.MaxQuality {
		return false
	}
	return req.CreatedFrom == 0 || req.CreatedTo == 0 || req.CreatedFrom <= req.CreatedTo
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

// GetRawDataList 查询原始数据列表（分页）
// @Summary 查询原始数据列表
// @Description 查询原始数据列表，支持分页、source、origin 和创建时间范围筛选
// @Tags 数据查询
// @Accept json
// @Produce json
// @Param data body requestbody.RawDataListQueryRequest true "查询参数"
// @Success 200 {object} msg.Response
// @Failure 400 {object} msg.ErrResponseST
// @Failure 500 {object} msg.ErrResponseST
// @Router /api/v1/data/raw/list [post]
func (ctrl *QueryController) GetRawDataList(c *gin.Context) {
	// 解析查询参数
	var req requestbody.RawDataListQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, msg.ErrResponse("无效的请求参数", err))
		return
	}

	// 设置默认值
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}

	// 调用服务查询数据
	result, err := ctrl.service.GetRawDataList(c.Request.Context(), req.Page, req.PageSize, req.Source, req.StartTime, req.EndTime, req.Origin)
	if err != nil {
		c.JSON(500, msg.ErrResponse("查询原始数据列表失败", err))
		return
	}

	// 返回成功响应
	c.JSON(200, msg.SuccessResponse("查询成功", &map[string]any{
		"list":        result.List,
		"total":       result.Total,
		"page":        result.Page,
		"page_size":   result.PageSize,
		"total_pages": result.TotalPages,
	}))
}
