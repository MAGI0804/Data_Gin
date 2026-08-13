package reportrepo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gin-biz-web-api/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const maxReportExportCleanupBatchSize = 500

var ErrReportExportCleanupLeaseLost = errors.New("report export cleanup: lease lost")

type ExportCleanupCandidate struct {
	ID              uint   `gorm:"column:id"`
	ExportUUID      string `gorm:"column:export_uuid"`
	ResultObjectKey string `gorm:"column:result_object_key"`
}

func (repository *Repository) ListExportCleanupCandidates(
	ctx context.Context,
	now time.Time,
	afterID uint,
	limit int,
) ([]ExportCleanupCandidate, error) {
	if repository == nil || repository.db == nil || ctx == nil || now.IsZero() ||
		limit < 1 || limit > maxReportExportCleanupBatchSize {
		return nil, fmt.Errorf("report export cleanup: invalid candidate query")
	}
	var candidates []ExportCleanupCandidate
	err := repository.exportCleanupCandidateQuery(ctx, now).
		Select("id", "export_uuid", "result_object_key").
		Where("id > ?", afterID).
		Order("id ASC").
		Limit(limit).
		Find(&candidates).Error
	if err != nil {
		return nil, fmt.Errorf("report export cleanup: list candidates: %w", err)
	}
	return candidates, nil
}

func (repository *Repository) ClaimExportCleanup(
	ctx context.Context,
	candidate ExportCleanupCandidate,
	leaseToken string,
	now time.Time,
	leaseTTL time.Duration,
) (bool, error) {
	if !repository.validExportCleanupLease(ctx, candidate, leaseToken) || now.IsZero() || leaseTTL <= 0 {
		return false, fmt.Errorf("report export cleanup: invalid claim")
	}
	now = now.UTC().Truncate(time.Millisecond)
	result := repository.exportCleanupCandidateQuery(ctx, now).
		Where("id = ? AND export_uuid = ? AND result_object_key = ?", candidate.ID, candidate.ExportUUID, candidate.ResultObjectKey).
		Updates(map[string]interface{}{
			"worker_id":        "report-export-cleanup",
			"lease_token":      leaseToken,
			"lease_expires_at": now.Add(leaseTTL).UTC().Truncate(time.Millisecond),
			"heartbeat_at":     now,
			"updated_at":       now,
		})
	if result.Error != nil {
		return false, fmt.Errorf("report export cleanup: claim: %w", result.Error)
	}
	return result.RowsAffected == 1, nil
}

func (repository *Repository) FinishExportCleanup(
	ctx context.Context,
	candidate ExportCleanupCandidate,
	leaseToken string,
	finishedAt time.Time,
) error {
	if !repository.validExportCleanupLease(ctx, candidate, leaseToken) || finishedAt.IsZero() {
		return fmt.Errorf("report export cleanup: invalid finish time")
	}
	finishedAt = finishedAt.UTC().Truncate(time.Millisecond)
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.ReportExport{}).
			Where("id = ? AND export_uuid = ? AND status = ?", candidate.ID, candidate.ExportUUID, model.ReportExportStatusReady).
			Where("result_object_key = ? AND purged_at IS NOT NULL AND lease_token = ?", candidate.ResultObjectKey, leaseToken).
			Updates(map[string]interface{}{
				"status":            model.ReportExportStatusExpired,
				"result_object_key": "",
				"worker_id":         "",
				"lease_token":       "",
				"lease_expires_at":  nil,
				"heartbeat_at":      nil,
				"updated_at":        finishedAt,
			})
		if result.Error != nil {
			return fmt.Errorf("report export cleanup: finish update: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return ErrReportExportCleanupLeaseLost
		}
		return repository.writeSystemAudit(ctx, tx, "REPORT_EXPORT_FILE_EXPIRED", "REPORT_EXPORT", candidate.ID, nil)
	})
}

func (repository *Repository) ReleaseExportCleanup(
	ctx context.Context,
	candidate ExportCleanupCandidate,
	leaseToken string,
	releasedAt time.Time,
) error {
	if releasedAt.IsZero() {
		return fmt.Errorf("report export cleanup: invalid release time")
	}
	return repository.updateExportCleanupLease(ctx, candidate, leaseToken, map[string]interface{}{
		"worker_id":        "",
		"lease_token":      "",
		"lease_expires_at": nil,
		"heartbeat_at":     nil,
		"updated_at":       releasedAt.UTC().Truncate(time.Millisecond),
	})
}

func (repository *Repository) exportCleanupCandidateQuery(ctx context.Context, now time.Time) *gorm.DB {
	now = now.UTC().Truncate(time.Millisecond)
	return repository.db.WithContext(ctx).Model(&model.ReportExport{}).
		Where("status = ?", model.ReportExportStatusReady).
		Where("expires_at IS NOT NULL AND expires_at <= ?", now).
		Where("purged_at IS NOT NULL").
		Where("result_object_key <> ?", "").
		Where("lease_token IS NULL OR lease_token = ? OR lease_expires_at IS NULL OR lease_expires_at <= ?", "", now)
}

func (repository *Repository) exportCleanupLeaseQuery(
	ctx context.Context,
	candidate ExportCleanupCandidate,
	leaseToken string,
) *gorm.DB {
	return repository.db.WithContext(ctx).Model(&model.ReportExport{}).
		Where("id = ? AND export_uuid = ? AND status = ?", candidate.ID, candidate.ExportUUID, model.ReportExportStatusReady).
		Where("result_object_key = ? AND purged_at IS NOT NULL AND lease_token = ?", candidate.ResultObjectKey, leaseToken)
}

func (repository *Repository) updateExportCleanupLease(
	ctx context.Context,
	candidate ExportCleanupCandidate,
	leaseToken string,
	updates map[string]interface{},
) error {
	if !repository.validExportCleanupLease(ctx, candidate, leaseToken) || len(updates) == 0 {
		return fmt.Errorf("report export cleanup: invalid lease update")
	}
	result := repository.exportCleanupLeaseQuery(ctx, candidate, leaseToken).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("report export cleanup: update lease: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrReportExportCleanupLeaseLost
	}
	return nil
}

func (repository *Repository) validExportCleanupLease(
	ctx context.Context,
	candidate ExportCleanupCandidate,
	leaseToken string,
) bool {
	return repository != nil && repository.db != nil && ctx != nil && candidate.ID > 0 &&
		uuid.Validate(candidate.ExportUUID) == nil && strings.TrimSpace(candidate.ResultObjectKey) != "" &&
		uuid.Validate(leaseToken) == nil
}
