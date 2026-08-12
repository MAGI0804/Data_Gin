package reportrepo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gin-biz-web-api/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrReportExportRunNotReady = errors.New("report export: run not ready")
	ErrReportExportNotFound    = errors.New("report export: not found")
)

type CreateExportCommand struct {
	Export model.ReportExport
	Outbox model.AsyncJobOutbox
}

func (repository *Repository) CreateOrGetExport(ctx context.Context, actor, runID uint, command *CreateExportCommand) (bool, error) {
	if repository == nil || repository.db == nil || repository.writeAudit == nil || ctx == nil || actor == 0 || runID == 0 || command == nil ||
		uuid.Validate(command.Export.ExportUUID) != nil || command.Export.RunID != runID || command.Export.CreatedBy != actor ||
		command.Export.Status != model.ReportExportStatusPending || !validReportExportOutbox(command.Outbox, command.Export.ExportUUID) {
		return false, fmt.Errorf("report export: invalid create request")
	}
	created := false
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var run model.ReportRun
		if err := buildActorRunQuery(tx.Clauses(clause.Locking{Strength: "UPDATE"}), actor, runID).First(&run).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrReportRunAccessNotFound
		} else if err != nil {
			return fmt.Errorf("report export: lock run: %w", err)
		}
		requestedAt := command.Outbox.AvailableAt.UTC()
		if run.Status != model.ReportRunStatusSucceeded || run.ResultPurgedAt != nil || run.ResultExpiresAt == nil || !requestedAt.Before(run.ResultExpiresAt.UTC()) {
			return ErrReportExportRunNotReady
		}
		if _, err := loadPublishedReport(ctx, tx, actor, run.DefinitionID, ReportActionExport, false); errors.Is(err, ErrReportActionDenied) {
			return ErrReportExportRunNotReady
		} else if err != nil {
			return fmt.Errorf("report export: authorize report: %w", err)
		}
		var existing model.ReportExport
		if err := tx.Where("run_id = ?", runID).First(&existing).Error; err == nil {
			command.Export = existing
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("report export: find existing: %w", err)
		}
		command.Export.FrozenColumnsJSON = run.PresentationSnapshotJSON
		if err := tx.Create(&command.Export).Error; err != nil {
			return fmt.Errorf("report export: create job: %w", err)
		}
		command.Outbox.PayloadJSON = model.JSONText(fmt.Sprintf(`{"export_id":%d}`, command.Export.ID))
		if err := tx.Create(&command.Outbox).Error; err != nil {
			return fmt.Errorf("report export: create outbox: %w", err)
		}
		detail, err := json.Marshal(map[string]interface{}{"runId": runID, "exportId": command.Export.ID})
		if err != nil {
			return fmt.Errorf("report export: encode audit: %w", err)
		}
		if err := repository.writeAudit(ctx, tx, model.ReportAudit{ActorUserID: actor, Action: "REPORT_EXPORT_CREATE", TargetType: "REPORT_EXPORT", TargetID: command.Export.ID, RequestID: uuid.NewString(), DetailJSON: model.JSONText(detail)}); err != nil {
			return err
		}
		created = true
		return nil
	})
	return created, err
}

func validReportExportOutbox(outbox model.AsyncJobOutbox, exportUUID string) bool {
	return outbox.ID == 0 && outbox.TaskKey == "report:export:"+exportUUID && outbox.TaskType == "report:export" &&
		outbox.QueueName == "report_export" && !outbox.AvailableAt.IsZero() && outbox.PublishedAt == nil && string(outbox.PayloadJSON) == `{"export_id":0}`
}

func NewReportExportOutbox(exportUUID string, now time.Time) model.AsyncJobOutbox {
	return model.AsyncJobOutbox{TaskKey: "report:export:" + exportUUID, TaskType: "report:export", PayloadJSON: model.JSONText(`{"export_id":0}`), QueueName: "report_export", AvailableAt: now.UTC()}
}
