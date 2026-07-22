package data_dao

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gin-biz-web-api/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const maxMallWeatherExportCleanupBatchSize = 500

var ErrMallWeatherExportCleanupLeaseLost = errors.New("mall weather export cleanup: lease lost")

type MallWeatherExportCleanupCandidate struct {
	ID              uint   `gorm:"column:id"`
	ResultObjectKey string `gorm:"column:result_object_key"`
}

type mallWeatherExportCleanupCheckpoint struct {
	CleanupToken string `json:"cleanupToken"`
}

func (dao *MallWeatherExportJobDAO) ListCleanupCandidates(
	ctx context.Context,
	now time.Time,
	staleBefore time.Time,
	afterID uint,
	limit int,
) ([]MallWeatherExportCleanupCandidate, error) {
	if dao == nil || dao.db == nil || ctx == nil || now.IsZero() || staleBefore.IsZero() ||
		!staleBefore.Before(now) || limit < 1 || limit > maxMallWeatherExportCleanupBatchSize {
		return nil, fmt.Errorf("mall weather export cleanup: invalid candidate query")
	}
	var rows []MallWeatherExportCleanupCandidate
	err := dao.cleanupCandidateQuery(ctx, now, staleBefore).
		Select("id", "result_object_key").
		Where("id > ?", afterID).
		Order("id ASC").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("mall weather export cleanup: list candidates: %w", err)
	}
	return rows, nil
}

func (dao *MallWeatherExportJobDAO) ClaimCleanup(
	ctx context.Context,
	candidate MallWeatherExportCleanupCandidate,
	cleanupToken string,
	now time.Time,
	staleBefore time.Time,
) (bool, error) {
	if dao == nil || dao.db == nil || ctx == nil || candidate.ID == 0 ||
		!validMallWeatherExportCleanupToken(cleanupToken) || now.IsZero() || staleBefore.IsZero() ||
		!staleBefore.Before(now) {
		return false, fmt.Errorf("mall weather export cleanup: invalid claim")
	}
	checkpoint, err := json.Marshal(mallWeatherExportCleanupCheckpoint{CleanupToken: cleanupToken})
	if err != nil {
		return false, fmt.Errorf("mall weather export cleanup: encode claim: %w", err)
	}
	result := dao.cleanupClaimQuery(ctx, candidate, now, staleBefore).
		Updates(map[string]interface{}{
			"status":           "expired",
			"last_cursor_json": model.JSONText(checkpoint),
			"updated_at":       now.UTC().Truncate(time.Millisecond),
		})
	if result.Error != nil {
		return false, fmt.Errorf("mall weather export cleanup: claim: %w", result.Error)
	}
	return result.RowsAffected == 1, nil
}

func (dao *MallWeatherExportJobDAO) FinishCleanup(
	ctx context.Context,
	candidate MallWeatherExportCleanupCandidate,
	cleanupToken string,
	finishedAt time.Time,
) error {
	if finishedAt.IsZero() {
		return fmt.Errorf("mall weather export cleanup: invalid finish time")
	}
	return dao.finishCleanupLease(ctx, candidate, cleanupToken, map[string]interface{}{
		"result_object_key": "",
		"last_cursor_json":  model.JSONText(`{}`),
		"updated_at":        finishedAt.UTC().Truncate(time.Millisecond),
	})
}

func (dao *MallWeatherExportJobDAO) ReleaseCleanup(
	ctx context.Context,
	candidate MallWeatherExportCleanupCandidate,
	cleanupToken string,
	releasedAt time.Time,
) error {
	if releasedAt.IsZero() {
		return fmt.Errorf("mall weather export cleanup: invalid release time")
	}
	return dao.finishCleanupLease(ctx, candidate, cleanupToken, map[string]interface{}{
		"last_cursor_json": model.JSONText(`{}`),
		"updated_at":       releasedAt.UTC().Truncate(time.Millisecond),
	})
}

func (dao *MallWeatherExportJobDAO) finishCleanupLease(
	ctx context.Context,
	candidate MallWeatherExportCleanupCandidate,
	cleanupToken string,
	updates map[string]interface{},
) error {
	if dao == nil || dao.db == nil || ctx == nil || candidate.ID == 0 ||
		!validMallWeatherExportCleanupToken(cleanupToken) || len(updates) == 0 {
		return fmt.Errorf("mall weather export cleanup: invalid lease update")
	}
	result := dao.cleanupLeaseQuery(ctx, candidate, cleanupToken).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("mall weather export cleanup: update lease: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrMallWeatherExportCleanupLeaseLost
	}
	return nil
}

func (dao *MallWeatherExportJobDAO) cleanupCandidateQuery(
	ctx context.Context,
	now time.Time,
	staleBefore time.Time,
) *gorm.DB {
	const cleanupTokenSQL = "JSON_UNQUOTE(JSON_EXTRACT(last_cursor_json, '$.cleanupToken'))"
	return dao.db.WithContext(ctx).Model(&model.MallWeatherExportJob{}).
		Where("expires_at IS NOT NULL AND expires_at <= ?", now.UTC().Truncate(time.Millisecond)).
		Where(
			"(status IN ? OR (status = ? AND result_object_key <> ? AND ("+cleanupTokenSQL+" IS NULL OR "+
				cleanupTokenSQL+" = ? OR updated_at <= ?)))",
			[]string{"succeeded", "failed", "cancelled"},
			"expired",
			"",
			"",
			staleBefore.UTC().Truncate(time.Millisecond),
		)
}

func (dao *MallWeatherExportJobDAO) cleanupClaimQuery(
	ctx context.Context,
	candidate MallWeatherExportCleanupCandidate,
	now time.Time,
	staleBefore time.Time,
) *gorm.DB {
	return dao.cleanupCandidateQuery(ctx, now, staleBefore).
		Where("id = ? AND result_object_key = ?", candidate.ID, candidate.ResultObjectKey)
}

func (dao *MallWeatherExportJobDAO) cleanupLeaseQuery(
	ctx context.Context,
	candidate MallWeatherExportCleanupCandidate,
	cleanupToken string,
) *gorm.DB {
	return dao.db.WithContext(ctx).Model(&model.MallWeatherExportJob{}).
		Where("id = ? AND status = ? AND result_object_key = ?", candidate.ID, "expired", candidate.ResultObjectKey).
		Where("JSON_UNQUOTE(JSON_EXTRACT(last_cursor_json, '$.cleanupToken')) = ?", cleanupToken)
}

func validMallWeatherExportCleanupToken(value string) bool {
	return len(value) == 36 && uuid.Validate(value) == nil
}
