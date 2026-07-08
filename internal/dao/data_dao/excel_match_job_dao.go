package data_dao

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"

	"gorm.io/gorm"
)

type ExcelMatchJobDAO struct {
	db *gorm.DB
}

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

func (dao *ExcelMatchJobDAO) FindBojunFieldByDocNos(ctx context.Context, docNos []string, valueField string) (map[string]string, error) {
	result := make(map[string]string, len(docNos))
	if len(docNos) == 0 {
		return result, nil
	}

	query := fmt.Sprintf("SELECT docno, CAST(%s AS CHAR) AS matched_value FROM bojun_retail_orders WHERE docno IN ?", valueField)
	rows, err := dao.db.WithContext(ctx).Raw(query, docNos).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var docNo, value sql.NullString
		if err := rows.Scan(&docNo, &value); err != nil {
			return nil, err
		}
		if docNo.Valid {
			if value.Valid {
				result[docNo.String] = value.String
			} else {
				result[docNo.String] = ""
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}
