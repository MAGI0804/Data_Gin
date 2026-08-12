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
	weatherdomain "gin-biz-web-api/internal/weather"
	"gin-biz-web-api/model"
)

type MallWeatherLifeIndexDTO struct {
	SourceAPI         string                  `json:"sourceApi"`
	ForecastDateLocal string                  `json:"forecastDateLocal"`
	IndexType         int                     `json:"indexType"`
	IndexCode         string                  `json:"indexCode"`
	IndexName         string                  `json:"indexName,omitempty"`
	Level             *int                    `json:"level,omitempty"`
	ShortDescription  string                  `json:"shortDescription,omitempty"`
	Detail            string                  `json:"detail,omitempty"`
	IsUnknownType     bool                    `json:"isUnknownType"`
	IssuedAtUTC       time.Time               `json:"issuedAtUtc"`
	IssuedAtLocal     string                  `json:"issuedAtLocal"`
	FetchedAtUTC      time.Time               `json:"fetchedAtUtc"`
	FetchedAtLocal    string                  `json:"fetchedAtLocal"`
	QualityStatus     string                  `json:"qualityStatus"`
	QualityWarnings   []MallWeatherWarningDTO `json:"qualityWarnings"`
}

type MallWeatherLifeIndexResult struct {
	Items      []MallWeatherLifeIndexDTO `json:"items"`
	Meta       MallWeatherQueryMeta      `json:"meta"`
	Pagination MallWeatherPagination     `json:"pagination"`
}

type weatherLifeIndexCursor struct {
	Version           int    `json:"v"`
	ForecastDateLocal string `json:"forecastDateLocal"`
	SourceAPI         string `json:"sourceApi"`
	IndexType         int    `json:"indexType"`
	IssuedAtUnixMS    int64  `json:"issuedAtUnixMs"`
	ID                uint   `json:"id"`
	Page              int    `json:"page,omitempty"`
}

func (service *MallWeatherQueryService) LifeIndices(ctx context.Context, actorUserID, mallID uint, request requestbody.MallWeatherLifeIndexQueryRequest) (*MallWeatherLifeIndexResult, error) {
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
	location, normalized, err := normalizeLifeIndexWeatherRequest(request, mall)
	if err != nil {
		return nil, err
	}
	cursor, err := decodeWeatherLifeIndexCursor(normalized.Cursor)
	if err != nil {
		return nil, err
	}
	page := 1
	if cursor != nil {
		page = openCursorPage(cursor.Page, true)
	}
	query := data_dao.LifeIndexQuery{
		MallID: mallID, SourceAPI: weatherdomain.SourceAPIV26Daily,
		StartLocal: weatherLocalDate(normalized.StartUTC, location), EndLocal: weatherLocalDate(normalized.EndUTC, location),
		AsOfUTC: normalized.AsOfUTC, Latest: normalized.Latest, QualityStatus: normalized.QualityStatus,
		Limit: normalized.PageSize + 1,
	}
	if cursor != nil {
		forecastDate, err := time.Parse("2006-01-02", cursor.ForecastDateLocal)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid cursor", ErrMallWeatherInvalidQuery)
		}
		issuedAt := time.UnixMilli(cursor.IssuedAtUnixMS).UTC()
		query.AfterForecastDateLocal = &forecastDate
		query.AfterSourceAPI = cursor.SourceAPI
		query.AfterIndexType = cursor.IndexType
		query.AfterIssuedAtUTC = &issuedAt
		query.AfterID = cursor.ID
	}
	rows, err := service.weather.QueryLifeIndices(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("mall weather query: life index rows: %w", err)
	}
	hasMore := len(rows) > normalized.PageSize
	if hasMore {
		rows = rows[:normalized.PageSize]
	}
	items := make([]MallWeatherLifeIndexDTO, len(rows))
	for index := range rows {
		items[index], err = lifeIndexWeatherDTO(&rows[index], location)
		if err != nil {
			return nil, err
		}
	}
	meta := weatherQueryMeta(mall, location, model.MallWeatherFreshnessFresh, nil)
	meta.APIVersion = "v2.6"
	result := &MallWeatherLifeIndexResult{Items: items, Meta: meta, Pagination: MallWeatherPagination{PageSize: normalized.PageSize}}
	if normalized.IncludeTotals {
		totalItems, err := service.weather.CountLifeIndices(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("mall weather query: count life index rows: %w", err)
		}
		applyOpenWeatherPagination(&result.Pagination, page, totalItems)
	}
	latest, err := service.weather.FindCurrentLatestLifeSource(ctx, mallID, weatherdomain.SourceAPIV26Daily)
	if err != nil && !errors.Is(err, data_dao.ErrMallWeatherLatestNotFound) {
		return nil, fmt.Errorf("mall weather query: life index freshness: %w", err)
	}
	if latest == nil {
		result.Meta.FreshnessStatus = "UNAVAILABLE"
	} else {
		status, age, err := currentWeatherFreshness(model.MallWeatherDataKindLife, latest, service.now().UTC())
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
		result.Pagination.NextCursor, err = encodeWeatherLifeIndexCursor(&rows[len(rows)-1], nextPage)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func normalizeLifeIndexWeatherRequest(request requestbody.MallWeatherLifeIndexQueryRequest, mall *model.Mall) (*time.Location, requestbody.MallWeatherLifeIndexQueryRequest, error) {
	location, normalized, err := normalizeWeatherTimeSeriesRequest(weatherTimeSeriesRequest{
		StartUTC: request.StartUTC, EndUTC: request.EndUTC, TimeZone: request.TimeZone,
		Latest: request.Latest, AsOfUTC: request.AsOfUTC, QualityStatus: request.QualityStatus,
		Cursor: request.Cursor, PageSize: request.PageSize,
	}, mall)
	if err != nil {
		return nil, request, err
	}
	if !weatherLocalDate(normalized.StartUTC, location).Before(weatherLocalDate(normalized.EndUTC, location)) {
		return nil, request, fmt.Errorf("%w: life index range must span at least one local date", ErrMallWeatherInvalidQuery)
	}
	request.StartUTC, request.EndUTC, request.TimeZone = normalized.StartUTC, normalized.EndUTC, normalized.TimeZone
	request.Latest, request.AsOfUTC, request.QualityStatus = normalized.Latest, normalized.AsOfUTC, normalized.QualityStatus
	request.Cursor, request.PageSize = normalized.Cursor, normalized.PageSize
	return location, request, nil
}

func lifeIndexWeatherDTO(row *model.MallWeatherLifeIndex, location *time.Location) (MallWeatherLifeIndexDTO, error) {
	if row == nil || row.ID == 0 || row.ForecastDateLocal.IsZero() || row.IssuedAtUTC.IsZero() || row.FetchedAtUTC.IsZero() ||
		row.SourceAPI == "" || row.IndexType < 0 || row.IndexCode == "" || location == nil {
		return MallWeatherLifeIndexDTO{}, fmt.Errorf("mall weather query: invalid life index row")
	}
	warnings, err := weatherQualityWarnings(row.QualityFlagsJSON)
	if err != nil {
		return MallWeatherLifeIndexDTO{}, err
	}
	return MallWeatherLifeIndexDTO{
		SourceAPI: row.SourceAPI, ForecastDateLocal: row.ForecastDateLocal.Format("2006-01-02"),
		IndexType: row.IndexType, IndexCode: row.IndexCode, IndexName: row.IndexName, Level: row.Level,
		ShortDescription: row.ShortDesc, Detail: row.Detail, IsUnknownType: row.IsUnknownType,
		IssuedAtUTC: row.IssuedAtUTC.UTC(), IssuedAtLocal: formatWeatherLocalTime(row.IssuedAtUTC, location),
		FetchedAtUTC: row.FetchedAtUTC.UTC(), FetchedAtLocal: formatWeatherLocalTime(row.FetchedAtUTC, location),
		QualityStatus: strings.ToUpper(row.QualityStatus), QualityWarnings: warnings,
	}, nil
}

func encodeWeatherLifeIndexCursor(row *model.MallWeatherLifeIndex, page int) (string, error) {
	if row == nil || row.ID == 0 || row.ForecastDateLocal.IsZero() || row.SourceAPI == "" || row.IndexType < 0 || row.IssuedAtUTC.IsZero() {
		return "", fmt.Errorf("mall weather query: invalid life index cursor row")
	}
	payload, err := json.Marshal(weatherLifeIndexCursor{
		Version: 1, ForecastDateLocal: row.ForecastDateLocal.Format("2006-01-02"), SourceAPI: row.SourceAPI,
		IndexType: row.IndexType, IssuedAtUnixMS: row.IssuedAtUTC.UTC().UnixMilli(), ID: row.ID, Page: page,
	})
	if err != nil {
		return "", fmt.Errorf("mall weather query: encode life index cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeWeatherLifeIndexCursor(value string) (*weatherLifeIndexCursor, error) {
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
	var cursor weatherLifeIndexCursor
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cursor); err != nil || cursor.Version != 1 || cursor.ID == 0 || cursor.IndexType < 0 || invalidOpenCursorPage(cursor.Page) ||
		cursor.SourceAPI != weatherdomain.SourceAPIV26Daily ||
		cursor.IssuedAtUnixMS < minWeatherCursorUnixMS || cursor.IssuedAtUnixMS >= maxWeatherCursorUnixMS {
		return nil, fmt.Errorf("%w: invalid cursor", ErrMallWeatherInvalidQuery)
	}
	date, err := time.Parse("2006-01-02", cursor.ForecastDateLocal)
	if err != nil || date.Format("2006-01-02") != cursor.ForecastDateLocal {
		return nil, fmt.Errorf("%w: invalid cursor", ErrMallWeatherInvalidQuery)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: invalid cursor", ErrMallWeatherInvalidQuery)
	}
	return &cursor, nil
}
