package data_dao

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gin-biz-web-api/model"
)

const dailyQueryLocalLayout = "2006-01-02 15:04:05.000"

type DailyQuery struct {
	MallID                 uint
	StartLocal             time.Time
	EndLocal               time.Time
	AsOfUTC                *time.Time
	Latest                 bool
	QualityStatus          string
	AfterForecastDateLocal *time.Time
	AfterIssuedAtUTC       *time.Time
	AfterID                uint
	Limit                  int
}

func (dao *MallWeatherDAO) QueryDaily(ctx context.Context, query DailyQuery) ([]model.MallWeatherDaily, error) {
	if dao == nil || dao.db == nil || ctx == nil {
		return nil, fmt.Errorf("mall weather: invalid daily query")
	}
	statement, args, err := buildDailyQuery(query)
	if err != nil {
		return nil, err
	}
	var rows []model.MallWeatherDaily
	if err := dao.db.WithContext(ctx).Raw(statement, args...).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("mall weather: query daily: %w", err)
	}
	return rows, nil
}

func buildDailyQuery(query DailyQuery) (string, []interface{}, error) {
	if query.MallID == 0 {
		return "", nil, fmt.Errorf("mall weather: mall id is required")
	}
	if query.StartLocal.IsZero() || query.EndLocal.IsZero() || !query.StartLocal.Before(query.EndLocal) {
		return "", nil, fmt.Errorf("mall weather: invalid daily time range")
	}
	if (query.AfterForecastDateLocal == nil && (query.AfterIssuedAtUTC != nil || query.AfterID != 0)) ||
		(query.AfterForecastDateLocal != nil && query.AfterID == 0) {
		return "", nil, fmt.Errorf("mall weather: incomplete daily cursor")
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
		if query.AfterForecastDateLocal != nil {
			outerWhere = append(outerWhere, "(ranked.forecast_date_local > ? OR (ranked.forecast_date_local = ? AND ranked.id > ?))")
			cursor := query.AfterForecastDateLocal.Format("2006-01-02")
			args = append(args[:len(args)-1], cursor, cursor, query.AfterID, limit)
		}
		return `SELECT ranked.* FROM (
SELECT w.*, ROW_NUMBER() OVER (
  PARTITION BY w.forecast_date_local
  ORDER BY w.issued_at_utc DESC, w.id DESC
) AS version_rank
FROM mall_weather_daily AS w
WHERE ` + strings.Join(where, " AND ") + `
) AS ranked
WHERE ` + strings.Join(outerWhere, " AND ") + `
ORDER BY ranked.forecast_date_local ASC, ranked.id ASC
LIMIT ?`, args, nil
	}
	if query.AfterForecastDateLocal != nil {
		if query.AfterIssuedAtUTC == nil {
			return "", nil, fmt.Errorf("mall weather: issued-at cursor is required for daily version history")
		}
		where = append(where, `(w.forecast_date_local > ?
OR (w.forecast_date_local = ? AND w.issued_at_utc < ?)
OR (w.forecast_date_local = ? AND w.issued_at_utc = ? AND w.id > ?))`)
		cursor := query.AfterForecastDateLocal.Format("2006-01-02")
		issuedCursor := query.AfterIssuedAtUTC.UTC()
		args = append(args[:len(args)-1], cursor, cursor, issuedCursor, cursor, issuedCursor, query.AfterID, limit)
	}

	return `SELECT w.*
FROM mall_weather_daily AS w
WHERE ` + strings.Join(where, " AND ") + `
ORDER BY w.forecast_date_local ASC, w.issued_at_utc DESC, w.id ASC
LIMIT ?`, args, nil
}
