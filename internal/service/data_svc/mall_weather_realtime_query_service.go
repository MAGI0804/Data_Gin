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

type MallWeatherRealtimeResult struct {
	Items      []MallWeatherRealtimeDTO `json:"items"`
	Meta       MallWeatherQueryMeta     `json:"meta"`
	Pagination MallWeatherPagination    `json:"pagination"`
}

type weatherRealtimeCursor struct {
	Version        int   `json:"v"`
	SnapshotUnixMS int64 `json:"snapshotUnixMs"`
	ID             uint  `json:"id"`
	Page           int   `json:"page,omitempty"`
}

func (service *MallWeatherQueryService) Realtime(
	ctx context.Context,
	actorUserID uint,
	mallID uint,
	request requestbody.MallWeatherRealtimeQueryRequest,
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
	return service.realtimeForMall(ctx, mallID, mall, request)
}

func (service *MallWeatherQueryService) realtimeForMall(
	ctx context.Context,
	mallID uint,
	mall *model.Mall,
	request requestbody.MallWeatherRealtimeQueryRequest,
) (*MallWeatherRealtimeResult, error) {
	location, normalized, err := normalizeRealtimeWeatherRequest(request, mall)
	if err != nil {
		return nil, err
	}
	cursor, err := decodeWeatherRealtimeCursor(normalized.Cursor)
	if err != nil {
		return nil, err
	}
	page := openCursorPage(weatherRealtimeCursorPage(cursor), cursor != nil)
	query := data_dao.RealtimeQuery{
		MallID: mallID, StartUTC: normalized.StartUTC, EndUTC: normalized.EndUTC,
		AsOfUTC: normalized.AsOfUTC, QualityStatus: normalized.QualityStatus, Limit: normalized.PageSize + 1,
	}
	if cursor != nil {
		snapshot := time.UnixMilli(cursor.SnapshotUnixMS).UTC()
		query.AfterSnapshot = &snapshot
		query.AfterID = cursor.ID
	}
	rows, err := service.weather.QueryRealtime(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("mall weather query: realtime rows: %w", err)
	}
	hasMore := len(rows) > normalized.PageSize
	if hasMore {
		rows = rows[:normalized.PageSize]
	}
	items := make([]MallWeatherRealtimeDTO, len(rows))
	for index := range rows {
		items[index], err = realtimeWeatherDTO(&rows[index], location)
		if err != nil {
			return nil, err
		}
	}
	result := &MallWeatherRealtimeResult{
		Items:      items,
		Meta:       weatherQueryMeta(mall, location, model.MallWeatherFreshnessFresh, nil),
		Pagination: MallWeatherPagination{PageSize: normalized.PageSize},
	}
	if normalized.IncludeTotals {
		totalItems, err := service.weather.CountRealtime(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("mall weather query: count realtime rows: %w", err)
		}
		applyOpenWeatherPagination(&result.Pagination, page, totalItems)
	}
	if err := service.applyRealtimeFreshness(ctx, mallID, &result.Meta); err != nil {
		return nil, err
	}
	if hasMore && len(rows) > 0 {
		nextPage, pageErr := nextOpenCursorPage(page)
		if pageErr != nil {
			return nil, pageErr
		}
		result.Pagination.NextCursor, err = encodeWeatherRealtimeCursor(&rows[len(rows)-1], nextPage)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (service *MallWeatherQueryService) applyRealtimeFreshness(
	ctx context.Context,
	mallID uint,
	meta *MallWeatherQueryMeta,
) error {
	if service == nil || ctx == nil || mallID == 0 || meta == nil {
		return fmt.Errorf("mall weather query: invalid realtime freshness request")
	}
	latest, err := service.weather.FindCurrentLatest(ctx, mallID, model.MallWeatherDataKindRealtime)
	if err != nil && !errors.Is(err, data_dao.ErrMallWeatherLatestNotFound) {
		return fmt.Errorf("mall weather query: realtime freshness: %w", err)
	}
	if latest == nil {
		meta.FreshnessStatus = "UNAVAILABLE"
		meta.DataAgeSeconds = nil
		return nil
	}
	status, age, err := currentWeatherFreshness(model.MallWeatherDataKindRealtime, latest, service.now().UTC())
	if err != nil {
		return err
	}
	meta.FreshnessStatus = strings.ToUpper(status)
	meta.DataAgeSeconds = &age
	return nil
}

func normalizeRealtimeWeatherRequest(request requestbody.MallWeatherRealtimeQueryRequest, mall *model.Mall) (*time.Location, requestbody.MallWeatherRealtimeQueryRequest, error) {
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

func encodeWeatherRealtimeCursor(row *model.MallWeatherRealtime, page int) (string, error) {
	if row == nil || row.ID == 0 || row.SnapshotAtUTC.IsZero() {
		return "", fmt.Errorf("mall weather query: invalid realtime cursor row")
	}
	payload, err := json.Marshal(weatherRealtimeCursor{Version: 1, SnapshotUnixMS: row.SnapshotAtUTC.UTC().UnixMilli(), ID: row.ID, Page: page})
	if err != nil {
		return "", fmt.Errorf("mall weather query: encode realtime cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func weatherRealtimeCursorPage(cursor *weatherRealtimeCursor) int {
	if cursor == nil {
		return 0
	}
	return cursor.Page
}

func decodeWeatherRealtimeCursor(value string) (*weatherRealtimeCursor, error) {
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
	var cursor weatherRealtimeCursor
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cursor); err != nil || cursor.Version != 1 || cursor.ID == 0 || invalidOpenCursorPage(cursor.Page) ||
		cursor.SnapshotUnixMS < minWeatherCursorUnixMS || cursor.SnapshotUnixMS >= maxWeatherCursorUnixMS {
		return nil, fmt.Errorf("%w: invalid cursor", ErrMallWeatherInvalidQuery)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: invalid cursor", ErrMallWeatherInvalidQuery)
	}
	return &cursor, nil
}
