package data_dao

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	defaultWeatherBatchSize = 100
	maxWeatherPageSize      = 500
)

type UpsertResult struct {
	AffectedRows int64
}

type HourlyQuery struct {
	MallID            uint
	StartUTC          time.Time
	EndUTC            time.Time
	AsOfUTC           *time.Time
	Latest            bool
	QualityStatus     string
	AfterForecastTime *time.Time
	AfterIssuedAtUTC  *time.Time
	AfterID           uint
	Limit             int
}

type MallWeatherDAO struct {
	db *gorm.DB
}

func NewMallWeatherDAO(databases ...*gorm.DB) *MallWeatherDAO {
	db := database.DB
	if len(databases) > 0 && databases[0] != nil {
		db = databases[0]
	}
	return &MallWeatherDAO{db: db}
}

func (dao *MallWeatherDAO) WithDB(db *gorm.DB) *MallWeatherDAO {
	return &MallWeatherDAO{db: db}
}

func (dao *MallWeatherDAO) CreateRawSnapshot(ctx context.Context, snapshot *model.ProviderRawSnapshot) error {
	if snapshot == nil {
		return fmt.Errorf("mall weather: create nil raw snapshot")
	}
	if err := dao.db.WithContext(ctx).Create(snapshot).Error; err != nil {
		return fmt.Errorf("mall weather: create raw snapshot: %w", err)
	}
	return nil
}

func (dao *MallWeatherDAO) GetOrCreateFetchRun(ctx context.Context, run *model.MallWeatherFetchRun) (*model.MallWeatherFetchRun, bool, error) {
	if run == nil {
		return nil, false, fmt.Errorf("mall weather: create nil fetch run")
	}
	query := dao.db.WithContext(ctx).Where(
		"mall_id = ? AND endpoint_kind = ? AND task_kind = ? AND task_window = ?",
		run.MallID,
		run.EndpointKind,
		run.TaskKind,
		run.TaskWindow,
	)
	var existing model.MallWeatherFetchRun
	err := query.First(&existing).Error
	if err == nil {
		return &existing, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, fmt.Errorf("mall weather: find fetch run: %w", err)
	}
	createErr := dao.db.WithContext(ctx).Create(run).Error
	if createErr == nil {
		return run, true, nil
	}
	// A concurrent worker may have won the unique-key race. Re-read the
	// logical run before surfacing the create error.
	if err := query.First(&existing).Error; err != nil {
		return nil, false, fmt.Errorf("mall weather: create fetch run: %w", createErr)
	}
	return &existing, false, nil
}

func (dao *MallWeatherDAO) CreateFetchAttempt(ctx context.Context, attempt *model.MallWeatherFetchAttempt) error {
	if attempt == nil {
		return fmt.Errorf("mall weather: create nil fetch attempt")
	}
	if err := dao.db.WithContext(ctx).Create(attempt).Error; err != nil {
		return fmt.Errorf("mall weather: create fetch attempt: %w", err)
	}
	return nil
}

func (dao *MallWeatherDAO) UpdateFetchRun(ctx context.Context, id uint, updates map[string]interface{}) error {
	safeUpdates, err := sanitizeFetchRunUpdates(updates)
	if err != nil {
		return err
	}
	if err := dao.db.WithContext(ctx).Model(&model.MallWeatherFetchRun{}).
		Where("id = ?", id).
		Updates(safeUpdates).Error; err != nil {
		return fmt.Errorf("mall weather: update fetch run: %w", err)
	}
	return nil
}

func sanitizeFetchRunUpdates(updates map[string]interface{}) (map[string]interface{}, error) {
	allowed := map[string]struct{}{
		"attempt_count": {}, "status": {}, "started_at": {}, "finished_at": {}, "duration_ms": {},
		"http_status": {}, "provider_status": {}, "provider_server_time": {}, "response_checksum": {},
		"raw_snapshot_id": {}, "row_counts_json": {}, "parse_warnings_json": {}, "error_class": {},
		"error_code": {}, "error_message_safe": {}, "parser_version": {},
	}
	if len(updates) == 0 {
		return nil, fmt.Errorf("mall weather: no fetch run fields to update")
	}
	result := make(map[string]interface{}, len(updates))
	for field, value := range updates {
		if _, ok := allowed[field]; !ok {
			return nil, fmt.Errorf("mall weather: fetch run field %q is not allowed", field)
		}
		result[field] = value
	}
	return result, nil
}

func (dao *MallWeatherDAO) UpsertRealtime(ctx context.Context, rows []model.MallWeatherRealtime) (UpsertResult, error) {
	return upsertWeatherRows(ctx, dao.db, rows, []string{"mall_id", "provider", "snapshot_at_utc"}, 1)
}

func (dao *MallWeatherDAO) UpsertMinutely(ctx context.Context, rows []model.MallWeatherMinutely) (UpsertResult, error) {
	return upsertWeatherRows(ctx, dao.db, rows, []string{"mall_id", "provider", "forecast_minute_utc", "issued_at_utc"}, 120)
}

func (dao *MallWeatherDAO) UpsertHourly(ctx context.Context, rows []model.MallWeatherHourly) (UpsertResult, error) {
	return upsertWeatherRows(ctx, dao.db, rows, []string{"mall_id", "provider", "forecast_time_utc", "issued_at_utc"}, 200)
}

func (dao *MallWeatherDAO) UpsertDaily(ctx context.Context, rows []model.MallWeatherDaily) (UpsertResult, error) {
	return upsertWeatherRows(ctx, dao.db, rows, []string{"mall_id", "provider", "forecast_date_local", "issued_at_utc"}, 15)
}

func (dao *MallWeatherDAO) UpsertAlerts(ctx context.Context, rows []model.MallWeatherAlert) (UpsertResult, error) {
	return upsertWeatherRows(ctx, dao.db, rows, []string{"provider", "alert_id"}, defaultWeatherBatchSize)
}

func (dao *MallWeatherDAO) UpsertAlertRelations(ctx context.Context, rows []model.MallWeatherAlertRelation) (UpsertResult, error) {
	return upsertWeatherRows(ctx, dao.db, rows, []string{"mall_id", "alert_pk"}, defaultWeatherBatchSize)
}

func (dao *MallWeatherDAO) UpsertLifeIndices(ctx context.Context, rows []model.MallWeatherLifeIndex) (UpsertResult, error) {
	return upsertWeatherRows(ctx, dao.db, rows, []string{"mall_id", "provider", "source_api", "forecast_date_local", "index_type", "issued_at_utc"}, 200)
}

func (dao *MallWeatherDAO) UpsertLatest(ctx context.Context, rows []model.MallWeatherLatest) (UpsertResult, error) {
	return upsertWeatherRows(ctx, dao.db, rows, []string{"mall_id", "data_kind", "business_key"}, defaultWeatherBatchSize)
}

func upsertWeatherRows[T any](ctx context.Context, db *gorm.DB, rows []T, conflictColumns []string, batchSize int) (UpsertResult, error) {
	if len(rows) == 0 {
		return UpsertResult{}, nil
	}
	columns := make([]clause.Column, 0, len(conflictColumns))
	for _, name := range conflictColumns {
		columns = append(columns, clause.Column{Name: name})
	}
	result := db.WithContext(ctx).
		Clauses(clause.OnConflict{Columns: columns, UpdateAll: true}).
		CreateInBatches(&rows, batchSize)
	if result.Error != nil {
		return UpsertResult{}, fmt.Errorf("mall weather: batch upsert: %w", result.Error)
	}
	return UpsertResult{AffectedRows: result.RowsAffected}, nil
}

func (dao *MallWeatherDAO) QueryHourly(ctx context.Context, query HourlyQuery) ([]model.MallWeatherHourly, error) {
	statement, args, err := buildHourlyQuery(query)
	if err != nil {
		return nil, err
	}
	var rows []model.MallWeatherHourly
	if err := dao.db.WithContext(ctx).Raw(statement, args...).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("mall weather: query hourly: %w", err)
	}
	return rows, nil
}

func buildHourlyQuery(query HourlyQuery) (string, []interface{}, error) {
	if query.MallID == 0 {
		return "", nil, fmt.Errorf("mall weather: mall id is required")
	}
	if query.StartUTC.IsZero() || query.EndUTC.IsZero() || !query.StartUTC.Before(query.EndUTC) {
		return "", nil, fmt.Errorf("mall weather: invalid hourly time range")
	}

	where := []string{
		"w.mall_id = ?",
		"w.forecast_time_utc >= ?",
		"w.forecast_time_utc < ?",
	}
	args := []interface{}{query.MallID, query.StartUTC.UTC(), query.EndUTC.UTC()}
	if query.AsOfUTC != nil {
		where = append(where, "w.issued_at_utc <= ?")
		args = append(args, query.AsOfUTC.UTC())
	}
	if query.QualityStatus != "" {
		where = append(where, "w.quality_status = ?")
		args = append(args, query.QualityStatus)
	}
	limit := query.Limit
	if limit <= 0 {
		limit = 200
	} else if limit > maxWeatherPageSize {
		limit = maxWeatherPageSize
	}
	args = append(args, limit)

	if query.Latest || query.AsOfUTC != nil {
		outerWhere := []string{"ranked.version_rank = 1"}
		if query.AfterForecastTime != nil {
			outerWhere = append(outerWhere, "(ranked.forecast_time_utc > ? OR (ranked.forecast_time_utc = ? AND ranked.id > ?))")
			cursor := query.AfterForecastTime.UTC()
			args = append(args[:len(args)-1], cursor, cursor, query.AfterID, limit)
		}
		return `SELECT ranked.* FROM (
SELECT w.*, ROW_NUMBER() OVER (
  PARTITION BY w.forecast_time_utc
  ORDER BY w.issued_at_utc DESC, w.id DESC
) AS version_rank
FROM mall_weather_hourly AS w
WHERE ` + strings.Join(where, " AND ") + `
) AS ranked
WHERE ` + strings.Join(outerWhere, " AND ") + `
ORDER BY ranked.forecast_time_utc ASC, ranked.id ASC
LIMIT ?`, args, nil
	}
	if query.AfterForecastTime != nil {
		if query.AfterIssuedAtUTC == nil {
			return "", nil, fmt.Errorf("mall weather: issued-at cursor is required for version history")
		}
		where = append(where, `(w.forecast_time_utc > ?
OR (w.forecast_time_utc = ? AND w.issued_at_utc < ?)
OR (w.forecast_time_utc = ? AND w.issued_at_utc = ? AND w.id > ?))`)
		cursor := query.AfterForecastTime.UTC()
		issuedCursor := query.AfterIssuedAtUTC.UTC()
		args = append(args[:len(args)-1], cursor, cursor, issuedCursor, cursor, issuedCursor, query.AfterID, limit)
	}

	return `SELECT w.*
FROM mall_weather_hourly AS w
WHERE ` + strings.Join(where, " AND ") + `
ORDER BY w.forecast_time_utc ASC, w.issued_at_utc DESC, w.id ASC
LIMIT ?`, args, nil
}
