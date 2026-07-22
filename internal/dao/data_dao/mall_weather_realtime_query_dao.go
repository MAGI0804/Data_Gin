package data_dao

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gin-biz-web-api/model"
)

type RealtimeQuery struct {
	MallID        uint
	StartUTC      time.Time
	EndUTC        time.Time
	AsOfUTC       *time.Time
	QualityStatus string
	AfterSnapshot *time.Time
	AfterID       uint
	Limit         int
}

func (dao *MallWeatherDAO) QueryRealtime(ctx context.Context, query RealtimeQuery) ([]model.MallWeatherRealtime, error) {
	if dao == nil || dao.db == nil || ctx == nil {
		return nil, fmt.Errorf("mall weather: invalid realtime query")
	}
	statement, args, err := buildRealtimeQuery(query)
	if err != nil {
		return nil, err
	}
	var rows []model.MallWeatherRealtime
	if err := dao.db.WithContext(ctx).Raw(statement, args...).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("mall weather: query realtime: %w", err)
	}
	return rows, nil
}

func buildRealtimeQuery(query RealtimeQuery) (string, []interface{}, error) {
	if query.MallID == 0 {
		return "", nil, fmt.Errorf("mall weather: mall id is required")
	}
	if query.StartUTC.IsZero() || query.EndUTC.IsZero() || !query.StartUTC.Before(query.EndUTC) {
		return "", nil, fmt.Errorf("mall weather: invalid realtime time range")
	}
	if (query.AfterSnapshot == nil) != (query.AfterID == 0) {
		return "", nil, fmt.Errorf("mall weather: incomplete realtime cursor")
	}

	where := []string{
		"w.mall_id = ?",
		"w.snapshot_at_utc >= ?",
		"w.snapshot_at_utc < ?",
	}
	args := []interface{}{query.MallID, query.StartUTC.UTC(), query.EndUTC.UTC()}
	if query.AsOfUTC != nil {
		where = append(where, "w.fetched_at_utc <= ?")
		args = append(args, query.AsOfUTC.UTC())
	}
	if query.QualityStatus != "" {
		where = append(where, "w.quality_status = ?")
		args = append(args, query.QualityStatus)
	}
	if query.AfterSnapshot != nil {
		where = append(where, "(w.snapshot_at_utc > ? OR (w.snapshot_at_utc = ? AND w.id > ?))")
		cursor := query.AfterSnapshot.UTC()
		args = append(args, cursor, cursor, query.AfterID)
	}
	limit := query.Limit
	if limit <= 0 {
		limit = 200
	} else if limit > maxWeatherPageSize {
		limit = maxWeatherPageSize
	}
	args = append(args, limit)

	return `SELECT w.*
FROM mall_weather_realtime AS w
WHERE ` + strings.Join(where, " AND ") + `
ORDER BY w.snapshot_at_utc ASC, w.id ASC
LIMIT ?`, args, nil
}
