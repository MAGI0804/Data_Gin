package reportrepo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"gin-biz-web-api/model"
)

var (
	ErrInputQueryNotFound        = errors.New("report input query: not found")
	ErrInputQueryVersionConflict = errors.New("report input query: version conflict")
	ErrInputQueryInUse           = errors.New("report input query: in use")
)

func (repository *Repository) ListReportInputQueryDefinitions(ctx context.Context) ([]model.ReportInputQueryDefinition, error) {
	if repository == nil || repository.db == nil || ctx == nil {
		return nil, fmt.Errorf("report input query: repository and context are required")
	}
	var items []model.ReportInputQueryDefinition
	if err := repository.db.WithContext(ctx).Order("name ASC, id ASC").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("report input query: list: %w", err)
	}
	return items, nil
}

func (repository *Repository) GetReportInputQueryDefinition(ctx context.Context, definitionID uint) (*model.ReportInputQueryDefinition, error) {
	if repository == nil || repository.db == nil || ctx == nil || definitionID == 0 {
		return nil, ErrInputQueryNotFound
	}
	var definition model.ReportInputQueryDefinition
	err := repository.db.WithContext(ctx).Where("id = ?", definitionID).First(&definition).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrInputQueryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("report input query: get: %w", err)
	}
	return &definition, nil
}

func (repository *Repository) FindEnabledReportInputQueryByName(ctx context.Context, name string) (*model.ReportInputQueryDefinition, error) {
	if repository == nil || repository.db == nil || ctx == nil || name == "" {
		return nil, ErrInputQueryNotFound
	}
	var definition model.ReportInputQueryDefinition
	err := repository.db.WithContext(ctx).Where("name = ? AND enabled = ?", name, true).First(&definition).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrInputQueryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("report input query: find enabled definition: %w", err)
	}
	return &definition, nil
}

func (repository *Repository) CreateReportInputQueryDefinition(ctx context.Context, actor uint, definition *model.ReportInputQueryDefinition) error {
	if repository == nil || repository.db == nil || ctx == nil || actor == 0 || definition == nil {
		return fmt.Errorf("report input query: repository, actor and definition are required")
	}
	definition.CreatedBy = actor
	definition.UpdatedBy = actor
	definition.LockVersion = 1
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(definition).Error; err != nil {
			return fmt.Errorf("report input query: create: %w", err)
		}
		return createInputQueryAudit(ctx, tx, actor, definition, "REPORT_INPUT_QUERY_CREATE")
	})
}

func (repository *Repository) UpdateReportInputQueryDefinition(ctx context.Context, actor uint, definition *model.ReportInputQueryDefinition, expectedLockVersion uint64) error {
	if repository == nil || repository.db == nil || ctx == nil || actor == 0 || definition == nil || definition.ID == 0 || expectedLockVersion == 0 {
		return ErrInputQueryNotFound
	}
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		current, err := lockInputQueryDefinition(tx.WithContext(ctx), definition.ID)
		if err != nil {
			return err
		}
		if current.LockVersion != expectedLockVersion {
			return ErrInputQueryVersionConflict
		}
		if current.Name != definition.Name {
			referenced, referenceErr := reportInputQueryNameReferenced(ctx, tx, current.Name)
			if referenceErr != nil {
				return referenceErr
			}
			if referenced {
				return ErrInputQueryInUse
			}
		}
		nextLockVersion := current.LockVersion + 1
		result := tx.Model(&model.ReportInputQueryDefinition{}).
			Where("id = ? AND lock_version = ?", definition.ID, expectedLockVersion).
			Updates(map[string]interface{}{
				"name": definition.Name, "select_sql": definition.SelectSQL, "enabled": definition.Enabled,
				"lock_version": nextLockVersion, "updated_by": actor,
			})
		if result.Error != nil {
			return fmt.Errorf("report input query: update: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return ErrInputQueryVersionConflict
		}
		definition.LockVersion = nextLockVersion
		definition.CreatedBy = current.CreatedBy
		definition.UpdatedBy = actor
		definition.CreatedAt = current.CreatedAt
		return createInputQueryAudit(ctx, tx, actor, definition, "REPORT_INPUT_QUERY_UPDATE")
	})
}

func (repository *Repository) DeleteReportInputQueryDefinition(ctx context.Context, actor, definitionID uint, expectedLockVersion uint64) error {
	if repository == nil || repository.db == nil || ctx == nil || actor == 0 || definitionID == 0 || expectedLockVersion == 0 {
		return ErrInputQueryNotFound
	}
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		current, err := lockInputQueryDefinition(tx.WithContext(ctx), definitionID)
		if err != nil {
			return err
		}
		if current.LockVersion != expectedLockVersion {
			return ErrInputQueryVersionConflict
		}
		referenced, err := reportInputQueryNameReferenced(ctx, tx, current.Name)
		if err != nil {
			return err
		}
		if referenced {
			return ErrInputQueryInUse
		}
		if err := createInputQueryAudit(ctx, tx, actor, current, "REPORT_INPUT_QUERY_DELETE"); err != nil {
			return err
		}
		result := tx.Where("id = ? AND lock_version = ?", definitionID, expectedLockVersion).Delete(&model.ReportInputQueryDefinition{})
		if result.Error != nil {
			return fmt.Errorf("report input query: delete: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return ErrInputQueryVersionConflict
		}
		return nil
	})
}

func (repository *Repository) RecordReportInputQueryTest(ctx context.Context, actor, definitionID uint, status, safeError string, testedAt time.Time) error {
	if repository == nil || repository.db == nil || ctx == nil || actor == 0 || definitionID == 0 {
		return ErrInputQueryNotFound
	}
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.ReportInputQueryDefinition{}).Where("id = ?", definitionID).Updates(map[string]interface{}{
			"last_test_status": status, "last_test_error_safe": safeError, "last_tested_at": testedAt, "updated_by": actor,
		})
		if result.Error != nil {
			return fmt.Errorf("report input query: record test: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return ErrInputQueryNotFound
		}
		audit := &model.ReportInputQueryDefinition{BaseModel: model.BaseModel{ID: definitionID}, LastTestStatus: status}
		return createInputQueryAudit(ctx, tx, actor, audit, "REPORT_INPUT_QUERY_TEST")
	})
}

func lockInputQueryDefinition(db *gorm.DB, definitionID uint) (*model.ReportInputQueryDefinition, error) {
	var definition model.ReportInputQueryDefinition
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", definitionID).First(&definition).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrInputQueryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("report input query: lock: %w", err)
	}
	return &definition, nil
}

func reportInputQueryNameReferenced(ctx context.Context, tx *gorm.DB, name string) (bool, error) {
	var count int64
	err := buildReportInputQueryReferenceQuery(tx.WithContext(ctx), name).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("report input query: inspect report references: %w", err)
	}
	return count > 0, nil
}

func buildReportInputQueryReferenceQuery(db *gorm.DB, name string) *gorm.DB {
	return db.Table("report_versions AS versions").
		Joins(`JOIN report_definitions AS definitions ON
			definitions.current_draft_version_id = versions.id OR definitions.current_published_version_id = versions.id`).
		Where("JSON_SEARCH(versions.input_schema_json, 'one', ?, NULL, '$.*.queryName') IS NOT NULL", name)
}

func createInputQueryAudit(ctx context.Context, tx *gorm.DB, actor uint, definition *model.ReportInputQueryDefinition, action string) error {
	detail := map[string]interface{}{"name": definition.Name, "enabled": definition.Enabled}
	if definition.SelectSQL != "" {
		digest := sha256.Sum256([]byte(definition.SelectSQL))
		detail["selectHash"] = hex.EncodeToString(digest[:])
	}
	if definition.LastTestStatus != "" {
		detail["status"] = definition.LastTestStatus
	}
	encoded, err := json.Marshal(detail)
	if err != nil {
		return fmt.Errorf("report input query: encode audit: %w", err)
	}
	audit := model.ReportAudit{
		ActorUserID: actor, Action: action, TargetType: "REPORT_INPUT_QUERY", TargetID: definition.ID,
		RequestID: uuid.NewString(), DetailJSON: model.JSONText(encoded),
	}
	if err := tx.WithContext(ctx).Create(&audit).Error; err != nil {
		return fmt.Errorf("report input query: create audit: %w", err)
	}
	return nil
}
