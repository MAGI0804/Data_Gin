package data_dao

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gin-biz-web-api/pkg/database"

	"gorm.io/gorm"
)

const maxMallWeatherExportPageSize = 1000

type MallWeatherExportDataPageRequest struct {
	Kind       string
	Fields     []string
	Filter     MallWeatherExportEstimateFilter
	Latest     bool
	AsOfUTC    *time.Time
	AfterID    uint
	Limit      int
	SnapshotAt time.Time
}

type MallWeatherExportDataRow struct {
	CursorID      uint
	Values        map[string]interface{}
	SplitCity     string
	SplitMall     string
	SplitDate     string
	SplitDataType string
}

type MallWeatherExportDataPage struct {
	Rows        []MallWeatherExportDataRow
	NextAfterID uint
	HasMore     bool
}

type MallWeatherExportDataDAO struct {
	db *gorm.DB
}

type mallWeatherExportDatasetSpec struct {
	from                    string
	joins                   []string
	cursorExpression        string
	createdExpression       string
	timeExpression          string
	qualityExpression       string
	issuedExpression        string
	splitDateExpression     string
	splitDataTypeExpression string
	fields                  map[string]string
	defaultFields           []string
	latestPredicate         string
}

func NewMallWeatherExportDataDAO(databases ...*gorm.DB) *MallWeatherExportDataDAO {
	db := database.DB
	if len(databases) > 0 && databases[0] != nil {
		db = databases[0]
	}
	return &MallWeatherExportDataDAO{db: db}
}

func MallWeatherExportDatasetFields(kind string) (map[string]struct{}, bool) {
	spec, ok := mallWeatherExportDatasetCatalog[strings.ToLower(strings.TrimSpace(kind))]
	if !ok {
		return nil, false
	}
	fields := make(map[string]struct{}, len(spec.fields))
	for field := range spec.fields {
		fields[field] = struct{}{}
	}
	return fields, true
}

func MallWeatherExportDefaultFields(kind string) ([]string, bool) {
	spec, ok := mallWeatherExportDatasetCatalog[strings.ToLower(strings.TrimSpace(kind))]
	if !ok {
		return nil, false
	}
	return append([]string(nil), spec.defaultFields...), true
}

func (dao *MallWeatherExportDataDAO) Page(
	ctx context.Context,
	request MallWeatherExportDataPageRequest,
) (*MallWeatherExportDataPage, error) {
	query, fields, err := dao.buildPageQuery(ctx, request)
	if err != nil {
		return nil, err
	}
	rows, err := query.Rows()
	if err != nil {
		return nil, fmt.Errorf("mall weather export data: query %s: %w", request.Kind, err)
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("mall weather export data: read columns: %w", err)
	}
	requestedFields := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		requestedFields[field] = struct{}{}
	}
	result := &MallWeatherExportDataPage{Rows: make([]MallWeatherExportDataRow, 0, request.Limit+1)}
	for rows.Next() {
		row, err := scanMallWeatherExportDataRow(rows, columns, requestedFields, len(fields))
		if err != nil {
			return nil, err
		}
		result.Rows = append(result.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mall weather export data: iterate %s: %w", request.Kind, err)
	}
	if len(result.Rows) > request.Limit {
		result.HasMore = true
		result.Rows = result.Rows[:request.Limit]
	}
	if len(result.Rows) > 0 {
		result.NextAfterID = result.Rows[len(result.Rows)-1].CursorID
	}
	return result, nil
}

func (dao *MallWeatherExportDataDAO) buildPageQuery(
	ctx context.Context,
	request MallWeatherExportDataPageRequest,
) (*gorm.DB, []string, error) {
	request.Kind = strings.ToLower(strings.TrimSpace(request.Kind))
	spec, exists := mallWeatherExportDatasetCatalog[request.Kind]
	if dao == nil || dao.db == nil || ctx == nil || !exists || request.Limit < 1 ||
		request.Limit > maxMallWeatherExportPageSize || request.SnapshotAt.IsZero() {
		return nil, nil, fmt.Errorf("mall weather export data: invalid page request")
	}
	fields := request.Fields
	if len(fields) == 0 {
		fields = spec.defaultFields
	}
	if len(fields) == 0 || len(fields) > 128 {
		return nil, nil, fmt.Errorf("mall weather export data: invalid selected fields")
	}
	seen := make(map[string]struct{}, len(fields))
	selectExpressions := make([]string, 0, len(fields)+5)
	for _, field := range fields {
		expression, allowed := spec.fields[field]
		if !allowed {
			return nil, nil, fmt.Errorf("mall weather export data: unsupported selected field")
		}
		if _, duplicate := seen[field]; duplicate {
			return nil, nil, fmt.Errorf("mall weather export data: duplicate selected field")
		}
		seen[field] = struct{}{}
		selectExpressions = append(selectExpressions, expression+" AS "+field)
	}
	selectExpressions = append(selectExpressions,
		spec.cursorExpression+" AS _cursor_id",
		"m.city AS _split_city",
		"m.mall_code AS _split_mall",
		spec.splitDateExpression+" AS _split_date",
		spec.splitDataTypeExpression+" AS _split_data_type",
	)
	// Every expression and alias comes from mallWeatherExportDatasetCatalog.
	// User-provided field names are allow-listed above before reaching this SQL boundary.
	query := dao.db.WithContext(ctx).Table(spec.from).Select(strings.Join(selectExpressions, ", "))
	for _, join := range spec.joins {
		query = query.Joins(join)
	}
	query = query.Where(spec.cursorExpression+" > ?", request.AfterID).
		Where(spec.createdExpression+" <= ?", request.SnapshotAt.UTC()).
		Where("m.deleted_at IS NULL")
	query = applyMallWeatherExportMallFilters(query, request.Filter)
	if spec.qualityExpression != "" && len(request.Filter.QualityStatuses) > 0 {
		query = query.Where(spec.qualityExpression+" IN ?", request.Filter.QualityStatuses)
	}
	if spec.timeExpression != "" {
		startValue, endValue := mallWeatherExportEstimateTimeValues(request.Kind, request.Filter)
		if startValue != nil {
			query = query.Where(spec.timeExpression+" >= ?", startValue)
		}
		if endValue != nil {
			query = query.Where(spec.timeExpression+" < ?", endValue)
		}
	}
	cutoff := request.AsOfUTC
	if spec.issuedExpression != "" && (cutoff != nil || request.Latest) {
		effectiveCutoff := request.SnapshotAt.UTC()
		if cutoff != nil {
			effectiveCutoff = cutoff.UTC()
		}
		query = query.Where(spec.issuedExpression+" <= ?", effectiveCutoff)
	}
	if request.Latest && spec.latestPredicate != "" {
		latestCutoff := request.SnapshotAt.UTC()
		if cutoff != nil {
			latestCutoff = cutoff.UTC()
		}
		query = query.Where(spec.latestPredicate, latestCutoff, request.SnapshotAt.UTC())
	}
	query = query.Order(spec.cursorExpression + " ASC").Limit(request.Limit + 1)
	return query, append([]string(nil), fields...), nil
}

func scanMallWeatherExportDataRow(
	rows *sql.Rows,
	columns []string,
	requestedFields map[string]struct{},
	fieldCount int,
) (MallWeatherExportDataRow, error) {
	values := make([]interface{}, len(columns))
	destinations := make([]interface{}, len(columns))
	for index := range values {
		destinations[index] = &values[index]
	}
	if err := rows.Scan(destinations...); err != nil {
		return MallWeatherExportDataRow{}, fmt.Errorf("mall weather export data: scan row: %w", err)
	}
	row := MallWeatherExportDataRow{Values: make(map[string]interface{}, fieldCount)}
	for index, column := range columns {
		value := normalizeMallWeatherExportSQLValue(values[index])
		switch column {
		case "_cursor_id":
			cursor, ok := mallWeatherExportCursorUint(value)
			if !ok || cursor == 0 {
				return MallWeatherExportDataRow{}, fmt.Errorf("mall weather export data: invalid cursor")
			}
			row.CursorID = cursor
		case "_split_city":
			row.SplitCity = mallWeatherExportString(value)
		case "_split_mall":
			row.SplitMall = mallWeatherExportString(value)
		case "_split_date":
			row.SplitDate = mallWeatherExportString(value)
		case "_split_data_type":
			row.SplitDataType = mallWeatherExportString(value)
		default:
			if _, ok := requestedFields[column]; ok {
				row.Values[column] = value
			}
		}
	}
	if row.CursorID == 0 || len(row.Values) != fieldCount {
		return MallWeatherExportDataRow{}, fmt.Errorf("mall weather export data: incomplete row")
	}
	return row, nil
}

func normalizeMallWeatherExportSQLValue(value interface{}) interface{} {
	if bytes, ok := value.([]byte); ok {
		return string(bytes)
	}
	return value
}

func mallWeatherExportString(value interface{}) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case time.Time:
		return typed.UTC().Format(time.RFC3339Nano)
	default:
		return fmt.Sprint(typed)
	}
}

func mallWeatherExportCursorUint(value interface{}) (uint, bool) {
	var parsed uint64
	var err error
	switch typed := value.(type) {
	case int64:
		if typed <= 0 {
			return 0, false
		}
		parsed = uint64(typed)
	case uint64:
		parsed = typed
	case string:
		parsed, err = strconv.ParseUint(typed, 10, 64)
	default:
		return 0, false
	}
	if err != nil || parsed == 0 || uint64(uint(parsed)) != parsed {
		return 0, false
	}
	return uint(parsed), true
}

func exportDatasetSpec(
	from string,
	joins []string,
	cursorExpression string,
	createdExpression string,
	timeExpression string,
	qualityExpression string,
	issuedExpression string,
	splitDateExpression string,
	splitDataTypeExpression string,
	fields map[string]string,
	defaultFields []string,
	latestPredicate string,
) mallWeatherExportDatasetSpec {
	return mallWeatherExportDatasetSpec{
		from: from, joins: joins, cursorExpression: cursorExpression, createdExpression: createdExpression,
		timeExpression: timeExpression, qualityExpression: qualityExpression, issuedExpression: issuedExpression,
		splitDateExpression: splitDateExpression, splitDataTypeExpression: splitDataTypeExpression,
		fields: fields, defaultFields: defaultFields, latestPredicate: latestPredicate,
	}
}

var mallWeatherExportDatasetCatalog = map[string]mallWeatherExportDatasetSpec{
	"malls": exportDatasetSpec(
		"malls AS m", nil, "m.id", "m.created_at", "", "", "", "m.created_at", "'mall'",
		map[string]string{
			"mall_code": "m.mall_code", "name_cn": "m.name_cn", "name_en": "m.name_en",
			"province": "m.province", "city": "m.city", "district": "m.district",
			"address":   "COALESCE(NULLIF(m.address_standardized, ''), m.address_raw)",
			"longitude": "m.longitude", "latitude": "m.latitude", "coordinate_system": "m.coordinate_system",
			"coverage_radius_m": "m.coverage_radius_m", "status": "m.status",
		},
		[]string{"mall_code", "name_cn", "name_en", "province", "city", "district", "address", "longitude", "latitude", "coordinate_system", "coverage_radius_m", "status"},
		"",
	),
	"realtime": exportDatasetSpec(
		"mall_weather_realtime AS w", []string{"JOIN malls AS m ON m.id = w.mall_id"}, "w.id", "w.created_at",
		"w.snapshot_at_utc", "w.quality_status", "w.fetched_at_utc", "w.snapshot_at_utc", "'realtime'",
		map[string]string{
			"mall_code": "m.mall_code", "snapshot_at": "w.snapshot_at_utc", "temperature_c": "w.temperature_c",
			"apparent_temperature_c": "w.apparent_temperature_c", "humidity_pct": "w.humidity_ratio * 100",
			"pressure_pa": "w.pressure_pa", "wind_speed_kph": "w.wind_speed_kph",
			"precipitation_mm_h": "w.local_precip_mm_h", "aqi_chn": "w.aqi_chn", "aqi_usa": "w.aqi_usa",
			"pm25_ug_m3": "w.pm25_ug_m3", "pm10_ug_m3": "w.pm10_ug_m3", "skycon": "w.skycon",
			"quality_status": "w.quality_status", "issued_at": "w.provider_server_time_utc", "fetched_at": "w.fetched_at_utc",
		},
		[]string{"mall_code", "snapshot_at", "temperature_c", "apparent_temperature_c", "humidity_pct", "pressure_pa", "wind_speed_kph", "precipitation_mm_h", "aqi_chn", "aqi_usa", "pm25_ug_m3", "pm10_ug_m3", "skycon", "quality_status", "issued_at", "fetched_at"},
		"NOT EXISTS (SELECT 1 FROM mall_weather_realtime AS newer WHERE newer.mall_id = w.mall_id AND newer.provider = w.provider AND newer.fetched_at_utc <= ? AND newer.snapshot_at_utc > w.snapshot_at_utc AND newer.created_at <= ?)",
	),
	"minutely": exportDatasetSpec(
		"mall_weather_minutely AS w", []string{"JOIN malls AS m ON m.id = w.mall_id"}, "w.id", "w.created_at",
		"w.forecast_minute_utc", "w.quality_status", "w.issued_at_utc", "w.forecast_minute_utc", "'minutely'",
		map[string]string{
			"mall_code": "m.mall_code", "forecast_minute": "w.forecast_minute_utc", "minute_offset": "w.minute_offset",
			"precipitation_mm_h": "w.precipitation_mm_h", "probability_pct": "w.probability_ratio * 100",
			"description": "w.description", "quality_status": "w.quality_status", "issued_at": "w.issued_at_utc",
			"fetched_at": "w.fetched_at_utc",
		},
		[]string{"mall_code", "forecast_minute", "minute_offset", "precipitation_mm_h", "probability_pct", "description", "quality_status", "issued_at", "fetched_at"},
		"NOT EXISTS (SELECT 1 FROM mall_weather_minutely AS newer WHERE newer.mall_id = w.mall_id AND newer.provider = w.provider AND newer.forecast_minute_utc = w.forecast_minute_utc AND newer.issued_at_utc <= ? AND newer.issued_at_utc > w.issued_at_utc AND newer.created_at <= ?)",
	),
	"hourly": exportDatasetSpec(
		"mall_weather_hourly AS w", []string{"JOIN malls AS m ON m.id = w.mall_id"}, "w.id", "w.created_at",
		"w.forecast_time_utc", "w.quality_status", "w.issued_at_utc", "w.forecast_time_utc", "'hourly'",
		map[string]string{
			"mall_code": "m.mall_code", "forecast_time": "w.forecast_time_utc", "temperature_c": "w.temperature_c",
			"apparent_temperature_c": "w.apparent_temperature_c", "humidity_pct": "w.humidity_ratio * 100",
			"pressure_pa": "w.pressure_pa", "wind_speed_kph": "w.wind_speed_kph", "wind_direction_deg": "w.wind_direction_deg",
			"precipitation_mm_h": "w.precipitation_mm_h", "precipitation_probability_pct": "w.precip_probability_pct",
			"aqi_chn": "w.aqi_chn", "aqi_usa": "w.aqi_usa", "pm25_ug_m3": "w.pm25_ug_m3", "skycon": "w.skycon",
			"quality_status": "w.quality_status", "issued_at": "w.issued_at_utc", "fetched_at": "w.fetched_at_utc",
		},
		[]string{"mall_code", "forecast_time", "temperature_c", "apparent_temperature_c", "humidity_pct", "pressure_pa", "wind_speed_kph", "wind_direction_deg", "precipitation_mm_h", "precipitation_probability_pct", "aqi_chn", "aqi_usa", "pm25_ug_m3", "skycon", "quality_status", "issued_at", "fetched_at"},
		"NOT EXISTS (SELECT 1 FROM mall_weather_hourly AS newer WHERE newer.mall_id = w.mall_id AND newer.provider = w.provider AND newer.forecast_time_utc = w.forecast_time_utc AND newer.issued_at_utc <= ? AND newer.issued_at_utc > w.issued_at_utc AND newer.created_at <= ?)",
	),
	"daily": exportDatasetSpec(
		"mall_weather_daily AS w", []string{"JOIN malls AS m ON m.id = w.mall_id"}, "w.id", "w.created_at",
		"w.forecast_date_local", "w.quality_status", "w.issued_at_utc", "w.forecast_date_local", "'daily'",
		map[string]string{
			"mall_code": "m.mall_code", "forecast_date": "w.forecast_date_local", "temperature_max_c": "w.temperature_max_c",
			"temperature_min_c": "w.temperature_min_c", "humidity_avg_pct": "w.humidity_avg_ratio * 100",
			"precipitation_probability_pct": "w.precipitation_probability_pct", "aqi_avg_chn": "w.aqi_avg_chn",
			"pm25_avg_ug_m3": "w.pm25_avg_ug_m3", "skycon": "w.skycon", "sunrise": "w.sunrise_local_time",
			"sunset": "w.sunset_local_time", "quality_status": "w.quality_status", "issued_at": "w.issued_at_utc",
			"fetched_at": "w.fetched_at_utc",
		},
		[]string{"mall_code", "forecast_date", "temperature_max_c", "temperature_min_c", "humidity_avg_pct", "precipitation_probability_pct", "aqi_avg_chn", "pm25_avg_ug_m3", "skycon", "sunrise", "sunset", "quality_status", "issued_at", "fetched_at"},
		"NOT EXISTS (SELECT 1 FROM mall_weather_daily AS newer WHERE newer.mall_id = w.mall_id AND newer.provider = w.provider AND newer.forecast_date_local = w.forecast_date_local AND newer.issued_at_utc <= ? AND newer.issued_at_utc > w.issued_at_utc AND newer.created_at <= ?)",
	),
	"alerts": exportDatasetSpec(
		"mall_weather_alert_relations AS relation", []string{"JOIN mall_weather_alerts AS w ON w.id = relation.alert_pk", "JOIN malls AS m ON m.id = relation.mall_id"},
		"relation.id", "relation.created_at", "COALESCE(w.published_at_utc, w.first_seen_at)", "w.quality_status", "COALESCE(w.published_at_utc, w.first_seen_at)",
		"COALESCE(w.published_at_utc, w.first_seen_at)", "w.alert_type_code",
		map[string]string{
			"mall_code": "m.mall_code", "alert_id": "w.alert_id", "status": "w.status", "title": "w.title",
			"description": "w.description", "source": "w.source", "published_at": "w.published_at_utc",
			"ended_at": "w.ended_at", "province": "w.province", "city": "w.city", "county": "w.county",
			"quality_status": "w.quality_status",
		},
		[]string{"mall_code", "alert_id", "status", "title", "description", "source", "published_at", "ended_at", "province", "city", "county", "quality_status"},
		"",
	),
	"life_indices": exportDatasetSpec(
		"mall_weather_life_indices AS w", []string{"JOIN malls AS m ON m.id = w.mall_id"}, "w.id", "w.created_at",
		"w.forecast_date_local", "w.quality_status", "w.issued_at_utc", "w.forecast_date_local", "w.index_code",
		map[string]string{
			"mall_code": "m.mall_code", "forecast_date": "w.forecast_date_local", "source_api": "w.source_api",
			"index_type": "w.index_type", "index_code": "w.index_code", "index_name": "w.index_name", "level": "w.level",
			"short_desc": "w.short_desc", "detail": "w.detail", "is_unknown_type": "w.is_unknown_type",
			"quality_status": "w.quality_status", "issued_at": "w.issued_at_utc", "fetched_at": "w.fetched_at_utc",
		},
		[]string{"mall_code", "forecast_date", "source_api", "index_type", "index_code", "index_name", "level", "short_desc", "detail", "is_unknown_type", "quality_status", "issued_at", "fetched_at"},
		"NOT EXISTS (SELECT 1 FROM mall_weather_life_indices AS newer WHERE newer.mall_id = w.mall_id AND newer.provider = w.provider AND newer.source_api = w.source_api AND newer.forecast_date_local = w.forecast_date_local AND newer.index_type = w.index_type AND newer.issued_at_utc <= ? AND newer.issued_at_utc > w.issued_at_utc AND newer.created_at <= ?)",
	),
	"fetch_runs": exportDatasetSpec(
		"mall_weather_fetch_runs AS w", []string{"JOIN malls AS m ON m.id = w.mall_id"}, "w.id", "w.created_at",
		"w.created_at", "", "", "w.created_at", "w.endpoint_kind",
		map[string]string{
			"mall_code": "m.mall_code", "run_uuid": "w.run_uuid", "task_kind": "w.task_kind", "endpoint_kind": "w.endpoint_kind",
			"status": "w.status", "attempt_count": "w.attempt_count", "duration_ms": "w.duration_ms", "http_status": "w.http_status",
			"provider_status": "w.provider_status", "row_counts": "w.row_counts_json", "error_class": "w.error_class",
			"error_code": "w.error_code", "started_at": "w.started_at", "finished_at": "w.finished_at",
		},
		[]string{"mall_code", "run_uuid", "task_kind", "endpoint_kind", "status", "attempt_count", "duration_ms", "http_status", "provider_status", "row_counts", "error_class", "error_code", "started_at", "finished_at"},
		"",
	),
}
