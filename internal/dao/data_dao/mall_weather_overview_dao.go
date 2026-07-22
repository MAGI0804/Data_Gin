package data_dao

import (
	"context"
	"fmt"
	"time"

	"gin-biz-web-api/model"
)

const (
	maxWeatherOverviewMinutely = 120
	maxWeatherOverviewAlerts   = 20
)

func (dao *MallWeatherDAO) FindOverviewRealtime(ctx context.Context, mallID uint) (*model.MallWeatherRealtime, error) {
	if dao == nil || dao.db == nil || ctx == nil || mallID == 0 {
		return nil, fmt.Errorf("mall weather: invalid overview realtime query")
	}
	var row model.MallWeatherRealtime
	err := dao.db.WithContext(ctx).
		Table("mall_weather_latest AS latest").
		Select("weather.*").
		Joins("INNER JOIN mall_weather_realtime AS weather ON weather.id = latest.source_row_id AND weather.mall_id = latest.mall_id").
		Where("latest.mall_id = ? AND latest.data_kind = ?", mallID, model.MallWeatherDataKindRealtime).
		Order("latest.fetched_at_utc DESC").
		Order("latest.id DESC").
		Limit(1).
		Scan(&row).Error
	if err != nil {
		return nil, fmt.Errorf("mall weather: query overview realtime: %w", err)
	}
	if row.ID == 0 {
		return nil, ErrMallWeatherLatestNotFound
	}
	return &row, nil
}

func (dao *MallWeatherDAO) ListOverviewMinutely(ctx context.Context, mallID uint, startUTC, endUTC time.Time, limit int) ([]model.MallWeatherMinutely, error) {
	if dao == nil || dao.db == nil || ctx == nil || mallID == 0 || startUTC.IsZero() || endUTC.IsZero() || !startUTC.Before(endUTC) {
		return nil, fmt.Errorf("mall weather: invalid overview minutely query")
	}
	if limit <= 0 || limit > maxWeatherOverviewMinutely {
		limit = maxWeatherOverviewMinutely
	}
	var rows []model.MallWeatherMinutely
	err := dao.db.WithContext(ctx).
		Table("mall_weather_latest AS latest").
		Select("weather.*").
		Joins("INNER JOIN mall_weather_minutely AS weather ON weather.id = latest.source_row_id AND weather.mall_id = latest.mall_id").
		Where("latest.mall_id = ? AND latest.data_kind = ?", mallID, model.MallWeatherDataKindMinutely).
		Where("latest.business_time >= ? AND latest.business_time < ?", startUTC.UTC(), endUTC.UTC()).
		Order("latest.business_time ASC").
		Order("latest.id ASC").
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("mall weather: query overview minutely: %w", err)
	}
	return rows, nil
}

func (dao *MallWeatherDAO) ListOverviewAlerts(ctx context.Context, mallID uint, limit int) ([]model.MallWeatherAlert, error) {
	if dao == nil || dao.db == nil || ctx == nil || mallID == 0 {
		return nil, fmt.Errorf("mall weather: invalid overview alert query")
	}
	if limit <= 0 || limit > maxWeatherOverviewAlerts {
		limit = maxWeatherOverviewAlerts
	}
	var rows []model.MallWeatherAlert
	err := dao.db.WithContext(ctx).
		Table("mall_weather_alert_relations AS relation").
		Select("alert.*").
		Joins("INNER JOIN mall_weather_alerts AS alert ON alert.id = relation.alert_pk").
		Where("relation.mall_id = ? AND relation.is_active = ?", mallID, true).
		Where("alert.ended_at IS NULL").
		Order("alert.published_at_utc DESC").
		Order("alert.id DESC").
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("mall weather: query overview alerts: %w", err)
	}
	return rows, nil
}
