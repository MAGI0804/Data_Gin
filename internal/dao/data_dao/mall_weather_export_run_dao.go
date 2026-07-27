package data_dao

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
	"unicode"

	"gin-biz-web-api/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const maxMallWeatherExportCheckpointBytes = 8 * 1024

var (
	ErrMallWeatherExportRunLeaseLost = errors.New("mall weather export run: lease lost")
	mallWeatherExportChecksumPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

type MallWeatherExportRunDisposition uint8

const (
	MallWeatherExportRunDispositionUnknown MallWeatherExportRunDisposition = iota
	MallWeatherExportRunDispositionAcquired
	MallWeatherExportRunDispositionBusy
	MallWeatherExportRunDispositionTerminal
)

type MallWeatherExportRunControl uint8

const (
	MallWeatherExportRunControlUnknown MallWeatherExportRunControl = iota
	MallWeatherExportRunControlContinue
	MallWeatherExportRunControlCancelRequested
	MallWeatherExportRunControlLeaseLost
)

type MallWeatherExportRunLease struct {
	Disposition MallWeatherExportRunDisposition
	RunToken    string
	Job         model.MallWeatherExportJob
}

type MallWeatherExportRunCheckpoint struct {
	RunToken     string          `json:"runToken"`
	DatasetIndex int             `json:"datasetIndex,omitempty"`
	SheetIndex   int             `json:"sheetIndex,omitempty"`
	RowsInSheet  int64           `json:"rowsInSheet,omitempty"`
	Cursor       json.RawMessage `json:"cursor,omitempty"`
}

type MallWeatherExportRunProgress struct {
	ProcessedRows int64
	CurrentSheet  string
	Checkpoint    MallWeatherExportRunCheckpoint
	UpdatedAt     time.Time
}

func (dao *MallWeatherExportJobDAO) BeginRun(
	ctx context.Context,
	jobID uint,
	runToken string,
	startedAt time.Time,
	staleAfter time.Duration,
) (*MallWeatherExportRunLease, error) {
	if dao == nil || dao.db == nil || ctx == nil || jobID == 0 || !validMallWeatherExportRunToken(runToken) ||
		startedAt.IsZero() || staleAfter <= 0 {
		return nil, fmt.Errorf("mall weather export run: invalid lease input")
	}
	startedAt = startedAt.UTC().Truncate(time.Millisecond)
	lease := &MallWeatherExportRunLease{}
	err := dao.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&lease.Job, jobID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrMallWeatherExportJobNotFound
		}
		if err != nil {
			return fmt.Errorf("mall weather export run: lock job: %w", err)
		}
		disposition, err := classifyMallWeatherExportRunStart(&lease.Job, startedAt, staleAfter)
		if err != nil {
			return err
		}
		lease.Disposition = disposition
		if disposition != MallWeatherExportRunDispositionAcquired {
			return nil
		}
		lease.RunToken = runToken
		checkpoint, err := encodeMallWeatherExportRunCheckpoint(MallWeatherExportRunCheckpoint{RunToken: runToken})
		if err != nil {
			return err
		}
		updates := map[string]interface{}{
			"status":             "running",
			"processed_rows":     0,
			"current_sheet":      "",
			"last_cursor_json":   checkpoint,
			"result_object_key":  "",
			"result_checksum":    "",
			"file_size_bytes":    0,
			"error_message_safe": "",
			"finished_at":        nil,
			"expires_at":         nil,
			"updated_at":         startedAt,
		}
		if lease.Job.StartedAt == nil {
			updates["started_at"] = &startedAt
			lease.Job.StartedAt = &startedAt
		}
		result := tx.Model(&model.MallWeatherExportJob{}).Where("id = ?", jobID).Updates(updates)
		if result.Error != nil {
			return fmt.Errorf("mall weather export run: claim job: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("mall weather export run: claim count changed")
		}
		lease.Job.Status = "running"
		lease.Job.ProcessedRows = 0
		lease.Job.CurrentSheet = ""
		lease.Job.LastCursorJSON = checkpoint
		lease.Job.ResultObjectKey = ""
		lease.Job.ResultChecksum = ""
		lease.Job.FileSizeBytes = 0
		lease.Job.ErrorMessageSafe = ""
		lease.Job.FinishedAt = nil
		lease.Job.ExpiresAt = nil
		lease.Job.UpdatedAt = startedAt
		return nil
	})
	if err != nil {
		return nil, err
	}
	return lease, nil
}

func classifyMallWeatherExportRunStart(
	job *model.MallWeatherExportJob,
	now time.Time,
	staleAfter time.Duration,
) (MallWeatherExportRunDisposition, error) {
	if job == nil || job.ID == 0 || now.IsZero() || staleAfter <= 0 || job.UpdatedAt.IsZero() {
		return MallWeatherExportRunDispositionUnknown, fmt.Errorf("mall weather export run: invalid stored job state")
	}
	switch strings.ToLower(strings.TrimSpace(job.Status)) {
	case "pending":
		return MallWeatherExportRunDispositionAcquired, nil
	case "running":
		if job.StartedAt == nil || job.StartedAt.IsZero() {
			return MallWeatherExportRunDispositionUnknown, fmt.Errorf("mall weather export run: running job has no start time")
		}
		if now.UTC().Before(job.UpdatedAt.UTC().Add(staleAfter)) {
			return MallWeatherExportRunDispositionBusy, nil
		}
		return MallWeatherExportRunDispositionAcquired, nil
	case "succeeded", "failed", "cancelled", "expired":
		return MallWeatherExportRunDispositionTerminal, nil
	default:
		return MallWeatherExportRunDispositionUnknown, fmt.Errorf("mall weather export run: unsupported job status")
	}
}

func (dao *MallWeatherExportJobDAO) UpdateRunProgress(
	ctx context.Context,
	jobID uint,
	runToken string,
	progress MallWeatherExportRunProgress,
) (MallWeatherExportRunControl, error) {
	if dao == nil || dao.db == nil || ctx == nil || jobID == 0 || progress.ProcessedRows < 0 ||
		!validMallWeatherExportCurrentSheet(progress.CurrentSheet) || progress.UpdatedAt.IsZero() ||
		progress.Checkpoint.RunToken != runToken {
		return MallWeatherExportRunControlUnknown, fmt.Errorf("mall weather export run: invalid progress update")
	}
	checkpoint, err := encodeMallWeatherExportRunCheckpoint(progress.Checkpoint)
	if err != nil {
		return MallWeatherExportRunControlUnknown, err
	}
	result := dao.ownedRunQuery(ctx, jobID, runToken).
		Where("cancel_requested = ?", false).
		Updates(map[string]interface{}{
			"processed_rows":   progress.ProcessedRows,
			"current_sheet":    progress.CurrentSheet,
			"last_cursor_json": checkpoint,
			"updated_at":       progress.UpdatedAt.UTC().Truncate(time.Millisecond),
		})
	if result.Error != nil {
		return MallWeatherExportRunControlUnknown, fmt.Errorf("mall weather export run: update progress: %w", result.Error)
	}
	if result.RowsAffected == 1 {
		return MallWeatherExportRunControlContinue, nil
	}
	return dao.InspectRun(ctx, jobID, runToken)
}

func (dao *MallWeatherExportJobDAO) HeartbeatRun(
	ctx context.Context,
	jobID uint,
	runToken string,
	now time.Time,
) (MallWeatherExportRunControl, error) {
	if dao == nil || dao.db == nil || ctx == nil || jobID == 0 || !validMallWeatherExportRunToken(runToken) || now.IsZero() {
		return MallWeatherExportRunControlUnknown, fmt.Errorf("mall weather export run: invalid heartbeat")
	}
	result := dao.ownedRunQuery(ctx, jobID, runToken).
		Where("cancel_requested = ?", false).
		Update("updated_at", now.UTC().Truncate(time.Millisecond))
	if result.Error != nil {
		return MallWeatherExportRunControlUnknown, fmt.Errorf("mall weather export run: heartbeat: %w", result.Error)
	}
	if result.RowsAffected == 1 {
		return MallWeatherExportRunControlContinue, nil
	}
	return dao.InspectRun(ctx, jobID, runToken)
}

func (dao *MallWeatherExportJobDAO) InspectRun(
	ctx context.Context,
	jobID uint,
	runToken string,
) (MallWeatherExportRunControl, error) {
	if dao == nil || dao.db == nil || ctx == nil || jobID == 0 || !validMallWeatherExportRunToken(runToken) {
		return MallWeatherExportRunControlUnknown, fmt.Errorf("mall weather export run: invalid inspection")
	}
	var row struct {
		Status          string         `gorm:"column:status"`
		CancelRequested bool           `gorm:"column:cancel_requested"`
		LastCursorJSON  model.JSONText `gorm:"column:last_cursor_json"`
	}
	err := dao.db.WithContext(ctx).Model(&model.MallWeatherExportJob{}).
		Select("status", "cancel_requested", "last_cursor_json").
		Where("id = ?", jobID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return MallWeatherExportRunControlLeaseLost, nil
	}
	if err != nil {
		return MallWeatherExportRunControlUnknown, fmt.Errorf("mall weather export run: inspect: %w", err)
	}
	checkpoint, err := decodeMallWeatherExportRunCheckpoint(row.LastCursorJSON)
	if err != nil || checkpoint.RunToken != runToken || strings.ToLower(strings.TrimSpace(row.Status)) != "running" {
		return MallWeatherExportRunControlLeaseLost, nil
	}
	if row.CancelRequested {
		return MallWeatherExportRunControlCancelRequested, nil
	}
	return MallWeatherExportRunControlContinue, nil
}

func (dao *MallWeatherExportJobDAO) MarkRunSucceeded(
	ctx context.Context,
	jobID uint,
	runToken string,
	resultObjectKey string,
	resultChecksum string,
	fileSizeBytes int64,
	finishedAt time.Time,
	expiresAt time.Time,
) error {
	if dao == nil || dao.db == nil || ctx == nil || jobID == 0 || !validMallWeatherExportRunToken(runToken) ||
		!validMallWeatherExportObjectKey(resultObjectKey) || !mallWeatherExportChecksumPattern.MatchString(resultChecksum) ||
		fileSizeBytes <= 0 || finishedAt.IsZero() || !expiresAt.After(finishedAt) {
		return fmt.Errorf("mall weather export run: invalid success update")
	}
	return dao.finishOwnedActiveRun(ctx, jobID, runToken, map[string]interface{}{
		"status":             "succeeded",
		"result_object_key":  resultObjectKey,
		"result_checksum":    resultChecksum,
		"file_size_bytes":    fileSizeBytes,
		"error_message_safe": "",
		"finished_at":        finishedAt.UTC().Truncate(time.Millisecond),
		"expires_at":         expiresAt.UTC().Truncate(time.Millisecond),
		"updated_at":         finishedAt.UTC().Truncate(time.Millisecond),
	})
}

// ConfirmRunSucceeded resolves the ambiguous outcome of MarkRunSucceeded.
// A database connection can fail after the server has committed the update, so
// callers must verify the stored artifact before deciding whether it is safe to
// delete the uploaded object.
func (dao *MallWeatherExportJobDAO) ConfirmRunSucceeded(
	ctx context.Context,
	jobID uint,
	resultObjectKey string,
	resultChecksum string,
	fileSizeBytes int64,
) (bool, error) {
	if dao == nil || dao.db == nil || ctx == nil || jobID == 0 ||
		!validMallWeatherExportObjectKey(resultObjectKey) ||
		!mallWeatherExportChecksumPattern.MatchString(resultChecksum) || fileSizeBytes <= 0 {
		return false, fmt.Errorf("mall weather export run: invalid success confirmation")
	}
	var row mallWeatherExportStoredResult
	err := dao.db.WithContext(ctx).Model(&model.MallWeatherExportJob{}).
		Select("status", "result_object_key", "result_checksum", "file_size_bytes").
		Where("id = ?", jobID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("mall weather export run: confirm success: %w", err)
	}
	return confirmMallWeatherExportStoredResult(
		row,
		resultObjectKey,
		resultChecksum,
		fileSizeBytes,
	)
}

type mallWeatherExportStoredResult struct {
	Status          string `gorm:"column:status"`
	ResultObjectKey string `gorm:"column:result_object_key"`
	ResultChecksum  string `gorm:"column:result_checksum"`
	FileSizeBytes   int64  `gorm:"column:file_size_bytes"`
}

func confirmMallWeatherExportStoredResult(
	row mallWeatherExportStoredResult,
	resultObjectKey string,
	resultChecksum string,
	fileSizeBytes int64,
) (bool, error) {
	if strings.EqualFold(strings.TrimSpace(row.Status), "succeeded") {
		if row.ResultObjectKey != resultObjectKey {
			return false, nil
		}
		if row.ResultChecksum != resultChecksum || row.FileSizeBytes != fileSizeBytes {
			return false, fmt.Errorf("mall weather export run: stored artifact metadata mismatch")
		}
		return true, nil
	}
	if row.ResultObjectKey == resultObjectKey {
		return false, fmt.Errorf("mall weather export run: unfinished job references uploaded artifact")
	}
	return false, nil
}

func (dao *MallWeatherExportJobDAO) MarkRunFailed(
	ctx context.Context,
	jobID uint,
	runToken string,
	safeError string,
	finishedAt time.Time,
	expiresAt time.Time,
) error {
	if dao == nil || dao.db == nil || ctx == nil || jobID == 0 || !validMallWeatherExportRunToken(runToken) ||
		!validMallWeatherExportSafeError(safeError) || finishedAt.IsZero() || !expiresAt.After(finishedAt) {
		return fmt.Errorf("mall weather export run: invalid failure update")
	}
	return dao.finishOwnedRun(ctx, jobID, runToken, map[string]interface{}{
		"status":             "failed",
		"error_message_safe": strings.TrimSpace(safeError),
		"finished_at":        finishedAt.UTC().Truncate(time.Millisecond),
		"expires_at":         expiresAt.UTC().Truncate(time.Millisecond),
		"updated_at":         finishedAt.UTC().Truncate(time.Millisecond),
	})
}

func (dao *MallWeatherExportJobDAO) MarkRunCancelled(
	ctx context.Context,
	jobID uint,
	runToken string,
	finishedAt time.Time,
	expiresAt time.Time,
) error {
	if dao == nil || dao.db == nil || ctx == nil || jobID == 0 || !validMallWeatherExportRunToken(runToken) ||
		finishedAt.IsZero() || !expiresAt.After(finishedAt) {
		return fmt.Errorf("mall weather export run: invalid cancellation update")
	}
	return dao.finishOwnedRun(ctx, jobID, runToken, map[string]interface{}{
		"status":             "cancelled",
		"error_message_safe": "",
		"finished_at":        finishedAt.UTC().Truncate(time.Millisecond),
		"expires_at":         expiresAt.UTC().Truncate(time.Millisecond),
		"updated_at":         finishedAt.UTC().Truncate(time.Millisecond),
	})
}

func (dao *MallWeatherExportJobDAO) ReleaseRunForRetry(
	ctx context.Context,
	jobID uint,
	runToken string,
	now time.Time,
) error {
	if dao == nil || dao.db == nil || ctx == nil || jobID == 0 || !validMallWeatherExportRunToken(runToken) || now.IsZero() {
		return fmt.Errorf("mall weather export run: invalid retry release")
	}
	result := dao.ownedRunQuery(ctx, jobID, runToken).Updates(map[string]interface{}{
		"status":             "pending",
		"processed_rows":     0,
		"current_sheet":      "",
		"last_cursor_json":   model.JSONText(`{}`),
		"error_message_safe": "",
		"finished_at":        nil,
		"expires_at":         nil,
		"updated_at":         now.UTC().Truncate(time.Millisecond),
	})
	if result.Error != nil {
		return fmt.Errorf("mall weather export run: release retry: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrMallWeatherExportRunLeaseLost
	}
	return nil
}

func (dao *MallWeatherExportJobDAO) ownedRunQuery(
	ctx context.Context,
	jobID uint,
	runToken string,
) *gorm.DB {
	return dao.db.WithContext(ctx).Model(&model.MallWeatherExportJob{}).
		Where("id = ? AND status = ?", jobID, "running").
		Where("JSON_UNQUOTE(JSON_EXTRACT(last_cursor_json, '$.runToken')) = ?", runToken)
}

func (dao *MallWeatherExportJobDAO) ownedActiveRunQuery(
	ctx context.Context,
	jobID uint,
	runToken string,
) *gorm.DB {
	return dao.ownedRunQuery(ctx, jobID, runToken).Where("cancel_requested = ?", false)
}

func (dao *MallWeatherExportJobDAO) finishOwnedRun(
	ctx context.Context,
	jobID uint,
	runToken string,
	updates map[string]interface{},
) error {
	return finishMallWeatherExportRun(dao.ownedRunQuery(ctx, jobID, runToken), updates)
}

func (dao *MallWeatherExportJobDAO) finishOwnedActiveRun(
	ctx context.Context,
	jobID uint,
	runToken string,
	updates map[string]interface{},
) error {
	return finishMallWeatherExportRun(dao.ownedActiveRunQuery(ctx, jobID, runToken), updates)
}

func finishMallWeatherExportRun(query *gorm.DB, updates map[string]interface{}) error {
	result := query.Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("mall weather export run: finish: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrMallWeatherExportRunLeaseLost
	}
	return nil
}

func encodeMallWeatherExportRunCheckpoint(
	checkpoint MallWeatherExportRunCheckpoint,
) (model.JSONText, error) {
	if !validMallWeatherExportRunToken(checkpoint.RunToken) || checkpoint.DatasetIndex < 0 ||
		checkpoint.SheetIndex < 0 || checkpoint.RowsInSheet < 0 || len(checkpoint.Cursor) > maxMallWeatherExportCheckpointBytes {
		return "", fmt.Errorf("mall weather export run: invalid checkpoint")
	}
	if len(checkpoint.Cursor) > 0 && !json.Valid(checkpoint.Cursor) {
		return "", fmt.Errorf("mall weather export run: invalid checkpoint cursor")
	}
	encoded, err := json.Marshal(checkpoint)
	if err != nil {
		return "", fmt.Errorf("mall weather export run: encode checkpoint: %w", err)
	}
	if len(encoded) > maxMallWeatherExportCheckpointBytes {
		return "", fmt.Errorf("mall weather export run: checkpoint is too large")
	}
	return model.JSONText(encoded), nil
}

func decodeMallWeatherExportRunCheckpoint(value model.JSONText) (MallWeatherExportRunCheckpoint, error) {
	if len(value) == 0 || len(value) > maxMallWeatherExportCheckpointBytes {
		return MallWeatherExportRunCheckpoint{}, fmt.Errorf("mall weather export run: invalid stored checkpoint")
	}
	var checkpoint MallWeatherExportRunCheckpoint
	decoder := json.NewDecoder(strings.NewReader(string(value)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&checkpoint); err != nil {
		return MallWeatherExportRunCheckpoint{}, fmt.Errorf("mall weather export run: decode checkpoint: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return MallWeatherExportRunCheckpoint{}, fmt.Errorf("mall weather export run: decode checkpoint: trailing data")
	}
	if _, err := encodeMallWeatherExportRunCheckpoint(checkpoint); err != nil {
		return MallWeatherExportRunCheckpoint{}, err
	}
	return checkpoint, nil
}

func validMallWeatherExportRunToken(value string) bool {
	return len(value) == 36 && uuid.Validate(value) == nil
}

func validMallWeatherExportSafeError(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 1024 {
		return false
	}
	for _, char := range value {
		if unicode.IsControl(char) {
			return false
		}
	}
	return true
}

func validMallWeatherExportObjectKey(value string) bool {
	if value == "" || value != strings.TrimSpace(value) || value != strings.TrimPrefix(value, "/") ||
		len(value) > 1024 || strings.Contains(value, "\\") {
		return false
	}
	for _, char := range value {
		if unicode.IsControl(char) {
			return false
		}
	}
	for _, part := range strings.Split(value, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

func validMallWeatherExportCurrentSheet(value string) bool {
	if len(value) > 255 {
		return false
	}
	for _, char := range value {
		if unicode.IsControl(char) {
			return false
		}
	}
	return true
}
