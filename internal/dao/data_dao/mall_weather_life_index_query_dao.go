package data_dao

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gin-biz-web-api/model"
)

type LifeIndexQuery struct {
	MallID                 uint
	SourceAPI              string
	StartLocal             time.Time
	EndLocal               time.Time
	AsOfUTC                *time.Time
	Latest                 bool
	QualityStatus          string
	AfterForecastDateLocal *time.Time
	AfterSourceAPI         string
	AfterIndexType         int
	AfterIssuedAtUTC       *time.Time
	AfterID                uint
	Limit                  int
}

func (dao *MallWeatherDAO) QueryLifeIndices(ctx context.Context, query LifeIndexQuery) ([]model.MallWeatherLifeIndex, error) {
	if dao == nil || dao.db == nil || ctx == nil {
		return nil, fmt.Errorf("mall weather: invalid life index query")
	}
	statement, args, err := buildLifeIndexQuery(query)
	if err != nil {
		return nil, err
	}
	var rows []model.MallWeatherLifeIndex
	if err := dao.db.WithContext(ctx).Raw(statement, args...).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("mall weather: query life indices: %w", err)
	}
	return rows, nil
}

func buildLifeIndexQuery(query LifeIndexQuery) (string, []interface{}, error) {
	if query.MallID == 0 {
		return "", nil, fmt.Errorf("mall weather: mall id is required")
	}
	if query.StartLocal.IsZero() || query.EndLocal.IsZero() || !query.StartLocal.Before(query.EndLocal) {
		return "", nil, fmt.Errorf("mall weather: invalid life index time range")
	}
	hasCursor := query.AfterForecastDateLocal != nil
	if (!hasCursor && (query.AfterSourceAPI != "" || query.AfterIndexType != 0 || query.AfterIssuedAtUTC != nil || query.AfterID != 0)) ||
		(hasCursor && (query.AfterSourceAPI == "" || query.AfterIndexType < 0 || query.AfterID == 0)) {
		return "", nil, fmt.Errorf("mall weather: incomplete life index cursor")
	}

	where := []string{
		"w.mall_id = ?",
		"w.forecast_date_local >= ?",
		"w.forecast_date_local < ?",
	}
	args := []interface{}{
		query.MallID,
		query.StartLocal.Format(dailyQueryLocalLayout),
		query.EndLocal.Format(dailyQueryLocalLayout),
	}
	if query.SourceAPI != "" {
		where = append(where, "w.source_api = ?")
		args = append(args, query.SourceAPI)
	}
	if query.AsOfUTC != nil {
		where = append(where, "w.issued_at_utc <= ?")
		args = append(args, query.AsOfUTC.UTC())
	}
	if query.QualityStatus != "" {
		where = append(where, "w.quality_status = ?")
		args = append(args, query.QualityStatus)
	}
	limit := query.Limit
	if limit <= 0 {
		limit = 200
	} else if limit > maxWeatherPageSize {
		limit = maxWeatherPageSize
	}
	args = append(args, limit)

	if query.Latest || query.AsOfUTC != nil {
		outerWhere := []string{"ranked.version_rank = 1"}
		if hasCursor {
			outerWhere = append(outerWhere, `(ranked.forecast_date_local > ?
OR (ranked.forecast_date_local = ? AND ranked.source_api > ?)
OR (ranked.forecast_date_local = ? AND ranked.source_api = ? AND ranked.index_type > ?)
OR (ranked.forecast_date_local = ? AND ranked.source_api = ? AND ranked.index_type = ? AND ranked.id > ?))`)
			date := query.AfterForecastDateLocal.Format("2006-01-02")
			args = append(args[:len(args)-1],
				date,
				date, query.AfterSourceAPI,
				date, query.AfterSourceAPI, query.AfterIndexType,
				date, query.AfterSourceAPI, query.AfterIndexType, query.AfterID,
				limit,
			)
		}
		return `SELECT ranked.* FROM (
SELECT w.*, ROW_NUMBER() OVER (
  PARTITION BY w.forecast_date_local, w.source_api, w.index_type
  ORDER BY w.issued_at_utc DESC, w.id DESC
) AS version_rank
FROM mall_weather_life_indices AS w
WHERE ` + strings.Join(where, " AND ") + `
) AS ranked
WHERE ` + strings.Join(outerWhere, " AND ") + `
ORDER BY ranked.forecast_date_local ASC, ranked.source_api ASC, ranked.index_type ASC, ranked.id ASC
LIMIT ?`, args, nil
	}
	if hasCursor {
		if query.AfterIssuedAtUTC == nil {
			return "", nil, fmt.Errorf("mall weather: issued-at cursor is required for life index version history")
		}
		where = append(where, `(w.forecast_date_local > ?
OR (w.forecast_date_local = ? AND w.source_api > ?)
OR (w.forecast_date_local = ? AND w.source_api = ? AND w.index_type > ?)
OR (w.forecast_date_local = ? AND w.source_api = ? AND w.index_type = ? AND w.issued_at_utc < ?)
OR (w.forecast_date_local = ? AND w.source_api = ? AND w.index_type = ? AND w.issued_at_utc = ? AND w.id > ?))`)
		date := query.AfterForecastDateLocal.Format("2006-01-02")
		issuedAt := query.AfterIssuedAtUTC.UTC()
		args = append(args[:len(args)-1],
			date,
			date, query.AfterSourceAPI,
			date, query.AfterSourceAPI, query.AfterIndexType,
			date, query.AfterSourceAPI, query.AfterIndexType, issuedAt,
			date, query.AfterSourceAPI, query.AfterIndexType, issuedAt, query.AfterID,
			limit,
		)
	}

	return `SELECT w.*
FROM mall_weather_life_indices AS w
WHERE ` + strings.Join(where, " AND ") + `
ORDER BY w.forecast_date_local ASC, w.source_api ASC, w.index_type ASC, w.issued_at_utc DESC, w.id ASC
LIMIT ?`, args, nil
}
