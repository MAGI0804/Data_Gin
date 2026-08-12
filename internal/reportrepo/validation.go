package reportrepo

import (
	"fmt"
	"strings"

	"gin-biz-web-api/model"
)

func validateNewDraft(draft *Draft) error {
	if draft == nil {
		return invalidDraft("draft is required")
	}
	if draft.Definition.ID != 0 || draft.Version.ID != 0 || draft.LockVersion != 0 {
		return invalidDraft("new draft cannot contain persisted ids or lock version")
	}
	return validateDraftContent(draft)
}

func validateExistingDraft(definitionID uint, expectedLockVersion uint64, draft *Draft) error {
	if definitionID == 0 || expectedLockVersion == 0 || draft == nil {
		return invalidDraft("definition id, lock version and draft are required")
	}
	if draft.Definition.ID != 0 && draft.Definition.ID != definitionID {
		return invalidDraft("definition id does not match update target")
	}
	if draft.LockVersion != 0 && draft.LockVersion != expectedLockVersion {
		return invalidDraft("draft lock version does not match expected version")
	}
	return validateDraftContent(draft)
}

func validateDraftContent(draft *Draft) error {
	if strings.TrimSpace(draft.Definition.Code) == "" || strings.TrimSpace(draft.Definition.Name) == "" {
		return invalidDraft("definition code and name are required")
	}
	if draft.Definition.DatasourceID == 0 || draft.Definition.OwnerUserID == 0 {
		return invalidDraft("datasource and owner are required")
	}
	if draft.Version.DatasourceID == 0 || draft.Version.DatasourceID != draft.Definition.DatasourceID {
		return invalidDraft("draft version datasource must match definition")
	}
	if draft.Definition.Status != "" && draft.Definition.Status != model.ReportDefinitionStatusDraft &&
		draft.Definition.Status != model.ReportDefinitionStatusActive {
		return invalidDraft("disabled definitions cannot save drafts")
	}
	if draft.Version.Status != "" && draft.Version.Status != model.ReportVersionStatusDraft {
		return invalidDraft("only draft versions can be saved")
	}
	return validateCollections(draft.Parameters, draft.Columns, draft.Grants)
}

func normalizeNewDraft(draft *Draft) {
	draft.Definition.Status = model.ReportDefinitionStatusDraft
	draft.Version.Status = model.ReportVersionStatusDraft
	draft.Version.VersionNumber = 1
	if draft.Version.CreatedBy == 0 {
		draft.Version.CreatedBy = draft.Definition.CreatedBy
	}
}

func validateCollections(
	parameters []model.ReportParameter,
	columns []model.ReportColumn,
	grants []model.ReportGrant,
) error {
	parameterCodes := make(map[string]struct{}, len(parameters))
	parameterPositions := make(map[int]struct{}, len(parameters))
	for _, parameter := range parameters {
		code := strings.TrimSpace(parameter.ParameterCode)
		if code == "" || parameter.Position <= 0 {
			return invalidDraft("parameter code and positive position are required")
		}
		if _, exists := parameterCodes[code]; exists {
			return invalidDraft(fmt.Sprintf("parameter code %q is duplicated", code))
		}
		if _, exists := parameterPositions[parameter.Position]; exists {
			return invalidDraft(fmt.Sprintf("parameter position %d is duplicated", parameter.Position))
		}
		parameterCodes[code] = struct{}{}
		parameterPositions[parameter.Position] = struct{}{}
	}

	columnCodes := make(map[string]struct{}, len(columns))
	fieldIDs := make(map[string]struct{}, len(columns))
	for _, column := range columns {
		code := strings.TrimSpace(column.LogicalCode)
		fieldID := strings.TrimSpace(column.FieldID)
		if code == "" || fieldID == "" {
			return invalidDraft("column logical code and field id are required")
		}
		if _, exists := columnCodes[code]; exists {
			return invalidDraft(fmt.Sprintf("column logical code %q is duplicated", code))
		}
		if _, exists := fieldIDs[fieldID]; exists {
			return invalidDraft(fmt.Sprintf("column field id %q is duplicated", fieldID))
		}
		columnCodes[code] = struct{}{}
		fieldIDs[fieldID] = struct{}{}
	}

	grantSubjects := make(map[string]struct{}, len(grants))
	for _, grant := range grants {
		subjectType := strings.TrimSpace(grant.SubjectType)
		if subjectType == "" || grant.SubjectID == 0 {
			return invalidDraft("grant subject type and id are required")
		}
		key := fmt.Sprintf("%s:%d", subjectType, grant.SubjectID)
		if _, exists := grantSubjects[key]; exists {
			return invalidDraft(fmt.Sprintf("grant subject %q is duplicated", key))
		}
		grantSubjects[key] = struct{}{}
	}
	return nil
}
