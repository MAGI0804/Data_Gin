package bootstrap

import (
	"fmt"

	"gin-biz-web-api/model"

	"gorm.io/gorm"
)

const (
	addReportVersionDatasourceSQL      = "ALTER TABLE `report_versions` ADD COLUMN `datasource_id` BIGINT UNSIGNED NULL AFTER `definition_id`"
	backfillReportVersionDatasourceSQL = `UPDATE report_versions AS versions
JOIN report_definitions AS definitions ON definitions.id = versions.definition_id
SET versions.datasource_id = definitions.datasource_id
WHERE versions.datasource_id IS NULL OR versions.datasource_id = 0`
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
