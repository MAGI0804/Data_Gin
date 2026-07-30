package data_dao

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type RealtimeDaySummaryQuery struct {
	MallID        uint
	StartUTC      time.Time
	EndUTC        time.Time
	QualityStatus string
}

type RealtimeDaySummary struct {
	SampleCount             int64      `gorm:"column:sample_count"`
	ObservedStartUTC        *time.Time `gorm:"column:observed_start_utc"`
	ObservedEndUTC          *time.Time `gorm:"column:observed_end_utc"`
	DominantSkycon          *string    `gorm:"column:dominant_skycon"`
	TemperatureMinC         *float64   `gorm:"column:temperature_min_c"`
	TemperatureMaxC         *float64   `gorm:"column:temperature_max_c"`
	TemperatureAvgC         *float64   `gorm:"column:temperature_avg_c"`
	ApparentTemperatureAvgC *float64   `gorm:"column:apparent_temperature_avg_c"`
	HumidityAvgRatio        *float64   `gorm:"column:humidity_avg_ratio"`
	PressureAvgPa           *float64   `gorm:"column:pressure_avg_pa"`
	WindSpeedAvgKPH         *float64   `gorm:"column:wind_speed_avg_kph"`
	WindSpeedMaxKPH         *float64   `gorm:"column:wind_speed_max_kph"`
	PrecipitationAvgMMH     *float64   `gorm:"column:precipitation_avg_mm_h"`
	PrecipitationMaxMMH     *float64   `gorm:"column:precipitation_max_mm_h"`
	RainySampleCount        int64      `gorm:"column:rainy_sample_count"`
	VisibilityMinKM         *float64   `gorm:"column:visibility_min_km"`
	VisibilityAvgKM         *float64   `gorm:"column:visibility_avg_km"`
	PM25AvgUGM3             *float64   `gorm:"column:pm25_avg_ug_m3"`
	PM25MaxUGM3             *float64   `gorm:"column:pm25_max_ug_m3"`
	AQIChnAvg               *float64   `gorm:"column:aqi_chn_avg"`
	AQIChnMax               *int       `gorm:"column:aqi_chn_max"`
}

func (dao *MallWeatherDAO) SummarizeRealtimeDay(
	ctx context.Context,
	query RealtimeDaySummaryQuery,
) (*RealtimeDaySummary, error) {
	if dao == nil || dao.db == nil || ctx == nil {
		return nil, fmt.Errorf("mall weather: invalid realtime day summary query")
	}
	statement, args, err := buildRealtimeDaySummaryQuery(query)
	if err != nil {
		return nil, err
	}
	var summary RealtimeDaySummary
	if err := dao.db.WithContext(ctx).Raw(statement, args...).Scan(&summary).Error; err != nil {
		return nil, fmt.Errorf("mall weather: summarize realtime day: %w", err)
	}
	if summary.SampleCount == 0 {
		return nil, nil
	}
	return &summary, nil
}

func buildRealtimeDaySummaryQuery(query RealtimeDaySummaryQuery) (string, []interface{}, error) {
	qualityStatus := strings.TrimSpace(query.QualityStatus)
	if query.MallID == 0 || query.StartUTC.IsZero() || query.EndUTC.IsZero() ||
		!query.StartUTC.Before(query.EndUTC) {
		return "", nil, fmt.Errorf("mall weather: invalid realtime day summary range")
	}

	where := []string{
		"%s.mall_id = ?",
		"%s.snapshot_at_utc >= ?",
		"%s.snapshot_at_utc < ?",
	}
	filterArgs := []interface{}{query.MallID, query.StartUTC.UTC(), query.EndUTC.UTC()}
	if qualityStatus != "" {
		where = append(where, "%s.quality_status = ?")
		filterArgs = append(filterArgs, qualityStatus)
	}
	formatWhere := func(alias string) string {
		parts := make([]string, len(where))
		for index, condition := range where {
			parts[index] = fmt.Sprintf(condition, alias)
		}
		return strings.Join(parts, " AND ")
	}

	statement := `SELECT
	COUNT(*) AS sample_count,
	MIN(w.snapshot_at_utc) AS observed_start_utc,
	MAX(w.snapshot_at_utc) AS observed_end_utc,
	(
		SELECT s.skycon
		FROM mall_weather_realtime AS s
		WHERE ` + formatWhere("s") + ` AND s.skycon <> ''
		GROUP BY s.skycon
		ORDER BY COUNT(*) DESC, MIN(s.snapshot_at_utc) ASC, s.skycon ASC
		LIMIT 1
	) AS dominant_skycon,
	MIN(w.temperature_c) AS temperature_min_c,
	MAX(w.temperature_c) AS temperature_max_c,
	AVG(w.temperature_c) AS temperature_avg_c,
	AVG(w.apparent_temperature_c) AS apparent_temperature_avg_c,
	AVG(w.humidity_ratio) AS humidity_avg_ratio,
	AVG(w.pressure_pa) AS pressure_avg_pa,
	AVG(w.wind_speed_kph) AS wind_speed_avg_kph,
	MAX(w.wind_speed_kph) AS wind_speed_max_kph,
	AVG(w.local_precip_mm_h) AS precipitation_avg_mm_h,
	MAX(w.local_precip_mm_h) AS precipitation_max_mm_h,
	COALESCE(SUM(CASE WHEN w.local_precip_mm_h > 0 THEN 1 ELSE 0 END), 0) AS rainy_sample_count,
	MIN(w.visibility_km) AS visibility_min_km,
	AVG(w.visibility_km) AS visibility_avg_km,
	AVG(w.pm25_ug_m3) AS pm25_avg_ug_m3,
	MAX(w.pm25_ug_m3) AS pm25_max_ug_m3,
	AVG(w.aqi_chn) AS aqi_chn_avg,
	MAX(w.aqi_chn) AS aqi_chn_max
FROM mall_weather_realtime AS w
WHERE ` + formatWhere("w")

	args := make([]interface{}, 0, len(filterArgs)*2)
	args = append(args, filterArgs...)
	args = append(args, filterArgs...)
	return statement, args, nil
}
