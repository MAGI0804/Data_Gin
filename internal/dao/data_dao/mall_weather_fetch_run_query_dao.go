package data_dao

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gin-biz-web-api/model"
)

type FetchRunQuery struct {
	MallID         uint
	StartUTC       time.Time
	EndUTC         time.Time
	CorrelationID  string
	TaskKind       string
	EndpointKind   string
	Status         string
	AfterCreatedAt *time.Time
	AfterID        uint
	Limit          int
}

func (dao *MallWeatherDAO) QueryFetchRuns(ctx context.Context, query FetchRunQuery) ([]model.MallWeatherFetchRun, error) {
	if dao == nil || dao.db == nil || ctx == nil {
		return nil, fmt.Errorf("mall weather: invalid fetch run query")
	}
	statement, args, err := buildFetchRunQuery(query)
	if err != nil {
		return nil, err
	}
	var rows []model.MallWeatherFetchRun
	if err := dao.db.WithContext(ctx).Raw(statement, args...).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("mall weather: query fetch runs: %w", err)
	}
	return rows, nil
}

func buildFetchRunQuery(query FetchRunQuery) (string, []interface{}, error) {
	if query.MallID == 0 {
		return "", nil, fmt.Errorf("mall weather: mall id is required")
	}
	if query.StartUTC.IsZero() || query.EndUTC.IsZero() || !query.StartUTC.Before(query.EndUTC) {
		return "", nil, fmt.Errorf("mall weather: invalid fetch run time range")
	}
	if (query.AfterCreatedAt == nil && query.AfterID != 0) || (query.AfterCreatedAt != nil && query.AfterID == 0) {
		return "", nil, fmt.Errorf("mall weather: incomplete fetch run cursor")
	}

	where := []string{
		"r.mall_id = ?",
		"r.created_at >= ?",
		"r.created_at < ?",
	}
	args := []interface{}{query.MallID, query.StartUTC.UTC(), query.EndUTC.UTC()}
	if query.CorrelationID != "" {
		where = append(where, "r.task_window = ?")
		args = append(args, query.CorrelationID)
	}
	if query.TaskKind != "" {
		where = append(where, "r.task_kind = ?")
		args = append(args, query.TaskKind)
	}
	if query.EndpointKind != "" {
		where = append(where, "r.endpoint_kind = ?")
		args = append(args, query.EndpointKind)
	}
	if query.Status != "" {
		where = append(where, "r.status = ?")
		args = append(args, query.Status)
	}
	if query.AfterCreatedAt != nil {
		where = append(where, "(r.created_at < ? OR (r.created_at = ? AND r.id < ?))")
		cursor := query.AfterCreatedAt.UTC()
		args = append(args, cursor, cursor, query.AfterID)
	}
	limit := query.Limit
	if limit <= 0 {
		limit = 200
	} else if limit > maxWeatherPageSize {
		limit = maxWeatherPageSize
	}
	args = append(args, limit)

	return `SELECT r.*
FROM mall_weather_fetch_runs AS r
WHERE ` + strings.Join(where, " AND ") + `
ORDER BY r.created_at DESC, r.id DESC
LIMIT ?`, args, nil
}
