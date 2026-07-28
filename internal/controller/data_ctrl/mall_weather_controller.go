package data_ctrl

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

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
	FetchRuns(context.Context, uint, uint, requestbody.MallWeatherFetchRunQueryRequest) (*data_svc.MallWeatherFetchRunResult, error)
}

type MallWeatherController struct {
	service MallWeatherQueryService
}

type openMallWeatherOverviewRequest struct {
	TimeZone string `json:"timeZone"`
}

type openMallWeatherTimeSeriesRequest struct {
	Start         string `json:"start"`
	End           string `json:"end"`
	TimeZone      string `json:"timeZone"`
	Latest        *bool  `json:"latest"`
	AsOf          string `json:"asOf"`
	QualityStatus string `json:"qualityStatus"`
	Cursor        string `json:"cursor"`
	PageSize      *int   `json:"pageSize"`
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

func (controller *MallWeatherController) FetchRuns(c *gin.Context) {
	mallID, err := parseMallUint(c.Param("id"), "mall id")
	if err != nil {
		writeMallWeatherError(c, err)
		return
	}
	request, err := parseMallWeatherFetchRunRequest(c)
	if err != nil {
		writeMallWeatherError(c, err)
		return
	}
	result, err := controller.service.FetchRuns(c.Request.Context(), auth.CurrentUserID(c), mallID, request)
	if err != nil {
		writeMallWeatherError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}

func (controller *MallWeatherController) OpenOverview(c *gin.Context) {
	mallID, err := parseMallUint(c.Param("id"), "mall id")
	if err != nil {
		writeMallWeatherError(c, err)
		return
	}
	var request openMallWeatherOverviewRequest
	if err := decodeMallJSON(c, &request); err != nil {
		writeMallWeatherError(c, fmt.Errorf("%w: invalid JSON body", data_svc.ErrMallWeatherInvalidQuery))
		return
	}
	result, err := controller.service.Overview(
		c.Request.Context(), auth.CurrentUserID(c), mallID, strings.TrimSpace(request.TimeZone),
	)
	if err != nil {
		writeMallWeatherError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}

func (controller *MallWeatherController) OpenRealtime(c *gin.Context) {
	mallID, request, err := parseOpenMallWeatherRequest(c)
	if err != nil {
		writeMallWeatherError(c, err)
		return
	}
	result, err := controller.service.Realtime(c.Request.Context(), auth.CurrentUserID(c), mallID, requestbody.MallWeatherRealtimeQueryRequest{
		StartUTC: request.StartUTC, EndUTC: request.EndUTC, TimeZone: request.TimeZone,
		Latest: request.Latest, AsOfUTC: request.AsOfUTC, QualityStatus: request.QualityStatus,
		Cursor: request.Cursor, PageSize: request.PageSize,
	})
	if err != nil {
		writeMallWeatherError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}

func (controller *MallWeatherController) OpenMinutely(c *gin.Context) {
	mallID, request, err := parseOpenMallWeatherRequest(c)
	if err != nil {
		writeMallWeatherError(c, err)
		return
	}
	result, err := controller.service.Minutely(c.Request.Context(), auth.CurrentUserID(c), mallID, requestbody.MallWeatherMinutelyQueryRequest{
		StartUTC: request.StartUTC, EndUTC: request.EndUTC, TimeZone: request.TimeZone,
		Latest: request.Latest, AsOfUTC: request.AsOfUTC, QualityStatus: request.QualityStatus,
		Cursor: request.Cursor, PageSize: request.PageSize,
	})
	if err != nil {
		writeMallWeatherError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}

func (controller *MallWeatherController) OpenHourly(c *gin.Context) {
	mallID, request, err := parseOpenMallWeatherRequest(c)
	if err != nil {
		writeMallWeatherError(c, err)
		return
	}
	result, err := controller.service.Hourly(c.Request.Context(), auth.CurrentUserID(c), mallID, requestbody.MallWeatherHourlyQueryRequest{
		StartUTC: request.StartUTC, EndUTC: request.EndUTC, TimeZone: request.TimeZone,
		Latest: request.Latest, AsOfUTC: request.AsOfUTC, QualityStatus: request.QualityStatus,
		Cursor: request.Cursor, PageSize: request.PageSize,
	})
	if err != nil {
		writeMallWeatherError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}

func (controller *MallWeatherController) OpenDaily(c *gin.Context) {
	mallID, request, err := parseOpenMallWeatherRequest(c)
	if err != nil {
		writeMallWeatherError(c, err)
		return
	}
	result, err := controller.service.Daily(c.Request.Context(), auth.CurrentUserID(c), mallID, requestbody.MallWeatherDailyQueryRequest{
		StartUTC: request.StartUTC, EndUTC: request.EndUTC, TimeZone: request.TimeZone,
		Latest: request.Latest, AsOfUTC: request.AsOfUTC, QualityStatus: request.QualityStatus,
		Cursor: request.Cursor, PageSize: request.PageSize,
	})
	if err != nil {
		writeMallWeatherError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}

func (controller *MallWeatherController) OpenAlerts(c *gin.Context) {
	mallID, request, err := parseOpenMallWeatherRequest(c)
	if err != nil {
		writeMallWeatherError(c, err)
		return
	}
	result, err := controller.service.Alerts(c.Request.Context(), auth.CurrentUserID(c), mallID, requestbody.MallWeatherAlertQueryRequest{
		StartUTC: request.StartUTC, EndUTC: request.EndUTC, TimeZone: request.TimeZone,
		Latest: request.Latest, AsOfUTC: request.AsOfUTC, QualityStatus: request.QualityStatus,
		Cursor: request.Cursor, PageSize: request.PageSize,
	})
	if err != nil {
		writeMallWeatherError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}

func (controller *MallWeatherController) OpenLifeIndices(c *gin.Context) {
	mallID, request, err := parseOpenMallWeatherRequest(c)
	if err != nil {
		writeMallWeatherError(c, err)
		return
	}
	result, err := controller.service.LifeIndices(c.Request.Context(), auth.CurrentUserID(c), mallID, requestbody.MallWeatherLifeIndexQueryRequest{
		StartUTC: request.StartUTC, EndUTC: request.EndUTC, TimeZone: request.TimeZone,
		Latest: request.Latest, AsOfUTC: request.AsOfUTC, QualityStatus: request.QualityStatus,
		Cursor: request.Cursor, PageSize: request.PageSize,
	})
	if err != nil {
		writeMallWeatherError(c, err)
		return
	}
	responses.New(c).ToResponseWithStatus(http.StatusOK, result)
}

func parseOpenMallWeatherRequest(c *gin.Context) (uint, parsedMallWeatherTimeSeriesRequest, error) {
	mallID, err := parseMallUint(c.Param("id"), "mall id")
	if err != nil {
		return 0, parsedMallWeatherTimeSeriesRequest{}, err
	}
	var body openMallWeatherTimeSeriesRequest
	if err := decodeMallJSON(c, &body); err != nil {
		return 0, parsedMallWeatherTimeSeriesRequest{}, fmt.Errorf("%w: invalid JSON body", data_svc.ErrMallWeatherInvalidQuery)
	}
	start, err := parseRequiredWeatherTime(body.Start, "start")
	if err != nil {
		return 0, parsedMallWeatherTimeSeriesRequest{}, err
	}
	end, err := parseRequiredWeatherTime(body.End, "end")
	if err != nil {
		return 0, parsedMallWeatherTimeSeriesRequest{}, err
	}
	request := parsedMallWeatherTimeSeriesRequest{
		StartUTC:      start.UTC(),
		EndUTC:        end.UTC(),
		TimeZone:      strings.TrimSpace(body.TimeZone),
		Latest:        true,
		QualityStatus: strings.TrimSpace(body.QualityStatus),
		Cursor:        strings.TrimSpace(body.Cursor),
	}
	if body.Latest != nil {
		request.Latest = *body.Latest
	}
	if body.PageSize != nil {
		if *body.PageSize <= 0 {
			return 0, parsedMallWeatherTimeSeriesRequest{}, fmt.Errorf("%w: invalid pageSize", data_svc.ErrMallWeatherInvalidQuery)
		}
		request.PageSize = *body.PageSize
	}
	if asOfValue := strings.TrimSpace(body.AsOf); asOfValue != "" {
		asOf, err := time.Parse(time.RFC3339Nano, asOfValue)
		if err != nil {
			return 0, parsedMallWeatherTimeSeriesRequest{}, fmt.Errorf("%w: invalid asOf", data_svc.ErrMallWeatherInvalidQuery)
		}
		asOf = asOf.UTC()
		request.AsOfUTC = &asOf
	}
	return mallID, request, nil
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

func parseMallWeatherFetchRunRequest(c *gin.Context) (requestbody.MallWeatherFetchRunQueryRequest, error) {
	var request requestbody.MallWeatherFetchRunQueryRequest
	start, err := parseRequiredWeatherTime(c.Query("start"), "start")
	if err != nil {
		return request, err
	}
	end, err := parseRequiredWeatherTime(c.Query("end"), "end")
	if err != nil {
		return request, err
	}
	request.StartUTC, request.EndUTC = start.UTC(), end.UTC()
	request.TimeZone, err = weatherAliasedQuery(c, "timeZone", "timezone")
	if err != nil {
		return request, err
	}
	request.CorrelationID, err = parseMallWeatherCorrelationID(c)
	if err != nil {
		return request, err
	}
	request.TaskKind, err = weatherAliasedQuery(c, "taskKind", "task_kind")
	if err != nil {
		return request, err
	}
	request.EndpointKind, err = weatherAliasedQuery(c, "endpointKind", "endpoint_kind")
	if err != nil {
		return request, err
	}
	request.Status = strings.TrimSpace(c.Query("status"))
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

func parseMallWeatherCorrelationID(c *gin.Context) (string, error) {
	values, exists := c.GetQueryArray("correlationId")
	if !exists {
		return "", nil
	}
	if len(values) != 1 {
		return "", fmt.Errorf("%w: invalid correlationId", data_svc.ErrMallWeatherInvalidQuery)
	}
	value := strings.TrimSpace(values[0])
	if len(value) > 128 || !utf8.ValidString(value) || strings.ContainsAny(value, "\x00\r\n") {
		return "", fmt.Errorf("%w: invalid correlationId", data_svc.ErrMallWeatherInvalidQuery)
	}
	return value, nil
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
