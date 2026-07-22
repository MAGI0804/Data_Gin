package data_ctrl

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/internal/service/data_svc"
	"gin-biz-web-api/pkg/auth"
	"gin-biz-web-api/pkg/errcode"
	"gin-biz-web-api/pkg/responses"

	"github.com/gin-gonic/gin"
)

type MallWeatherQueryService interface {
	Overview(context.Context, uint, uint, string) (*data_svc.MallWeatherOverviewResult, error)
	Realtime(context.Context, uint, uint, requestbody.MallWeatherRealtimeQueryRequest) (*data_svc.MallWeatherRealtimeResult, error)
	Minutely(context.Context, uint, uint, requestbody.MallWeatherMinutelyQueryRequest) (*data_svc.MallWeatherMinutelyResult, error)
	Hourly(context.Context, uint, uint, requestbody.MallWeatherHourlyQueryRequest) (*data_svc.MallWeatherHourlyResult, error)
	Daily(context.Context, uint, uint, requestbody.MallWeatherDailyQueryRequest) (*data_svc.MallWeatherDailyResult, error)
	Alerts(context.Context, uint, uint, requestbody.MallWeatherAlertQueryRequest) (*data_svc.MallWeatherAlertResult, error)
	LifeIndices(context.Context, uint, uint, requestbody.MallWeatherLifeIndexQueryRequest) (*data_svc.MallWeatherLifeIndexResult, error)
}

type MallWeatherController struct {
	service MallWeatherQueryService
}

func NewMallWeatherController() *MallWeatherController {
	return NewMallWeatherControllerWithService(data_svc.NewMallWeatherQueryService())
}

func NewMallWeatherControllerWithService(service MallWeatherQueryService) *MallWeatherController {
	if service == nil {
		panic("mall weather controller: nil service")
	}
	return &MallWeatherController{service: service}
}

func (controller *MallWeatherController) Overview(c *gin.Context) {
	mallID, err := parseMallUint(c.Param("id"), "mall id")
	if err != nil {
		writeMallWeatherError(c, err)
		return
	}
	timeZone, err := weatherAliasedQuery(c, "timeZone", "timezone")
	if err != nil {
		writeMallWeatherError(c, err)
		return
	}
	result, err := controller.service.Overview(c.Request.Context(), auth.CurrentUserID(c), mallID, timeZone)
	if err != nil {
		writeMallWeatherError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}

func (controller *MallWeatherController) Realtime(c *gin.Context) {
	mallID, err := parseMallUint(c.Param("id"), "mall id")
	if err != nil {
		writeMallWeatherError(c, err)
		return
	}
	request, err := parseMallWeatherRealtimeRequest(c)
	if err != nil {
		writeMallWeatherError(c, err)
		return
	}
	result, err := controller.service.Realtime(c.Request.Context(), auth.CurrentUserID(c), mallID, request)
	if err != nil {
		writeMallWeatherError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}

func (controller *MallWeatherController) Minutely(c *gin.Context) {
	mallID, err := parseMallUint(c.Param("id"), "mall id")
	if err != nil {
		writeMallWeatherError(c, err)
		return
	}
	request, err := parseMallWeatherMinutelyRequest(c)
	if err != nil {
		writeMallWeatherError(c, err)
		return
	}
	result, err := controller.service.Minutely(c.Request.Context(), auth.CurrentUserID(c), mallID, request)
	if err != nil {
		writeMallWeatherError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}

func (controller *MallWeatherController) Hourly(c *gin.Context) {
	mallID, err := parseMallUint(c.Param("id"), "mall id")
	if err != nil {
		writeMallWeatherError(c, err)
		return
	}
	request, err := parseMallWeatherHourlyRequest(c)
	if err != nil {
		writeMallWeatherError(c, err)
		return
	}
	result, err := controller.service.Hourly(c.Request.Context(), auth.CurrentUserID(c), mallID, request)
	if err != nil {
		writeMallWeatherError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}

func (controller *MallWeatherController) Daily(c *gin.Context) {
	mallID, err := parseMallUint(c.Param("id"), "mall id")
	if err != nil {
		writeMallWeatherError(c, err)
		return
	}
	request, err := parseMallWeatherDailyRequest(c)
	if err != nil {
		writeMallWeatherError(c, err)
		return
	}
	result, err := controller.service.Daily(c.Request.Context(), auth.CurrentUserID(c), mallID, request)
	if err != nil {
		writeMallWeatherError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}

func (controller *MallWeatherController) Alerts(c *gin.Context) {
	mallID, err := parseMallUint(c.Param("id"), "mall id")
	if err != nil {
		writeMallWeatherError(c, err)
		return
	}
	request, err := parseMallWeatherAlertRequest(c)
	if err != nil {
		writeMallWeatherError(c, err)
		return
	}
	result, err := controller.service.Alerts(c.Request.Context(), auth.CurrentUserID(c), mallID, request)
	if err != nil {
		writeMallWeatherError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}

func (controller *MallWeatherController) LifeIndices(c *gin.Context) {
	mallID, err := parseMallUint(c.Param("id"), "mall id")
	if err != nil {
		writeMallWeatherError(c, err)
		return
	}
	request, err := parseMallWeatherLifeIndexRequest(c)
	if err != nil {
		writeMallWeatherError(c, err)
		return
	}
	result, err := controller.service.LifeIndices(c.Request.Context(), auth.CurrentUserID(c), mallID, request)
	if err != nil {
		writeMallWeatherError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}

func parseMallWeatherRealtimeRequest(c *gin.Context) (requestbody.MallWeatherRealtimeQueryRequest, error) {
	shared, err := parseMallWeatherTimeSeriesRequest(c)
	if err != nil {
		return requestbody.MallWeatherRealtimeQueryRequest{}, err
	}
	return requestbody.MallWeatherRealtimeQueryRequest{
		StartUTC: shared.StartUTC, EndUTC: shared.EndUTC, TimeZone: shared.TimeZone,
		Latest: shared.Latest, AsOfUTC: shared.AsOfUTC, QualityStatus: shared.QualityStatus,
		Cursor: shared.Cursor, PageSize: shared.PageSize,
	}, nil
}

func parseMallWeatherMinutelyRequest(c *gin.Context) (requestbody.MallWeatherMinutelyQueryRequest, error) {
	shared, err := parseMallWeatherTimeSeriesRequest(c)
	if err != nil {
		return requestbody.MallWeatherMinutelyQueryRequest{}, err
	}
	return requestbody.MallWeatherMinutelyQueryRequest{
		StartUTC: shared.StartUTC, EndUTC: shared.EndUTC, TimeZone: shared.TimeZone,
		Latest: shared.Latest, AsOfUTC: shared.AsOfUTC, QualityStatus: shared.QualityStatus,
		Cursor: shared.Cursor, PageSize: shared.PageSize,
	}, nil
}

func parseMallWeatherHourlyRequest(c *gin.Context) (requestbody.MallWeatherHourlyQueryRequest, error) {
	shared, err := parseMallWeatherTimeSeriesRequest(c)
	if err != nil {
		return requestbody.MallWeatherHourlyQueryRequest{}, err
	}
	return requestbody.MallWeatherHourlyQueryRequest{
		StartUTC: shared.StartUTC, EndUTC: shared.EndUTC, TimeZone: shared.TimeZone,
		Latest: shared.Latest, AsOfUTC: shared.AsOfUTC, QualityStatus: shared.QualityStatus,
		Cursor: shared.Cursor, PageSize: shared.PageSize,
	}, nil
}

func parseMallWeatherDailyRequest(c *gin.Context) (requestbody.MallWeatherDailyQueryRequest, error) {
	shared, err := parseMallWeatherTimeSeriesRequest(c)
	if err != nil {
		return requestbody.MallWeatherDailyQueryRequest{}, err
	}
	return requestbody.MallWeatherDailyQueryRequest{
		StartUTC: shared.StartUTC, EndUTC: shared.EndUTC, TimeZone: shared.TimeZone,
		Latest: shared.Latest, AsOfUTC: shared.AsOfUTC, QualityStatus: shared.QualityStatus,
		Cursor: shared.Cursor, PageSize: shared.PageSize,
	}, nil
}

func parseMallWeatherAlertRequest(c *gin.Context) (requestbody.MallWeatherAlertQueryRequest, error) {
	shared, err := parseMallWeatherTimeSeriesRequest(c)
	if err != nil {
		return requestbody.MallWeatherAlertQueryRequest{}, err
	}
	return requestbody.MallWeatherAlertQueryRequest{
		StartUTC: shared.StartUTC, EndUTC: shared.EndUTC, TimeZone: shared.TimeZone,
		Latest: shared.Latest, AsOfUTC: shared.AsOfUTC, QualityStatus: shared.QualityStatus,
		Cursor: shared.Cursor, PageSize: shared.PageSize,
	}, nil
}

func parseMallWeatherLifeIndexRequest(c *gin.Context) (requestbody.MallWeatherLifeIndexQueryRequest, error) {
	shared, err := parseMallWeatherTimeSeriesRequest(c)
	if err != nil {
		return requestbody.MallWeatherLifeIndexQueryRequest{}, err
	}
	return requestbody.MallWeatherLifeIndexQueryRequest{
		StartUTC: shared.StartUTC, EndUTC: shared.EndUTC, TimeZone: shared.TimeZone,
		Latest: shared.Latest, AsOfUTC: shared.AsOfUTC, QualityStatus: shared.QualityStatus,
		Cursor: shared.Cursor, PageSize: shared.PageSize,
	}, nil
}

type parsedMallWeatherTimeSeriesRequest struct {
	StartUTC      time.Time
	EndUTC        time.Time
	TimeZone      string
	Latest        bool
	AsOfUTC       *time.Time
	QualityStatus string
	Cursor        string
	PageSize      int
}

func parseMallWeatherTimeSeriesRequest(c *gin.Context) (parsedMallWeatherTimeSeriesRequest, error) {
	var request parsedMallWeatherTimeSeriesRequest
	start, err := parseRequiredWeatherTime(c.Query("start"), "start")
	if err != nil {
		return request, err
	}
	end, err := parseRequiredWeatherTime(c.Query("end"), "end")
	if err != nil {
		return request, err
	}
	request.StartUTC = start.UTC()
	request.EndUTC = end.UTC()

	request.TimeZone, err = weatherAliasedQuery(c, "timeZone", "timezone")
	if err != nil {
		return request, err
	}
	request.QualityStatus, err = weatherAliasedQuery(c, "qualityStatus", "quality_status")
	if err != nil {
		return request, err
	}
	asOfValue, err := weatherAliasedQuery(c, "asOf", "as_of")
	if err != nil {
		return request, err
	}
	if asOfValue != "" {
		asOf, err := time.Parse(time.RFC3339Nano, asOfValue)
		if err != nil {
			return request, fmt.Errorf("%w: invalid asOf", data_svc.ErrMallWeatherInvalidQuery)
		}
		asOf = asOf.UTC()
		request.AsOfUTC = &asOf
	}

	request.Latest = true
	if latestValue := strings.TrimSpace(c.Query("latest")); latestValue != "" {
		request.Latest, err = strconv.ParseBool(latestValue)
		if err != nil {
			return request, fmt.Errorf("%w: invalid latest", data_svc.ErrMallWeatherInvalidQuery)
		}
	}
	pageSizeValue, err := weatherAliasedQuery(c, "pageSize", "page_size")
	if err != nil {
		return request, err
	}
	if pageSizeValue != "" {
		request.PageSize, err = strconv.Atoi(pageSizeValue)
		if err != nil || request.PageSize <= 0 {
			return request, fmt.Errorf("%w: invalid pageSize", data_svc.ErrMallWeatherInvalidQuery)
		}
	}
	request.Cursor = strings.TrimSpace(c.Query("cursor"))
	return request, nil
}

func parseRequiredWeatherTime(value, field string) (time.Time, error) {
	value = strings.TrimSpace(value)
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil || value == "" {
		return time.Time{}, fmt.Errorf("%w: invalid %s", data_svc.ErrMallWeatherInvalidQuery, field)
	}
	return parsed, nil
}

func weatherAliasedQuery(c *gin.Context, primary, alias string) (string, error) {
	primaryValue := strings.TrimSpace(c.Query(primary))
	aliasValue := strings.TrimSpace(c.Query(alias))
	if primaryValue != "" && aliasValue != "" && primaryValue != aliasValue {
		return "", fmt.Errorf("%w: conflicting %s", data_svc.ErrMallWeatherInvalidQuery, primary)
	}
	if primaryValue != "" {
		return primaryValue, nil
	}
	return aliasValue, nil
}

func writeMallWeatherError(c *gin.Context, err error) {
	code, message := classifyMallWeatherError(err)
	responses.New(c).ToSafeErrorResponse(code, message)
}

func classifyMallWeatherError(err error) (*errcode.Error, string) {
	switch {
	case errors.Is(err, data_svc.ErrMallForbidden):
		return errcode.Forbidden, "无权查询天气数据"
	case errors.Is(err, data_dao.ErrMallNotFound):
		return errcode.NotFound, "商场不存在"
	case errors.Is(err, data_svc.ErrMallWeatherCoordinateUnconfirmed):
		return errcode.UnprocessableEntity, "商场天气坐标尚未确认"
	case errors.Is(err, data_svc.ErrMallWeatherInvalidQuery):
		return errcode.UnprocessableEntity, "天气查询参数校验失败"
	case errors.Is(err, data_svc.ErrMallInvalidInput):
		return errcode.UnprocessableEntity, "天气查询参数校验失败"
	default:
		return errcode.InternalServerError, "天气查询服务暂时不可用"
	}
}
