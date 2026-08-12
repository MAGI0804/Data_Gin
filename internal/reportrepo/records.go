package reportrepo

import (
	"context"
	"fmt"
	"time"

	"gin-biz-web-api/model"

	"gorm.io/gorm"
)

type definitionRecord struct {
	model.ReportDefinition `gorm:"embedded"`
}

func (definitionRecord) TableName() string { return "report_definitions" }

type versionRecord struct {
	model.ReportVersion `gorm:"embedded"`
}

func (versionRecord) TableName() string { return "report_versions" }

type parameterRecord struct {
	model.ReportParameter `gorm:"embedded"`
}

func (parameterRecord) TableName() string { return "report_parameters" }

type columnRecord struct {
	model.ReportColumn `gorm:"embedded"`
}

func (columnRecord) TableName() string { return "report_columns" }

type grantRecord struct {
	model.ReportGrant `gorm:"embedded"`
}

func (grantRecord) TableName() string { return "report_grants" }

func newDefinitionRecord(definition model.ReportDefinition) definitionRecord {
	definition.ID = 0
	definition.CurrentDraftVersionID = 0
	definition.CurrentPublishedVersionID = 0
	definition.WeatherTimestamps = model.WeatherTimestamps{}
	return definitionRecord{ReportDefinition: definition}
}

func newVersionRecord(version model.ReportVersion) versionRecord {
	version.ID = 0
	version.WeatherTimestamps = model.WeatherTimestamps{}
	return versionRecord{ReportVersion: version}
}

func loadCollections(
	ctx context.Context,
	db *gorm.DB,
	_ uint, definitionID, versionID uint,
	draft *Draft,
) error {
	var parameters []parameterRecord
	if err := db.WithContext(ctx).Model(&parameterRecord{}).
		Where("version_id = ?", versionID).
		Order("position ASC, id ASC").Find(&parameters).Error; err != nil {
		return fmt.Errorf("report draft: list parameters: %w", err)
	}
	draft.Parameters = make([]model.ReportParameter, 0, len(parameters))
	for _, record := range parameters {
		draft.Parameters = append(draft.Parameters, record.ReportParameter)
	}

	var columns []columnRecord
	if err := db.WithContext(ctx).Model(&columnRecord{}).
		Where("version_id = ?", versionID).
		Order("display_order ASC, id ASC").Find(&columns).Error; err != nil {
		return fmt.Errorf("report draft: list columns: %w", err)
	}
	draft.Columns = make([]model.ReportColumn, 0, len(columns))
	for _, record := range columns {
		draft.Columns = append(draft.Columns, record.ReportColumn)
	}

	var grants []grantRecord
	if err := db.WithContext(ctx).Model(&grantRecord{}).
		Where("definition_id = ?", definitionID).
		Order("subject_type ASC, subject_id ASC, id ASC").Find(&grants).Error; err != nil {
		return fmt.Errorf("report draft: list grants: %w", err)
	}
	draft.Grants = make([]model.ReportGrant, 0, len(grants))
	for _, record := range grants {
		draft.Grants = append(draft.Grants, record.ReportGrant)
	}
	return nil
}

func replaceCollections(
	ctx context.Context,
	tx *gorm.DB,
	ownerUserID, definitionID, versionID uint,
	parameters []model.ReportParameter,
	columns []model.ReportColumn,
	grants []model.ReportGrant,
) error {
	if err := validateCollections(parameters, columns, grants); err != nil {
		return err
	}

	if ownerUserID == 0 {
		return invalidDraft("owner scope is required")
	}
	if err := tx.WithContext(ctx).
		Where("version_id = ?", versionID).
		Delete(&parameterRecord{}).Error; err != nil {
		return fmt.Errorf("report draft: replace parameters: delete old set: %w", err)
	}
	parameterRows := make([]parameterRecord, 0, len(parameters))
	for _, parameter := range parameters {
		parameter.ID = 0
		parameter.VersionID = versionID
		parameter.WeatherTimestamps = model.WeatherTimestamps{}
		parameterRows = append(parameterRows, parameterRecord{ReportParameter: parameter})
	}
	if len(parameterRows) > 0 {
		if err := tx.WithContext(ctx).Create(&parameterRows).Error; err != nil {
			return fmt.Errorf("report draft: replace parameters: create new set: %w", err)
		}
	}

	if err := tx.WithContext(ctx).
		Where("version_id = ?", versionID).
		Delete(&columnRecord{}).Error; err != nil {
		return fmt.Errorf("report draft: replace columns: delete old set: %w", err)
	}
	columnRows := make([]columnRecord, 0, len(columns))
	for _, column := range columns {
		column.ID = 0
		column.VersionID = versionID
		column.WeatherTimestamps = model.WeatherTimestamps{}
		columnRows = append(columnRows, columnRecord{ReportColumn: column})
	}
	if len(columnRows) > 0 {
		if err := tx.WithContext(ctx).Create(&columnRows).Error; err != nil {
			return fmt.Errorf("report draft: replace columns: create new set: %w", err)
		}
	}

	if err := tx.WithContext(ctx).
		Where("definition_id = ?", definitionID).
		Delete(&grantRecord{}).Error; err != nil {
		return fmt.Errorf("report draft: replace grants: delete old set: %w", err)
	}
	grantRows := make([]grantRecord, 0, len(grants))
	now := time.Now().UTC()
	for _, grant := range grants {
		grant.ID = 0
		grant.DefinitionID = definitionID
		grant.CreatedAt = now
		grant.UpdatedAt = now
		grantRows = append(grantRows, grantRecord{ReportGrant: grant})
	}
	if len(grantRows) > 0 {
		if err := tx.WithContext(ctx).Create(&grantRows).Error; err != nil {
			return fmt.Errorf("report draft: replace grants: create new set: %w", err)
		}
	}
	return nil
}
