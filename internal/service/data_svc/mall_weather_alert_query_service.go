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

type MallWeatherAlertResult struct {
	Items      []MallWeatherAlertDTO `json:"items"`
	Meta       MallWeatherQueryMeta  `json:"meta"`
	Pagination MallWeatherPagination `json:"pagination"`
}

type weatherAlertCursor struct {
	Version        int   `json:"v"`
	SortTimeUnixMS int64 `json:"sortTimeUnixMs"`
	ID             uint  `json:"id"`
	Page           int   `json:"page,omitempty"`
}

func (service *MallWeatherQueryService) Alerts(ctx context.Context, actorUserID, mallID uint, request requestbody.MallWeatherAlertQueryRequest) (*MallWeatherAlertResult, error) {
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
	location, normalized, err := normalizeAlertWeatherRequest(request, mall)
	if err != nil {
		return nil, err
	}
	cursor, err := decodeWeatherAlertCursor(normalized.Cursor)
	if err != nil {
		return nil, err
	}
	page := 1
	if cursor != nil {
		page = openCursorPage(cursor.Page, true)
	}
	query := data_dao.AlertQuery{
		MallID: mallID, StartUTC: normalized.StartUTC, EndUTC: normalized.EndUTC,
		AsOfUTC: normalized.AsOfUTC, Latest: normalized.Latest, QualityStatus: normalized.QualityStatus,
		Limit: normalized.PageSize + 1,
	}
	if cursor != nil {
		sortTime := time.UnixMilli(cursor.SortTimeUnixMS).UTC()
		query.AfterSortTime = &sortTime
		query.AfterID = cursor.ID
	}
	rows, err := service.weather.QueryAlerts(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("mall weather query: alert rows: %w", err)
	}
	hasMore := len(rows) > normalized.PageSize
	if hasMore {
		rows = rows[:normalized.PageSize]
	}
	items := make([]MallWeatherAlertDTO, len(rows))
	for index := range rows {
		items[index], err = alertWeatherDTO(&rows[index], location)
		if err != nil {
			return nil, err
		}
	}
	result := &MallWeatherAlertResult{Items: items, Meta: weatherQueryMeta(mall, location, model.MallWeatherFreshnessFresh, nil), Pagination: MallWeatherPagination{PageSize: normalized.PageSize}}
	if normalized.IncludeTotals {
		totalItems, err := service.weather.CountAlerts(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("mall weather query: count alert rows: %w", err)
		}
		applyOpenWeatherPagination(&result.Pagination, page, totalItems)
	}
	latest, err := service.weather.FindCurrentLatest(ctx, mallID, model.MallWeatherDataKindRealtime)
	if err != nil && !errors.Is(err, data_dao.ErrMallWeatherLatestNotFound) {
		return nil, fmt.Errorf("mall weather query: alert freshness: %w", err)
	}
	if latest == nil {
		result.Meta.FreshnessStatus = "UNAVAILABLE"
	} else {
		status, age, err := currentWeatherFreshness(model.MallWeatherDataKindRealtime, latest, service.now().UTC())
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
		result.Pagination.NextCursor, err = encodeWeatherAlertCursor(&rows[len(rows)-1], nextPage)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func normalizeAlertWeatherRequest(request requestbody.MallWeatherAlertQueryRequest, mall *model.Mall) (*time.Location, requestbody.MallWeatherAlertQueryRequest, error) {
	location, normalized, err := normalizeWeatherTimeSeriesRequest(weatherTimeSeriesRequest{
		StartUTC: request.StartUTC, EndUTC: request.EndUTC, TimeZone: request.TimeZone,
		Latest: request.Latest, AsOfUTC: request.AsOfUTC, QualityStatus: request.QualityStatus,
		Cursor: request.Cursor, PageSize: request.PageSize,
	}, mall)
	if err != nil {
		return nil, request, err
	}
	request.StartUTC, request.EndUTC, request.TimeZone = normalized.StartUTC, normalized.EndUTC, normalized.TimeZone
	request.Latest, request.AsOfUTC, request.QualityStatus = normalized.Latest, normalized.AsOfUTC, normalized.QualityStatus
	request.Cursor, request.PageSize = normalized.Cursor, normalized.PageSize
	return location, request, nil
}

func encodeWeatherAlertCursor(row *model.MallWeatherAlert, page int) (string, error) {
	if row == nil || row.ID == 0 {
		return "", fmt.Errorf("mall weather query: invalid alert cursor row")
	}
	sortTime := row.FirstSeenAt
	if row.PublishedAtUTC != nil {
		sortTime = *row.PublishedAtUTC
	}
	if sortTime.IsZero() {
		return "", fmt.Errorf("mall weather query: invalid alert cursor time")
	}
	payload, err := json.Marshal(weatherAlertCursor{Version: 1, SortTimeUnixMS: sortTime.UTC().UnixMilli(), ID: row.ID, Page: page})
	if err != nil {
		return "", fmt.Errorf("mall weather query: encode alert cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeWeatherAlertCursor(value string) (*weatherAlertCursor, error) {
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
	var cursor weatherAlertCursor
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cursor); err != nil || cursor.Version != 1 || cursor.ID == 0 || invalidOpenCursorPage(cursor.Page) ||
		cursor.SortTimeUnixMS < minWeatherCursorUnixMS || cursor.SortTimeUnixMS >= maxWeatherCursorUnixMS {
		return nil, fmt.Errorf("%w: invalid cursor", ErrMallWeatherInvalidQuery)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: invalid cursor", ErrMallWeatherInvalidQuery)
	}
	return &cursor, nil
}
