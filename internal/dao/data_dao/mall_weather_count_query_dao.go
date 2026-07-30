package data_dao

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type weatherCountFilter struct {
	Table          string
	RangeColumn    string
	Start          interface{}
	End            interface{}
	CutoffColumn   string
	Cutoff         interface{}
	QualityStatus  string
	ExtraWhere     []string
	ExtraArgs      []interface{}
	DistinctFields string
}

func (dao *MallWeatherDAO) CountRealtime(ctx context.Context, query RealtimeQuery) (int64, error) {
	query.AfterSnapshot, query.AfterID, query.Limit = nil, 0, 1
	_, _, err := buildRealtimeQuery(query)
	statement, args := buildFilteredWeatherCount(query.MallID, weatherCountFilter{
		Table: "mall_weather_realtime", RangeColumn: "snapshot_at_utc",
		Start: query.StartUTC.UTC(), End: query.EndUTC.UTC(),
		CutoffColumn: "fetched_at_utc", Cutoff: utcCountCutoff(query.AsOfUTC),
		QualityStatus: query.QualityStatus,
	})
	return dao.countWeatherQuery(ctx, "realtime", statement, args, err)
}

func (dao *MallWeatherDAO) CountMinutely(ctx context.Context, query MinutelyQuery) (int64, error) {
	query.AfterForecastMinute, query.AfterIssuedAtUTC, query.AfterID, query.Limit = nil, nil, 0, 1
	_, _, err := buildMinutelyQuery(query)
	distinct := ""
	if query.Latest || query.AsOfUTC != nil {
		distinct = "w.forecast_minute_utc"
	}
	statement, args := buildFilteredWeatherCount(query.MallID, weatherCountFilter{
		Table: "mall_weather_minutely", RangeColumn: "forecast_minute_utc",
		Start: query.StartUTC.UTC(), End: query.EndUTC.UTC(),
		CutoffColumn: "issued_at_utc", Cutoff: utcCountCutoff(query.AsOfUTC),
		QualityStatus: query.QualityStatus, DistinctFields: distinct,
	})
	return dao.countWeatherQuery(ctx, "minutely", statement, args, err)
}

func (dao *MallWeatherDAO) CountHourly(ctx context.Context, query HourlyQuery) (int64, error) {
	query.AfterForecastTime, query.AfterIssuedAtUTC, query.AfterID, query.Limit = nil, nil, 0, 1
	_, _, err := buildHourlyQuery(query)
	distinct := ""
	if query.Latest || query.AsOfUTC != nil {
		distinct = "w.forecast_time_utc"
	}
	statement, args := buildFilteredWeatherCount(query.MallID, weatherCountFilter{
		Table: "mall_weather_hourly", RangeColumn: "forecast_time_utc",
		Start: query.StartUTC.UTC(), End: query.EndUTC.UTC(),
		CutoffColumn: "issued_at_utc", Cutoff: utcCountCutoff(query.AsOfUTC),
		QualityStatus: query.QualityStatus, DistinctFields: distinct,
	})
	return dao.countWeatherQuery(ctx, "hourly", statement, args, err)
}

func (dao *MallWeatherDAO) CountDaily(ctx context.Context, query DailyQuery) (int64, error) {
	query.AfterForecastDateLocal, query.AfterIssuedAtUTC, query.AfterID, query.Limit = nil, nil, 0, 1
	_, _, err := buildDailyQuery(query)
	distinct := ""
	if query.Latest || query.AsOfUTC != nil {
		distinct = "w.forecast_date_local"
	}
	statement, args := buildFilteredWeatherCount(query.MallID, weatherCountFilter{
		Table: "mall_weather_daily", RangeColumn: "forecast_date_local",
		Start: query.StartLocal.Format(dailyQueryLocalLayout), End: query.EndLocal.Format(dailyQueryLocalLayout),
		CutoffColumn: "issued_at_utc", Cutoff: utcCountCutoff(query.AsOfUTC),
		QualityStatus: query.QualityStatus, DistinctFields: distinct,
	})
	return dao.countWeatherQuery(ctx, "daily", statement, args, err)
}

func (dao *MallWeatherDAO) CountAlerts(ctx context.Context, query AlertQuery) (int64, error) {
	query.AfterSortTime, query.AfterID, query.Limit = nil, 0, 1
	_, _, err := buildAlertQuery(query)
	statement, args := buildAlertCountQuery(query)
	return dao.countWeatherQuery(ctx, "alerts", statement, args, err)
}

func (dao *MallWeatherDAO) CountLifeIndices(ctx context.Context, query LifeIndexQuery) (int64, error) {
	query.AfterForecastDateLocal, query.AfterSourceAPI, query.AfterIndexType = nil, "", 0
	query.AfterIssuedAtUTC, query.AfterID, query.Limit = nil, 0, 1
	_, _, err := buildLifeIndexQuery(query)
	distinct := ""
	if query.Latest || query.AsOfUTC != nil {
		distinct = "w.forecast_date_local, w.source_api, w.index_type"
	}
	extraWhere := []string(nil)
	extraArgs := []interface{}(nil)
	if query.SourceAPI != "" {
		extraWhere = append(extraWhere, "w.source_api = ?")
		extraArgs = append(extraArgs, query.SourceAPI)
	}
	statement, args := buildFilteredWeatherCount(query.MallID, weatherCountFilter{
		Table: "mall_weather_life_indices", RangeColumn: "forecast_date_local",
		Start: query.StartLocal.Format(dailyQueryLocalLayout), End: query.EndLocal.Format(dailyQueryLocalLayout),
		CutoffColumn: "issued_at_utc", Cutoff: utcCountCutoff(query.AsOfUTC),
		QualityStatus: query.QualityStatus, ExtraWhere: extraWhere, ExtraArgs: extraArgs,
		DistinctFields: distinct,
	})
	return dao.countWeatherQuery(ctx, "life indices", statement, args, err)
}

func utcCountCutoff(cutoff *time.Time) interface{} {
	if cutoff == nil {
		return nil
	}
	return cutoff.UTC()
}

func buildFilteredWeatherCount(mallID uint, filter weatherCountFilter) (string, []interface{}) {
	where := []string{
		"w.mall_id = ?",
		"w." + filter.RangeColumn + " >= ?",
		"w." + filter.RangeColumn + " < ?",
	}
	args := []interface{}{mallID, filter.Start, filter.End}
	if filter.Cutoff != nil {
		where = append(where, "w."+filter.CutoffColumn+" <= ?")
		args = append(args, filter.Cutoff)
	}
	if filter.QualityStatus != "" {
		where = append(where, "w.quality_status = ?")
		args = append(args, filter.QualityStatus)
	}
	where = append(where, filter.ExtraWhere...)
	args = append(args, filter.ExtraArgs...)
	countExpression := "*"
	if filter.DistinctFields != "" {
		countExpression = "DISTINCT " + filter.DistinctFields
	}
	return "SELECT COUNT(" + countExpression + ") FROM " + filter.Table + " AS w\nWHERE " + strings.Join(where, " AND "), args
}

func buildAlertCountQuery(query AlertQuery) (string, []interface{}) {
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
	return `SELECT COUNT(*)
FROM mall_weather_alert_relations AS relation
INNER JOIN mall_weather_alerts AS alert ON alert.id = relation.alert_pk
WHERE ` + strings.Join(where, " AND "), args
}

func (dao *MallWeatherDAO) countWeatherQuery(
	ctx context.Context,
	kind string,
	statement string,
	args []interface{},
	buildErr error,
) (int64, error) {
	if buildErr != nil {
		return 0, buildErr
	}
	if dao == nil || dao.db == nil || ctx == nil {
		return 0, fmt.Errorf("mall weather: invalid %s count query", kind)
	}
	var total int64
	if err := dao.db.WithContext(ctx).Raw(statement, args...).Scan(&total).Error; err != nil {
		return 0, fmt.Errorf("mall weather: count %s: %w", kind, err)
	}
	return total, nil
}
