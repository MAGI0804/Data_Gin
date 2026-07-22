package data_dao

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode"

	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	maxMallWeatherFeishuProfileSnapshotBytes     = 256 * 1024
	maxMallWeatherFeishuFiltersBytes             = 64 * 1024
	maxMallWeatherFeishuDestinationSnapshotBytes = 64 * 1024
	maxMallWeatherFeishuRunSafeErrorBytes        = 2 * 1024
)

var (
	ErrMallWeatherFeishuRunNotFound  = errors.New("mall weather feishu run: not found")
	ErrMallWeatherFeishuRunLeaseLost = errors.New("mall weather feishu run: lease lost")
)

type MallWeatherFeishuRunDisposition uint8

const (
	MallWeatherFeishuRunDispositionUnknown MallWeatherFeishuRunDisposition = iota
	MallWeatherFeishuRunDispositionAcquired
	MallWeatherFeishuRunDispositionBusy
	MallWeatherFeishuRunDispositionTerminal
)

type MallWeatherFeishuRunRecord struct {
	Pipeline model.PipelineRun
	Detail   model.MallWeatherFeishuRun
}

type MallWeatherFeishuRunLease struct {
	Disposition MallWeatherFeishuRunDisposition
	RunToken    string
	Record      MallWeatherFeishuRunRecord
}

type MallWeatherFeishuRunProgress struct {
	SuccessCount int
	FailedCount  int
	UpdatedAt    time.Time
}

type MallWeatherFeishuRunFinish struct {
	Status       string
	SuccessCount int
	FailedCount  int
	SafeError    string
	FinishedAt   time.Time
}

type MallWeatherFeishuRunDAO struct {
	db *gorm.DB
}

func NewMallWeatherFeishuRunDAO(databases ...*gorm.DB) *MallWeatherFeishuRunDAO {
	db := database.DB
	if len(databases) > 0 && databases[0] != nil {
		db = databases[0]
	}
	return &MallWeatherFeishuRunDAO{db: db}
}

func (dao *MallWeatherFeishuRunDAO) WithDB(db *gorm.DB) *MallWeatherFeishuRunDAO {
	return &MallWeatherFeishuRunDAO{db: db}
}

// Create persists the public pipeline row and its immutable Feishu inputs in
// one transaction. If dao was created with an outer GORM transaction, the
// nested transaction remains part of that caller-owned transaction.
func (dao *MallWeatherFeishuRunDAO) Create(
	ctx context.Context,
	record *MallWeatherFeishuRunRecord,
) error {
	if dao == nil || dao.db == nil || ctx == nil || !validNewMallWeatherFeishuRun(record) {
		return fmt.Errorf("mall weather feishu run: invalid create input")
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	return dao.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		record.Pipeline.CreatedAt = int(now.Unix())
		record.Pipeline.UpdatedAt = int(now.Unix())
		if err := tx.Create(&record.Pipeline).Error; err != nil {
			return fmt.Errorf("mall weather feishu run: create pipeline: %w", err)
		}
		record.Detail.PipelineRunID = record.Pipeline.ID
		record.Detail.CreatedAt = now
		record.Detail.UpdatedAt = now
		if err := tx.Create(&record.Detail).Error; err != nil {
			return fmt.Errorf("mall weather feishu run: create detail: %w", err)
		}
		return nil
	})
}

func (dao *MallWeatherFeishuRunDAO) FindByPipelineRunID(
	ctx context.Context,
	pipelineRunID uint,
) (*MallWeatherFeishuRunRecord, error) {
	if dao == nil || dao.db == nil || ctx == nil || pipelineRunID == 0 {
		return nil, fmt.Errorf("mall weather feishu run: invalid find input")
	}
	record := &MallWeatherFeishuRunRecord{}
	err := dao.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := loadMallWeatherFeishuRunRecord(tx, pipelineRunID, false, record); err != nil {
			return err
		}
		return validateStoredMallWeatherFeishuRun(record)
	})
	if err != nil {
		return nil, err
	}
	record.Detail.RunToken = ""
	return record, nil
}

func (dao *MallWeatherFeishuRunDAO) BeginRun(
	ctx context.Context,
	pipelineRunID uint,
	runToken string,
	startedAt time.Time,
	staleAfter time.Duration,
) (*MallWeatherFeishuRunLease, error) {
	if dao == nil || dao.db == nil || ctx == nil || pipelineRunID == 0 ||
		!validMallWeatherFeishuRunToken(runToken) || startedAt.IsZero() || staleAfter <= 0 {
		return nil, fmt.Errorf("mall weather feishu run: invalid lease input")
	}
	startedAt = startedAt.UTC().Truncate(time.Millisecond)
	lease := &MallWeatherFeishuRunLease{}
	err := dao.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := loadMallWeatherFeishuRunRecord(tx, pipelineRunID, true, &lease.Record); err != nil {
			return err
		}
		disposition, err := classifyMallWeatherFeishuRunStart(&lease.Record, startedAt, staleAfter)
		if err != nil {
			return err
		}
		lease.Disposition = disposition
		if disposition != MallWeatherFeishuRunDispositionAcquired {
			return nil
		}
		detailResult := tx.Model(&model.MallWeatherFeishuRun{}).
			Where("id = ?", lease.Record.Detail.ID).
			Updates(map[string]interface{}{"run_token": runToken, "updated_at": startedAt})
		if detailResult.Error != nil {
			return fmt.Errorf("mall weather feishu run: claim detail: %w", detailResult.Error)
		}
		if detailResult.RowsAffected != 1 {
			return fmt.Errorf("mall weather feishu run: claim detail count changed")
		}
		pipelineUpdates := map[string]interface{}{
			"updated_at":    startedAt.Unix(),
			"finished_at":   nil,
			"error_message": "",
		}
		if lease.Record.Pipeline.StartedAt == nil {
			pipelineUpdates["started_at"] = model.TimeNormal{Time: startedAt}
			lease.Record.Pipeline.StartedAt = &model.TimeNormal{Time: startedAt}
		}
		pipelineResult := tx.Model(&model.PipelineRun{}).
			Where("id = ? AND status = ?", pipelineRunID, "running").
			Updates(pipelineUpdates)
		if pipelineResult.Error != nil {
			return fmt.Errorf("mall weather feishu run: claim pipeline: %w", pipelineResult.Error)
		}
		if pipelineResult.RowsAffected != 1 {
			return fmt.Errorf("mall weather feishu run: claim pipeline count changed")
		}
		lease.RunToken = runToken
		lease.Record.Detail.UpdatedAt = startedAt
		lease.Record.Pipeline.UpdatedAt = int(startedAt.Unix())
		lease.Record.Pipeline.FinishedAt = nil
		lease.Record.Pipeline.ErrorMessage = ""
		return nil
	})
	if err != nil {
		return nil, err
	}
	lease.Record.Detail.RunToken = ""
	return lease, nil
}

func (dao *MallWeatherFeishuRunDAO) HeartbeatRun(
	ctx context.Context,
	pipelineRunID uint,
	runToken string,
	now time.Time,
) error {
	if dao == nil || dao.db == nil || ctx == nil || pipelineRunID == 0 ||
		!validMallWeatherFeishuRunToken(runToken) || now.IsZero() {
		return fmt.Errorf("mall weather feishu run: invalid heartbeat")
	}
	now = now.UTC().Truncate(time.Millisecond)
	return dao.updateOwnedRun(ctx, pipelineRunID, runToken, now, nil)
}

func (dao *MallWeatherFeishuRunDAO) UpdateRunProgress(
	ctx context.Context,
	pipelineRunID uint,
	runToken string,
	progress MallWeatherFeishuRunProgress,
) error {
	if progress.SuccessCount < 0 || progress.FailedCount < 0 ||
		progress.SuccessCount > math.MaxInt-progress.FailedCount || progress.UpdatedAt.IsZero() {
		return fmt.Errorf("mall weather feishu run: invalid progress")
	}
	updates := map[string]interface{}{
		"total_count":   progress.SuccessCount + progress.FailedCount,
		"success_count": progress.SuccessCount,
		"failed_count":  progress.FailedCount,
	}
	return dao.updateOwnedRun(ctx, pipelineRunID, runToken, progress.UpdatedAt, updates)
}

func (dao *MallWeatherFeishuRunDAO) FinishRun(
	ctx context.Context,
	pipelineRunID uint,
	runToken string,
	finish MallWeatherFeishuRunFinish,
) error {
	finish.SafeError = strings.TrimSpace(finish.SafeError)
	if dao == nil || dao.db == nil || ctx == nil || pipelineRunID == 0 ||
		!validMallWeatherFeishuRunToken(runToken) || !validMallWeatherFeishuRunFinish(finish) {
		return fmt.Errorf("mall weather feishu run: invalid finish")
	}
	finish.FinishedAt = finish.FinishedAt.UTC().Truncate(time.Millisecond)
	return dao.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		record := &MallWeatherFeishuRunRecord{}
		if err := loadMallWeatherFeishuRunRecord(tx, pipelineRunID, true, record); err != nil {
			return err
		}
		if err := validateStoredMallWeatherFeishuRun(record); err != nil {
			return err
		}
		if record.Detail.RunToken != runToken {
			return ErrMallWeatherFeishuRunLeaseLost
		}
		if record.Pipeline.Status != "running" {
			if sameMallWeatherFeishuRunFinish(record.Pipeline, finish) {
				return nil
			}
			return ErrMallWeatherFeishuRunLeaseLost
		}
		if record.Pipeline.StartedAt == nil || record.Pipeline.StartedAt.IsZero() ||
			record.Pipeline.FinishedAt != nil || finish.FinishedAt.Before(record.Detail.UpdatedAt) {
			return fmt.Errorf("mall weather feishu run: invalid active finish state")
		}
		result := ownedMallWeatherFeishuPipelineQuery(tx, pipelineRunID, runToken).
			Updates(map[string]interface{}{
				"status":        finish.Status,
				"total_count":   finish.SuccessCount + finish.FailedCount,
				"success_count": finish.SuccessCount,
				"failed_count":  finish.FailedCount,
				"error_message": finish.SafeError,
				"finished_at":   model.TimeNormal{Time: finish.FinishedAt},
				"updated_at":    finish.FinishedAt.Unix(),
			})
		if result.Error != nil {
			return fmt.Errorf("mall weather feishu run: finish pipeline: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return ErrMallWeatherFeishuRunLeaseLost
		}
		if err := tx.Model(&model.MallWeatherFeishuRun{}).
			Where("id = ? AND run_token = ?", record.Detail.ID, runToken).
			Update("updated_at", finish.FinishedAt).Error; err != nil {
			return fmt.Errorf("mall weather feishu run: finish detail: %w", err)
		}
		return nil
	})
}

func (dao *MallWeatherFeishuRunDAO) updateOwnedRun(
	ctx context.Context,
	pipelineRunID uint,
	runToken string,
	updatedAt time.Time,
	pipelineUpdates map[string]interface{},
) error {
	if dao == nil || dao.db == nil || ctx == nil || pipelineRunID == 0 ||
		!validMallWeatherFeishuRunToken(runToken) || updatedAt.IsZero() {
		return fmt.Errorf("mall weather feishu run: invalid owned update")
	}
	updatedAt = updatedAt.UTC().Truncate(time.Millisecond)
	return dao.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		record := &MallWeatherFeishuRunRecord{}
		if err := loadMallWeatherFeishuRunRecord(tx, pipelineRunID, true, record); err != nil {
			return err
		}
		if err := validateStoredMallWeatherFeishuRun(record); err != nil {
			return err
		}
		if record.Detail.RunToken != runToken || record.Pipeline.Status != "running" {
			return ErrMallWeatherFeishuRunLeaseLost
		}
		if record.Pipeline.StartedAt == nil || record.Pipeline.StartedAt.IsZero() ||
			record.Pipeline.FinishedAt != nil || updatedAt.Before(record.Detail.UpdatedAt) {
			return fmt.Errorf("mall weather feishu run: invalid active update state")
		}
		if pipelineUpdates == nil {
			pipelineUpdates = make(map[string]interface{}, 1)
		}
		pipelineUpdates["updated_at"] = updatedAt.Unix()
		result := ownedMallWeatherFeishuPipelineQuery(tx, pipelineRunID, runToken).Updates(pipelineUpdates)
		if result.Error != nil {
			return fmt.Errorf("mall weather feishu run: update pipeline: %w", result.Error)
		}
		if err := tx.Model(&model.MallWeatherFeishuRun{}).
			Where("id = ? AND run_token = ?", record.Detail.ID, runToken).
			Update("updated_at", updatedAt).Error; err != nil {
			return fmt.Errorf("mall weather feishu run: update detail: %w", err)
		}
		return nil
	})
}

func loadMallWeatherFeishuRunRecord(
	tx *gorm.DB,
	pipelineRunID uint,
	lock bool,
	record *MallWeatherFeishuRunRecord,
) error {
	if tx == nil || pipelineRunID == 0 || record == nil {
		return fmt.Errorf("mall weather feishu run: invalid load input")
	}
	detailQuery := tx.Where("pipeline_run_id = ?", pipelineRunID)
	if lock {
		detailQuery = detailQuery.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := detailQuery.First(&record.Detail).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrMallWeatherFeishuRunNotFound
		}
		return fmt.Errorf("mall weather feishu run: load detail: %w", err)
	}
	pipelineQuery := tx.Where("id = ?", pipelineRunID)
	if lock {
		pipelineQuery = pipelineQuery.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := pipelineQuery.First(&record.Pipeline).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("mall weather feishu run: orphan detail")
		}
		return fmt.Errorf("mall weather feishu run: load pipeline: %w", err)
	}
	return nil
}

func ownedMallWeatherFeishuPipelineQuery(tx *gorm.DB, pipelineRunID uint, runToken string) *gorm.DB {
	return tx.Model(&model.PipelineRun{}).
		Where("id = ? AND status = ?", pipelineRunID, "running").
		Where(
			"EXISTS (SELECT 1 FROM mall_weather_feishu_runs WHERE pipeline_run_id = pipeline_runs.id AND run_token = ?)",
			runToken,
		)
}

func classifyMallWeatherFeishuRunStart(
	record *MallWeatherFeishuRunRecord,
	now time.Time,
	staleAfter time.Duration,
) (MallWeatherFeishuRunDisposition, error) {
	if now.IsZero() || staleAfter <= 0 {
		return MallWeatherFeishuRunDispositionUnknown, fmt.Errorf("mall weather feishu run: invalid lease state")
	}
	if err := validateStoredMallWeatherFeishuRun(record); err != nil {
		return MallWeatherFeishuRunDispositionUnknown, err
	}
	run := record.Pipeline
	detail := record.Detail
	if now.UTC().Before(detail.UpdatedAt.UTC()) {
		return MallWeatherFeishuRunDispositionUnknown, fmt.Errorf("mall weather feishu run: lease time moved backwards")
	}
	switch run.Status {
	case "running":
		if run.FinishedAt != nil {
			return MallWeatherFeishuRunDispositionUnknown, fmt.Errorf("mall weather feishu run: running row is finished")
		}
		if run.StartedAt == nil {
			if detail.RunToken != "" {
				return MallWeatherFeishuRunDispositionUnknown, fmt.Errorf("mall weather feishu run: pending row has a lease")
			}
			return MallWeatherFeishuRunDispositionAcquired, nil
		}
		if run.StartedAt.IsZero() || !validMallWeatherFeishuRunToken(detail.RunToken) {
			return MallWeatherFeishuRunDispositionUnknown, fmt.Errorf("mall weather feishu run: running row has an invalid lease")
		}
		if now.UTC().Before(detail.UpdatedAt.UTC().Add(staleAfter)) {
			return MallWeatherFeishuRunDispositionBusy, nil
		}
		return MallWeatherFeishuRunDispositionAcquired, nil
	case "success", "failed", "partial_success":
		if !validTerminalMallWeatherFeishuRun(run, detail.RunToken) {
			return MallWeatherFeishuRunDispositionUnknown, fmt.Errorf("mall weather feishu run: invalid terminal state")
		}
		return MallWeatherFeishuRunDispositionTerminal, nil
	default:
		return MallWeatherFeishuRunDispositionUnknown, fmt.Errorf("mall weather feishu run: unsupported pipeline status")
	}
}

func validNewMallWeatherFeishuRun(record *MallWeatherFeishuRunRecord) bool {
	if record == nil {
		return false
	}
	run := record.Pipeline
	detail := record.Detail
	return run.ID == 0 && detail.ID == 0 && detail.PipelineRunID == 0 && detail.RunToken == "" &&
		run.TraceID != "" && len(run.TraceID) <= 64 && uuid.Validate(run.TraceID) == nil &&
		run.RunType == "delivery" && run.TriggerType == "api" && run.SourceID == 0 && run.DestinationID > 0 &&
		run.Status == "running" && run.TotalCount == 0 && run.SuccessCount == 0 && run.FailedCount == 0 &&
		run.StartedAt == nil && run.FinishedAt == nil && run.ErrorMessage == "" &&
		detail.ProfileID > 0 && detail.ProfileVersion > 0 && detail.CreatedBy > 0 &&
		validMallWeatherFeishuRunJSON(detail.ProfileSnapshotJSON, maxMallWeatherFeishuProfileSnapshotBytes) &&
		validMallWeatherFeishuRunJSON(detail.FiltersJSON, maxMallWeatherFeishuFiltersBytes) &&
		validMallWeatherFeishuRunJSON(detail.DestinationSnapshotJSON, maxMallWeatherFeishuDestinationSnapshotBytes)
}

func validateStoredMallWeatherFeishuRun(record *MallWeatherFeishuRunRecord) error {
	if record == nil {
		return fmt.Errorf("mall weather feishu run: missing stored record")
	}
	run := record.Pipeline
	detail := record.Detail
	if run.ID == 0 || detail.ID == 0 || detail.PipelineRunID != run.ID ||
		run.TraceID == "" || len(run.TraceID) > 64 || uuid.Validate(run.TraceID) != nil ||
		run.RunType != "delivery" || run.TriggerType != "api" || run.SourceID != 0 || run.DestinationID == 0 ||
		run.TotalCount < 0 || run.SuccessCount < 0 || run.FailedCount < 0 ||
		run.SuccessCount > math.MaxInt-run.FailedCount || run.TotalCount != run.SuccessCount+run.FailedCount ||
		run.CreatedAt <= 0 || run.UpdatedAt <= 0 || detail.ProfileID == 0 || detail.ProfileVersion == 0 ||
		detail.CreatedBy == 0 || detail.CreatedAt.IsZero() || detail.UpdatedAt.IsZero() ||
		!validMallWeatherFeishuRunJSON(detail.ProfileSnapshotJSON, maxMallWeatherFeishuProfileSnapshotBytes) ||
		!validMallWeatherFeishuRunJSON(detail.FiltersJSON, maxMallWeatherFeishuFiltersBytes) ||
		!validMallWeatherFeishuRunJSON(detail.DestinationSnapshotJSON, maxMallWeatherFeishuDestinationSnapshotBytes) {
		return fmt.Errorf("mall weather feishu run: invalid stored record")
	}
	switch run.Status {
	case "running":
		if run.FinishedAt != nil ||
			(run.StartedAt == nil && detail.RunToken != "") ||
			(run.StartedAt != nil && (run.StartedAt.IsZero() || !validMallWeatherFeishuRunToken(detail.RunToken))) {
			return fmt.Errorf("mall weather feishu run: invalid running state")
		}
	case "success", "failed", "partial_success":
		if !validTerminalMallWeatherFeishuRun(run, detail.RunToken) {
			return fmt.Errorf("mall weather feishu run: invalid terminal state")
		}
	default:
		return fmt.Errorf("mall weather feishu run: unsupported stored status")
	}
	return nil
}

func validTerminalMallWeatherFeishuRun(run model.PipelineRun, runToken string) bool {
	if run.StartedAt == nil || run.StartedAt.IsZero() || run.FinishedAt == nil || run.FinishedAt.IsZero() ||
		run.FinishedAt.Before(run.StartedAt.Time) ||
		!validMallWeatherFeishuRunToken(runToken) || !validMallWeatherFeishuRunSafeError(run.ErrorMessage) {
		return false
	}
	switch run.Status {
	case "success":
		return run.FailedCount == 0 && run.ErrorMessage == ""
	case "failed":
		return run.SuccessCount == 0 && run.FailedCount > 0 && run.ErrorMessage != ""
	case "partial_success":
		return run.SuccessCount > 0 && run.FailedCount > 0 && run.ErrorMessage != ""
	default:
		return false
	}
}

func validMallWeatherFeishuRunFinish(finish MallWeatherFeishuRunFinish) bool {
	if finish.SuccessCount < 0 || finish.FailedCount < 0 ||
		finish.SuccessCount > math.MaxInt-finish.FailedCount || finish.FinishedAt.IsZero() ||
		!validMallWeatherFeishuRunSafeError(finish.SafeError) {
		return false
	}
	switch finish.Status {
	case "success":
		return finish.FailedCount == 0 && finish.SafeError == ""
	case "failed":
		return finish.SuccessCount == 0 && finish.FailedCount > 0 && finish.SafeError != ""
	case "partial_success":
		return finish.SuccessCount > 0 && finish.FailedCount > 0 && finish.SafeError != ""
	default:
		return false
	}
}

func sameMallWeatherFeishuRunFinish(run model.PipelineRun, finish MallWeatherFeishuRunFinish) bool {
	return run.FinishedAt != nil && run.Status == finish.Status &&
		run.SuccessCount == finish.SuccessCount && run.FailedCount == finish.FailedCount &&
		run.TotalCount == finish.SuccessCount+finish.FailedCount && run.ErrorMessage == finish.SafeError &&
		run.FinishedAt.UTC().Truncate(time.Millisecond).Equal(finish.FinishedAt)
}

func validMallWeatherFeishuRunJSON(value model.JSONText, maxBytes int) bool {
	trimmed := strings.TrimSpace(string(value))
	return len(trimmed) >= 2 && len(trimmed) <= maxBytes && strings.HasPrefix(trimmed, "{") &&
		strings.HasSuffix(trimmed, "}") && json.Valid([]byte(trimmed))
}

func validMallWeatherFeishuRunToken(value string) bool {
	return uuid.Validate(value) == nil && value == strings.ToLower(value)
}

func validMallWeatherFeishuRunSafeError(value string) bool {
	if len(value) > maxMallWeatherFeishuRunSafeErrorBytes || value != strings.TrimSpace(value) {
		return false
	}
	return !strings.ContainsFunc(value, unicode.IsControl)
}
