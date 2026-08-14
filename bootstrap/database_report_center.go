package bootstrap

import (
	"fmt"

	"gin-biz-web-api/model"

	"gorm.io/gorm"
)

type reportSchemaMigrator interface {
	HasTable(dst interface{}) bool
	HasColumn(dst interface{}, field string) bool
	AddColumn(dst interface{}, field string) error
}

var reportJSONProcedureContractColumns = []string{
	"ExecutionMode",
	"JSONInputArgName",
	"ResultCursorArgName",
	"InputSchemaJSON",
}

const (
	addReportVersionDatasourceSQL      = "ALTER TABLE `report_versions` ADD COLUMN `datasource_id` BIGINT UNSIGNED NULL AFTER `definition_id`"
	backfillReportVersionDatasourceSQL = `UPDATE report_versions AS versions
JOIN report_definitions AS definitions ON definitions.id = versions.definition_id
SET versions.datasource_id = definitions.datasource_id
WHERE versions.datasource_id IS NULL OR versions.datasource_id = 0`
	addReportGrantVersionSQL      = "ALTER TABLE `report_grants` ADD COLUMN `version_id` BIGINT UNSIGNED NULL AFTER `definition_id`"
	backfillReportGrantVersionSQL = `UPDATE report_grants AS grants
JOIN report_definitions AS definitions ON definitions.id = grants.definition_id
SET grants.version_id = CASE
	WHEN definitions.current_published_version_id <> 0 THEN definitions.current_published_version_id
	ELSE definitions.current_draft_version_id
END
WHERE grants.version_id IS NULL OR grants.version_id = 0`
	copyReportGrantDraftVersionSQL = `INSERT INTO report_grants
	(definition_id, version_id, subject_type, subject_id, actions_json, created_by, updated_by, created_at, updated_at)
SELECT grants.definition_id, definitions.current_draft_version_id, grants.subject_type, grants.subject_id,
	grants.actions_json, grants.created_by, grants.updated_by, grants.created_at, grants.updated_at
FROM report_grants AS grants
JOIN report_definitions AS definitions ON definitions.id = grants.definition_id
WHERE definitions.current_draft_version_id <> 0
	AND definitions.current_draft_version_id <> grants.version_id
	AND NOT EXISTS (
		SELECT 1 FROM report_grants AS existing
		WHERE existing.version_id = definitions.current_draft_version_id
			AND existing.subject_type = grants.subject_type
			AND existing.subject_id = grants.subject_id
	)`
)

func prepareReportVersionDatasourceSnapshot(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("report version datasource migration: database is unavailable")
	}
	migrator := db.Migrator()
	if !migrator.HasTable(&model.ReportVersion{}) {
		return nil
	}
	if !migrator.HasColumn(&model.ReportVersion{}, "DatasourceID") {
		if err := db.Exec(addReportVersionDatasourceSQL).Error; err != nil {
			return fmt.Errorf("report version datasource migration: add nullable column: %w", err)
		}
	}
	if err := db.Exec(backfillReportVersionDatasourceSQL).Error; err != nil {
		return fmt.Errorf("report version datasource migration: backfill snapshots: %w", err)
	}
	return nil
}

func prepareReportGrantVersionSnapshot(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("report grant version migration: database is unavailable")
	}
	migrator := db.Migrator()
	if !migrator.HasTable(&model.ReportGrant{}) {
		return nil
	}
	if !migrator.HasColumn(&model.ReportGrant{}, "VersionID") {
		if err := db.Exec(addReportGrantVersionSQL).Error; err != nil {
			return fmt.Errorf("report grant version migration: add nullable column: %w", err)
		}
	}
	if err := db.Exec(backfillReportGrantVersionSQL).Error; err != nil {
		return fmt.Errorf("report grant version migration: backfill published snapshots: %w", err)
	}
	if migrator.HasIndex(&model.ReportGrant{}, "uk_report_grant_subject") {
		if err := migrator.DropIndex(&model.ReportGrant{}, "uk_report_grant_subject"); err != nil {
			return fmt.Errorf("report grant version migration: drop legacy subject index: %w", err)
		}
	}
	if err := db.Exec(copyReportGrantDraftVersionSQL).Error; err != nil {
		return fmt.Errorf("report grant version migration: copy draft snapshots: %w", err)
	}
	return nil
}

func prepareReportJSONProcedureContract(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("report json procedure contract migration: database is unavailable")
	}
	return migrateReportJSONProcedureContract(db.Migrator())
}

func migrateReportJSONProcedureContract(migrator reportSchemaMigrator) error {
	version := &model.ReportVersion{}
	if !migrator.HasTable(version) {
		return fmt.Errorf("report json procedure contract migration: report_versions table is unavailable")
	}
	for _, field := range reportJSONProcedureContractColumns {
		if migrator.HasColumn(version, field) {
			continue
		}
		if err := migrator.AddColumn(version, field); err != nil {
			return fmt.Errorf("report json procedure contract migration: add %s: %w", field, err)
		}
	}
	return nil
}
