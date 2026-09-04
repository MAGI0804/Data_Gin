package reportrepo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"gin-biz-web-api/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrReportExportExecutionNotFound = errors.New("report export execution: not found")
	ErrReportExportLeaseLost         = errors.New("report export execution: lease lost")
	ErrReportExportResultUnavailable = errors.New("report export execution: result unavailable")
	ErrReportResultPurgeConflict     = errors.New("report result purge: state conflict")
	reportExportChecksumPattern      = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

const (
	maxReportExportCheckpointBytes = 64 * 1024
	defaultResultPurgeLeaseTTL     = 10 * time.Minute
)

type ExportDisposition uint8

const (
	ExportDispositionUnknown ExportDisposition = iota
	ExportDispositionAcquired
	ExportDispositionBusy
	ExportDispositionTerminal
)

type ExportControl uint8

const (
	ExportControlUnknown ExportControl = iota
	ExportControlContinue
	ExportControlCancelRequested
	ExportControlLeaseLost
)

type ExportLease struct {
	Disposition ExportDisposition
	Export      model.ReportExport
	Run         model.ReportRun
}

type ExportRuntime struct {
	Export     model.ReportExport
	Run        model.ReportRun
	Version    model.ReportVersion
	Datasource model.ReportDatasource
}

func (repository *Repository) BeginExport(ctx context.Context, exportID uint, workerID, leaseToken string, now time.Time, leaseTTL time.Duration) (*ExportLease, error) {
	if repository == nil || repository.db == nil || ctx == nil || exportID == 0 || strings.TrimSpace(workerID) == "" ||
		len(workerID) > 128 || uuid.Validate(leaseToken) != nil || now.IsZero() || leaseTTL <= 0 {
		return nil, fmt.Errorf("report export execution: invalid lease input")
	}
	now = now.UTC().Truncate(time.Millisecond)
	lease := &ExportLease{}
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&lease.Export, exportID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrReportExportExecutionNotFound
		} else if err != nil {
			return fmt.Errorf("report export execution: lock export: %w", err)
		}
		if err := tx.First(&lease.Run, lease.Export.RunID).Error; err != nil {
			return fmt.Errorf("report export execution: load run: %w", err)
		}
		disposition, err := classifyExportStart(lease.Export, now)
		if err != nil {
			return err
		}
		lease.Disposition = disposition
		if disposition != ExportDispositionAcquired {
			return nil
		}
		if lease.Export.Status == model.ReportExportStatusRunning && lease.Export.PurgeStartedAt != nil {
			return ErrReportExportResultUnavailable
		}
		if lease.Run.Status != model.ReportRunStatusSucceeded || lease.Run.ResultPurgedAt != nil ||
			lease.Run.ResultExpiresAt == nil || !now.Before(lease.Run.ResultExpiresAt.UTC()) {
			return ErrReportExportResultUnavailable
		}
		expiresAt := now.Add(leaseTTL).UTC().Truncate(time.Millisecond)
		updates := map[string]interface{}{
			"status": model.ReportExportStatusRunning, "worker_id": strings.TrimSpace(workerID), "lease_token": leaseToken,
			"lease_expires_at": expiresAt, "heartbeat_at": now, "attempt": gorm.Expr("attempt + 1"),
			"error_code": "", "error_message_safe": "", "updated_at": now,
		}
		if lease.Export.StartedAt == nil {
			updates["started_at"] = now
			lease.Export.StartedAt = &now
		}
		result := tx.Model(&model.ReportExport{}).Where("id = ? AND status IN ?", exportID, []string{model.ReportExportStatusPending, model.ReportExportStatusRunning}).Updates(updates)
		if result.Error != nil {
			return fmt.Errorf("report export execution: claim export: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return ErrReportExportLeaseLost
		}
		lease.Export.Status = model.ReportExportStatusRunning
		lease.Export.WorkerID = strings.TrimSpace(workerID)
		lease.Export.LeaseToken = leaseToken
		lease.Export.LeaseExpiresAt = &expiresAt
		lease.Export.HeartbeatAt = &now
		lease.Export.Attempt++
		return repository.writeSystemAudit(ctx, tx, "REPORT_EXPORT_STARTED", "REPORT_EXPORT", exportID, map[string]interface{}{"attempt": lease.Export.Attempt})
	})
	if err != nil {
		return nil, err
	}
	return lease, nil
}

func classifyExportStart(export model.ReportExport, now time.Time) (ExportDisposition, error) {
	if export.ID == 0 || now.IsZero() {
		return ExportDispositionUnknown, fmt.Errorf("report export execution: invalid stored export")
	}
	switch export.Status {
	case model.ReportExportStatusPending:
		return ExportDispositionAcquired, nil
	case model.ReportExportStatusRunning:
		if export.LeaseExpiresAt != nil && now.Before(export.LeaseExpiresAt.UTC()) {
			return ExportDispositionBusy, nil
		}
		return ExportDispositionAcquired, nil
	case model.ReportExportStatusReady, model.ReportExportStatusFailed, model.ReportExportStatusCancelled, model.ReportExportStatusExpired:
		return ExportDispositionTerminal, nil
	default:
		return ExportDispositionUnknown, fmt.Errorf("report export execution: unsupported status")
	}
}

func (repository *Repository) HeartbeatExport(ctx context.Context, exportID uint, leaseToken string, now time.Time, leaseTTL time.Duration) (ExportControl, error) {
	if repository == nil || repository.db == nil || ctx == nil || exportID == 0 || uuid.Validate(leaseToken) != nil || now.IsZero() || leaseTTL <= 0 {
		return ExportControlUnknown, fmt.Errorf("report export execution: invalid heartbeat")
	}
	now = now.UTC().Truncate(time.Millisecond)
	result := repository.ownedExport(ctx, exportID, leaseToken).Where("cancel_requested = ?", false).Updates(map[string]interface{}{
		"heartbeat_at": now, "lease_expires_at": now.Add(leaseTTL).UTC().Truncate(time.Millisecond), "updated_at": now,
	})
	if result.Error != nil {
		return ExportControlUnknown, fmt.Errorf("report export execution: heartbeat: %w", result.Error)
	}
	if result.RowsAffected == 1 {
		return ExportControlContinue, nil
	}
	return repository.InspectExport(ctx, exportID, leaseToken)
}

func (repository *Repository) InspectExport(ctx context.Context, exportID uint, leaseToken string) (ExportControl, error) {
	if repository == nil || repository.db == nil || ctx == nil || exportID == 0 || uuid.Validate(leaseToken) != nil {
		return ExportControlUnknown, fmt.Errorf("report export execution: invalid inspection")
	}
	var export model.ReportExport
	err := repository.db.WithContext(ctx).Select("status", "cancel_requested", "lease_token").First(&export, exportID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ExportControlLeaseLost, nil
	}
	if err != nil {
		return ExportControlUnknown, fmt.Errorf("report export execution: inspect: %w", err)
	}
	if export.Status != model.ReportExportStatusRunning || export.LeaseToken != leaseToken {
		return ExportControlLeaseLost, nil
	}
	if export.CancelRequested {
		return ExportControlCancelRequested, nil
	}
	return ExportControlContinue, nil
}

func (repository *Repository) UpdateExportProgress(ctx context.Context, exportID uint, leaseToken string, processedRows int64, currentSheet string, checkpoint model.JSONText, now time.Time) error {
	if repository == nil || repository.db == nil || ctx == nil || exportID == 0 || uuid.Validate(leaseToken) != nil ||
		processedRows < 0 || len([]byte(currentSheet)) > 255 || !validExportCheckpoint(checkpoint) || now.IsZero() {
		return fmt.Errorf("report export execution: invalid progress")
	}
	return repository.updateOwnedExport(ctx, exportID, leaseToken, map[string]interface{}{
		"processed_rows": processedRows, "current_sheet": currentSheet, "checkpoint_json": checkpoint,
		"heartbeat_at": now.UTC().Truncate(time.Millisecond), "updated_at": now.UTC().Truncate(time.Millisecond),
	}, true)
}

func (repository *Repository) LoadExportRuntime(ctx context.Context, exportID uint, leaseToken string) (*ExportRuntime, error) {
	if repository == nil || repository.db == nil || ctx == nil || exportID == 0 || uuid.Validate(leaseToken) != nil {
		return nil, fmt.Errorf("report export execution: invalid runtime request")
	}
	runtime := &ExportRuntime{}
	if err := repository.ownedExport(ctx, exportID, leaseToken).First(&runtime.Export).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrReportExportLeaseLost
	} else if err != nil {
		return nil, fmt.Errorf("report export execution: load export: %w", err)
	}
	if err := repository.db.WithContext(ctx).Where("id = ? AND status = ? AND result_purged_at IS NULL", runtime.Export.RunID, model.ReportRunStatusSucceeded).First(&runtime.Run).Error; err != nil {
		return nil, fmt.Errorf("report export execution: load run: %w", err)
	}
	if err := repository.db.WithContext(ctx).Where("id = ? AND definition_id = ?", runtime.Run.VersionID, runtime.Run.DefinitionID).First(&runtime.Version).Error; err != nil {
		return nil, fmt.Errorf("report export execution: load version: %w", err)
	}
	if runtime.Version.ContractHash != runtime.Run.ContractHash || runtime.Version.ProcedureSignatureHash != runtime.Run.ProcedureSignatureHash ||
		runtime.Version.ResultSchemaHash != runtime.Run.ResultSchemaHash || runtime.Version.DatasourceID == 0 {
		return nil, fmt.Errorf("report export execution: immutable contract mismatch")
	}
	if err := repository.db.WithContext(ctx).Where("id = ? AND driver = ?", runtime.Version.DatasourceID, model.ReportDatasourceDriverOracle).First(&runtime.Datasource).Error; err != nil {
		return nil, fmt.Errorf("report export execution: load datasource: %w", err)
	}
	return runtime, nil
}

func (repository *Repository) MarkExportReady(ctx context.Context, exportID uint, leaseToken, objectKey, checksum string, sizeBytes, rows int64, sheets int, truncated int64, readyAt, expiresAt time.Time) error {
	if !validExportArtifact(objectKey, checksum, sizeBytes) || rows < 0 || sheets < 1 || truncated < 0 || readyAt.IsZero() || !expiresAt.After(readyAt) {
		return fmt.Errorf("report export execution: invalid ready update")
	}
	readyAt = readyAt.UTC().Truncate(time.Millisecond)
	return repository.updateOwnedExportWithAudit(ctx, exportID, leaseToken, map[string]interface{}{
		"status": model.ReportExportStatusReady, "result_object_key": objectKey, "result_checksum": checksum,
		"file_size_bytes": sizeBytes, "exported_rows": rows, "processed_rows": rows, "sheet_count": sheets,
		"truncated_cell_count": truncated, "ready_at": readyAt, "expires_at": expiresAt.UTC().Truncate(time.Millisecond),
		"error_code": "", "error_message_safe": "", "worker_id": "", "lease_token": "", "lease_expires_at": nil,
		"heartbeat_at": nil, "updated_at": readyAt,
	}, true, "REPORT_EXPORT_READY", map[string]interface{}{"exportedRows": rows, "fileSizeBytes": sizeBytes})
}

func (repository *Repository) ConfirmExportReady(ctx context.Context, exportID uint, objectKey, checksum string, sizeBytes, rows int64) (bool, error) {
	if repository == nil || repository.db == nil || ctx == nil || exportID == 0 || !validExportArtifact(objectKey, checksum, sizeBytes) || rows < 0 {
		return false, fmt.Errorf("report export execution: invalid ready confirmation")
	}
	var export model.ReportExport
	err := repository.db.WithContext(ctx).Select("status", "result_object_key", "result_checksum", "file_size_bytes", "exported_rows").First(&export, exportID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("report export execution: confirm ready: %w", err)
	}
	return export.Status == model.ReportExportStatusReady && export.ResultObjectKey == objectKey && export.ResultChecksum == checksum && export.FileSizeBytes == sizeBytes && export.ExportedRows == rows, nil
}

func (repository *Repository) ReleaseExportForRetry(ctx context.Context, exportID uint, leaseToken string, now time.Time) error {
	if now.IsZero() {
		return fmt.Errorf("report export execution: invalid retry release")
	}
	now = now.UTC().Truncate(time.Millisecond)
	return repository.updateOwnedExportWithAudit(ctx, exportID, leaseToken, map[string]interface{}{
		"status": model.ReportExportStatusPending, "worker_id": "", "lease_token": "", "lease_expires_at": nil,
		"heartbeat_at": nil, "error_code": "", "error_message_safe": "", "updated_at": now,
	}, false, "REPORT_EXPORT_RETRY_QUEUED", nil)
}

func (repository *Repository) MarkExportFailed(ctx context.Context, exportID uint, leaseToken, code, safeMessage string, finishedAt time.Time) error {
	if repository == nil || repository.db == nil || ctx == nil || exportID == 0 || uuid.Validate(leaseToken) != nil ||
		!validRunError(code, safeMessage) || finishedAt.IsZero() {
		return fmt.Errorf("report export execution: invalid failure update")
	}
	finishedAt = finishedAt.UTC().Truncate(time.Millisecond)
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.ReportExport{}).
			Where("id = ? AND status = ? AND lease_token = ?", exportID, model.ReportExportStatusRunning, leaseToken).
			Updates(map[string]interface{}{
				"status": model.ReportExportStatusFailed, "error_code": code, "error_message_safe": strings.TrimSpace(safeMessage),
				"worker_id": "", "lease_token": "", "lease_expires_at": nil, "heartbeat_at": nil, "updated_at": finishedAt,
			})
		if result.Error != nil {
			return fmt.Errorf("report export execution: persist failure: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return ErrReportExportLeaseLost
		}
		runID := tx.Model(&model.ReportExport{}).Select("run_id").Where("id = ?", exportID)
		if err := tx.Model(&model.ReportRun{}).
			Where("id = (?) AND status = ? AND result_purged_at IS NULL", runID, model.ReportRunStatusSucceeded).
			Update("result_expires_at", finishedAt).Error; err != nil {
			return fmt.Errorf("report export execution: schedule failed result cleanup: %w", err)
		}
		return repository.writeSystemAudit(ctx, tx, "REPORT_EXPORT_FAILED", "REPORT_EXPORT", exportID, map[string]interface{}{"errorCode": code})
	})
}

func (repository *Repository) MarkExportCancelled(ctx context.Context, exportID uint, leaseToken string, finishedAt time.Time) error {
	if finishedAt.IsZero() {
		return fmt.Errorf("report export execution: invalid cancellation update")
	}
	finishedAt = finishedAt.UTC().Truncate(time.Millisecond)
	return repository.updateOwnedExportWithAudit(ctx, exportID, leaseToken, map[string]interface{}{
		"status": model.ReportExportStatusCancelled, "cancel_requested": true,
		"error_code": "CANCELLED", "error_message_safe": "报表导出已取消",
		"worker_id": "", "lease_token": "", "lease_expires_at": nil, "heartbeat_at": nil, "updated_at": finishedAt,
	}, false, "REPORT_EXPORT_CANCELLED", nil)
}

func (repository *Repository) ClaimResultPurge(ctx context.Context, exportID uint, leaseToken string, now time.Time) (*ExportRuntime, error) {
	if repository == nil || repository.db == nil || ctx == nil || exportID == 0 || uuid.Validate(leaseToken) != nil || now.IsZero() {
		return nil, fmt.Errorf("report result purge: invalid claim")
	}
	now = now.UTC().Truncate(time.Millisecond)
	runtime := &ExportRuntime{}
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&runtime.Export, exportID).Error; err != nil {
			return fmt.Errorf("report result purge: lock export: %w", err)
		}
		if runtime.Export.Status != model.ReportExportStatusReady || runtime.Export.PurgedAt != nil ||
			(runtime.Export.LeaseToken != "" && runtime.Export.LeaseExpiresAt != nil && now.Before(runtime.Export.LeaseExpiresAt.UTC())) {
			return ErrReportResultPurgeConflict
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&runtime.Run, runtime.Export.RunID).Error; err != nil {
			return fmt.Errorf("report result purge: lock run: %w", err)
		}
		if runtime.Run.ResultPurgedAt != nil || (runtime.Run.Status != model.ReportRunStatusSucceeded && runtime.Run.Status != model.ReportRunStatusResultPurging) {
			return ErrReportResultPurgeConflict
		}
		if err := tx.Where("run_id = ? AND expires_at <= ?", runtime.Run.ID, now).Delete(&model.ReportResultReadLease{}).Error; err != nil {
			return fmt.Errorf("report result purge: delete expired read leases: %w", err)
		}
		var activeReaders int64
		if err := tx.Model(&model.ReportResultReadLease{}).Where("run_id = ? AND expires_at > ?", runtime.Run.ID, now).Count(&activeReaders).Error; err != nil {
			return fmt.Errorf("report result purge: count active readers: %w", err)
		}
		if activeReaders > 0 {
			return ErrReportResultPurgeConflict
		}
		if err := tx.Where("id = ? AND definition_id = ?", runtime.Run.VersionID, runtime.Run.DefinitionID).First(&runtime.Version).Error; err != nil {
			return fmt.Errorf("report result purge: load version: %w", err)
		}
		if err := tx.Where("id = ? AND driver = ?", runtime.Version.DatasourceID, model.ReportDatasourceDriverOracle).First(&runtime.Datasource).Error; err != nil {
			return fmt.Errorf("report result purge: load datasource: %w", err)
		}
		if runtime.Run.Status == model.ReportRunStatusSucceeded {
			result := tx.Model(&model.ReportRun{}).Where("id = ? AND status = ? AND result_purged_at IS NULL", runtime.Run.ID, model.ReportRunStatusSucceeded).Update("status", model.ReportRunStatusResultPurging)
			if result.Error != nil || result.RowsAffected != 1 {
				if result.Error != nil {
					return fmt.Errorf("report result purge: claim run: %w", result.Error)
				}
				return ErrReportResultPurgeConflict
			}
		}
		expiresAt := now.Add(defaultResultPurgeLeaseTTL).UTC().Truncate(time.Millisecond)
		result := tx.Model(&model.ReportExport{}).
			Where("id = ? AND status = ? AND purged_at IS NULL AND (lease_token = ? OR lease_expires_at IS NULL OR lease_expires_at <= ?)", exportID, model.ReportExportStatusReady, "", now).
			Updates(map[string]interface{}{
				"lease_token": leaseToken, "lease_expires_at": expiresAt, "heartbeat_at": now,
				"purge_started_at": now, "updated_at": now,
			})
		if result.Error != nil || result.RowsAffected != 1 {
			if result.Error != nil {
				return fmt.Errorf("report result purge: claim export: %w", result.Error)
			}
			return ErrReportResultPurgeConflict
		}
		runtime.Export.LeaseToken = leaseToken
		runtime.Export.LeaseExpiresAt = &expiresAt
		runtime.Export.HeartbeatAt = &now
		runtime.Export.PurgeStartedAt = &now
		runtime.Run.Status = model.ReportRunStatusResultPurging
		return repository.writeSystemAudit(ctx, tx, "REPORT_RESULT_PURGE_STARTED", "REPORT_RUN", runtime.Run.ID, map[string]interface{}{"exportId": exportID})
	})
	if err != nil {
		return nil, err
	}
	return runtime, nil
}

// UpdateResultPurgeProgress fences each committed Oracle delete batch and
// renews the cleanup lease. purgedRows is the cumulative deleted row count.
func (repository *Repository) UpdateResultPurgeProgress(ctx context.Context, exportID uint, leaseToken string, purgedRows int64, now time.Time) error {
	if repository == nil || repository.db == nil || ctx == nil || exportID == 0 || uuid.Validate(leaseToken) != nil || purgedRows < 0 || now.IsZero() {
		return fmt.Errorf("report result purge: invalid progress")
	}
	now = now.UTC().Truncate(time.Millisecond)
	result := repository.db.WithContext(ctx).Model(&model.ReportExport{}).
		Where("id = ? AND status = ? AND purged_at IS NULL AND lease_token = ? AND purged_rows <= ?", exportID, model.ReportExportStatusReady, leaseToken, purgedRows).
		Updates(map[string]interface{}{
			"purged_rows": purgedRows, "heartbeat_at": now,
			"lease_expires_at": now.Add(defaultResultPurgeLeaseTTL).UTC().Truncate(time.Millisecond), "updated_at": now,
		})
	if result.Error != nil {
		return fmt.Errorf("report result purge: update progress: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrReportExportLeaseLost
	}
	return nil
}

func (repository *Repository) MarkResultPurged(ctx context.Context, exportID uint, leaseToken string, purgedRows int64, now time.Time) error {
	if repository == nil || repository.db == nil || ctx == nil || exportID == 0 || uuid.Validate(leaseToken) != nil || purgedRows < 0 || now.IsZero() {
		return fmt.Errorf("report result purge: invalid completion")
	}
	now = now.UTC().Truncate(time.Millisecond)
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var export model.ReportExport
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND status = ? AND lease_token = ?", exportID, model.ReportExportStatusReady, leaseToken).First(&export).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrReportExportLeaseLost
		} else if err != nil {
			return fmt.Errorf("report result purge: lock owned export: %w", err)
		}
		result := tx.Model(&model.ReportRun{}).Where("id = ? AND status = ? AND result_purged_at IS NULL", export.RunID, model.ReportRunStatusResultPurging).Updates(map[string]interface{}{
			"status": model.ReportRunStatusResultPurged, "result_purged_at": now, "updated_at": now,
		})
		if result.Error != nil || result.RowsAffected != 1 {
			if result.Error != nil {
				return fmt.Errorf("report result purge: finish run: %w", result.Error)
			}
			return ErrReportResultPurgeConflict
		}
		result = tx.Model(&model.ReportExport{}).Where("id = ? AND status = ? AND lease_token = ? AND purged_rows <= ?", exportID, model.ReportExportStatusReady, leaseToken, purgedRows).Updates(map[string]interface{}{
			"purged_at": now, "purged_rows": purgedRows, "lease_token": "", "lease_expires_at": nil,
			"heartbeat_at": nil, "updated_at": now,
		})
		if result.Error != nil || result.RowsAffected != 1 {
			if result.Error != nil {
				return fmt.Errorf("report result purge: finish export: %w", result.Error)
			}
			return ErrReportExportLeaseLost
		}
		return repository.writeSystemAudit(ctx, tx, "REPORT_RESULT_PURGED", "REPORT_RUN", export.RunID, map[string]interface{}{"exportId": exportID, "purgedRows": purgedRows})
	})
}

func (repository *Repository) ConfirmResultPurged(ctx context.Context, exportID uint) (bool, error) {
	if repository == nil || repository.db == nil || ctx == nil || exportID == 0 {
		return false, fmt.Errorf("report result purge: invalid confirmation")
	}
	var export model.ReportExport
	if err := repository.db.WithContext(ctx).Select("run_id", "purged_at").First(&export, exportID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("report result purge: confirm export: %w", err)
	}
	if export.PurgedAt == nil {
		return false, nil
	}
	var run model.ReportRun
	if err := repository.db.WithContext(ctx).Select("status", "result_purged_at").First(&run, export.RunID).Error; err != nil {
		return false, fmt.Errorf("report result purge: confirm run: %w", err)
	}
	return run.Status == model.ReportRunStatusResultPurged && run.ResultPurgedAt != nil, nil
}

func (repository *Repository) ReleaseResultPurge(ctx context.Context, exportID uint, leaseToken string, now time.Time) error {
	if repository == nil || repository.db == nil || ctx == nil || exportID == 0 || uuid.Validate(leaseToken) != nil || now.IsZero() {
		return fmt.Errorf("report result purge: invalid release")
	}
	now = now.UTC().Truncate(time.Millisecond)
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var export model.ReportExport
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND status = ? AND lease_token = ?", exportID, model.ReportExportStatusReady, leaseToken).First(&export).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrReportExportLeaseLost
		} else if err != nil {
			return fmt.Errorf("report result purge: lock owned export: %w", err)
		}
		var runCount int64
		if err := tx.Model(&model.ReportRun{}).Where("id = ? AND status = ? AND result_purged_at IS NULL", export.RunID, model.ReportRunStatusResultPurging).Count(&runCount).Error; err != nil {
			return fmt.Errorf("report result purge: inspect run before release: %w", err)
		}
		if runCount != 1 {
			return ErrReportResultPurgeConflict
		}
		result := tx.Model(&model.ReportExport{}).Where("id = ? AND status = ? AND lease_token = ?", exportID, model.ReportExportStatusReady, leaseToken).Updates(map[string]interface{}{
			"lease_token": "", "lease_expires_at": nil, "heartbeat_at": nil, "purge_started_at": nil, "updated_at": now,
		})
		if result.Error != nil || result.RowsAffected != 1 {
			if result.Error != nil {
				return fmt.Errorf("report result purge: release export: %w", result.Error)
			}
			return ErrReportExportLeaseLost
		}
		return repository.writeSystemAudit(ctx, tx, "REPORT_RESULT_PURGE_RETRY", "REPORT_RUN", export.RunID, map[string]interface{}{"exportId": exportID})
	})
}

func (repository *Repository) ownedExport(ctx context.Context, exportID uint, leaseToken string) *gorm.DB {
	if repository == nil || repository.db == nil || ctx == nil || exportID == 0 || uuid.Validate(leaseToken) != nil {
		return &gorm.DB{Error: fmt.Errorf("report export execution: invalid owned query")}
	}
	return repository.db.WithContext(ctx).Model(&model.ReportExport{}).Where("id = ? AND status = ? AND lease_token = ?", exportID, model.ReportExportStatusRunning, leaseToken)
}

func (repository *Repository) updateOwnedExport(ctx context.Context, exportID uint, leaseToken string, updates map[string]interface{}, requireActive bool) error {
	query := repository.ownedExport(ctx, exportID, leaseToken)
	if query.Error != nil {
		return query.Error
	}
	if requireActive {
		query = query.Where("cancel_requested = ?", false)
	}
	result := query.Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("report export execution: fenced update: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrReportExportLeaseLost
	}
	return nil
}

func (repository *Repository) updateOwnedExportWithAudit(ctx context.Context, exportID uint, leaseToken string, updates map[string]interface{}, requireActive bool, action string, detail map[string]interface{}) error {
	if repository == nil || repository.db == nil || ctx == nil || exportID == 0 || uuid.Validate(leaseToken) != nil {
		return fmt.Errorf("report export execution: invalid audited update")
	}
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Model(&model.ReportExport{}).Where("id = ? AND status = ? AND lease_token = ?", exportID, model.ReportExportStatusRunning, leaseToken)
		if requireActive {
			query = query.Where("cancel_requested = ?", false)
		}
		result := query.Updates(updates)
		if result.Error != nil {
			return fmt.Errorf("report export execution: audited update: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return ErrReportExportLeaseLost
		}
		return repository.writeSystemAudit(ctx, tx, action, "REPORT_EXPORT", exportID, detail)
	})
}

func validExportCheckpoint(checkpoint model.JSONText) bool {
	trimmed := strings.TrimSpace(string(checkpoint))
	return len(trimmed) > 0 && len([]byte(trimmed)) <= maxReportExportCheckpointBytes && json.Valid([]byte(trimmed))
}

func validExportArtifact(objectKey, checksum string, sizeBytes int64) bool {
	objectKey = strings.TrimSpace(objectKey)
	return objectKey != "" && len([]byte(objectKey)) <= 1024 && !strings.ContainsAny(objectKey, "\r\n") && reportExportChecksumPattern.MatchString(checksum) && sizeBytes > 0
}
