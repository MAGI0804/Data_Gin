package data_dao

import (
	"context"
	"fmt"
	"strings"
)

func (dao *MallWeatherDAO) CountRealtime(ctx context.Context, query RealtimeQuery) (int64, error) {
	query.AfterSnapshot, query.AfterID, query.Limit = nil, 0, 1
	statement, args, err := buildRealtimeQuery(query)
	return dao.countBuiltWeatherQuery(ctx, "realtime", statement, args, err)
}

func (dao *MallWeatherDAO) CountMinutely(ctx context.Context, query MinutelyQuery) (int64, error) {
	query.AfterForecastMinute, query.AfterIssuedAtUTC, query.AfterID, query.Limit = nil, nil, 0, 1
	statement, args, err := buildMinutelyQuery(query)
	return dao.countBuiltWeatherQuery(ctx, "minutely", statement, args, err)
}

func (dao *MallWeatherDAO) CountHourly(ctx context.Context, query HourlyQuery) (int64, error) {
	query.AfterForecastTime, query.AfterIssuedAtUTC, query.AfterID, query.Limit = nil, nil, 0, 1
	statement, args, err := buildHourlyQuery(query)
	return dao.countBuiltWeatherQuery(ctx, "hourly", statement, args, err)
}

func (dao *MallWeatherDAO) CountDaily(ctx context.Context, query DailyQuery) (int64, error) {
	query.AfterForecastDateLocal, query.AfterIssuedAtUTC, query.AfterID, query.Limit = nil, nil, 0, 1
	statement, args, err := buildDailyQuery(query)
	return dao.countBuiltWeatherQuery(ctx, "daily", statement, args, err)
}

func (dao *MallWeatherDAO) CountAlerts(ctx context.Context, query AlertQuery) (int64, error) {
	query.AfterSortTime, query.AfterID, query.Limit = nil, 0, 1
	statement, args, err := buildAlertQuery(query)
	return dao.countBuiltWeatherQuery(ctx, "alerts", statement, args, err)
}

func (dao *MallWeatherDAO) CountLifeIndices(ctx context.Context, query LifeIndexQuery) (int64, error) {
	query.AfterForecastDateLocal, query.AfterSourceAPI, query.AfterIndexType = nil, "", 0
	query.AfterIssuedAtUTC, query.AfterID, query.Limit = nil, 0, 1
	statement, args, err := buildLifeIndexQuery(query)
	return dao.countBuiltWeatherQuery(ctx, "life indices", statement, args, err)
}

func (dao *MallWeatherDAO) countBuiltWeatherQuery(
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
	countStatement, countArgs, err := buildWeatherCountStatement(statement, args)
	if err != nil {
		return 0, err
	}
	var total int64
	if err := dao.db.WithContext(ctx).Raw(countStatement, countArgs...).Scan(&total).Error; err != nil {
		return 0, fmt.Errorf("mall weather: count %s: %w", kind, err)
	}
	return total, nil
}

func buildWeatherCountStatement(statement string, args []interface{}) (string, []interface{}, error) {
	orderIndex := strings.LastIndex(statement, "\nORDER BY ")
	if orderIndex <= 0 || len(args) == 0 || !strings.HasSuffix(strings.TrimSpace(statement), "LIMIT ?") {
		return "", nil, fmt.Errorf("mall weather: invalid count source query")
	}
	baseStatement := strings.TrimSpace(statement[:orderIndex])
	return "SELECT COUNT(*) FROM (" + baseStatement + ") AS open_weather_count", args[:len(args)-1], nil
}
