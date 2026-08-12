package data_svc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/model"
)

type MallWeatherMinutelyResult struct {
	Items      []MallWeatherMinutelyDTO `json:"items"`
	Meta       MallWeatherQueryMeta     `json:"meta"`
	Pagination MallWeatherPagination    `json:"pagination"`
}

type weatherMinutelyCursor struct {
	Version              int   `json:"v"`
	ForecastMinuteUnixMS int64 `json:"forecastMinuteUnixMs"`
	IssuedAtUnixMS       int64 `json:"issuedAtUnixMs,omitempty"`
	ID                   uint  `json:"id"`
	Page                 int   `json:"page,omitempty"`
}

func (service *MallWeatherQueryService) Minutely(
	ctx context.Context,
	actorUserID uint,
	mallID uint,
	request requestbody.MallWeatherMinutelyQueryRequest,
) (*MallWeatherMinutelyResult, error) {
	if service == nil || ctx == nil || mallID == 0 {
		return nil, fmt.Errorf("%w: invalid request", ErrMallWeatherInvalidQuery)
	}
	if err := service.authorize(ctx, actorUserID); err != nil {
		return nil, err
	}
	if err := service.requireMallScope(ctx, actorUserID, mallID); err != nil {
		return nil, err
	}
	mall, err := service.malls.FindByID(ctx, mallID)
	if err != nil {
		return nil, err
	}
	location, normalized, err := normalizeMinutelyWeatherRequest(request, mall)
	if err != nil {
		return nil, err
	}
	cursor, err := decodeWeatherMinutelyCursor(normalized.Cursor)
	if err != nil {
		return nil, err
	}
	page := 1
	if cursor != nil {
		page = openCursorPage(cursor.Page, true)
	}
	query := data_dao.MinutelyQuery{
		MallID: mallID, StartUTC: normalized.StartUTC, EndUTC: normalized.EndUTC,
		AsOfUTC: normalized.AsOfUTC, Latest: normalized.Latest, QualityStatus: normalized.QualityStatus,
		Limit: normalized.PageSize + 1,
	}
	if cursor != nil {
		forecastMinute := time.UnixMilli(cursor.ForecastMinuteUnixMS).UTC()
		query.AfterForecastMinute = &forecastMinute
		query.AfterID = cursor.ID
		if cursor.IssuedAtUnixMS != 0 {
			issuedAt := time.UnixMilli(cursor.IssuedAtUnixMS).UTC()
			query.AfterIssuedAtUTC = &issuedAt
		}
	}
	rows, err := service.weather.QueryMinutely(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("mall weather query: minutely rows: %w", err)
	}
	hasMore := len(rows) > normalized.PageSize
	if hasMore {
		rows = rows[:normalized.PageSize]
	}
	items := make([]MallWeatherMinutelyDTO, len(rows))
	for index := range rows {
		items[index], err = minutelyWeatherDTO(&rows[index], location)
		if err != nil {
			return nil, err
		}
	}
	result := &MallWeatherMinutelyResult{
		Items:      items,
		Meta:       weatherQueryMeta(mall, location, model.MallWeatherFreshnessFresh, nil),
		Pagination: MallWeatherPagination{PageSize: normalized.PageSize},
	}
	if normalized.IncludeTotals {
		totalItems, err := service.weather.CountMinutely(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("mall weather query: count minutely rows: %w", err)
		}
		applyOpenWeatherPagination(&result.Pagination, page, totalItems)
	}
	latest, err := service.weather.FindCurrentLatest(ctx, mallID, model.MallWeatherDataKindMinutely)
	if err != nil && !errors.Is(err, data_dao.ErrMallWeatherLatestNotFound) {
		return nil, fmt.Errorf("mall weather query: minutely freshness: %w", err)
	}
	if latest == nil {
		result.Meta.FreshnessStatus = "UNAVAILABLE"
	} else {
		status, age, err := currentWeatherFreshness(model.MallWeatherDataKindMinutely, latest, service.now().UTC())
		if err != nil {
			return nil, err
		}
		result.Meta.FreshnessStatus = strings.ToUpper(status)
		result.Meta.DataAgeSeconds = &age
	}
	if hasMore && len(rows) > 0 {
		nextPage, pageErr := nextOpenCursorPage(page)
		if pageErr != nil {
			return nil, pageErr
		}
		result.Pagination.NextCursor, err = encodeWeatherMinutelyCursor(&rows[len(rows)-1], nextPage)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func normalizeMinutelyWeatherRequest(request requestbody.MallWeatherMinutelyQueryRequest, mall *model.Mall) (*time.Location, requestbody.MallWeatherMinutelyQueryRequest, error) {
	location, normalized, err := normalizeWeatherTimeSeriesRequest(weatherTimeSeriesRequest{
		StartUTC: request.StartUTC, EndUTC: request.EndUTC, TimeZone: request.TimeZone,
		Latest: request.Latest, AsOfUTC: request.AsOfUTC, QualityStatus: request.QualityStatus,
		Cursor: request.Cursor, PageSize: request.PageSize,
	}, mall)
	if err != nil {
		return nil, request, err
	}
	request.StartUTC = normalized.StartUTC
	request.EndUTC = normalized.EndUTC
	request.TimeZone = normalized.TimeZone
	request.Latest = normalized.Latest
	request.AsOfUTC = normalized.AsOfUTC
	request.QualityStatus = normalized.QualityStatus
	request.Cursor = normalized.Cursor
	request.PageSize = normalized.PageSize
	return location, request, nil
}

func encodeWeatherMinutelyCursor(row *model.MallWeatherMinutely, page int) (string, error) {
	if row == nil || row.ID == 0 || row.ForecastMinuteUTC.IsZero() || row.IssuedAtUTC.IsZero() {
		return "", fmt.Errorf("mall weather query: invalid minutely cursor row")
	}
	payload, err := json.Marshal(weatherMinutelyCursor{
		Version: 1, ForecastMinuteUnixMS: row.ForecastMinuteUTC.UTC().UnixMilli(),
		IssuedAtUnixMS: row.IssuedAtUTC.UTC().UnixMilli(), ID: row.ID, Page: page,
	})
	if err != nil {
		return "", fmt.Errorf("mall weather query: encode minutely cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeWeatherMinutelyCursor(value string) (*weatherMinutelyCursor, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	if len(value) > maxWeatherCursorLength {
		return nil, fmt.Errorf("%w: invalid cursor", ErrMallWeatherInvalidQuery)
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid cursor", ErrMallWeatherInvalidQuery)
	}
	var cursor weatherMinutelyCursor
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cursor); err != nil || cursor.Version != 1 || cursor.ID == 0 || invalidOpenCursorPage(cursor.Page) ||
		cursor.ForecastMinuteUnixMS < minWeatherCursorUnixMS || cursor.ForecastMinuteUnixMS >= maxWeatherCursorUnixMS ||
		cursor.IssuedAtUnixMS < minWeatherCursorUnixMS || cursor.IssuedAtUnixMS >= maxWeatherCursorUnixMS {
		return nil, fmt.Errorf("%w: invalid cursor", ErrMallWeatherInvalidQuery)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: invalid cursor", ErrMallWeatherInvalidQuery)
	}
	return &cursor, nil
}
