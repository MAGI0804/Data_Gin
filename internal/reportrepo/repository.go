package reportrepo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrDraftNotFound         = errors.New("report draft: not found")
	ErrDatasourceUnavailable = errors.New("report draft: datasource unavailable")
	ErrDraftVersionConflict  = errors.New("report draft: version conflict")
	ErrInvalidDraft          = errors.New("report draft: invalid input")
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
	IsOwner     bool
}

type draftSummaryRecord struct {
	model.ReportDefinition `gorm:"embedded"`
	LockVersion            uint64 `gorm:"column:lock_version"`
	IsOwner                bool   `gorm:"column:is_owner"`
}

type DraftPage struct {
	Items       []DraftSummary
	HasMore     bool
	NextAfterID uint
}

type Publication struct {
	CompiledSpecJSON       model.JSONText
	ContractHash           string
	ParameterSchemaHash    string
	ProcedureSignatureHash string
	ResultSchemaHash       string
	PermissionHash         string
	ExportSchemaHash       string
	SchemaProbeToken       string
	SchemaValidatedAt      time.Time
}

type transactionRunner func(context.Context, *gorm.DB, func(*gorm.DB) error) error
type draftReferenceValidator func(context.Context, *gorm.DB, uint, []model.ReportGrant) error
type draftDefinitionLocker func(context.Context, *gorm.DB, uint, uint) (*definitionRecord, error)
type draftVersionLocker func(context.Context, *gorm.DB, uint, uint) (*versionRecord, error)
type reportAuditWriter func(context.Context, *gorm.DB, model.ReportAudit) error
type systemReportAuditWriter func(context.Context, *gorm.DB, string, string, uint, map[string]interface{}) error
type draftCollectionsLoader func(context.Context, *gorm.DB, uint, uint, uint, *Draft) error
type publishedVersionWriter func(context.Context, *gorm.DB, uint, uint, uint64, map[string]interface{}) error
type draftVersionCreator func(context.Context, *gorm.DB, *versionRecord) error
type versionCollectionsCopier func(context.Context, *gorm.DB, uint, uint, []model.ReportParameter, []model.ReportColumn, []model.ReportGrant) error
type publishedDefinitionSwitcher func(context.Context, *gorm.DB, uint, uint, uint, uint, uint) error
type publishedReportLoader func(context.Context, *gorm.DB, uint, uint, string, bool) (*PublishedReport, error)
type reportRunWriter func(context.Context, *gorm.DB, *model.ReportRun) error
type reportRunOutboxWriter func(context.Context, *gorm.DB, *model.AsyncJobOutbox) error
type enabledDatasourceValidator func(context.Context, *gorm.DB, uint) error
type reportRunSlotPreparer func(context.Context, *gorm.DB, uint, time.Time) ([]uint, error)

type Repository struct {
	db                 *gorm.DB
	transact           transactionRunner
	validateReferences draftReferenceValidator
	lockDefinition     draftDefinitionLocker
	lockVersion        draftVersionLocker
	writeAudit         reportAuditWriter
	writeSystemAudit   systemReportAuditWriter
	loadCollections    draftCollectionsLoader
	publishVersion     publishedVersionWriter
	createVersion      draftVersionCreator
	copyCollections    versionCollectionsCopier
	switchDefinition   publishedDefinitionSwitcher
	loadPublished      publishedReportLoader
	createReportRun    reportRunWriter
	createRunOutbox    reportRunOutboxWriter
	validateRunSource  enabledDatasourceValidator
	prepareRunSlot     reportRunSlotPreparer
}

func New(databases ...*gorm.DB) *Repository {
	db := database.DB
	if len(databases) > 0 && databases[0] != nil {
		db = databases[0]
	}
	return &Repository{
		db: db, transact: runTransaction, validateReferences: validateDraftReferences,
		lockDefinition: lockDraftDefinition, lockVersion: lockDraftVersion, writeAudit: createReportAudit, writeSystemAudit: writeSystemReportAudit,
		loadCollections: loadCollections, publishVersion: writePublishedVersion, createVersion: createDraftVersion,
		copyCollections: replaceVersionCollections, switchDefinition: switchPublishedDefinition,
		loadPublished: loadPublishedReport, createReportRun: writeReportRun, createRunOutbox: writeReportRunOutbox,
		validateRunSource: requireEnabledOracleDatasource, prepareRunSlot: prepareReportRunSlot,
	}
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
		if err := repository.validateReferences(ctx, tx, draft.Definition.DatasourceID, draft.Grants); err != nil {
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
		if err := finalizeReportMutation(ctx, tx, newDraftAudit("REPORT_DRAFT_CREATE", ownerUserID, definitionRecord.ID, versionRecord.VersionNumber, draft), repository.writeAudit); err != nil {
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

func (repository *Repository) FindDatasource(ctx context.Context, datasourceID uint) (*model.ReportDatasource, error) {
	if repository == nil || repository.db == nil || ctx == nil || datasourceID == 0 {
		return nil, invalidDraft("repository, context and datasource id are required")
	}
	var datasource model.ReportDatasource
	err := repository.db.WithContext(ctx).Where("id = ? AND enabled = ? AND driver = ?", datasourceID, true, model.ReportDatasourceDriverOracle).First(&datasource).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrDatasourceUnavailable
	}
	if err != nil {
		return nil, fmt.Errorf("report draft: find datasource: %w", err)
	}
	return &datasource, nil
}

func (repository *Repository) PublishDraft(ctx context.Context, ownerUserID, definitionID uint, expectedLockVersion uint64, publication Publication) (*Draft, error) {
	if err := repository.validate(ctx, ownerUserID); err != nil {
		return nil, err
	}
	if definitionID == 0 || expectedLockVersion == 0 || !validPublication(publication) {
		return nil, invalidDraft("definition, lock version and publication are required")
	}
	var published Draft
	var publishedAt time.Time
	var nextDraftVersionID uint
	err := repository.transact(ctx, repository.db, func(tx *gorm.DB) error {
		definition, err := repository.lockDefinition(ctx, tx, ownerUserID, definitionID)
		if err != nil {
			return err
		}
		current, err := repository.lockVersion(ctx, tx, definitionID, definition.CurrentDraftVersionID)
		if err != nil {
			return err
		}
		if current.VersionNumber != expectedLockVersion {
			return ErrDraftVersionConflict
		}
		published = Draft{Definition: definition.ReportDefinition, Version: current.ReportVersion, LockVersion: current.VersionNumber}
		if err := repository.loadCollections(ctx, tx, ownerUserID, definitionID, current.ID, &published); err != nil {
			return err
		}
		if current.DatasourceID == 0 || current.DatasourceID != definition.DatasourceID {
			return invalidDraft("draft datasource snapshot is inconsistent")
		}
		if err := repository.validateReferences(ctx, tx, current.DatasourceID, published.Grants); err != nil {
			return err
		}
		publishedAt = time.Now().UTC()
		publishedUpdates := map[string]interface{}{
			"status": model.ReportVersionStatusPublished, "compiled_spec_json": publication.CompiledSpecJSON,
			"contract_hash": publication.ContractHash, "parameter_schema_hash": publication.ParameterSchemaHash,
			"procedure_signature_hash": publication.ProcedureSignatureHash, "result_schema_hash": publication.ResultSchemaHash,
			"permission_hash": publication.PermissionHash, "export_schema_hash": publication.ExportSchemaHash,
			"schema_probe_token": publication.SchemaProbeToken, "schema_validated_at": publication.SchemaValidatedAt,
			"published_by": ownerUserID, "published_at": publishedAt,
		}
		if err := repository.publishVersion(ctx, tx, current.ID, definitionID, expectedLockVersion, publishedUpdates); err != nil {
			return err
		}

		next := nextDraftAfterPublication(current.ReportVersion, ownerUserID)
		nextRecord := newVersionRecord(next)
		if err := repository.createVersion(ctx, tx, &nextRecord); err != nil {
			return err
		}
		nextDraftVersionID = nextRecord.ID
		if err := repository.copyCollections(
			ctx,
			tx,
			definitionID,
			nextRecord.ID,
			published.Parameters,
			published.Columns,
			published.Grants,
		); err != nil {
			return err
		}
		if err := repository.switchDefinition(ctx, tx, ownerUserID, definitionID, current.ID, nextRecord.ID, ownerUserID); err != nil {
			return err
		}
		return repository.writeAudit(ctx, tx, buildReportAudit("REPORT_PUBLISH", ownerUserID, definitionID, reportDraftAuditDetail{VersionNumber: expectedLockVersion, Code: definition.Code, DatasourceID: current.DatasourceID, ParameterCount: len(published.Parameters), ColumnCount: len(published.Columns), GrantCount: len(published.Grants)}))
	})
	if err != nil {
		return nil, err
	}
	published.Version.Status = model.ReportVersionStatusPublished
	published.Version.CompiledSpecJSON = publication.CompiledSpecJSON
	published.Version.ContractHash = publication.ContractHash
	published.Version.ParameterSchemaHash = publication.ParameterSchemaHash
	published.Version.ProcedureSignatureHash = publication.ProcedureSignatureHash
	published.Version.ResultSchemaHash = publication.ResultSchemaHash
	published.Version.PermissionHash = publication.PermissionHash
	published.Version.ExportSchemaHash = publication.ExportSchemaHash
	published.Version.SchemaProbeToken = publication.SchemaProbeToken
	published.Version.SchemaValidatedAt = &publication.SchemaValidatedAt
	published.Version.PublishedBy = ownerUserID
	published.Version.PublishedAt = &publishedAt
	published.Definition.Status = model.ReportDefinitionStatusActive
	published.Definition.CurrentPublishedVersionID = published.Version.ID
	published.Definition.CurrentDraftVersionID = nextDraftVersionID
	return &published, nil
}

func nextDraftAfterPublication(current model.ReportVersion, actor uint) model.ReportVersion {
	current.ID = 0
	current.VersionNumber++
	current.Status = model.ReportVersionStatusDraft
	current.CompiledSpecJSON = ""
	current.ContractHash, current.ParameterSchemaHash, current.ProcedureSignatureHash = "", "", ""
	current.ResultSchemaHash, current.PermissionHash, current.ExportSchemaHash = "", "", ""
	current.SchemaProbeToken, current.PublishedBy, current.PublishedAt, current.SchemaValidatedAt = "", 0, nil, nil
	current.CreatedBy = actor
	current.WeatherTimestamps = model.WeatherTimestamps{}
	return current
}

func writePublishedVersion(ctx context.Context, tx *gorm.DB, versionID, definitionID uint, expectedVersion uint64, updates map[string]interface{}) error {
	result := tx.WithContext(ctx).Model(&versionRecord{}).
		Where("id = ? AND definition_id = ? AND status = ? AND version_number = ?", versionID, definitionID, model.ReportVersionStatusDraft, expectedVersion).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("report draft: publish version: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrDraftVersionConflict
	}
	return nil
}

func createDraftVersion(ctx context.Context, tx *gorm.DB, version *versionRecord) error {
	if err := tx.WithContext(ctx).Create(version).Error; err != nil {
		return fmt.Errorf("report draft: create post-publication draft: %w", err)
	}
	return nil
}

func switchPublishedDefinition(ctx context.Context, tx *gorm.DB, ownerUserID, definitionID, publishedVersionID, draftVersionID, updatedBy uint) error {
	result := definitionScope(tx.WithContext(ctx), ownerUserID).Where("id = ? AND current_draft_version_id = ?", definitionID, publishedVersionID).
		Updates(map[string]interface{}{"status": model.ReportDefinitionStatusActive, "current_published_version_id": publishedVersionID, "current_draft_version_id": draftVersionID, "updated_by": updatedBy})
	if result.Error != nil {
		return fmt.Errorf("report draft: switch published version: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrDraftVersionConflict
	}
	return nil
}

func validPublication(value Publication) bool {
	return json.Valid([]byte(value.CompiledSpecJSON)) && len(value.ContractHash) == 64 && len(value.ParameterSchemaHash) == 64 &&
		len(value.ProcedureSignatureHash) == 64 && len(value.ResultSchemaHash) == 64 && len(value.PermissionHash) == 64 &&
		len(value.ExportSchemaHash) == 64 && len(value.SchemaProbeToken) == 36 && !value.SchemaValidatedAt.IsZero()
}

func (repository *Repository) ListDrafts(ctx context.Context, actor uint, query DraftListQuery) (DraftPage, error) {
	if err := repository.validate(ctx, actor); err != nil {
		return DraftPage{}, err
	}
	if query.Limit < 1 || query.Limit > maxDraftPageSize {
		return DraftPage{}, invalidDraft("page limit must be between 1 and 100")
	}

	dbQuery := buildDraftListQuery(repository.db.WithContext(ctx), actor, query)

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
			Definition: record.ReportDefinition, LockVersion: record.LockVersion, IsOwner: record.IsOwner,
		})
	}
	if len(page.Items) > 0 {
		page.NextAfterID = page.Items[len(page.Items)-1].Definition.ID
	}
	return page, nil
}

func buildDraftListQuery(db *gorm.DB, actor uint, query DraftListQuery) *gorm.DB {
	dbQuery := db.Table("report_definitions AS definitions").
		Select(`definitions.*,
			CASE WHEN definitions.owner_user_id = ? THEN draft_versions.version_number ELSE 0 END AS lock_version,
			definitions.owner_user_id = ? AS is_owner`, actor, actor).
		Joins("LEFT JOIN report_versions AS draft_versions ON draft_versions.id = definitions.current_draft_version_id AND draft_versions.definition_id = definitions.id AND draft_versions.status = ?", model.ReportVersionStatusDraft).
		Joins("LEFT JOIN report_versions AS published_versions ON published_versions.id = definitions.current_published_version_id AND published_versions.definition_id = definitions.id AND published_versions.status = ?", model.ReportVersionStatusPublished).
		Where(`(
			definitions.owner_user_id = ?
			AND definitions.status IN ?
			AND draft_versions.id IS NOT NULL
		) OR (
			definitions.status = ?
			AND published_versions.id IS NOT NULL
			AND EXISTS (
				SELECT 1
				FROM report_grants AS grants
				WHERE grants.definition_id = definitions.id
					AND grants.version_id = published_versions.id
					AND JSON_CONTAINS(grants.actions_json, JSON_QUOTE(?))
					AND (
						(grants.subject_type = ? AND grants.subject_id = ?)
						OR (
							grants.subject_type = ?
							AND EXISTS (
								SELECT 1
								FROM user_roles AS memberships
								JOIN roles ON roles.id = memberships.role_id AND roles.status = ?
								WHERE memberships.user_id = ? AND memberships.role_id = grants.subject_id
							)
						)
					)
			)
		)`, actor, []string{model.ReportDefinitionStatusDraft, model.ReportDefinitionStatusActive},
			model.ReportDefinitionStatusActive, ReportActionQuery, "USER", actor, "ROLE", model.RoleStatusActive, actor)
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
		current, err := repository.lockDefinition(ctx, tx, ownerUserID, definitionID)
		if err != nil {
			return err
		}
		version, err := repository.lockVersion(ctx, tx, definitionID, current.CurrentDraftVersionID)
		if err != nil {
			return err
		}
		if version.VersionNumber != expectedLockVersion {
			return ErrDraftVersionConflict
		}
		if draft.Definition.OwnerUserID != 0 && draft.Definition.OwnerUserID != ownerUserID {
			return invalidDraft("definition owner cannot be changed")
		}
		if err := repository.validateReferences(ctx, tx, draft.Definition.DatasourceID, draft.Grants); err != nil {
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
		if err := finalizeReportMutation(ctx, tx, newDraftAudit("REPORT_DRAFT_UPDATE", ownerUserID, definitionID, nextRecord.VersionNumber, draft), repository.writeAudit); err != nil {
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
		definition, err := repository.lockDefinition(ctx, tx, ownerUserID, definitionID)
		if err != nil {
			return err
		}
		version, err := repository.lockVersion(ctx, tx, definitionID, definition.CurrentDraftVersionID)
		if err != nil {
			return err
		}
		if version.VersionNumber != expectedLockVersion {
			return ErrDraftVersionConflict
		}
		if err := repository.validateReferences(ctx, tx, definition.DatasourceID, grants); err != nil {
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
		if err := replaceCollections(ctx, tx, ownerUserID, definitionID, nextRecord.ID, parameters, columns, grants); err != nil {
			return err
		}
		return finalizeReportMutation(ctx, tx, newCollectionAudit(ownerUserID, definitionID, nextRecord.VersionNumber, parameters, columns, grants), repository.writeAudit)
	})
	if err != nil {
		return 0, err
	}
	return nextLockVersion, nil
}

func (repository *Repository) validate(ctx context.Context, ownerUserID uint) error {
	if repository == nil || repository.db == nil || repository.transact == nil || repository.validateReferences == nil ||
		repository.lockDefinition == nil || repository.lockVersion == nil || repository.writeAudit == nil || repository.loadCollections == nil ||
		repository.publishVersion == nil || repository.createVersion == nil || repository.copyCollections == nil || repository.switchDefinition == nil ||
		ctx == nil || ownerUserID == 0 {
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
