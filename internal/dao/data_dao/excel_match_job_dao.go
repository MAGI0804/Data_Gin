package data_dao

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"

	"gorm.io/gorm"
)

type ExcelMatchJobDAO struct {
	db *gorm.DB
}

var allowedBojunExcelFields = map[string]struct{}{
	"billdate":             {},
	"c_store_code":         {},
	"c_store_name":         {},
	"docno":                {},
	"matched_docno":        {},
	"o2o_so_docno":         {},
	"order_type_code":      {},
	"order_type_name":      {},
	"otherdocno":           {},
	"related_normal_docno": {},
	"retailbilltype":       {},
	"tot_amt_actual":       {},
	"tot_amt_list":         {},
	"tot_qty":              {},
	"vipno":                {},
}

var excelSQLIdentifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func NewExcelMatchJobDAO() *ExcelMatchJobDAO {
	return &ExcelMatchJobDAO{db: database.DB}
}

func (dao *ExcelMatchJobDAO) Create(ctx context.Context, job *model.ExcelMatchJob) (uint, error) {
	now := int(time.Now().Unix())
	job.CreatedAt = now
	job.UpdatedAt = now
	err := dao.db.WithContext(ctx).Create(job).Error
	return job.ID, err
}

func (dao *ExcelMatchJobDAO) FindByID(ctx context.Context, id uint) (*model.ExcelMatchJob, error) {
	var job model.ExcelMatchJob
	err := dao.db.WithContext(ctx).First(&job, id).Error
	return &job, err
}

func (dao *ExcelMatchJobDAO) ListJobs(ctx context.Context, limit int) ([]model.ExcelMatchJob, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	var jobs []model.ExcelMatchJob
	err := dao.db.WithContext(ctx).
		Order("id DESC").
		Limit(limit).
		Find(&jobs).Error
	return jobs, err
}

func (dao *ExcelMatchJobDAO) CreateLog(ctx context.Context, log *model.ExcelMatchJobLog) error {
	now := int(time.Now().Unix())
	log.CreatedAt = now
	log.UpdatedAt = now
	return dao.db.WithContext(ctx).Create(log).Error
}

func (dao *ExcelMatchJobDAO) FindLogsByJobID(ctx context.Context, jobID uint, limit int) ([]model.ExcelMatchJobLog, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	var logs []model.ExcelMatchJobLog
	err := dao.db.WithContext(ctx).
		Where("job_id = ?", jobID).
		Order("id ASC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}

func (dao *ExcelMatchJobDAO) ListSchemes(ctx context.Context, operation string) ([]model.ExcelMatchScheme, error) {
	var schemes []model.ExcelMatchScheme
	query := dao.db.WithContext(ctx).Model(&model.ExcelMatchScheme{})
	if operation != "" {
		query = query.Where("operation = ?", operation)
	}
	err := query.Order("updated_at DESC, id DESC").Find(&schemes).Error
	return schemes, err
}

func (dao *ExcelMatchJobDAO) SaveScheme(ctx context.Context, scheme *model.ExcelMatchScheme) error {
	now := int(time.Now().Unix())
	var existing model.ExcelMatchScheme
	err := dao.db.WithContext(ctx).
		Where("operation = ? AND name = ?", scheme.Operation, scheme.Name).
		First(&existing).Error
	if err == nil {
		return dao.db.WithContext(ctx).
			Model(&model.ExcelMatchScheme{}).
			Where("id = ?", existing.ID).
			Updates(map[string]interface{}{
				"config_json": scheme.ConfigJSON,
				"updated_at":  now,
			}).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	scheme.CreatedAt = now
	scheme.UpdatedAt = now
	return dao.db.WithContext(ctx).Create(scheme).Error
}

func (dao *ExcelMatchJobDAO) DeleteScheme(ctx context.Context, id uint) error {
	return dao.db.WithContext(ctx).Delete(&model.ExcelMatchScheme{}, id).Error
}

func (dao *ExcelMatchJobDAO) UpdatePaths(ctx context.Context, id uint, workDir, sourcePath, resultPath string) error {
	return dao.db.WithContext(ctx).
		Model(&model.ExcelMatchJob{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"work_dir":         workDir,
			"source_file_path": sourcePath,
			"result_file_path": resultPath,
			"updated_at":       time.Now().Unix(),
		}).Error
}

func (dao *ExcelMatchJobDAO) UpdateResultStorage(ctx context.Context, id uint, objectKey, resultURL string) error {
	return dao.db.WithContext(ctx).
		Model(&model.ExcelMatchJob{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"result_object_key": objectKey,
			"result_url":        resultURL,
			"updated_at":        time.Now().Unix(),
		}).Error
}

func (dao *ExcelMatchJobDAO) MarkRunning(ctx context.Context, id uint) error {
	now := time.Now()
	return dao.db.WithContext(ctx).
		Model(&model.ExcelMatchJob{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     "running",
			"started_at": model.TimeNormal{Time: now},
			"updated_at": now.Unix(),
		}).Error
}

func (dao *ExcelMatchJobDAO) UpdateProgress(ctx context.Context, id uint, stats ExcelMatchJobStats) error {
	return dao.db.WithContext(ctx).
		Model(&model.ExcelMatchJob{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"total_rows":     stats.TotalRows,
			"processed_rows": stats.ProcessedRows,
			"filtered_rows":  stats.FilteredRows,
			"matched_rows":   stats.MatchedRows,
			"unmatched_rows": stats.UnmatchedRows,
			"updated_at":     time.Now().Unix(),
		}).Error
}

func (dao *ExcelMatchJobDAO) MarkSuccess(ctx context.Context, id uint, stats ExcelMatchJobStats, expiresAt time.Time) error {
	now := time.Now()
	return dao.db.WithContext(ctx).
		Model(&model.ExcelMatchJob{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":         "success",
			"total_rows":     stats.TotalRows,
			"processed_rows": stats.ProcessedRows,
			"filtered_rows":  stats.FilteredRows,
			"matched_rows":   stats.MatchedRows,
			"unmatched_rows": stats.UnmatchedRows,
			"finished_at":    model.TimeNormal{Time: now},
			"expires_at":     model.TimeNormal{Time: expiresAt},
			"updated_at":     now.Unix(),
		}).Error
}

func (dao *ExcelMatchJobDAO) MarkFailed(ctx context.Context, id uint, errMessage string, expiresAt time.Time) error {
	now := time.Now()
	return dao.db.WithContext(ctx).
		Model(&model.ExcelMatchJob{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":        "failed",
			"error_message": errMessage,
			"finished_at":   model.TimeNormal{Time: now},
			"expires_at":    model.TimeNormal{Time: expiresAt},
			"updated_at":    now.Unix(),
		}).Error
}

func (dao *ExcelMatchJobDAO) MarkExpired(ctx context.Context, id uint) error {
	return dao.db.WithContext(ctx).
		Model(&model.ExcelMatchJob{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     "expired",
			"updated_at": time.Now().Unix(),
		}).Error
}

func (dao *ExcelMatchJobDAO) FindExpired(ctx context.Context, now time.Time, limit int) ([]model.ExcelMatchJob, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	var jobs []model.ExcelMatchJob
	err := dao.db.WithContext(ctx).
		Where("expires_at IS NOT NULL AND expires_at <= ? AND status IN ?", now, []string{"success", "failed"}).
		Order("id ASC").
		Limit(limit).
		Find(&jobs).Error
	return jobs, err
}

type ExcelMatchJobStats struct {
	TotalRows     int
	ProcessedRows int
	FilteredRows  int
	MatchedRows   int
	UnmatchedRows int
}

// ExcelMatchModelColumn describes one application database column for the
// Excel matching model catalog. The catalog query returns every model and
// field in one round trip so the web UI can render cascading selectors.
type ExcelMatchModelColumn struct {
	TableName       string `gorm:"column:table_name"`
	TableComment    string `gorm:"column:table_comment"`
	ColumnName      string `gorm:"column:column_name"`
	DataType        string `gorm:"column:data_type"`
	ColumnType      string `gorm:"column:column_type"`
	ColumnComment   string `gorm:"column:column_comment"`
	OrdinalPosition int    `gorm:"column:ordinal_position"`
	IsNullable      string `gorm:"column:is_nullable"`
}

// ListModelColumns reads the current MySQL schema with a single joined query.
// Unknown/custom tables are intentionally included because Excel matching is
// configurable and may target tables that do not have a Go model yet.
func (dao *ExcelMatchJobDAO) ListModelColumns(ctx context.Context) ([]ExcelMatchModelColumn, error) {
	if dao.db == nil {
		return nil, errors.New("数据库未初始化")
	}

	var columns []ExcelMatchModelColumn
	err := dao.db.WithContext(ctx).Raw(`
		SELECT
			c.TABLE_NAME AS table_name,
			COALESCE(t.TABLE_COMMENT, '') AS table_comment,
			c.COLUMN_NAME AS column_name,
			c.DATA_TYPE AS data_type,
			c.COLUMN_TYPE AS column_type,
			COALESCE(c.COLUMN_COMMENT, '') AS column_comment,
			c.ORDINAL_POSITION AS ordinal_position,
			c.IS_NULLABLE AS is_nullable
		FROM information_schema.COLUMNS AS c
		INNER JOIN information_schema.TABLES AS t
			ON t.TABLE_SCHEMA = c.TABLE_SCHEMA
			AND t.TABLE_NAME = c.TABLE_NAME
		WHERE c.TABLE_SCHEMA = DATABASE()
			AND t.TABLE_TYPE = 'BASE TABLE'
		ORDER BY c.TABLE_NAME ASC, c.ORDINAL_POSITION ASC
	`).Scan(&columns).Error
	return columns, err
}

func (dao *ExcelMatchJobDAO) FindBojunFieldByKeys(ctx context.Context, matchField string, keys []string, valueField string) (map[string]string, error) {
	result := make(map[string]string, len(keys))
	if len(keys) == 0 {
		return result, nil
	}
	if !isAllowedBojunExcelField(matchField) {
		return nil, fmt.Errorf("bojun match field is not allowed: %s", matchField)
	}
	if !isAllowedBojunExcelField(valueField) {
		return nil, fmt.Errorf("bojun field is not allowed: %s", valueField)
	}

	query := fmt.Sprintf(
		"SELECT CAST(%s AS CHAR) AS match_key, CAST(%s AS CHAR) AS matched_value FROM bojun_retail_orders WHERE %s IN ?",
		matchField,
		valueField,
		matchField,
	)
	rows, err := dao.db.WithContext(ctx).Raw(query, keys).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var key, value sql.NullString
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		if key.Valid {
			if value.Valid {
				result[key.String] = value.String
			} else {
				result[key.String] = ""
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func (dao *ExcelMatchJobDAO) ValidateTableColumns(ctx context.Context, tableName string, columns []string) error {
	if !isSafeExcelSQLIdentifier(tableName) {
		return fmt.Errorf("数据库表名不合法: %s", tableName)
	}
	for _, column := range columns {
		if !isSafeExcelSQLIdentifier(column) {
			return fmt.Errorf("数据库字段名不合法: %s", column)
		}
	}
	if dao.db == nil {
		return errors.New("数据库未初始化")
	}
	if !dao.db.WithContext(ctx).Migrator().HasTable(tableName) {
		return fmt.Errorf("数据库表不存在: %s", tableName)
	}
	columnTypes, err := dao.db.WithContext(ctx).Migrator().ColumnTypes(tableName)
	if err != nil {
		return err
	}
	existing := make(map[string]struct{}, len(columnTypes))
	for _, columnType := range columnTypes {
		existing[columnType.Name()] = struct{}{}
	}
	for _, column := range columns {
		if _, ok := existing[column]; !ok {
			return fmt.Errorf("数据库表 %s 不存在字段: %s", tableName, column)
		}
	}
	return nil
}

func (dao *ExcelMatchJobDAO) FindTableFieldByKeys(ctx context.Context, tableName, matchField string, keys []string, valueField string) (map[string]string, error) {
	result := make(map[string]string, len(keys))
	if len(keys) == 0 {
		return result, nil
	}
	if !isSafeExcelSQLIdentifier(tableName) || !isSafeExcelSQLIdentifier(matchField) || !isSafeExcelSQLIdentifier(valueField) {
		return nil, errors.New("数据库表名或字段名不合法")
	}
	query := fmt.Sprintf(
		"SELECT CAST(`%s` AS CHAR) AS match_key, CAST(`%s` AS CHAR) AS matched_value FROM `%s` WHERE `%s` IN ?",
		matchField,
		valueField,
		tableName,
		matchField,
	)
	rows, err := dao.db.WithContext(ctx).Raw(query, keys).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var key, value sql.NullString
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		if key.Valid {
			result[key.String] = value.String
		}
	}
	return result, rows.Err()
}

func (dao *ExcelMatchJobDAO) FindBojunKeys(ctx context.Context, matchField string, keys []string) (map[string]struct{}, error) {
	result := make(map[string]struct{}, len(keys))
	if len(keys) == 0 {
		return result, nil
	}
	if !isAllowedBojunExcelField(matchField) {
		return nil, fmt.Errorf("bojun match field is not allowed: %s", matchField)
	}

	query := fmt.Sprintf("SELECT CAST(%s AS CHAR) AS match_key FROM bojun_retail_orders WHERE %s IN ?", matchField, matchField)
	rows, err := dao.db.WithContext(ctx).Raw(query, keys).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var key sql.NullString
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		if key.Valid {
			result[key.String] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (dao *ExcelMatchJobDAO) BatchUpdateBojunFieldByKeys(ctx context.Context, matchField, writeField string, values map[string]string) (int64, error) {
	if len(values) == 0 {
		return 0, nil
	}
	if !isAllowedBojunExcelField(matchField) || writeField != "matched_docno" {
		return 0, fmt.Errorf("bojun update field is not allowed")
	}

	keys := make([]string, 0, len(values))
	caseSQL := strings.Builder{}
	args := make([]interface{}, 0, len(values)*2+2)
	for key, value := range values {
		keys = append(keys, key)
		caseSQL.WriteString(" WHEN ? THEN ?")
		args = append(args, key, value)
	}
	args = append(args, time.Now().Unix(), keys)

	query := fmt.Sprintf(
		"UPDATE bojun_retail_orders SET %s = CASE %s%s ELSE %s END, updated_at = ? WHERE %s IN ?",
		writeField,
		matchField,
		caseSQL.String(),
		writeField,
		matchField,
	)
	result := dao.db.WithContext(ctx).Exec(query, args...)
	return result.RowsAffected, result.Error
}

func (dao *ExcelMatchJobDAO) UpdateBojunFieldByKey(ctx context.Context, matchField, key, writeField, value string) (int64, error) {
	if key == "" {
		return 0, nil
	}
	if !isAllowedBojunExcelField(matchField) || writeField != "matched_docno" {
		return 0, fmt.Errorf("bojun update field is not allowed")
	}

	query := fmt.Sprintf(
		"UPDATE bojun_retail_orders SET %s = ? WHERE %s = ?",
		writeField,
		matchField,
	)
	args := []interface{}{value, key}
	if writeField == "matched_docno" && value != "" {
		query += " AND (matched_docno IS NULL OR matched_docno = '')"
	}
	result := dao.db.WithContext(ctx).Exec(query, args...)
	return result.RowsAffected, result.Error
}

func isAllowedBojunExcelField(field string) bool {
	_, ok := allowedBojunExcelFields[field]
	return ok
}

func isSafeExcelSQLIdentifier(value string) bool {
	return excelSQLIdentifierPattern.MatchString(value)
}
