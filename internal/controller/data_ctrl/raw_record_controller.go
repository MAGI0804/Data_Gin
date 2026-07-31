package data_ctrl

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gin-biz-web-api/internal/msg"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/internal/service/data_svc"

	"github.com/gin-gonic/gin"
)

const rawRecordDateTimeLayout = "2006-01-02 15:04:05"

var rawRecordTimeZone = time.FixedZone("Asia/Shanghai", 8*60*60)

type RawRecordListService interface {
	List(context.Context, data_svc.RawRecordListQuery) (*data_svc.RawRecordListResult, error)
}

type RawRecordController struct {
	service RawRecordListService
}

func NewRawRecordController() *RawRecordController {
	return NewRawRecordControllerWithService(data_svc.NewRawRecordService())
}

func NewRawRecordControllerWithService(service RawRecordListService) *RawRecordController {
	if service == nil {
		panic("raw record controller: nil service")
	}
	return &RawRecordController{service: service}
}

// List returns safe, paginated raw_records metadata for management views.
func (controller *RawRecordController) List(c *gin.Context) {
	var req requestbody.RawRecordListQueryRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(400, msg.ErrResponseStr("无效的原始记录查询参数"))
		return
	}
	if rawRecordPaginationParameterInvalid(c, req) {
		c.JSON(400, msg.ErrResponseStr("无效的原始记录查询参数"))
		return
	}

	startTime, err := parseRawRecordListTime(req.StartTime)
	if err != nil {
		c.JSON(400, msg.ErrResponseStr("无效的原始记录查询时间"))
		return
	}
	endTime, err := parseRawRecordListTime(req.EndTime)
	if err != nil || rawRecordTimeRangeInvalid(startTime, endTime) {
		c.JSON(400, msg.ErrResponseStr("无效的原始记录查询时间"))
		return
	}

	result, err := controller.service.List(c.Request.Context(), data_svc.RawRecordListQuery{
		Page:      rawRecordPage(req.Page),
		PageSize:  rawRecordPageSize(req.PageSize),
		Source:    strings.TrimSpace(req.Source),
		Status:    req.Status,
		TraceID:   strings.TrimSpace(req.TraceID),
		StartTime: startTime,
		EndTime:   endTime,
		Origin:    req.Origin,
	})
	if err != nil {
		c.JSON(500, msg.ErrResponseStr("查询原始记录失败"))
		return
	}

	c.JSON(200, msg.SuccessResponse("查询原始记录成功", &map[string]any{
		"list":        result.List,
		"total":       result.Total,
		"page":        result.Page,
		"page_size":   result.PageSize,
		"total_pages": result.TotalPages,
	}))
}

func rawRecordPage(value int) int {
	if value == 0 {
		return 1
	}
	return value
}

func rawRecordPageSize(value int) int {
	if value == 0 {
		return 20
	}
	return value
}

func rawRecordPaginationParameterInvalid(c *gin.Context, req requestbody.RawRecordListQueryRequest) bool {
	_, hasPage := c.GetQuery("page")
	_, hasPageSize := c.GetQuery("page_size")
	return (hasPage && req.Page == 0) || (hasPageSize && req.PageSize == 0)
}

func parseRawRecordListTime(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.ParseInLocation(rawRecordDateTimeLayout, value, rawRecordTimeZone)
	if err != nil {
		return nil, fmt.Errorf("parse raw record list time: %w", err)
	}
	return &parsed, nil
}

func rawRecordTimeRangeInvalid(startTime, endTime *time.Time) bool {
	return startTime != nil && endTime != nil && endTime.Before(*startTime)
}
