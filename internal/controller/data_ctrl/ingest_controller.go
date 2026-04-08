package data_ctrl

import (
	"bytes"
	"encoding/json"
	"io/ioutil"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"gin-biz-web-api/internal/msg"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/internal/service/data_svc"
	"gin-biz-web-api/pkg/logger"
)

type IngestController struct {
	service *data_svc.IngestService
}

func NewIngestController() *IngestController {
	return &IngestController{
		service: data_svc.NewIngestService(),
	}
}

// IngestData 接收数据推送
// @Summary 接收数据推送
// @Description 接收外部系统推送的数据
// @Tags 数据接收
// @Accept json
// @Produce json
// @Param data body requestbody.IngestRequest true "数据推送请求"
// @Success 200 {object} msg.Response
// @Failure 400 {object} msg.ErrResponseST
// @Failure 500 {object} msg.ErrResponseST
// @Router /api/v1/data/ingest [post]
func (ctrl *IngestController) IngestData(c *gin.Context) {
	// 解析请求参数
	var req requestbody.IngestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, msg.ErrResponse("无效的请求参数", err))
		return
	}

	// 调用服务处理数据
	result, err := ctrl.service.IngestData(c.Request.Context(), &req)
	if err != nil {
		c.JSON(500, msg.ErrResponse("数据接收失败", err))
		return
	}

	// 构建响应数据
	data := map[string]any{
		"request_id":     result.RequestID,
		"status":         "accepted",
		"accepted_count": result.AcceptedCount,
		"failed_count":   result.FailedCount,
		"message":        "数据接收成功，已进入处理队列",
	}

	// 返回成功响应
	c.JSON(200, msg.SuccessResponse("数据接收成功", &data))
}

// IngestBatchData 批量接收数据
// @Summary 批量接收数据
// @Description 批量接收外部系统推送的数据
// @Tags 数据接收
// @Accept json
// @Produce json
// @Param data body requestbody.BatchIngestRequest true "批量数据推送请求"
// @Success 200 {object} msg.Response
// @Failure 400 {object} msg.ErrResponseST
// @Failure 500 {object} msg.ErrResponseST
// @Router /api/v1/data/ingest/batch [post]
func (ctrl *IngestController) IngestBatchData(c *gin.Context) {
	// 解析请求参数
	var req requestbody.BatchIngestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, msg.ErrResponse("无效的请求参数", err))
		return
	}

	// 调用服务处理数据
	result, err := ctrl.service.IngestBatchData(c.Request.Context(), &req)
	if err != nil {
		c.JSON(500, msg.ErrResponse("批量数据接收失败", err))
		return
	}

	// 构建响应数据
	data := map[string]any{
		"batch_id":       result.BatchID,
		"status":         "accepted",
		"accepted_count": result.AcceptedCount,
		"failed_count":   result.FailedCount,
		"message":        "批量数据接收成功，已进入处理队列",
	}

	// 返回成功响应
	c.JSON(200, msg.SuccessResponse("批量数据接收成功", &data))
}

// RawIngestData 接收原始格式数据
// @Summary 接收原始格式数据
// @Description 接收任意格式的数据，直接全部留存（支持两种格式：1. 带raw_content字段 2. 直接发送数据）
// @Tags 数据接收
// @Accept json
// @Produce json
// @Param data body requestbody.RawIngestRequest true "原始数据推送请求"
// @Success 200 {object} msg.Response
// @Failure 400 {object} msg.ErrResponseST
// @Failure 500 {object} msg.ErrResponseST
// @Router /api/v1/data/ingest/raw [post]
func (ctrl *IngestController) RawIngestData(c *gin.Context) {
	// 获取原始请求体
	rawData, err := c.GetRawData()
	if err != nil {
		c.JSON(400, msg.ErrResponse("获取请求数据失败", err))
		return
	}

	// 首先尝试按标准格式解析
	var req requestbody.RawIngestRequest
	if len(rawData) > 0 {
		if err := json.Unmarshal(rawData, &req); err != nil {
			// 如果解析失败，可能是直接发送数据的情况
			// 解析整个请求体作为原始内容
			var rawContent interface{}
			if err := json.Unmarshal(rawData, &rawContent); err != nil {
				c.JSON(400, msg.ErrResponse("无效的请求参数", err))
				return
			}
			// 构建标准请求
			req = requestbody.RawIngestRequest{
				RawContent: rawContent,
			}
		}
	} else {
		// 如果请求体为空，创建一个空的请求对象
		req = requestbody.RawIngestRequest{}
		logger.Info("Empty request body, creating empty request")
	}

	// 从查询参数中获取 remark
	req.Remark = c.Query("remark")

	// 重置请求体，确保后续操作能正确读取
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(rawData))

	// 打印传递给服务层的数据
	logger.Info("Request sent to service", zap.Any("req", req))

	// 调用服务处理数据，传递客户端IP
	result, err := ctrl.service.RawIngestData(c.Request.Context(), &req, c.ClientIP())
	if err != nil {
		c.JSON(500, msg.ErrResponse("原始数据接收失败", err))
		return
	}

	// 构建响应数据
	data := map[string]any{
		"request_id":     result.RequestID,
		"status":         "accepted",
		"accepted_count": result.AcceptedCount,
		"failed_count":   result.FailedCount,
		"message":        "原始数据接收成功，已进入处理队列",
	}

	// 返回成功响应
	c.JSON(200, msg.SuccessResponse("原始数据接收成功", &data))
}
