package data_dao

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"

	"gorm.io/gorm"
)

var ErrMallWeatherExportJobNotFound = errors.New("mall weather export job: not found")

var mallWeatherExportJobQueryColumns = []string{
	"id",
	"job_uuid",
	"profile_id",
	"profile_version",
	"status",
	"total_rows",
	"processed_rows",
	"current_sheet",
	"cancel_requested",
	"result_checksum",
	"file_size_bytes",
	"error_message_safe",
	"started_at",
	"finished_at",
	"expires_at",
	"created_at",
	"updated_at",
}

type MallWeatherExportEstimateFilter struct {
	MallIDs         []uint
	Cities          []string
	MallStatuses    []string
	QualityStatuses []string
	StartUTC        *time.Time
	EndUTC          *time.Time
	StartDate       string
	EndDate         string
}

type MallWeatherExportEstimateDataset struct {
	Kind    string
	Latest  bool
	AsOfUTC *time.Time
}

type MallWeatherExportEstimateRequest struct {
	Datasets  []MallWeatherExportEstimateDataset
	Filter    MallWeatherExportEstimateFilter
	StopAfter int64
}

type MallWeatherExportJobDAO struct {
	db *gorm.DB
}

func NewMallWeatherExportJobDAO(databases ...*gorm.DB) *MallWeatherExportJobDAO {
	db := database.DB
	if len(databases) > 0 && databases[0] != nil {
		db = databases[0]
	}
	return &MallWeatherExportJobDAO{db: db}
}

func (dao *MallWeatherExportJobDAO) EstimateRows(
	ctx context.Context,
	request MallWeatherExportEstimateRequest,
) (int64, error) {
	if dao == nil || dao.db == nil || ctx == nil || len(request.Datasets) == 0 || request.StopAfter < 1 {
		return 0, fmt.Errorf("mall weather export job: invalid estimate request")
	}
	var total int64
	for _, dataset := range request.Datasets {
		count, err := dao.estimateDatasetRows(ctx, dataset, request.Filter)
		if err != nil {
			return 0, err
		}
		if count < 0 || total > request.StopAfter-count {
			return request.StopAfter + 1, nil
		}
		total += count
		if total > request.StopAfter {
			return total, nil
		}
	}
	return total, nil
}

func (dao *MallWeatherExportJobDAO) FindByUUIDAndActor(
	ctx context.Context,
	jobUUID string,
	actorUserID uint,
) (*model.MallWeatherExportJob, error) {
	if dao == nil || dao.db == nil || ctx == nil || len(jobUUID) != 36 || actorUserID == 0 {
		return nil, fmt.Errorf("mall weather export job: invalid lookup")
	}
	var row model.MallWeatherExportJob
	err := dao.db.WithContext(ctx).
		Select(mallWeatherExportJobQueryColumns).
		Where("job_uuid = ? AND created_by = ?", jobUUID, actorUserID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMallWeatherExportJobNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("mall weather export job: find: %w", err)
	}
	return &row, nil
}

func (dao *MallWeatherExportJobDAO) estimateDatasetRows(
	ctx context.Context,
	dataset MallWeatherExportEstimateDataset,
	filter MallWeatherExportEstimateFilter,
) (int64, error) {
	query, countExpression, timeColumn, qualityColumn, issuedColumn, err := dao.exportEstimateQuery(ctx, dataset)
	if err != nil {
		return 0, err
	}
	query = applyMallWeatherExportMallFilters(query, filter)
	if qualityColumn != "" && len(filter.QualityStatuses) > 0 {
		query = query.Where(qualityColumn+" IN ?", filter.QualityStatuses)
	}
	if timeColumn != "" {
		startValue, endValue := mallWeatherExportEstimateTimeValues(dataset.Kind, filter)
		if startValue != nil {
			query = query.Where(timeColumn+" >= ?", startValue)
		}
		if endValue != nil {
			query = query.Where(timeColumn+" < ?", endValue)
		}
	}
	if issuedColumn != "" && dataset.AsOfUTC != nil {
		query = query.Where(issuedColumn+" <= ?", dataset.AsOfUTC.UTC())
	}
	var count int64
	if err := query.Select(countExpression).Scan(&count).Error; err != nil {
		return 0, fmt.Errorf("mall weather export job: estimate %s: %w", dataset.Kind, err)
	}
	return count, nil
}

func (dao *MallWeatherExportJobDAO) exportEstimateQuery(
	ctx context.Context,
	dataset MallWeatherExportEstimateDataset,
) (*gorm.DB, string, string, string, string, error) {
	switch dataset.Kind {
	case "malls":
		return dao.db.WithContext(ctx).Table("malls AS m").Where("m.deleted_at IS NULL"),
			"COUNT(m.id)", "", "", "", nil
	case "realtime":
		count := "COUNT(w.id)"
		if dataset.Latest {
			count = "COUNT(DISTINCT w.mall_id)"
		}
		return mallWeatherExportWeatherTable(dao.db, ctx, "mall_weather_realtime"),
			count, "w.snapshot_at_utc", "w.quality_status", "w.fetched_at_utc", nil
	case "minutely":
		count := "COUNT(w.id)"
		if dataset.Latest {
			count = "COUNT(DISTINCT w.mall_id, w.forecast_minute_utc)"
		}
		return mallWeatherExportWeatherTable(dao.db, ctx, "mall_weather_minutely"),
			count, "w.forecast_minute_utc", "w.quality_status", "w.issued_at_utc", nil
	case "hourly":
		count := "COUNT(w.id)"
		if dataset.Latest {
			count = "COUNT(DISTINCT w.mall_id, w.forecast_time_utc)"
		}
		return mallWeatherExportWeatherTable(dao.db, ctx, "mall_weather_hourly"),
			count, "w.forecast_time_utc", "w.quality_status", "w.issued_at_utc", nil
	case "daily":
		count := "COUNT(w.id)"
		if dataset.Latest {
			count = "COUNT(DISTINCT w.mall_id, w.forecast_date_local)"
		}
		return mallWeatherExportWeatherTable(dao.db, ctx, "mall_weather_daily"),
			count, "w.forecast_date_local", "w.quality_status", "w.issued_at_utc", nil
	case "alerts":
		query := dao.db.WithContext(ctx).Table("mall_weather_alert_relations AS relation").
			Joins("JOIN mall_weather_alerts AS w ON w.id = relation.alert_pk").
			Joins("JOIN malls AS m ON m.id = relation.mall_id AND m.deleted_at IS NULL")
		return query, "COUNT(DISTINCT relation.mall_id, w.id)",
			"COALESCE(w.published_at_utc, w.first_seen_at)", "w.quality_status", "", nil
	case "life_indices":
		count := "COUNT(w.id)"
		if dataset.Latest {
			count = "COUNT(DISTINCT w.mall_id, w.forecast_date_local, w.source_api, w.index_type)"
		}
		return mallWeatherExportWeatherTable(dao.db, ctx, "mall_weather_life_indices").Where("w.source_api = ?", "v26_daily"),
			count, "w.forecast_date_local", "w.quality_status", "w.issued_at_utc", nil
	case "fetch_runs":
		return mallWeatherExportWeatherTable(dao.db, ctx, "mall_weather_fetch_runs"),
			"COUNT(w.id)", "w.created_at", "", "", nil
	default:
		return nil, "", "", "", "", fmt.Errorf("mall weather export job: unsupported estimate dataset")
	}
}

func mallWeatherExportWeatherTable(db *gorm.DB, ctx context.Context, table string) *gorm.DB {
	return db.WithContext(ctx).Table(table + " AS w").
		Joins("JOIN malls AS m ON m.id = w.mall_id AND m.deleted_at IS NULL")
}

func applyMallWeatherExportMallFilters(
	query *gorm.DB,
	filter MallWeatherExportEstimateFilter,
) *gorm.DB {
	if len(filter.MallIDs) > 0 {
		query = query.Where("m.id IN ?", filter.MallIDs)
	}
	if len(filter.Cities) > 0 {
		query = query.Where("m.city IN ?", filter.Cities)
	}
	if len(filter.MallStatuses) > 0 {
		query = query.Where("m.status IN ?", filter.MallStatuses)
	}
	return query
}

func mallWeatherExportEstimateTimeValues(
	datasetKind string,
	filter MallWeatherExportEstimateFilter,
) (interface{}, interface{}) {
	if datasetKind == "daily" || datasetKind == "life_indices" {
		var start interface{}
		var end interface{}
		if filter.StartDate != "" {
			start = filter.StartDate
		}
		if filter.EndDate != "" {
			end = filter.EndDate
		}
		return start, end
	}
	return filter.StartUTC, filter.EndUTC
}
