package data_dao

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gin-biz-web-api/model"
)

type MinutelyQuery struct {
	MallID              uint
	StartUTC            time.Time
	EndUTC              time.Time
	AsOfUTC             *time.Time
	Latest              bool
	QualityStatus       string
	AfterForecastMinute *time.Time
	AfterIssuedAtUTC    *time.Time
	AfterID             uint
	Limit               int
}

func (dao *MallWeatherDAO) QueryMinutely(ctx context.Context, query MinutelyQuery) ([]model.MallWeatherMinutely, error) {
	if dao == nil || dao.db == nil || ctx == nil {
		return nil, fmt.Errorf("mall weather: invalid minutely query")
	}
	statement, args, err := buildMinutelyQuery(query)
	if err != nil {
		return nil, err
	}
	var rows []model.MallWeatherMinutely
	if err := dao.db.WithContext(ctx).Raw(statement, args...).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("mall weather: query minutely: %w", err)
	}
	return rows, nil
}

func buildMinutelyQuery(query MinutelyQuery) (string, []interface{}, error) {
	if query.MallID == 0 {
		return "", nil, fmt.Errorf("mall weather: mall id is required")
	}
	if query.StartUTC.IsZero() || query.EndUTC.IsZero() || !query.StartUTC.Before(query.EndUTC) {
		return "", nil, fmt.Errorf("mall weather: invalid minutely time range")
	}
	if (query.AfterForecastMinute == nil && (query.AfterIssuedAtUTC != nil || query.AfterID != 0)) ||
		(query.AfterForecastMinute != nil && query.AfterID == 0) {
		return "", nil, fmt.Errorf("mall weather: incomplete minutely cursor")
	}

	where := []string{
		"w.mall_id = ?",
		"w.forecast_minute_utc >= ?",
		"w.forecast_minute_utc < ?",
	}
	args := []interface{}{query.MallID, query.StartUTC.UTC(), query.EndUTC.UTC()}
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
		if query.AfterForecastMinute != nil {
			outerWhere = append(outerWhere, "(ranked.forecast_minute_utc > ? OR (ranked.forecast_minute_utc = ? AND ranked.id > ?))")
			cursor := query.AfterForecastMinute.UTC()
			args = append(args[:len(args)-1], cursor, cursor, query.AfterID, limit)
		}
		return `SELECT ranked.* FROM (
SELECT w.*, ROW_NUMBER() OVER (
  PARTITION BY w.forecast_minute_utc
  ORDER BY w.issued_at_utc DESC, w.id DESC
) AS version_rank
FROM mall_weather_minutely AS w
WHERE ` + strings.Join(where, " AND ") + `
) AS ranked
WHERE ` + strings.Join(outerWhere, " AND ") + `
ORDER BY ranked.forecast_minute_utc ASC, ranked.id ASC
LIMIT ?`, args, nil
	}
	if query.AfterForecastMinute != nil {
		if query.AfterIssuedAtUTC == nil {
			return "", nil, fmt.Errorf("mall weather: issued-at cursor is required for minutely version history")
		}
		where = append(where, `(w.forecast_minute_utc > ?
OR (w.forecast_minute_utc = ? AND w.issued_at_utc < ?)
OR (w.forecast_minute_utc = ? AND w.issued_at_utc = ? AND w.id > ?))`)
		cursor := query.AfterForecastMinute.UTC()
		issuedCursor := query.AfterIssuedAtUTC.UTC()
		args = append(args[:len(args)-1], cursor, cursor, issuedCursor, cursor, issuedCursor, query.AfterID, limit)
	}

	return `SELECT w.*
FROM mall_weather_minutely AS w
WHERE ` + strings.Join(where, " AND ") + `
ORDER BY w.forecast_minute_utc ASC, w.issued_at_utc DESC, w.id ASC
LIMIT ?`, args, nil
}
