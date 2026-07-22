package data_dao

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"gin-biz-web-api/connector/caiyun"
	weatherdomain "gin-biz-web-api/internal/weather"
	"gin-biz-web-api/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const weatherLatestTimeLayout = "20060102T150405.000Z"

type latestFreshnessPlan struct {
	DataKind       string
	WarningBefore  time.Time
	CriticalBefore time.Time
}

type MallWeatherLatestSources struct {
	Realtime    []model.MallWeatherRealtime
	Minutely    []model.MallWeatherMinutely
	Hourly      []model.MallWeatherHourly
	Daily       []model.MallWeatherDaily
	LifeIndices []model.MallWeatherLifeIndex
}

type MallWeatherLatestStaleScope struct {
	DataKinds      []string
	LifeSourceAPIs []string
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

func (dao *MallWeatherDAO) ReconcileLatestFreshness(ctx context.Context, now time.Time) (int64, error) {
	if dao == nil || dao.db == nil || ctx == nil || now.IsZero() {
		return 0, fmt.Errorf("mall weather: latest freshness store is not configured")
	}
	plans, err := buildLatestFreshnessPlans(now)
	if err != nil {
		return 0, err
	}
	var affected int64
	for _, plan := range plans {
		result := dao.db.WithContext(ctx).
			Model(&model.MallWeatherLatest{}).
			Where("data_kind = ? AND freshness_status <> ?", plan.DataKind, model.MallWeatherFreshnessStale).
			Where(`
				(fetched_at_utc <= ? AND freshness_status <> ?) OR
				(fetched_at_utc > ? AND fetched_at_utc <= ? AND freshness_status <> ?) OR
				(fetched_at_utc > ? AND freshness_status <> ?)`,
				plan.CriticalBefore, model.MallWeatherFreshnessCritical,
				plan.CriticalBefore, plan.WarningBefore, model.MallWeatherFreshnessWarning,
				plan.WarningBefore, model.MallWeatherFreshnessFresh,
			).
			Update("freshness_status", gorm.Expr(`CASE
				WHEN fetched_at_utc <= ? THEN ?
				WHEN fetched_at_utc <= ? THEN ?
				ELSE ? END`,
				plan.CriticalBefore, model.MallWeatherFreshnessCritical,
				plan.WarningBefore, model.MallWeatherFreshnessWarning,
				model.MallWeatherFreshnessFresh,
			))
		if result.Error != nil {
			return affected, fmt.Errorf("mall weather: reconcile %s latest freshness: %w", plan.DataKind, result.Error)
		}
		affected += result.RowsAffected
	}
	return affected, nil
}

func (dao *MallWeatherDAO) MarkLatestStaleForEndpoint(ctx context.Context, mallID uint, endpointKind string, observedBefore time.Time) (int64, error) {
	if dao == nil || dao.db == nil || ctx == nil || mallID == 0 || observedBefore.IsZero() {
		return 0, fmt.Errorf("mall weather: invalid latest stale input")
	}
	var scope MallWeatherLatestStaleScope
	switch endpointKind {
	case caiyun.EndpointWeatherV26:
		scope = MallWeatherLatestStaleScope{
			DataKinds: []string{
				model.MallWeatherDataKindRealtime,
				model.MallWeatherDataKindMinutely,
				model.MallWeatherDataKindHourly,
				model.MallWeatherDataKindDaily,
			},
			LifeSourceAPIs: []string{weatherdomain.SourceAPIV26Daily},
		}
	case caiyun.EndpointLifeIndexV3:
		scope = MallWeatherLatestStaleScope{LifeSourceAPIs: []string{weatherdomain.SourceAPIV3LifeIndex}}
	default:
		return 0, fmt.Errorf("mall weather: unsupported stale endpoint")
	}
	return dao.MarkLatestStale(ctx, mallID, scope, observedBefore)
}

func (dao *MallWeatherDAO) MarkLatestStale(ctx context.Context, mallID uint, scope MallWeatherLatestStaleScope, observedBefore time.Time) (int64, error) {
	if dao == nil || dao.db == nil || ctx == nil || mallID == 0 || observedBefore.IsZero() {
		return 0, fmt.Errorf("mall weather: invalid latest stale input")
	}
	predicate, args, err := latestStaleScopePredicate(scope)
	if err != nil {
		return 0, err
	}
	result := dao.db.WithContext(ctx).
		Model(&model.MallWeatherLatest{}).
		Where("mall_id = ? AND freshness_status <> ? AND fetched_at_utc <= ?", mallID, model.MallWeatherFreshnessStale, observedBefore.UTC()).
		Where(predicate, args...).
		Update("freshness_status", model.MallWeatherFreshnessStale)
	if result.Error != nil {
		return 0, fmt.Errorf("mall weather: mark latest stale: %w", result.Error)
	}
	return result.RowsAffected, nil
}

func buildLatestFreshnessPlans(now time.Time) ([]latestFreshnessPlan, error) {
	if now.IsZero() {
		return nil, fmt.Errorf("mall weather: invalid freshness reconciliation time")
	}
	now = now.UTC()
	kinds := []string{
		model.MallWeatherDataKindRealtime,
		model.MallWeatherDataKindMinutely,
		model.MallWeatherDataKindHourly,
		model.MallWeatherDataKindDaily,
		model.MallWeatherDataKindLife,
	}
	plans := make([]latestFreshnessPlan, 0, len(kinds))
	for _, kind := range kinds {
		thresholds, err := weatherdomain.FreshnessThresholdsForKind(kind)
		if err != nil {
			return nil, err
		}
		plans = append(plans, latestFreshnessPlan{
			DataKind: kind, WarningBefore: now.Add(-thresholds.Warning), CriticalBefore: now.Add(-thresholds.Critical),
		})
	}
	return plans, nil
}

func latestStaleScopePredicate(scope MallWeatherLatestStaleScope) (string, []interface{}, error) {
	allowedKinds := map[string]struct{}{
		model.MallWeatherDataKindRealtime: {},
		model.MallWeatherDataKindMinutely: {},
		model.MallWeatherDataKindHourly:   {},
		model.MallWeatherDataKindDaily:    {},
	}
	allowedLifeSources := map[string]struct{}{
		weatherdomain.SourceAPIV26Daily:    {},
		weatherdomain.SourceAPIV3LifeIndex: {},
	}
	kinds := uniqueAllowedLatestScopeValues(scope.DataKinds, allowedKinds)
	lifeSources := uniqueAllowedLatestScopeValues(scope.LifeSourceAPIs, allowedLifeSources)
	if kinds == nil || lifeSources == nil || len(kinds)+len(lifeSources) == 0 {
		return "", nil, fmt.Errorf("mall weather: invalid latest stale scope")
	}
	conditions := make([]string, 0, 1+len(lifeSources))
	args := make([]interface{}, 0, 1+2*len(lifeSources))
	if len(kinds) > 0 {
		conditions = append(conditions, "data_kind IN ?")
		args = append(args, kinds)
	}
	for _, sourceAPI := range lifeSources {
		conditions = append(conditions, "(data_kind = ? AND subtype LIKE ?)")
		args = append(args, model.MallWeatherDataKindLife, sourceAPI+":%")
	}
	return "(" + strings.Join(conditions, " OR ") + ")", args, nil
}

func uniqueAllowedLatestScopeValues(values []string, allowed map[string]struct{}) []string {
	unique := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, ok := allowed[value]; !ok {
			return nil
		}
		unique[value] = struct{}{}
	}
	result := make([]string, 0, len(unique))
	for value := range unique {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
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
