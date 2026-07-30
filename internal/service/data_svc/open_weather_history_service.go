package data_svc

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gin-biz-web-api/internal/requestbody"
)

const openWeatherDateLayout = "2006-01-02"

// HistoryDay returns observed realtime snapshots for one completed local day.
func (service *MallWeatherQueryService) HistoryDay(
	ctx context.Context,
	actorUserID uint,
	mallID uint,
	request requestbody.OpenWeatherHistoryDayQueryRequest,
) (*MallWeatherRealtimeResult, error) {
	if service == nil || ctx == nil || mallID == 0 {
		return nil, fmt.Errorf("%w: invalid request", ErrMallWeatherInvalidQuery)
	}
	if err := service.authorize(ctx, actorUserID); err != nil {
		return nil, err
	}
	mall, err := service.malls.FindByID(ctx, mallID)
	if err != nil {
		return nil, err
	}
	location, err := weatherMallLocation(mall, request.TimeZone)
	if err != nil {
		return nil, err
	}
	dateValue := strings.TrimSpace(request.Date)
	date, err := time.ParseInLocation(openWeatherDateLayout, dateValue, location)
	if err != nil || date.Format(openWeatherDateLayout) != dateValue {
		return nil, fmt.Errorf("%w: invalid date", ErrMallWeatherInvalidQuery)
	}
	today := service.now().In(location)
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, location)
	if !date.Before(today) {
		return nil, fmt.Errorf("%w: date must be in the past", ErrMallWeatherInvalidQuery)
	}

	return service.realtimeForMall(ctx, mallID, mall, requestbody.MallWeatherRealtimeQueryRequest{
		StartUTC:      date.UTC(),
		EndUTC:        date.AddDate(0, 0, 1).UTC(),
		TimeZone:      location.String(),
		QualityStatus: request.QualityStatus,
		Cursor:        request.Cursor,
		PageSize:      request.PageSize,
		IncludeTotals: true,
	})
}

// HistoryRange returns observed realtime snapshots in [startTime, endTime).
func (service *MallWeatherQueryService) HistoryRange(
	ctx context.Context,
	actorUserID uint,
	mallID uint,
	request requestbody.OpenWeatherHistoryRangeQueryRequest,
) (*MallWeatherRealtimeResult, error) {
	if service == nil || ctx == nil || mallID == 0 {
		return nil, fmt.Errorf("%w: invalid request", ErrMallWeatherInvalidQuery)
	}
	if err := service.authorize(ctx, actorUserID); err != nil {
		return nil, err
	}
	mall, err := service.malls.FindByID(ctx, mallID)
	if err != nil {
		return nil, err
	}
	location, err := weatherMallLocation(mall, request.TimeZone)
	if err != nil {
		return nil, err
	}
	start, err := parseOpenWeatherHistoryTime(request.StartTime, "startTime", location)
	if err != nil {
		return nil, err
	}
	end, err := parseOpenWeatherHistoryTime(request.EndTime, "endTime", location)
	if err != nil {
		return nil, err
	}
	if end.After(service.now()) {
		return nil, fmt.Errorf("%w: endTime must not be in the future", ErrMallWeatherInvalidQuery)
	}

	return service.realtimeForMall(ctx, mallID, mall, requestbody.MallWeatherRealtimeQueryRequest{
		StartUTC:      start.UTC(),
		EndUTC:        end.UTC(),
		TimeZone:      location.String(),
		QualityStatus: request.QualityStatus,
		Cursor:        request.Cursor,
		PageSize:      request.PageSize,
		IncludeTotals: true,
	})
}

func parseOpenWeatherHistoryTime(value, field string, location *time.Location) (time.Time, error) {
	value = strings.TrimSpace(value)
	parsed, err := time.ParseInLocation("2006-01-02 15:04:05", value, location)
	if err != nil || parsed.Format("2006-01-02 15:04:05") != value {
		return time.Time{}, fmt.Errorf("%w: invalid %s", ErrMallWeatherInvalidQuery, field)
	}
	return parsed, nil
}
