package data_dao

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	"gin-biz-web-api/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const weatherLatestTimeLayout = "20060102T150405.000Z"

type MallWeatherLatestSources struct {
	Realtime    []model.MallWeatherRealtime
	Minutely    []model.MallWeatherMinutely
	Hourly      []model.MallWeatherHourly
	Daily       []model.MallWeatherDaily
	LifeIndices []model.MallWeatherLifeIndex
}

func (dao *MallWeatherDAO) RefreshLatest(ctx context.Context, sources MallWeatherLatestSources) (UpsertResult, error) {
	if dao == nil || dao.db == nil || ctx == nil {
		return UpsertResult{}, fmt.Errorf("mall weather: latest store is not configured")
	}
	latest := make([]model.MallWeatherLatest, 0,
		len(sources.Realtime)+len(sources.Minutely)+len(sources.Hourly)+len(sources.Daily)+len(sources.LifeIndices))
	var err error
	latest, err = appendResolvedLatest(ctx, dao.db, latest, sources.Realtime,
		[]string{"mall_id", "provider", "snapshot_at_utc"}, latestFromRealtime)
	if err != nil {
		return UpsertResult{}, err
	}
	latest, err = appendResolvedLatest(ctx, dao.db, latest, sources.Minutely,
		[]string{"mall_id", "provider", "forecast_minute_utc", "issued_at_utc"}, latestFromMinutely)
	if err != nil {
		return UpsertResult{}, err
	}
	latest, err = appendResolvedLatest(ctx, dao.db, latest, sources.Hourly,
		[]string{"mall_id", "provider", "forecast_time_utc", "issued_at_utc"}, latestFromHourly)
	if err != nil {
		return UpsertResult{}, err
	}
	latest, err = appendResolvedLatest(ctx, dao.db, latest, sources.Daily,
		[]string{"mall_id", "provider", "forecast_date_local", "issued_at_utc"}, latestFromDaily)
	if err != nil {
		return UpsertResult{}, err
	}
	latest, err = appendResolvedLatest(ctx, dao.db, latest, sources.LifeIndices,
		[]string{"mall_id", "provider", "source_api", "forecast_date_local", "index_type", "issued_at_utc"}, latestFromLifeIndex)
	if err != nil {
		return UpsertResult{}, err
	}
	return dao.UpsertLatest(ctx, latest)
}

func appendResolvedLatest[T any](
	ctx context.Context,
	db *gorm.DB,
	destination []model.MallWeatherLatest,
	sources []T,
	identityColumns []string,
	convert func(T) (model.MallWeatherLatest, error),
) ([]model.MallWeatherLatest, error) {
	if len(sources) == 0 {
		return destination, nil
	}
	predicate, args, err := checksumIdentityPredicate(ctx, db, sources, identityColumns)
	if err != nil {
		return nil, err
	}
	stored := make([]T, 0, len(sources))
	if err := db.WithContext(ctx).Where(predicate, args...).Find(&stored).Error; err != nil {
		return nil, fmt.Errorf("mall weather: resolve latest source rows: %w", err)
	}
	if len(stored) != len(sources) {
		return nil, fmt.Errorf("mall weather: latest source identity mismatch")
	}
	for index := range stored {
		row, err := convert(stored[index])
		if err != nil {
			return nil, err
		}
		destination = append(destination, row)
	}
	return destination, nil
}

func latestFromRealtime(row model.MallWeatherRealtime) (model.MallWeatherLatest, error) {
	businessTime := row.SnapshotAtUTC.UTC()
	return newWeatherLatest(row.ID, row.MallID, model.MallWeatherDataKindRealtime, model.MallWeatherDataKindRealtime,
		&businessTime, nil, "", businessTime, row.FetchedAtUTC)
}

func latestFromMinutely(row model.MallWeatherMinutely) (model.MallWeatherLatest, error) {
	businessTime := row.ForecastMinuteUTC.UTC()
	return newWeatherLatest(row.ID, row.MallID, model.MallWeatherDataKindMinutely, weatherTimeBusinessKey(businessTime),
		&businessTime, nil, "", row.IssuedAtUTC, row.FetchedAtUTC)
}

func latestFromHourly(row model.MallWeatherHourly) (model.MallWeatherLatest, error) {
	businessTime := row.ForecastTimeUTC.UTC()
	return newWeatherLatest(row.ID, row.MallID, model.MallWeatherDataKindHourly, weatherTimeBusinessKey(businessTime),
		&businessTime, nil, "", row.IssuedAtUTC, row.FetchedAtUTC)
}

func latestFromDaily(row model.MallWeatherDaily) (model.MallWeatherLatest, error) {
	businessDate := row.ForecastDateLocal
	return newWeatherLatest(row.ID, row.MallID, model.MallWeatherDataKindDaily, weatherDateBusinessKey(businessDate),
		nil, &businessDate, "", row.IssuedAtUTC, row.FetchedAtUTC)
}

func latestFromLifeIndex(row model.MallWeatherLifeIndex) (model.MallWeatherLatest, error) {
	businessDate := row.ForecastDateLocal
	subtype := row.SourceAPI + ":" + strconv.Itoa(row.IndexType)
	return newWeatherLatest(row.ID, row.MallID, model.MallWeatherDataKindLife,
		weatherDateBusinessKey(businessDate)+"|"+subtype, nil, &businessDate, subtype, row.IssuedAtUTC, row.FetchedAtUTC)
}

func newWeatherLatest(
	sourceRowID, mallID uint,
	dataKind, businessKey string,
	businessTime, businessDate *time.Time,
	subtype string,
	issuedAt, fetchedAt time.Time,
) (model.MallWeatherLatest, error) {
	if sourceRowID == 0 || mallID == 0 || dataKind == "" || businessKey == "" || issuedAt.IsZero() || fetchedAt.IsZero() {
		return model.MallWeatherLatest{}, fmt.Errorf("mall weather: invalid latest source")
	}
	switch dataKind {
	case model.MallWeatherDataKindRealtime, model.MallWeatherDataKindMinutely, model.MallWeatherDataKindHourly:
		if businessTime == nil || businessTime.IsZero() || businessDate != nil || subtype != "" {
			return model.MallWeatherLatest{}, fmt.Errorf("mall weather: invalid time-based latest source")
		}
		normalized := businessTime.UTC()
		businessTime = &normalized
	case model.MallWeatherDataKindDaily:
		if businessTime != nil || businessDate == nil || businessDate.IsZero() || subtype != "" {
			return model.MallWeatherLatest{}, fmt.Errorf("mall weather: invalid date-based latest source")
		}
	case model.MallWeatherDataKindLife:
		if businessTime != nil || businessDate == nil || businessDate.IsZero() || subtype == "" {
			return model.MallWeatherLatest{}, fmt.Errorf("mall weather: invalid life-index latest source")
		}
	default:
		return model.MallWeatherLatest{}, fmt.Errorf("mall weather: unsupported latest data kind")
	}
	return model.MallWeatherLatest{
		MallID: mallID, DataKind: dataKind, BusinessKey: businessKey,
		BusinessTime: businessTime, BusinessDate: businessDate, Subtype: subtype,
		SourceRowID: sourceRowID, IssuedAtUTC: issuedAt.UTC(), FetchedAtUTC: fetchedAt.UTC(),
		FreshnessStatus: model.MallWeatherFreshnessFresh,
	}, nil
}

func weatherTimeBusinessKey(value time.Time) string {
	return value.UTC().Truncate(time.Millisecond).Format(weatherLatestTimeLayout)
}

func weatherDateBusinessKey(value time.Time) string {
	return value.Format("2006-01-02")
}

func upsertLatestRows(ctx context.Context, db *gorm.DB, rows []model.MallWeatherLatest) (UpsertResult, error) {
	if len(rows) == 0 {
		return UpsertResult{}, nil
	}
	sort.Slice(rows, func(left, right int) bool {
		if rows[left].MallID != rows[right].MallID {
			return rows[left].MallID < rows[right].MallID
		}
		if rows[left].DataKind != rows[right].DataKind {
			return rows[left].DataKind < rows[right].DataKind
		}
		return rows[left].BusinessKey < rows[right].BusinessKey
	})
	updates := latestMonotonicUpdateSet()
	result := db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "mall_id"}, {Name: "data_kind"}, {Name: "business_key"}},
			DoUpdates: updates,
		}).
		CreateInBatches(&rows, defaultWeatherBatchSize)
	if result.Error != nil {
		return UpsertResult{}, fmt.Errorf("mall weather: upsert latest pointers: %w", result.Error)
	}
	return UpsertResult{AffectedRows: result.RowsAffected}, nil
}

func latestMonotonicUpdateSet() clause.Set {
	winner := "VALUES(`issued_at_utc`) > `issued_at_utc` OR " +
		"(VALUES(`issued_at_utc`) = `issued_at_utc` AND VALUES(`fetched_at_utc`) >= `fetched_at_utc`)"
	return clause.Set{
		{Column: clause.Column{Name: "source_row_id"}, Value: clause.Expr{SQL: "IF(" + winner + ", VALUES(`source_row_id`), `source_row_id`)"}},
		{Column: clause.Column{Name: "freshness_status"}, Value: clause.Expr{SQL: "IF(" + winner + ", VALUES(`freshness_status`), `freshness_status`)"}},
		{Column: clause.Column{Name: "updated_at"}, Value: clause.Expr{SQL: "IF(" + winner + ", VALUES(`updated_at`), `updated_at`)"}},
		{Column: clause.Column{Name: "fetched_at_utc"}, Value: clause.Expr{SQL: "IF(VALUES(`issued_at_utc`) >= `issued_at_utc`, GREATEST(`fetched_at_utc`, VALUES(`fetched_at_utc`)), `fetched_at_utc`)"}},
		// MySQL evaluates assignments left to right. Keep issued_at_utc last so
		// all winner expressions above compare against the stored version.
		{Column: clause.Column{Name: "issued_at_utc"}, Value: clause.Expr{SQL: "GREATEST(`issued_at_utc`, VALUES(`issued_at_utc`))"}},
	}
}
