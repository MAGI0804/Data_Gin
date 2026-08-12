package reportrepo

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrDraftNotFound        = errors.New("report draft: not found")
	ErrDraftVersionConflict = errors.New("report draft: version conflict")
	ErrInvalidDraft         = errors.New("report draft: invalid input")
)

const maxDraftPageSize = 100

type Draft struct {
	Definition  model.ReportDefinition
	Version     model.ReportVersion
	Parameters  []model.ReportParameter
	Columns     []model.ReportColumn
	Grants      []model.ReportGrant
	LockVersion uint64
}

type DraftListQuery struct {
	AfterID  uint
	Limit    int
	Category string
	Search   string
}

type DraftSummary struct {
	Definition  model.ReportDefinition
	LockVersion uint64
}

type draftSummaryRecord struct {
	model.ReportDefinition `gorm:"embedded"`
	LockVersion            uint64 `gorm:"column:lock_version"`
}

type DraftPage struct {
	Items       []DraftSummary
	HasMore     bool
	NextAfterID uint
}

type transactionRunner func(context.Context, *gorm.DB, func(*gorm.DB) error) error

type Repository struct {
	db       *gorm.DB
	transact transactionRunner
}

func New(databases ...*gorm.DB) *Repository {
	db := database.DB
	if len(databases) > 0 && databases[0] != nil {
		db = databases[0]
	}
	return &Repository{db: db, transact: runTransaction}
}

func runTransaction(ctx context.Context, db *gorm.DB, operation func(*gorm.DB) error) error {
	return db.WithContext(ctx).Transaction(operation)
}

func (repository *Repository) CreateDraft(ctx context.Context, ownerUserID uint, draft *Draft) error {
	if err := repository.validate(ctx, ownerUserID); err != nil {
		return err
	}
	if err := validateNewDraft(draft); err != nil {
		return err
	}

	normalizeNewDraft(draft)
	err := repository.transact(ctx, repository.db, func(tx *gorm.DB) error {
		if draft.Definition.OwnerUserID != ownerUserID {
			return invalidDraft("definition owner does not match owner scope")
		}
		if err := validateDraftReferences(ctx, tx, draft.Definition.DatasourceID, draft.Grants); err != nil {
			return err
		}
		definitionRecord := newDefinitionRecord(draft.Definition)
		if err := tx.WithContext(ctx).Create(&definitionRecord).Error; err != nil {
			return fmt.Errorf("report draft: create definition: %w", err)
		}

		draft.Definition.ID = definitionRecord.ID
		draft.Version.DefinitionID = definitionRecord.ID
		versionRecord := newVersionRecord(draft.Version)
		if err := tx.WithContext(ctx).Create(&versionRecord).Error; err != nil {
			return fmt.Errorf("report draft: create version: %w", err)
		}

		draft.Version.ID = versionRecord.ID
		result := definitionScope(tx.WithContext(ctx), ownerUserID).
			Where("id = ?", definitionRecord.ID).
			Updates(map[string]interface{}{
				"current_draft_version_id": versionRecord.ID,
			})
		if result.Error != nil {
			return fmt.Errorf("report draft: link draft version: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return ErrDraftVersionConflict
		}
		if err := replaceCollections(ctx, tx, ownerUserID, definitionRecord.ID, versionRecord.ID,
			draft.Parameters, draft.Columns, draft.Grants); err != nil {
			return err
		}
		draft.Definition.CurrentDraftVersionID = versionRecord.ID
		draft.LockVersion = versionRecord.VersionNumber
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (repository *Repository) FindDraftByID(ctx context.Context, ownerUserID, definitionID uint) (*Draft, error) {
	if err := repository.validate(ctx, ownerUserID); err != nil || definitionID == 0 {
		return nil, invalidDraft("owner and definition id are required")
	}

	var definition definitionRecord
	err := definitionScope(repository.db.WithContext(ctx), ownerUserID).
		Where("id = ?", definitionID).
		First(&definition).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrDraftNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("report draft: find definition: %w", err)
	}
	if definition.CurrentDraftVersionID == 0 {
		return nil, ErrDraftNotFound
	}

	var version versionRecord
	err = versionScope(repository.db.WithContext(ctx)).
		Where("id = ? AND definition_id = ?", definition.CurrentDraftVersionID, definitionID).
		First(&version).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrDraftNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("report draft: find version: %w", err)
	}

	draft := &Draft{
		Definition:  definition.ReportDefinition,
		Version:     version.ReportVersion,
		LockVersion: version.VersionNumber,
	}
	if err := loadCollections(ctx, repository.db, ownerUserID, definitionID, version.ID, draft); err != nil {
		return nil, err
	}
	return draft, nil
}

func (repository *Repository) ListDrafts(ctx context.Context, ownerUserID uint, query DraftListQuery) (DraftPage, error) {
	if err := repository.validate(ctx, ownerUserID); err != nil {
		return DraftPage{}, err
	}
	if query.Limit < 1 || query.Limit > maxDraftPageSize {
		return DraftPage{}, invalidDraft("page limit must be between 1 and 100")
	}

	dbQuery := buildDraftListQuery(repository.db.WithContext(ctx), ownerUserID, query)

	var records []draftSummaryRecord
	if err := dbQuery.Scan(&records).Error; err != nil {
		return DraftPage{}, fmt.Errorf("report draft: list definitions: %w", err)
	}
	page := DraftPage{Items: make([]DraftSummary, 0, min(query.Limit, len(records)))}
	if len(records) > query.Limit {
		page.HasMore = true
		records = records[:query.Limit]
	}
	for _, record := range records {
		page.Items = append(page.Items, DraftSummary{
			Definition: record.ReportDefinition, LockVersion: record.LockVersion,
		})
	}
	if len(page.Items) > 0 {
		page.NextAfterID = page.Items[len(page.Items)-1].Definition.ID
	}
	return page, nil
}

func buildDraftListQuery(db *gorm.DB, ownerUserID uint, query DraftListQuery) *gorm.DB {
	dbQuery := db.Table("report_definitions AS definitions").
		Select("definitions.*, versions.version_number AS lock_version").
		Joins("JOIN report_versions AS versions ON versions.id = definitions.current_draft_version_id AND versions.definition_id = definitions.id AND versions.status = ?", model.ReportVersionStatusDraft).
		Where("definitions.owner_user_id = ? AND definitions.status IN ?", ownerUserID,
			[]string{model.ReportDefinitionStatusDraft, model.ReportDefinitionStatusActive})
	if query.AfterID > 0 {
		dbQuery = dbQuery.Where("definitions.id > ?", query.AfterID)
	}
	if category := strings.TrimSpace(query.Category); category != "" {
		dbQuery = dbQuery.Where("definitions.category = ?", category)
	}
	if search := strings.TrimSpace(query.Search); search != "" {
		like := "%" + escapeLike(search) + "%"
		dbQuery = dbQuery.Where("(definitions.code LIKE ? ESCAPE '\\\\' OR definitions.name LIKE ? ESCAPE '\\\\')", like, like)
	}

	return dbQuery.Order("definitions.id ASC").Limit(query.Limit + 1)
}

func (repository *Repository) UpdateDraft(
	ctx context.Context,
	ownerUserID, definitionID uint,
	expectedLockVersion uint64,
	draft *Draft,
) error {
	if err := repository.validate(ctx, ownerUserID); err != nil {
		return err
	}
	if err := validateExistingDraft(definitionID, expectedLockVersion, draft); err != nil {
		return err
	}

	err := repository.transact(ctx, repository.db, func(tx *gorm.DB) error {
		current, err := lockDraftDefinition(ctx, tx, ownerUserID, definitionID)
		if err != nil {
			return err
		}
		version, err := lockDraftVersion(ctx, tx, definitionID, current.CurrentDraftVersionID)
		if err != nil {
			return err
		}
		if version.VersionNumber != expectedLockVersion {
			return ErrDraftVersionConflict
		}
		if draft.Definition.OwnerUserID != 0 && draft.Definition.OwnerUserID != ownerUserID {
			return invalidDraft("definition owner cannot be changed")
		}
		if err := validateDraftReferences(ctx, tx, draft.Definition.DatasourceID, draft.Grants); err != nil {
			return err
		}

		nextVersion := draft.Version
		nextVersion.ID = 0
		nextVersion.DefinitionID = definitionID
		nextVersion.VersionNumber = expectedLockVersion + 1
		nextVersion.Status = model.ReportVersionStatusDraft
		nextVersion.PublishedBy = 0
		nextVersion.PublishedAt = nil
		nextVersion.WeatherTimestamps = model.WeatherTimestamps{}
		if nextVersion.CreatedBy == 0 {
			nextVersion.CreatedBy = draft.Definition.UpdatedBy
		}
		nextRecord := newVersionRecord(nextVersion)
		if err := tx.WithContext(ctx).Create(&nextRecord).Error; err != nil {
			return fmt.Errorf("report draft: create next version: %w", err)
		}

		result := definitionScope(tx.WithContext(ctx), ownerUserID).
			Where("id = ? AND current_draft_version_id = ?", definitionID, version.ID).
			Updates(definitionUpdates(draft.Definition, nextRecord.ID))
		if result.Error != nil {
			return fmt.Errorf("report draft: update definition: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return ErrDraftVersionConflict
		}

		if err := replaceCollections(ctx, tx, ownerUserID, definitionID, nextRecord.ID,
			draft.Parameters, draft.Columns, draft.Grants); err != nil {
			return err
		}

		draft.Definition.ID = definitionID
		draft.Definition.OwnerUserID = ownerUserID
		draft.Definition.Status = current.Status
		draft.Definition.CreatedAt = current.CreatedAt
		draft.Definition.CurrentDraftVersionID = nextRecord.ID
		draft.Definition.CurrentPublishedVersionID = current.CurrentPublishedVersionID
		draft.Version.ID = nextRecord.ID
		draft.Version.DefinitionID = definitionID
		draft.Version.VersionNumber = expectedLockVersion + 1
		draft.Version.Status = model.ReportVersionStatusDraft
		draft.LockVersion = expectedLockVersion + 1
		return nil
	})
	return err
}

func (repository *Repository) SaveDraftCollections(
	ctx context.Context,
	ownerUserID, definitionID uint,
	expectedLockVersion uint64,
	parameters []model.ReportParameter,
	columns []model.ReportColumn,
	grants []model.ReportGrant,
) (uint64, error) {
	if err := repository.validate(ctx, ownerUserID); err != nil {
		return 0, err
	}
	if definitionID == 0 || expectedLockVersion == 0 {
		return 0, invalidDraft("definition id and lock version are required")
	}
	if err := validateCollections(parameters, columns, grants); err != nil {
		return 0, err
	}

	nextLockVersion := expectedLockVersion + 1
	err := repository.transact(ctx, repository.db, func(tx *gorm.DB) error {
		definition, err := lockDraftDefinition(ctx, tx, ownerUserID, definitionID)
		if err != nil {
			return err
		}
		version, err := lockDraftVersion(ctx, tx, definitionID, definition.CurrentDraftVersionID)
		if err != nil {
			return err
		}
		if version.VersionNumber != expectedLockVersion {
			return ErrDraftVersionConflict
		}
		if err := validateDraftReferences(ctx, tx, definition.DatasourceID, grants); err != nil {
			return err
		}
		nextVersion := version.ReportVersion
		nextVersion.ID = 0
		nextVersion.VersionNumber = nextLockVersion
		nextVersion.PublishedBy = 0
		nextVersion.PublishedAt = nil
		nextVersion.WeatherTimestamps = model.WeatherTimestamps{}
		nextRecord := newVersionRecord(nextVersion)
		if err := tx.WithContext(ctx).Create(&nextRecord).Error; err != nil {
			return fmt.Errorf("report draft: create next collection version: %w", err)
		}
		result := definitionScope(tx.WithContext(ctx), ownerUserID).
			Where("id = ? AND current_draft_version_id = ?", definitionID, version.ID).
			Updates(map[string]interface{}{"current_draft_version_id": nextRecord.ID})
		if result.Error != nil {
			return fmt.Errorf("report draft: switch collection version: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return ErrDraftVersionConflict
		}
		return replaceCollections(ctx, tx, ownerUserID, definitionID, nextRecord.ID, parameters, columns, grants)
	})
	if err != nil {
		return 0, err
	}
	return nextLockVersion, nil
}

func (repository *Repository) validate(ctx context.Context, ownerUserID uint) error {
	if repository == nil || repository.db == nil || repository.transact == nil || ctx == nil || ownerUserID == 0 {
		return invalidDraft("repository, context and owner scope are required")
	}
	return nil
}

func lockDraftDefinition(ctx context.Context, tx *gorm.DB, ownerUserID, definitionID uint) (*definitionRecord, error) {
	var definition definitionRecord
	err := definitionScope(tx.WithContext(ctx), ownerUserID).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", definitionID).
		First(&definition).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrDraftVersionConflict
	}
	if err != nil {
		return nil, fmt.Errorf("report draft: lock definition: %w", err)
	}
	return &definition, nil
}

func lockDraftVersion(ctx context.Context, tx *gorm.DB, definitionID, versionID uint) (*versionRecord, error) {
	if versionID == 0 {
		return nil, ErrDraftVersionConflict
	}
	var version versionRecord
	err := versionScope(tx.WithContext(ctx)).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND definition_id = ?", versionID, definitionID).
		First(&version).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrDraftVersionConflict
	}
	if err != nil {
		return nil, fmt.Errorf("report draft: lock version: %w", err)
	}
	return &version, nil
}

func definitionUpdates(definition model.ReportDefinition, currentDraftVersionID uint) map[string]interface{} {
	return map[string]interface{}{
		"code": definition.Code, "name": definition.Name, "category": definition.Category,
		"description": definition.Description, "datasource_id": definition.DatasourceID,
		"updated_by": definition.UpdatedBy, "current_draft_version_id": currentDraftVersionID,
	}
}

func definitionScope(db *gorm.DB, ownerUserID uint) *gorm.DB {
	return db.Model(&definitionRecord{}).Where("owner_user_id = ? AND status IN ?", ownerUserID,
		[]string{model.ReportDefinitionStatusDraft, model.ReportDefinitionStatusActive})
}

func versionScope(db *gorm.DB) *gorm.DB {
	return db.Model(&versionRecord{}).Where("status = ?", model.ReportVersionStatusDraft)
}

func invalidDraft(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalidDraft, message)
}

func escapeLike(value string) string {
	replacer := strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_")
	return replacer.Replace(value)
}
