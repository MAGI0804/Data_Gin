package data_dao

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gin-biz-web-api/model"
)

type AlertQuery struct {
	MallID        uint
	StartUTC      time.Time
	EndUTC        time.Time
	AsOfUTC       *time.Time
	Latest        bool
	QualityStatus string
	AfterSortTime *time.Time
	AfterID       uint
	Limit         int
}

func (dao *MallWeatherDAO) QueryAlerts(ctx context.Context, query AlertQuery) ([]model.MallWeatherAlert, error) {
	if dao == nil || dao.db == nil || ctx == nil {
		return nil, fmt.Errorf("mall weather: invalid alert query")
	}
	statement, args, err := buildAlertQuery(query)
	if err != nil {
		return nil, err
	}
	var rows []model.MallWeatherAlert
	if err := dao.db.WithContext(ctx).Raw(statement, args...).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("mall weather: query alerts: %w", err)
	}
	return rows, nil
}

func buildAlertQuery(query AlertQuery) (string, []interface{}, error) {
	if query.MallID == 0 || query.StartUTC.IsZero() || query.EndUTC.IsZero() || !query.StartUTC.Before(query.EndUTC) {
		return "", nil, fmt.Errorf("mall weather: invalid alert query")
	}
	if (query.AfterSortTime == nil) != (query.AfterID == 0) {
		return "", nil, fmt.Errorf("mall weather: incomplete alert cursor")
	}

	sortTime := "COALESCE(alert.published_at_utc, alert.first_seen_at)"
	where := []string{
		"relation.mall_id = ?",
		sortTime + " >= ?",
		sortTime + " < ?",
	}
	args := []interface{}{query.MallID, query.StartUTC.UTC(), query.EndUTC.UTC()}
	if query.AsOfUTC != nil {
		where = append(where,
			"relation.first_seen_at <= ?",
			sortTime+" <= ?",
			"(alert.ended_at IS NULL OR alert.ended_at > ?)",
		)
		args = append(args, query.AsOfUTC.UTC(), query.AsOfUTC.UTC(), query.AsOfUTC.UTC())
	} else if query.Latest {
		where = append(where, "relation.is_active = ?", "alert.ended_at IS NULL")
		args = append(args, true)
	}
	if query.QualityStatus != "" {
		where = append(where, "alert.quality_status = ?")
		args = append(args, query.QualityStatus)
	}
	if query.AfterSortTime != nil {
		where = append(where, "("+sortTime+" < ? OR ("+sortTime+" = ? AND alert.id < ?))")
		cursor := query.AfterSortTime.UTC()
		args = append(args, cursor, cursor, query.AfterID)
	}
	limit := query.Limit
	if limit <= 0 {
		limit = 200
	} else if limit > maxWeatherPageSize {
		limit = maxWeatherPageSize
	}
	args = append(args, limit)

	return `SELECT alert.*
FROM mall_weather_alert_relations AS relation
INNER JOIN mall_weather_alerts AS alert ON alert.id = relation.alert_pk
WHERE ` + strings.Join(where, " AND ") + `
ORDER BY ` + sortTime + ` DESC, alert.id DESC
LIMIT ?`, args, nil
}
